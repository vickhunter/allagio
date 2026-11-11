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
import tarfile
import tempfile
import sys
import unittest
from unittest.mock import patch
from urllib.parse import urlencode
import zipfile

import mvp_release as release


FIXTURES = Path(__file__).resolve().parent / "testdata"


def with_labels(script, body):
    name = PurePosixPath(script).name
    return release.required_label_script(release.required_label_manifest()[name]) + body


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
                override = (bodies or {}).get(name)
                if override is None:
                    leaf = PurePosixPath(name).name
                    labels = release.required_label_manifest().get(leaf) if name.startswith(release.PLUGIN_ROOT + "admin/build/") else None
                    body = release.required_label_script(labels) if labels else "runtime"
                else:
                    body = override
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
        self.assert_zip_php_omits_seams(data)
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

    def test_keyword_database_url_identifies_the_database(self):
        release.validate_config(configuration() | {
            "DATABASE_URL": "host=postgres.railway.internal port=5432 dbname=railway user=qa password=qa sslmode=require",
        })
        with self.assertRaises(release.ReleaseError):
            release.validate_config(configuration() | {"DATABASE_URL": "host=postgres.railway.internal user=qa password=qa"})

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

    def test_built_plugin_zip_omits_test_only_seams(self):
        self.assert_zip_php_omits_seams(dockerfile_mvp_plugin_zip())
        probe = subprocess.run(
            ["git", "rev-parse", "--is-inside-work-tree"],
            cwd=release.ROOT,
            capture_output=True,
            text=True,
        )
        if probe.returncode == 0 and probe.stdout.strip() == "true":
            with tempfile.TemporaryDirectory() as directory:
                result = subprocess.run(
                    [str(release.ROOT / "scripts" / "package-wp-zip"), str(Path(directory) / "affilio.zip")],
                    cwd=release.ROOT,
                    capture_output=True,
                    text=True,
                    timeout=60,
                )
                self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
                written = Path(directory) / ("affilio-%s.zip" % release.PLUGIN_VERSION)
                self.assert_zip_php_omits_seams(written.read_bytes())
        for seam in release.TEST_ONLY_PLUGIN_SEAMS:
            with self.subTest(seam=seam):
                poisoned = plugin_archive(bodies={
                    "affilio-wp/includes/class-affilio-api.php": "<?php\n" + seam + "\n",
                })
                with self.assertRaisesRegex(release.ReleaseError, re.escape(seam)):
                    release.validate_plugin(poisoned)

    def assert_zip_php_omits_seams(self, data):
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            php_names = [name for name in archive.namelist() if name.endswith(".php")]
            self.assertTrue(php_names)
            for name in php_names:
                text = archive.read(name).decode("utf8", "replace")
                for seam in release.TEST_ONLY_PLUGIN_SEAMS:
                    self.assertNotIn(seam, text, "%s contains %s" % (name, seam))

    def test_plugin_origin_and_hosted_asset_exclusion(self):
        version = release.PLUGIN_VERSION
        guarded = (
            "<?php\n/* Plugin Name: Affilio\n * Version: %s\n */\n"
            "define('AFFILIO_VERSION', '%s');\n"
            "if (!function_exists('affilio_init')) {\nfunction affilio_init() {}\n}\n"
        ) % (version, version)

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
        with self.assertRaisesRegex(release.ReleaseError, re.escape(version)):
            release.validate_plugin(archive(plugin=guarded.replace(version, "0.1.9")))
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
                release.validate_plugin(plugin_archive(bodies={script: with_labels(script, body)}))
        release.validate_plugin(plugin_archive(bodies={script: with_labels(script, "// import('./landing-scene.bundle.js');\nimport * as THREE from 'three';\nimport './analytics.js';\n")}))
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
            "home.html", "agent-poc.html", "protocol.html", "publishers-setup.html", "setup-controls.js", "use-cases.html",
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
                self.assertIn("passed the release gate with 28 files: affilio-%s.zip" % version, result.stdout)
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
        self.assertEqual(len(names), 28)
        self.assertIn("affilio-wp/languages/affilio-wp-it_IT.mo", names)
        self.assertTrue(hosted.isdisjoint(Path(name).name for name in names))
        for relative in needed:
            self.assertIn(release.PLUGIN_ROOT + relative, names)

    def _admin_src_plugin(self, directory, built):
        plugin = Path(directory) / "src-plugin"
        (plugin / "admin" / "src").mkdir(parents=True)
        (plugin / "admin" / "build").mkdir(parents=True)
        (plugin / "admin" / "src" / "dashboard.js").write_text("export const label = 'Analisi in corso';\n", encoding="utf-8")
        (plugin / "admin" / "build" / "dashboard.js").write_text(built, encoding="utf-8")
        return plugin

    def test_fresh_admin_bundle_matches_source(self):
        release.validate_built_admin_js(release.ROOT / "affilio-wp")
        source = "export const label = 'Analisi in corso';\n"
        release.validate_plugin(plugin_archive(
            extra=("affilio-wp/admin/src/dashboard.js", source),
            bodies={"affilio-wp/admin/build/dashboard.js": with_labels("affilio-wp/admin/build/dashboard.js", "(function () { return 'Analisi in corso'; })();\n")},
        ))
        with tempfile.TemporaryDirectory() as directory:
            plugin = self._admin_src_plugin(directory, "(function () { return 'Analisi in corso'; })();\n")
            release.validate_built_admin_js(plugin)

    def test_stale_admin_bundle_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            dest = Path(directory) / "affilio-wp"
            shutil.copytree(release.ROOT / "affilio-wp", dest, ignore=shutil.ignore_patterns("node_modules"))
            bundle = dest / "admin/build/landing-scene.bundle.js"
            bundle.write_bytes(bundle.read_bytes() + b"\n/* stale */\n")
            with self.assertRaisesRegex(release.ReleaseError, r"admin/build/landing-scene\.bundle\.js"):
                release.validate_built_admin_js(dest)
        source = "export const label = 'Analisi in corso';\n"
        stale = plugin_archive(
            extra=("affilio-wp/admin/src/dashboard.js", source),
            bodies={"affilio-wp/admin/build/dashboard.js": "(function () { return 'Analisi completata'; })();\n"},
        )
        with self.assertRaisesRegex(release.ReleaseError, "Analisi in corso"):
            release.validate_plugin(stale)
        with tempfile.TemporaryDirectory() as directory:
            plugin = self._admin_src_plugin(directory, "(function () { return 'Analisi completata'; })();\n")
            with self.assertRaisesRegex(release.ReleaseError, "Analisi in corso"):
                release.validate_built_admin_js(plugin)
        commented = plugin_archive(
            extra=("affilio-wp/admin/src/dashboard.js", "// 'Analisi in corso'\nconst label = 'Analisi completata';\n"),
            bodies={"affilio-wp/admin/build/dashboard.js": with_labels("affilio-wp/admin/build/dashboard.js", "(function () { return 'Analisi completata'; })();\n")},
        )
        release.validate_plugin(commented)

    def test_required_labels_match_whole_literals_in_one_file(self):
        stale = (FIXTURES / "dashboard-02a5051.js").read_text(encoding="utf-8")
        current = (FIXTURES / "dashboard-5e5e468.js").read_text(encoding="utf-8")
        self.assertNotIn("Analisi in corso", release.js_string_literals(stale))
        self.assertIn("Analisi in corso", release.js_string_literals(current))
        self.assertIn("Analisi completata", release.js_string_literals(current))
        with self.assertRaisesRegex(release.ReleaseError, r"admin/build/dashboard\.js is missing the label 'Analisi in corso'"):
            release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": stale}))
        release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": current}))

        ellipsis = "const status = 'Analisi in corso...';\nconst done = 'Analisi completata';\n"
        sidebar = "const status = 'Analisi in corso';\n" + release.required_label_script(release.required_label_manifest()["sidebar.js"])
        with self.assertRaisesRegex(release.ReleaseError, r"admin/build/dashboard\.js is missing the label 'Analisi in corso'"):
            release.validate_plugin(plugin_archive(bodies={
                "affilio-wp/admin/build/dashboard.js": ellipsis,
                "affilio-wp/admin/build/sidebar.js": sidebar,
            }))
        longer = "const status = 'See Analisi in corso now';\nconst done = 'Analisi completata';\n"
        with self.assertRaisesRegex(release.ReleaseError, "Analisi in corso"):
            release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": longer}))
        commented = "// 'Analisi in corso'\nconst done = 'Analisi completata';\n"
        with self.assertRaisesRegex(release.ReleaseError, "Analisi in corso"):
            release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": commented}))

    def test_label_escapes_and_template_static_parts(self):
        encoded = "const progress = '\\u0041nalisi\\u0020in\\x20corso';\nconst done = `\\u{41}nalisi completata`;\n"
        self.assertIn("Analisi in corso", release.js_string_literals(encoded))
        self.assertIn("Analisi completata", release.js_string_literals(encoded))
        release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": encoded}))
        static = "const progress = `Analisi in corso${extra}`;\nconst done = 'Analisi completata';\n"
        self.assertIn("Analisi in corso", release.js_string_literals(static))
        self.assertNotIn("Analisi in corso ", release.user_visible_string_literals("const label = `Analisi in corso${count}`;\n"))
        release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": static}))
        spaced = "const progress = `Analisi in corso ${extra}`;\nconst done = 'Analisi completata';\n"
        self.assertNotIn("Analisi in corso", release.js_string_literals(spaced))
        self.assertIn("Analisi in corso ", release.js_string_literals(spaced))
        with self.assertRaisesRegex(release.ReleaseError, "Analisi in corso"):
            release.validate_plugin(plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": spaced}))
        hidden = "const label = `Analisi in corso${count}`;\n// 'not a visible label'\n"
        self.assertEqual(release.user_visible_string_literals(hidden), ["Analisi in corso"])

    def test_verify_and_bundle_read_admin_files_from_the_zip(self):
        stale = (FIXTURES / "dashboard-02a5051.js").read_text(encoding="utf-8")
        current = (FIXTURES / "dashboard-5e5e468.js").read_text(encoding="utf-8")
        bad = plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": stale})
        good = plugin_archive(bodies={"affilio-wp/admin/build/dashboard.js": current})

        def refuse_checkout(plugin_dir):
            raise AssertionError("release check read the local admin build at %s" % plugin_dir)

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "assets").mkdir()
            (root / "downloads").mkdir()
            for name in release.ASSETS:
                (root / "assets" / name).write_text("asset", encoding="utf-8")
            (root / "downloads" / release.INSTALLER).write_bytes(b"installer")
            (root / "downloads" / "affilio.zip").write_bytes(bad)
            with patch.object(release, "validate_installer"), patch.object(release, "validate_built_admin_js", refuse_checkout), self.assertRaisesRegex(release.ReleaseError, r"admin/build/dashboard\.js"):
                release.validate_bundle(root)
            (root / "downloads" / "affilio.zip").write_bytes(good)
            with patch.object(release, "validate_installer"), patch.object(release, "validate_built_admin_js", refuse_checkout), patch("sys.stdout", io.StringIO()):
                release.validate_bundle(root)

        responses = public_responses()
        responses["/downloads/affilio.zip"] = response(body=bad)
        with patch.object(release, "fetch", side_effect=lambda path, *args: responses[path]), patch.object(release, "validate_installer"), patch.object(release, "validate_built_admin_js", refuse_checkout), self.assertRaisesRegex(release.ReleaseError, "Analisi in corso"):
            release.verify()
        responses["/downloads/affilio.zip"] = response(body=good)
        with patch.object(release, "fetch", side_effect=lambda path, *args: responses[path]), patch.object(release, "validate_installer"), patch.object(release, "validate_built_admin_js", refuse_checkout), patch("sys.stdout", io.StringIO()):
            release.verify()

    def test_npm_ci_failure_shows_stderr_tail(self):
        def fake_run(args, **kwargs):
            if list(args[:2]) == ["npm", "ci"]:
                stderr = "\n".join(["noise-%d" % i for i in range(40)] + ["npm ERR! distinct-tail"])
                return subprocess.CompletedProcess(args, 1, "", stderr)
            if list(args[:3]) == ["npm", "run", "build"]:
                return subprocess.CompletedProcess(args, 1, "", "npm ERR! build-secret")
            return subprocess.CompletedProcess(args, 0, "", "")

        with tempfile.TemporaryDirectory() as directory:
            build = Path(directory) / "affilio-wp" / "admin" / "build"
            build.mkdir(parents=True)
            (build / "package.json").write_text('{"scripts":{"build":"true"}}\n', encoding="utf-8")
            (build / "dashboard.js").write_text("var kept = 1;\n", encoding="utf-8")
            with patch.object(subprocess, "run", fake_run), self.assertRaises(release.ReleaseError) as caught:
                release.validate_built_admin_js(Path(directory) / "affilio-wp")
        message = str(caught.exception)
        self.assertIn("npm ERR! distinct-tail", message)
        self.assertNotIn("noise-0", message)
        self.assertNotIn("build-secret", message)

        def build_fails(args, **kwargs):
            if list(args[:2]) == ["npm", "ci"]:
                return subprocess.CompletedProcess(args, 0, "", "")
            return subprocess.CompletedProcess(args, 1, "", "npm ERR! build-secret")

        with tempfile.TemporaryDirectory() as directory:
            build = Path(directory) / "affilio-wp" / "admin" / "build"
            build.mkdir(parents=True)
            (build / "package.json").write_text('{"scripts":{"build":"true"}}\n', encoding="utf-8")
            (build / "dashboard.js").write_text("var kept = 1;\n", encoding="utf-8")
            with patch.object(subprocess, "run", build_fails), self.assertRaises(release.ReleaseError) as caught:
                release.validate_built_admin_js(Path(directory) / "affilio-wp")
        self.assertIn("withheld", str(caught.exception))
        self.assertNotIn("build-secret", str(caught.exception))

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
            "PUBLIC_BASE_URL": "https://allagio.example",
            "ALLOWED_ORIGINS": "https://allagio.example",
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
        origin = "https://allagio.example"
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

    def test_image_build_and_release_script_agree_on_the_installer(self):
        script = (release.ROOT / "image/build-image.sh").read_text()

        def assignment(name):
            found = re.search(r"(?m)^%s=(['\"]?)([^'\"\s]+)\1\s*$" % re.escape(name), script)
            self.assertIsNotNone(found, name)
            return found.group(2)

        self.assertEqual(assignment("installer_tarball"), release.INSTALLER)
        self.assertEqual(assignment("package_name"), release.INSTALLER_PACKAGE)
        dockerfile = (release.ROOT / "Dockerfile.mvp").read_text()

        def docker_default(name):
            found = re.search(r"(?m)^ARG %s=(\S+)\s*$" % re.escape(name), dockerfile)
            self.assertIsNotNone(found, name)
            return found.group(1)

        self.assertEqual(docker_default("INSTALLER_TGZ"), release.INSTALLER)
        self.assertEqual(docker_default("PACKAGE_NAME"), release.INSTALLER_PACKAGE)

    def test_installer_package_is_allagio_with_old_command_aliases(self):
        self.assertEqual(release.INSTALLER, "allagio-0.1.0.tgz")
        root = Path(release.__file__).resolve().parent.parent
        for rel in ("scripts/mvp_release.py", ".github/workflows/mvp-release.yml"):
            text = (root / rel).read_text()
            self.assertNotIn("offer-rails-0.1.0.tgz", text, rel)
        self.assertIn("protocol.html", release.ASSETS)
        self.assertIn("protocol.html", release.TEMPLATE_ASSETS)

        def archive(package):
            output = io.BytesIO()
            with tarfile.open(fileobj=output, mode="w:gz") as packed:
                def add(name, data):
                    payload = data.encode()
                    info = tarfile.TarInfo(name)
                    info.size = len(payload)
                    packed.addfile(info, io.BytesIO(payload))
                add("package/package.json", json.dumps(package))
                add("package/bin/setup.mjs", "#!/usr/bin/env node\n")
                add("package/lib/setup.mjs", "export {}\n")
                for skill in release.SKILLS:
                    add(skill, "# skill\n")
            return output.getvalue()

        bins = {
            "allagio": "bin/setup.mjs",
            "offer-rails": "bin/setup.mjs",
            "affilio": "bin/setup.mjs",
            "distributoor": "bin/setup.mjs",
            "distributoor-setup": "bin/setup.mjs",
        }
        release.validate_installer(archive({"name": "allagio", "version": "0.1.0", "bin": bins}))
        old_name = archive({"name": "offer-rails", "version": "0.1.0", "bin": bins})
        with self.assertRaisesRegex(release.ReleaseError, "Installer package metadata"):
            release.validate_installer(old_name)
        stale = archive({"name": "@distributoor/setup", "version": "0.1.0", "bin": {"distributoor-setup": "bin/setup.mjs"}})
        with self.assertRaisesRegex(release.ReleaseError, "Installer package metadata"):
            release.validate_installer(stale)


if __name__ == "__main__":
    unittest.main()
