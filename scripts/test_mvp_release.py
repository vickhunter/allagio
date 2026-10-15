import base64
from email.message import Message
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tempfile
import sys
import unittest
from unittest.mock import patch
from urllib.parse import urlencode
import zipfile

import mvp_release as release


def configuration():
    return {
        "DATABASE_URL": "postgresql://qa:qa@postgres.railway.internal/railway",
        "ADMIN_API_KEY": "a" * 32,
        "CONNECTION_ENCRYPTION_KEY": base64.b64encode(b"b" * 32).decode(),
        "OIDC_ISSUER": "https://accounts.google.com",
        "OIDC_CLIENT_ID": "qa.apps.googleusercontent.com",
        "OIDC_CLIENT_SECRET": "private-client-secret",
        "PUBLIC_BASE_URL": release.ORIGIN,
        "HOST": "0.0.0.0", "PORT": "8080", "AUTO_MIGRATE": "true",
        "ALLOWED_ORIGINS": release.ORIGIN,
        "RAILWAY_DOCKERFILE_PATH": "Dockerfile.mvp",
        "API_KEY_SECRET": "c" * 32,
        "GIN_MODE": "release",
        "TRUSTED_PROXIES": "173.245.48.0/20",
    }


def response(status=200, body=b"ok", **headers):
    message = Message()
    for key, value in headers.items():
        message[key.replace("_", "-")] = value
    return status, message, body


def public_responses():
    origin = release.ORIGIN
    resource = {"resource": origin + "/mcp", "authorization_servers": [origin], "scopes_supported": ["maker", "publisher"]}
    authorization = {"issuer": origin, "authorization_endpoint": origin + "/oauth/authorize", "token_endpoint": origin + "/oauth/token", "registration_endpoint": origin + "/oauth/register", "code_challenge_methods_supported": ["S256"]}
    result = {
        "/health": response(body=json.dumps({"status": "ok", "site_installation_registration": True, "native_distribution": True}).encode()),
        "/": response(body=(release.INSTALLER + " Download WordPress plugin").encode(), Content_Security_Policy="script-src 'self'", X_Content_Type_Options="nosniff"),
        "/.well-known/oauth-protected-resource/mcp": response(body=json.dumps(resource).encode()),
        "/.well-known/oauth-authorization-server": response(body=json.dumps(authorization).encode()),
        "/owner": response(302, Location="/auth/login?return_to=%2Fowner"),
        "/mcp": response(401, WWW_Authenticate='Bearer resource_metadata="' + origin + '/.well-known/oauth-protected-resource/mcp"'),
        "/auth/login?return_to=/owner": response(302, Location="https://accounts.google.com/o/oauth2/v2/auth?" + urlencode({"redirect_uri": origin + "/auth/callback", "code_challenge_method": "S256"}), Set_Cookie="__Host-dtr_login=fixture; Path=/; Secure; HttpOnly; SameSite=Lax"),
        "/downloads/affilio.zip": response(),
        "/downloads/" + release.INSTALLER: response(),
    }
    for name in release.ASSETS:
        result["/assets/" + name] = response()
    return result


def _dockerfile_lines(text):
    logical = []
    pending = ""
    for raw in text.splitlines():
        piece = raw.split("#", 1)[0].rstrip()
        if not piece.strip():
            continue
        continued = piece.rstrip().endswith("\\")
        if continued:
            piece = piece.rstrip()[:-1].rstrip()
        pending = (pending + " " + piece.strip()).strip()
        if continued:
            continue
        logical.append(pending)
        pending = ""
    if pending:
        logical.append(pending)
    return logical


def _docker_match(pattern, rel):
    directory = pattern.endswith("/")
    pattern = pattern[1:] if pattern.startswith("/") else pattern
    pattern = pattern.rstrip("/")
    escaped = re.escape(pattern).replace(r"\*\*", "\0").replace(r"\*", "[^/]*").replace(r"\?", "[^/]").replace("\0", ".*")
    if "/" not in pattern and not pattern.startswith("**"):
        escaped = r"(?:^|.*/)" + escaped
    else:
        escaped = "^" + escaped
    escaped += r"(?:/.*)?$" if not directory else r"(?:/.*)$"
    return re.match(escaped, rel) is not None


def _docker_ignored(rel):
    ignored = False
    dockerignore = release.ROOT / ".dockerignore"
    if not dockerignore.is_file():
        return False
    for raw in dockerignore.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        negate = line.startswith("!")
        pattern = line[1:] if negate else line
        if _docker_match(pattern, rel):
            ignored = not negate
    return ignored


