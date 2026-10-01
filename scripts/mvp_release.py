#!/usr/bin/env python3
import argparse
import base64
import io
import json
from pathlib import Path, PurePosixPath
import subprocess
import sys
import tarfile
from urllib import error, parse, request
import zipfile


ROOT = Path(__file__).resolve().parent.parent
PROJECT = "cf3a1d9d-d8f8-44bf-94e1-fcb590055ae0"
ENVIRONMENT = "2534a1db-d231-45ae-868e-680c48b11dfb"
SERVICE = "c3a5be47-e442-4ff4-8dda-ee808944c233"
ORIGIN = "https://distributoor-production.up.railway.app"
INSTALLER = "distributoor-setup-0.1.0.tgz"
CHECKS = {"onboarding", "affilio-api-tests", "mvp-release-package"}
ASSETS = (
    "home.html", "workspace.html", "workspace.css", "workspace.js",
    "setup-controls.js", "landing.css", "landing.js",
    "landing-scene.bundle.js", "landing-scene.bundle.js.LEGAL.txt",
    "THIRD_PARTY_NOTICES.md",
)
WP_FILES = {
    "affilio/affilio.php", "affilio/includes/class-affilio-api.php",
    "affilio/includes/class-affilio-editor-proxy.php",
    "affilio/includes/class-affilio-onboarding.php",
    "affilio/includes/class-affilio-distribution.php",
    "affilio/admin/build/onboarding.js", "affilio/admin/build/distribution.js",
    "affilio/admin/build/distribution.css",
}
SKILLS = {f"package/skills/distributoor-{name}/SKILL.md" for name in ("onboard", "instrument", "launch")}
MAX_DOWNLOAD = 20 * 1024 * 1024


class ReleaseError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise ReleaseError(message)


def run(args):
    try:
        result = subprocess.run(args, cwd=ROOT, capture_output=True, text=True, timeout=180)
    except (OSError, subprocess.TimeoutExpired):
        raise ReleaseError(f"{args[0]} could not run; check the CLI and its authentication.") from None
    require(result.returncode == 0, f"{args[0]} failed; inspect it locally. Command output was withheld to protect credentials.")
    return result.stdout.strip()


def run_json(args):
    try:
        return json.loads(run(args))
    except ValueError:
        raise ReleaseError(f"{args[0]} returned an unexpected response; raw output was withheld.") from None


def validate_config(values):
    names = {
        "DATABASE_URL", "ADMIN_API_KEY", "CONNECTION_ENCRYPTION_KEY",
        "OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
        "PUBLIC_BASE_URL", "HOST", "PORT", "AUTO_MIGRATE", "ALLOWED_ORIGINS",
        "RAILWAY_DOCKERFILE_PATH",
    }
    require(isinstance(values, dict) and all(isinstance(v, str) for v in values.values()), "Unexpected Railway variable format.")
    missing = sorted(name for name in names if not values.get(name, "").strip())
    require(not missing, "Missing Railway variables: " + ", ".join(missing))
    expected = {
        "PUBLIC_BASE_URL": ORIGIN, "OIDC_ISSUER": "https://accounts.google.com",
        "HOST": "0.0.0.0", "PORT": "8080", "AUTO_MIGRATE": "true",
        "RAILWAY_DOCKERFILE_PATH": "Dockerfile.mvp",
    }
    for name, value in expected.items():
        require(values[name] == value, f"Incorrect Railway configuration: {name}.")
    require(values["OIDC_CLIENT_ID"].endswith(".apps.googleusercontent.com"), "OIDC_CLIENT_ID must be the Google Web application client ID.")
    require(set(values["ALLOWED_ORIGINS"].split(",")) == {ORIGIN}, "ALLOWED_ORIGINS must contain only the MVP origin.")
    require(len(values["ADMIN_API_KEY"]) >= 32, "ADMIN_API_KEY must contain at least 32 characters.")
    try:
        valid_key = len(base64.b64decode(values["CONNECTION_ENCRYPTION_KEY"], validate=True)) == 32
    except ValueError:
        valid_key = False
    require(valid_key, "CONNECTION_ENCRYPTION_KEY must encode 32 bytes in base64.")
    database = parse.urlsplit(values["DATABASE_URL"])
    require(database.scheme in {"postgres", "postgresql"} and database.hostname and database.path not in {"", "/"}, "DATABASE_URL must identify the isolated PostgreSQL database.")


