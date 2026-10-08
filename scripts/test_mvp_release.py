import base64
from email.message import Message
import io
import json
from pathlib import Path
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


class ReleaseTests(unittest.TestCase):
    def test_runtime_image_defaults_to_release_mode(self):
        dockerfile = (release.ROOT / "Dockerfile.mvp").read_text()
        runtime = dockerfile.split("FROM ubuntu:24.04", 1)[1]
        self.assertIn("ENV GIN_MODE=release", runtime)
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
        def archive(origin, extra=None):
            output = io.BytesIO()
            with zipfile.ZipFile(output, "w") as z:
                for name in release.WP_FILES:
                    z.writestr(name, f"<?php /* Plugin Name: Affilio */ define('AFFILIO_API_URL', '{origin}/v1');" if name.endswith("/affilio.php") else "runtime")
                if extra:
                    z.writestr(extra, "unexpected")
            return output.getvalue()

        release.validate_plugin(archive(release.ORIGIN), release.ORIGIN)
        with self.assertRaisesRegex(release.ReleaseError, "wrong API origin"):
            release.validate_plugin(archive("https://affilio-production.up.railway.app"), release.ORIGIN)
        with self.assertRaisesRegex(release.ReleaseError, "hosted UI"):
            release.validate_plugin(archive(release.ORIGIN, "affilio/admin/build/landing-scene.bundle.js"))

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