def _copy_build_source(source, target):
    if source.is_file():
        if _docker_ignored(source.relative_to(release.ROOT).as_posix()):
            return
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)
        return
    for dirpath, dirnames, filenames in os.walk(source):
        current = Path(dirpath)
        dirnames[:] = [name for name in dirnames if not _docker_ignored((current / name).relative_to(release.ROOT).as_posix())]
        for name in filenames:
            path = current / name
            rel = path.relative_to(release.ROOT).as_posix()
            if _docker_ignored(rel):
                continue
            dest = target / path.relative_to(source)
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(path, dest)


def dockerfile_mvp_plugin_zip():
    text = (release.ROOT / "Dockerfile.mvp").read_text()
    stages = re.split(r"(?m)^FROM ", text)
    if len(stages) < 2:
        raise AssertionError("Dockerfile.mvp has no build stage")
    lines = _dockerfile_lines(stages[1])
    copies = [line for line in lines if line.startswith("COPY ") and "/wordpress/" in line]
    zip_lines = [line for line in lines if "zip -qr" in line and "affilio.zip" in line]
    if len(copies) == 0 or len(zip_lines) != 1 or " -x " not in zip_lines[0]:
        raise AssertionError("Dockerfile.mvp releases stage does not zip the WordPress plugin")
    excludes = re.findall(r"'([^']*)'", zip_lines[0].split(" -x ", 1)[1])
    with tempfile.TemporaryDirectory() as directory:
        context = Path(directory)
        for line in copies:
            tokens = line.split()
            if tokens[0] != "COPY" or len(tokens) != 3:
                raise AssertionError("Dockerfile.mvp plugin COPY must be one source and one destination")
            source = release.ROOT / tokens[1]
            target = context.joinpath(*PurePosixPath(tokens[2]).parts)
            _copy_build_source(source, target)
        archive = context / "affilio.zip"
        result = subprocess.run(
            ["zip", "-qr", str(archive), "affilio-wp", "-x", *excludes],
            cwd=context / "wordpress",
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            raise AssertionError(result.stderr or "zip failed while building the Dockerfile.mvp plugin archive")
        return archive.read_bytes()


def plugin_archive(plugin=None, extra=None, drop=None, bodies=None):
    version = release.PLUGIN_VERSION
    if plugin is None:
        plugin = (
            "<?php\n/* Plugin Name: Affilio\n * Version: %s\n */\n"
            "define('AFFILIO_VERSION', '%s');\n"
            "if (!function_exists('affilio_init')) {\nfunction affilio_init() {}\n}\n"
        ) % (version, version)
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED) as zipped:
        for name in release.WP_FILES:
            if drop and name == drop:
                continue
            if name.endswith("/affilio.php"):
                body = plugin
            elif name.endswith("/readme.txt"):
                body = "=== Affilio ===\nStable tag: %s\n" % version
            else:
                body = (bodies or {}).get(name, "runtime")
            zipped.writestr(name, body)
        if extra:
            zipped.writestr(extra[0], extra[1])
    return output.getvalue()