def validate_checks(checks):
    latest = {}
    for check in checks:
        name = check.get("name")
        if name in CHECKS and check.get("id", 0) > latest.get(name, {}).get("id", -1):
            latest[name] = check
    failed = sorted(name for name in CHECKS if latest.get(name, {}).get("status") != "completed" or latest.get(name, {}).get("conclusion") != "success")
    require(not failed, "Required GitHub checks are missing, pending or failed: " + ", ".join(failed))


def preflight():
    values = run_json(["railway", "variables", "--project", PROJECT, "--environment", ENVIRONMENT, "--service", SERVICE, "--json"])
    validate_config(values)
    require(not run(["git", "status", "--porcelain"]), "Commit the working tree before deploying.")
    branch = run(["git", "branch", "--show-current"])
    require(bool(branch), "Deploy from a pushed branch, not a detached checkout.")
    sha = run(["git", "rev-parse", "HEAD"])
    remote = run(["git", "ls-remote", "origin", f"refs/heads/{branch}"])
    require(remote.split()[:1] == [sha], "Push the exact commit before deploying.")
    data = run_json(["gh", "api", f"repos/vickhunter/affilio/commits/{sha}/check-runs?per_page=100"])
    validate_checks(data.get("check_runs", []))
    print(f"Preflight passed for {sha[:12]} on distributoor-mvp.")
    if not values.get("OPERATOR_OWNER_ID"):
        print("Operator access remains pending: set OPERATOR_OWNER_ID after the first real owner login.")
    return sha


def archive_names(names):
    require(len(names) == len(set(names)), "Archive contains duplicate entries.")
    for name in names:
        parts = PurePosixPath(name).parts
        require(not name.startswith("/") and ".." not in parts and "\\" not in name, "Archive contains an unsafe path.")
        require(not any(part in {".git", ".local", "node_modules", ".env"} or part.startswith(".env.") for part in parts), "Archive contains private or development files.")


def validate_plugin(data, origin=None):
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        names = archive.namelist()
        archive_names(names)
        require(WP_FILES <= set(names), "WordPress ZIP is missing runtime files.")
        require(sum(file.file_size for file in archive.infolist()) <= MAX_DOWNLOAD, "WordPress ZIP expands beyond the release size limit.")
        require(archive.testzip() is None, "WordPress ZIP failed its checksum check.")
        for name in names:
            require(name.startswith("affilio/"), "WordPress ZIP has an unexpected top-level path.")
            if name.startswith("affilio/admin/build/"):
                file = PurePosixPath(name).name
                require(not file.startswith(("workspace", "landing", "package")) and file not in {"home.html", "agent-poc.html", "setup-controls.js", "THIRD_PARTY_NOTICES.md"}, "WordPress ZIP includes hosted UI or build tools.")
        plugin = archive.read("affilio/affilio.php").decode("utf8")
        require("Plugin Name:" in plugin, "WordPress plugin header is missing.")
        if origin:
            require(f"'{origin}/v1'" in plugin and "https://affilio-production.up.railway.app/v1" not in plugin, "WordPress download points at the wrong API origin.")


def validate_installer(data):
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        members = archive.getmembers()
        archive_names([member.name for member in members])
        require(all(member.isfile() or member.isdir() for member in members), "Installer includes non-regular files.")
        require(sum(member.size for member in members) <= MAX_DOWNLOAD, "Installer expands beyond the release size limit.")
        require(SKILLS | {"package/bin/setup.mjs", "package/lib/setup.mjs", "package/package.json"} <= set(archive.getnames()), "Installer is missing its command or skills.")
        package = json.load(archive.extractfile("package/package.json"))
        require(package.get("name") == "@distributoor/setup" and package.get("version") == "0.1.0" and package.get("bin", {}).get("distributoor-setup") == "bin/setup.mjs", "Installer package metadata is incorrect.")


def validate_bundle(path):
    for name in ASSETS:
        asset = path / "assets" / name
        require(asset.is_file() and asset.stat().st_size > 0, f"Release is missing assets/{name}.")
    validate_plugin((path / "downloads" / "affilio.zip").read_bytes())
    validate_installer((path / "downloads" / INSTALLER).read_bytes())
    print("Docker release package passed: hosted assets, Three.js notices, installer skills and WordPress runtime ZIP.")