class ReleaseTests(unittest.TestCase):
    def test_runtime_image_defaults_to_release_mode(self):
        dockerfile = (release.ROOT / "Dockerfile.mvp").read_text()
        runtime = dockerfile.split("FROM ubuntu:24.04", 1)[1]
        self.assertIn("ENV GIN_MODE=release", runtime)

    def test_dockerfile_mvp_zip_includes_public_placement_css(self):
        data = dockerfile_mvp_plugin_zip()
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            names = [name for name in archive.namelist() if not name.endswith("/")]
            self.assertIn("affilio-wp/public/css/placement.css", names)
            self.assertEqual(
                archive.read("affilio-wp/public/css/placement.css"),
                (release.ROOT / "affilio-wp/public/css/placement.css").read_bytes(),
            )
        release.validate_plugin(data)
        tracked = subprocess.run(
            ["git", "ls-files", "-z", "--", "affilio-wp"],
            cwd=release.ROOT,
            capture_output=True,
            check=True,
        )
        expected = sorted(
            name.decode()
            for name in tracked.stdout.split(b"\0")
            if name and not release.omit_from_plugin_zip(name.decode())
        )
        self.assertEqual(sorted(names), expected)

    def test_missing_oidc_stops_deployment_without_exposing_values(self):
        values = configuration()
        del values["OIDC_CLIENT_ID"]
        output = io.StringIO()
        with patch.object(sys, "argv", ["mvp_release.py", "deploy"]), patch.object(release, "run_json", return_value=values), patch.object(release, "run") as run, patch("sys.stderr", output):
            self.assertEqual(release.main(), 1)
        run.assert_not_called()
        self.assertIn("OIDC_CLIENT_ID", output.getvalue())
        self.assertNotIn(values["OIDC_CLIENT_SECRET"], output.getvalue())

    def test_configuration_rejects_wrong_service_origin_and_invalid_cipher(self):
        release.validate_config(configuration())
        for name, value in {"PUBLIC_BASE_URL": "https://affilio-production.up.railway.app", "ALLOWED_ORIGINS": "*", "CONNECTION_ENCRYPTION_KEY": "invalid", "AUTO_MIGRATE": "false"}.items():
            with self.subTest(name=name), self.assertRaisesRegex(release.ReleaseError, name):
                release.validate_config(configuration() | {name: value})

    def test_cli_failure_never_reports_secret_output(self):
        result = subprocess.CompletedProcess(["railway"], 1, "private-output", "private-secret")
        with patch.object(subprocess, "run", return_value=result), self.assertRaises(release.ReleaseError) as caught:
            release.run(["railway", "variables"])
        self.assertNotIn("private-", str(caught.exception))

    def test_only_latest_successful_required_checks_pass(self):
        checks = [{"id": i, "name": name, "status": "completed", "conclusion": "success"} for i, name in enumerate(sorted(release.CHECKS))]
        release.validate_checks(checks)
        with self.assertRaises(release.ReleaseError):
            release.validate_checks(checks[:-1])
        with self.assertRaises(release.ReleaseError):
            release.validate_checks(checks + [{"id": 99, "name": checks[0]["name"], "status": "in_progress"}])

    def test_dirty_unpushed_or_unchecked_revision_never_uploads(self):
        sha = "a" * 40
        cases = (
            {"status": " M changed.go", "remote": sha},
            {"status": "", "remote": "b" * 40},
            {"status": "", "remote": sha},
        )
        for case in cases:
            def git(args):
                outputs = {
                    ("git", "status", "--porcelain"): case["status"],
                    ("git", "branch", "--show-current"): "release",
                    ("git", "rev-parse", "HEAD"): sha,
                    ("git", "ls-remote", "origin", "refs/heads/release"): case["remote"] + " refs/heads/release",
                }
                self.assertIn(tuple(args), outputs)
                return outputs[tuple(args)]

            with self.subTest(case=case), patch.object(sys, "argv", ["mvp_release.py", "deploy"]), patch.object(release, "run_json", side_effect=[configuration(), {"check_runs": []}]), patch.object(release, "run", side_effect=git), patch("sys.stderr", io.StringIO()):
                self.assertEqual(release.main(), 1)

    def test_archive_paths_reject_private_files_and_traversal(self):
        for name in ("../secret", "/etc/passwd", "package/.env", "package/.env.local", "package/node_modules/a", "affilio/..\\secret"):
            with self.subTest(name=name), self.assertRaises(release.ReleaseError):
                release.archive_names([name])
        with self.assertRaises(release.ReleaseError):
            release.archive_names(["package/package.json", "package/package.json"])

    def test_plugin_origin_and_hosted_asset_exclusion(self):
        guarded = "<?php\n/* Plugin Name: Affilio\n * Version: 0.1.26\n */\ndefine('AFFILIO_VERSION', '0.1.26');\nif (!function_exists('affilio_init')) {\nfunction affilio_init() {}\n}\n"

        def archive(plugin=guarded, extra=None, drop=None):
            return plugin_archive(plugin, extra, drop)

        release.validate_plugin(archive(), release.ORIGIN)
        with self.assertRaisesRegex(release.ReleaseError, "railway.app"):
            release.validate_plugin(archive(plugin=guarded + "/* https://distributoor-production.up.railway.app/v1 */\n"))
        with self.assertRaisesRegex(release.ReleaseError, "affilio-wp/"):
            release.validate_plugin(archive(extra=("affilio/affilio.php", guarded)))
        with self.assertRaisesRegex(release.ReleaseError, "analytics.js"):
            release.validate_plugin(archive(drop="affilio-wp/admin/build/analytics.js"))
        with self.assertRaisesRegex(release.ReleaseError, "unguarded function affilio_init"):
            release.validate_plugin(archive(plugin="<?php\n/* Plugin Name: Affilio */\nfunction affilio_init() {}\n"))
        with self.assertRaisesRegex(release.ReleaseError, "0.1.26"):
            release.validate_plugin(archive(plugin=guarded.replace("0.1.26", "0.1.9")))
        with self.assertRaisesRegex(release.ReleaseError, "hosted UI"):
            release.validate_plugin(archive(extra=("affilio-wp/admin/build/landing-scene.bundle.js", "unexpected")))
        for extra in (
            "affilio-wp/e2e/offer-rail.spec.js",
            "affilio-wp/phpunit.xml.dist",
            "affilio-wp/tests/d033-one-box.php",
            "affilio-wp/e2e/test-results/x",
            "affilio-wp/tests/phpunit/.phpunit.result.cache",
        ):
            with self.subTest(extra=extra), self.assertRaisesRegex(release.ReleaseError, "tests or test results"):
                release.validate_plugin(archive(extra=(extra, "unexpected")))
        with self.assertRaisesRegex(release.ReleaseError, "class-affilio-missing.php"):
            release.validate_plugin(archive(plugin=guarded + "require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-missing.php';\n"))
        with self.assertRaisesRegex(release.ReleaseError, "marketplace.js"):
            release.validate_plugin(archive(plugin=guarded + "wp_enqueue_script($handle, AFFILIO_PLUGIN_URL . 'admin/build/' . $script);\n$script = 'marketplace.js';\n", drop="affilio-wp/admin/build/marketplace.js"))

    def test_missing_landing_scene_bundle_fails_the_zip(self):
        script = "affilio-wp/admin/build/dashboard.js"
        php = "affilio-wp/includes/class-affilio-scanner.php"
        for body in (
            "import './landing-scene.bundle.js';\n",
            "import scene from './landing-scene.bundle.js';\n",
            "const { mountScene } = await import('./landing-scene.bundle.js');\n",
            'await import("./landing-scene.bundle.js");\n',
        ):
            with self.subTest(body=body), self.assertRaisesRegex(release.ReleaseError, "landing-scene.bundle.js"):
                release.validate_plugin(plugin_archive(bodies={script: body}))
        release.validate_plugin(plugin_archive(bodies={script: "// import('./landing-scene.bundle.js');\nimport * as THREE from 'three';\nimport './analytics.js';\n"}))
        header = (
            "<?php\n/* Plugin Name: Affilio\n * Version: %s\n */\n"
            "define('AFFILIO_VERSION', '%s');\n"
            "if (!function_exists('affilio_init')) {\nfunction affilio_init() {}\n}\n"
        ) % (release.PLUGIN_VERSION, release.PLUGIN_VERSION)
        with self.assertRaisesRegex(release.ReleaseError, "missing-script.js"):
            release.validate_plugin(plugin_archive(plugin=header + "wp_enqueue_script('affilio-missing', 'admin/build/missing-script.js');\n"))
        with self.assertRaisesRegex(release.ReleaseError, "missing-style.css"):
            release.validate_plugin(plugin_archive(plugin=header + "wp_enqueue_style('affilio-missing', 'admin/css/missing-style.css');\n"))
        with self.assertRaisesRegex(release.ReleaseError, "missing-include.php"):
            release.validate_plugin(plugin_archive(bodies={php: "<?php\nrequire_once __DIR__ . '/missing-include.php';\n"}))
        with self.assertRaisesRegex(release.ReleaseError, "missing-required.php"):
            release.validate_plugin(plugin_archive(bodies={php: "<?php\ninclude 'includes/missing-required.php';\n"}))
        with self.assertRaisesRegex(release.ReleaseError, "outside the plugin"):
            release.validate_plugin(plugin_archive(bodies={php: "<?php\nrequire __DIR__ . '/../../outside.php';\n"}))

    def test_package_wp_zip_passes_the_release_gate(self):
        probe = subprocess.run(
            ["git", "rev-parse", "--is-inside-work-tree"],
            cwd=release.ROOT,
            capture_output=True,
            text=True,
        )
        if probe.returncode != 0 or probe.stdout.strip() != "true":
            self.skipTest("not a git checkout, so package-wp-zip cannot list tracked files")
        hosted = {
            "landing.js", "landing-scene.js", "landing.css",
            "landing-scene.bundle.js", "landing-scene.bundle.js.LEGAL.txt",
            "workspace.js", "workspace.css", "workspace.html",
            "home.html", "agent-poc.html", "setup-controls.js",
            "THIRD_PARTY_NOTICES.md", "package.json", "package-lock.json",
        }
        admin = release.ROOT / "affilio-wp/includes/class-affilio-admin.php"
        needed, outside = release.referenced_plugin_files(str(admin.relative_to(release.ROOT)), admin.read_text())
        self.assertEqual(outside, [])
        self.assertNotIn("setup-controls.js", admin.read_text())
        for relative in needed:
            self.assertFalse(release.hosted_build_file(Path(relative).name), relative)
        header = (release.ROOT / "affilio-wp/affilio.php").read_text()
        found = re.search(r"(?m)^[ \t]*\*[ \t]*Version:[ \t]*([0-9]+\.[0-9]+\.[0-9]+)[ \t]*$", header)
        self.assertIsNotNone(found)
        version = found.group(1)
        script = release.ROOT / "scripts" / "package-wp-zip"
        archives = []
        for _ in range(2):
            with tempfile.TemporaryDirectory() as directory:
                stale = Path(directory) / "affilio-0.1.19.zip"
                result = subprocess.run([str(script), str(stale)], cwd=release.ROOT, capture_output=True, text=True, timeout=60)
                self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
                self.assertIn("passed the release gate with 25 files: affilio-%s.zip" % version, result.stdout)
                written = Path(directory) / ("affilio-%s.zip" % version)
                self.assertTrue(written.is_file(), result.stdout)
                self.assertFalse(stale.exists())
                archives.append(written.read_bytes())
        self.assertEqual(hashlib.sha256(archives[0]).hexdigest(), hashlib.sha256(archives[1]).hexdigest())
        release.validate_plugin(archives[0])
        with zipfile.ZipFile(io.BytesIO(archives[0])) as archive:
            names = [name for name in archive.namelist() if not name.endswith("/")]
            self.assertEqual(names, sorted(names))
            for info in archive.infolist():
                self.assertEqual(info.date_time, (1980, 1, 1, 0, 0, 0))
        self.assertEqual(len(names), 25)
        self.assertTrue(hosted.isdisjoint(Path(name).name for name in names))
        for relative in needed:
            self.assertIn(release.PLUGIN_ROOT + relative, names)

    def test_plugin_zip_rejects_stored_entries(self):
        output = io.BytesIO()
        with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED) as archive:
            for name in release.WP_FILES:
                version = release.PLUGIN_VERSION
                body = f"<?php\n/* Plugin Name: Affilio\n * Version: {version}\n */\ndefine('AFFILIO_VERSION', '{version}');\nif (!function_exists('affilio_init')) {{\nfunction affilio_init() {{}}\n}}\n" if name.endswith("/affilio.php") else (f"=== Affilio ===\nStable tag: {version}\n" if name.endswith("/readme.txt") else "runtime")
                archive.writestr(name, body)
        with self.assertRaisesRegex(release.ReleaseError, "uncompressed"):
            release.validate_plugin(output.getvalue())

    def test_plugin_zip_requires_analytics_script(self):
        legacy = {
            "affilio/affilio.php", "affilio/includes/class-affilio-api.php",
            "affilio/includes/class-affilio-editor-proxy.php",
            "affilio/includes/class-affilio-onboarding.php",
            "affilio/includes/class-affilio-distribution.php",
            "affilio/admin/build/onboarding.js", "affilio/admin/build/distribution.js",
            "affilio/admin/build/distribution.css",
        }
        output = io.BytesIO()
        with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED) as archive:
            for name in legacy:
                body = "runtime"
                if name.endswith("/affilio.php"):
                    body = "<?php\n/* Plugin Name: Affilio */\ndefine('AFFILIO_API_URL', '%s/v1');\n" % release.ORIGIN
                archive.writestr(name, body)
        with self.assertRaisesRegex(release.ReleaseError, "analytics.js"):
            release.validate_plugin(output.getvalue(), release.ORIGIN)

    def test_generic_host_does_not_require_railway(self):
        values = configuration()
        del values["RAILWAY_DOCKERFILE_PATH"]
        values.update({
            "PUBLIC_BASE_URL": "https://affilio-mvp.onrender.com",
            "ALLOWED_ORIGINS": "https://affilio-mvp.onrender.com",
            "PORT": "10000",
            "API_KEY_SECRET": "c" * 32,
            "GIN_MODE": "release",
            "TRUSTED_PROXIES": "173.245.48.0/20",
        })
        env = tempfile.NamedTemporaryFile("w", delete=False)
        env.write("".join(f"{key}={value}\n" for key, value in values.items()))
        env.close()
        self.addCleanup(lambda: Path(env.name).unlink(missing_ok=True))
        sha = "a" * 40
        calls = []

        def git(args):
            calls.append(args[0])
            outputs = {
                ("git", "status", "--porcelain"): "",
                ("git", "branch", "--show-current"): "release",
                ("git", "rev-parse", "HEAD"): sha,
                ("git", "ls-remote", "origin", "refs/heads/release"): sha + " refs/heads/release",
            }
            return outputs[tuple(args)]

        checks = [{"id": i, "name": name, "status": "completed", "conclusion": "success"} for i, name in enumerate(sorted(release.CHECKS))]
        with patch.object(sys, "argv", ["mvp_release.py", "check", "--host", "generic", "--env-file", env.name]), patch.object(release, "run", side_effect=git), patch.object(release, "run_json", return_value={"check_runs": checks}), patch("sys.stdout", io.StringIO()):
            self.assertEqual(release.main(), 0)
        self.assertNotIn("railway", calls)

        for name, bad in {
            "API_KEY_SECRET": "dev-secret",
            "GIN_MODE": "debug",
            "TRUSTED_PROXIES": "",
        }.items():
            with self.subTest(name=name), self.assertRaisesRegex(release.ReleaseError, name):
                release.validate_config(values | {name: bad}, origin=values["PUBLIC_BASE_URL"])
        with self.assertRaisesRegex(release.ReleaseError, "API_KEY_SECRET"):
            release.validate_config(values | {"API_KEY_SECRET": "short"}, origin=values["PUBLIC_BASE_URL"])

    def test_verify_accepts_any_https_base_url(self):
        origin = "https://affilio-mvp.onrender.com"
        responses = public_responses()
        responses["/.well-known/oauth-protected-resource/mcp"] = response(body=json.dumps({"resource": origin + "/mcp", "authorization_servers": [origin], "scopes_supported": ["maker", "publisher"]}).encode())
        responses["/.well-known/oauth-authorization-server"] = response(body=json.dumps({"issuer": origin, "authorization_endpoint": origin + "/oauth/authorize", "token_endpoint": origin + "/oauth/token", "registration_endpoint": origin + "/oauth/register", "code_challenge_methods_supported": ["S256"]}).encode())
        responses["/mcp"] = response(401, WWW_Authenticate='Bearer resource_metadata="' + origin + '/.well-known/oauth-protected-resource/mcp"')
        responses["/auth/login?return_to=/owner"] = response(302, Location="https://accounts.google.com/o/oauth2/v2/auth?" + urlencode({"redirect_uri": origin + "/auth/callback", "code_challenge_method": "S256"}), Set_Cookie="__Host-dtr_login=fixture; Path=/; Secure; HttpOnly; SameSite=Lax")
        with patch.object(sys, "argv", ["mvp_release.py", "verify", "--base-url", origin]), patch.object(release, "fetch", side_effect=lambda path, *args: responses[path]), patch.object(release, "validate_plugin") as plugin, patch.object(release, "validate_installer"), patch("sys.stdout", io.StringIO()):
            self.assertEqual(release.main(), 0)
        plugin.assert_called_once_with(b"ok", origin)

    def test_public_check_rejects_broken_delivery_and_unprotected_auth(self):
        mutations = {
            "/health": response(503),
            "/downloads/affilio.zip": response(404),
            "/assets/landing-scene.bundle.js": response(404),
            "/.well-known/oauth-protected-resource/mcp": response(body=b'{"resource":"https://wrong.example/mcp"}'),
            "/owner": response(200),
            "/mcp": response(200),
            "/auth/login?return_to=/owner": response(302, Location="https://wrong.example/login"),
        }
        with patch.object(release, "validate_plugin") as plugin, patch.object(release, "validate_installer"), patch("sys.stdout", io.StringIO()):
            good = public_responses()
            with patch.object(release, "fetch", side_effect=lambda path, *args: good[path]):
                release.verify()
            plugin.assert_called_once_with(b"ok", release.ORIGIN)
            for path, value in mutations.items():
                responses = public_responses() | {path: value}
                with self.subTest(path=path), patch.object(release, "fetch", side_effect=lambda path, *args: responses[path]), self.assertRaises(release.ReleaseError):
                    release.verify()


if __name__ == "__main__":
    unittest.main()