class NoRedirect(request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def fetch(path, method="GET", data=None):
    req = request.Request(ORIGIN + path, method=method, data=data, headers={"Content-Type": "application/json", "User-Agent": "Distributoor-Release-Check"})
    try:
        response = request.build_opener(NoRedirect).open(req, timeout=20)
    except error.HTTPError as exc:
        response = exc
    except (error.URLError, TimeoutError):
        raise ReleaseError(f"Public service could not be reached at {path}.") from None
    with response:
        body = response.read(MAX_DOWNLOAD + 1)
        require(len(body) <= MAX_DOWNLOAD, f"Response exceeds the release size limit at {path}.")
        return response.code, response.headers, body


def verify():
    def get(path):
        status, headers, body = fetch(path)
        require(status == 200, f"Public check failed at {path}: HTTP {status}.")
        return headers, body

    health = json.loads(get("/health")[1])
    require(health.get("status") == "ok" and health.get("site_installation_registration") is True and health.get("native_distribution") is True, "Health response does not confirm the native distribution service.")
    headers, home = get("/")
    require("script-src 'self'" in headers.get("Content-Security-Policy", "") and headers.get("X-Content-Type-Options") == "nosniff", "Public page security headers are missing.")
    require(INSTALLER.encode() in home and b"Download WordPress plugin" in home, "Home is not offering both packaged installation paths.")
    for name in ASSETS:
        if name not in {"home.html", "workspace.html"}:
            require(bool(get("/assets/" + name)[1]), f"Empty public asset: {name}.")
    validate_plugin(get("/downloads/affilio.zip")[1], ORIGIN)
    validate_installer(get("/downloads/" + INSTALLER)[1])
    resource = json.loads(get("/.well-known/oauth-protected-resource/mcp")[1])
    authorization = json.loads(get("/.well-known/oauth-authorization-server")[1])
    require(resource.get("resource") == ORIGIN + "/mcp" and resource.get("authorization_servers") == [ORIGIN] and set(resource.get("scopes_supported", [])) == {"maker", "publisher"}, "MCP discovery is configured for the wrong resource or scopes.")
    for field, path in {"issuer": "", "authorization_endpoint": "/oauth/authorize", "token_endpoint": "/oauth/token", "registration_endpoint": "/oauth/register"}.items():
        require(authorization.get(field) == ORIGIN + path, "OAuth discovery is configured for the wrong origin.")
    require("S256" in authorization.get("code_challenge_methods_supported", []), "OAuth discovery does not advertise S256 PKCE.")
    status, headers, _ = fetch("/owner")
    require(status == 302 and headers.get("Location", "").startswith("/auth/login?"), "Anonymous owner access did not require sign-in.")
    status, headers, _ = fetch("/mcp", "POST", b'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"release-check","version":"1"}}}')
    require(status == 401 and ORIGIN + "/.well-known/oauth-protected-resource/mcp" in headers.get("WWW-Authenticate", ""), "Anonymous MCP access did not require OAuth.")
    status, headers, _ = fetch("/auth/login?return_to=/owner")
    target = parse.urlsplit(headers.get("Location", ""))
    params = parse.parse_qs(target.query)
    require(status == 302 and target.scheme == "https" and target.hostname == "accounts.google.com" and params.get("redirect_uri") == [ORIGIN + "/auth/callback"] and params.get("code_challenge_method") == ["S256"], "Google login start has the wrong callback or PKCE configuration.")
    cookie = headers.get("Set-Cookie", "")
    require(cookie.startswith("__Host-dtr_login=") and all(flag in cookie for flag in ("Path=/", "Secure", "HttpOnly", "SameSite=Lax")), "Google login attempt cookie is missing security attributes.")
    print("Public delivery checks passed: health, assets, both downloads, OAuth discovery, protected owner/MCP routes and Google login redirect.")
    print("Real Google login, operator access and merchant checkout still require verification.")


def main():
    parser = argparse.ArgumentParser(description="Release only the isolated Distributoor MVP. Configuration values are never printed.")
    parser.add_argument("command", choices=("check", "deploy", "verify", "bundle"), nargs="?", default="check")
    parser.add_argument("path", type=Path, nargs="?", help="Extracted Docker release directory for bundle validation")
    args = parser.parse_args()
    try:
        if args.command == "bundle":
            require(args.path is not None, "Provide the extracted release directory.")
            validate_bundle(args.path)
        elif args.command == "verify":
            verify()
        else:
            sha = preflight()
            if args.command == "deploy":
                run(["railway", "up", "--project", PROJECT, "--environment", ENVIRONMENT, "--service", SERVICE, "--detach", "--json", "--message", f"Distributoor MVP {sha}"])
                print(f"Deployment requested for {sha[:12]}. Wait for Railway SUCCESS, then run: python3 scripts/mvp_release.py verify")
    except (ReleaseError, ValueError, OSError, tarfile.TarError, zipfile.BadZipFile) as exc:
        message = str(exc) if isinstance(exc, ReleaseError) else "Invalid or unavailable release data; inspect the package locally."
        print("Release stopped: " + message, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
