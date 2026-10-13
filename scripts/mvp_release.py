#!/usr/bin/env python3
import argparse
import base64
import io
import json
from pathlib import Path, PurePosixPath
import re
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
    "affilio-wp/affilio.php",
    "affilio-wp/includes/class-affilio-api.php",
    "affilio-wp/includes/class-affilio-scanner.php",
    "affilio-wp/includes/class-affilio-inserter.php",
    "affilio-wp/includes/class-affilio-tracker.php",
    "affilio-wp/includes/class-affilio-admin.php",
    "affilio-wp/includes/class-affilio-editor-proxy.php",
    "affilio-wp/includes/class-affilio-onboarding.php",
    "affilio-wp/includes/class-affilio-distribution.php",
    "affilio-wp/includes/class-affilio-cli.php",
    "affilio-wp/admin/build/onboarding.js",
    "affilio-wp/admin/build/distribution.js",
    "affilio-wp/admin/build/distribution.css",
    "affilio-wp/admin/build/sidebar.js",
    "affilio-wp/admin/build/discovery.js",
    "affilio-wp/admin/build/analytics.js",
    "affilio-wp/admin/build/dashboard.js",
    "affilio-wp/admin/build/marketplace.js",
    "affilio-wp/admin/build/earnings.js",
    "affilio-wp/admin/css/affilio-admin.css",
    "affilio-wp/readme.txt",
}
PLUGIN_ROOT = "affilio-wp/"
PLUGIN_VERSION = "0.1.21"
ASSET_CONCAT = re.compile(r"""AFFILIO_PLUGIN_(?:URL|DIR)\s*\.\s*(['"])([^'"]+)\1""")
DYNAMIC_BUILD = re.compile(r"""admin/build/['"]\s*\.\s*\$""")
QUOTED_BUILD = re.compile(r"""(['"])([A-Za-z0-9_.-]+\.(?:js|css))\1""")
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


OPEN_PROXIES = {"*", "0.0.0.0/0", "::/0"}


def load_env_file(path):
    values = {}
    for raw in Path(path).read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def https_origin(value):
    parsed = parse.urlsplit(value)
    return parsed.scheme == "https" and bool(parsed.hostname) and value == f"https://{parsed.netloc}"


def validate_config(values, origin=None):
    generic = origin is not None
    expected_origin = origin if generic else ORIGIN
    label = "service" if generic else "Railway"
    names = {
        "DATABASE_URL", "ADMIN_API_KEY", "CONNECTION_ENCRYPTION_KEY",
        "OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
        "PUBLIC_BASE_URL", "HOST", "PORT", "AUTO_MIGRATE", "ALLOWED_ORIGINS",
        "API_KEY_SECRET", "GIN_MODE", "TRUSTED_PROXIES",
    }
    if not generic:
        names.add("RAILWAY_DOCKERFILE_PATH")
    require(isinstance(values, dict) and all(isinstance(v, str) for v in values.values()), f"Unexpected {label} variable format.")
    missing = sorted(name for name in names if not values.get(name, "").strip())
    require(not missing, f"Missing {label} variables: " + ", ".join(missing))
    require(https_origin(expected_origin), "PUBLIC_BASE_URL must be an https origin.")
    expected = {
        "PUBLIC_BASE_URL": expected_origin, "OIDC_ISSUER": "https://accounts.google.com",
        "HOST": "0.0.0.0", "AUTO_MIGRATE": "true", "GIN_MODE": "release",
    }
    if generic:
        require(values["PORT"].isdigit() and 1 <= int(values["PORT"]) <= 65535, "Incorrect service configuration: PORT.")
    else:
        expected["PORT"] = "8080"
        expected["RAILWAY_DOCKERFILE_PATH"] = "Dockerfile.mvp"
    for name, value in expected.items():
        require(values[name] == value, f"Incorrect {label} configuration: {name}.")
    require(values["OIDC_CLIENT_ID"].endswith(".apps.googleusercontent.com"), "OIDC_CLIENT_ID must be the Google Web application client ID.")
    require(set(part.strip() for part in values["ALLOWED_ORIGINS"].split(",")) == {expected_origin}, "ALLOWED_ORIGINS must contain only the MVP origin.")
    require(len(values["ADMIN_API_KEY"]) >= 32, "ADMIN_API_KEY must contain at least 32 characters.")
    require(values["API_KEY_SECRET"] != "dev-secret" and len(values["API_KEY_SECRET"]) >= 32, "API_KEY_SECRET must contain at least 32 characters and must not be the development default.")
    proxies = [part.strip() for part in values["TRUSTED_PROXIES"].split(",") if part.strip()]
    require(proxies and OPEN_PROXIES.isdisjoint(proxies), "TRUSTED_PROXIES must list platform proxy ranges and must not trust every address.")
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


def git_preflight():
    require(not run(["git", "status", "--porcelain"]), "Commit the working tree before deploying.")
    branch = run(["git", "branch", "--show-current"])
    require(bool(branch), "Deploy from a pushed branch, not a detached checkout.")
    sha = run(["git", "rev-parse", "HEAD"])
    remote = run(["git", "ls-remote", "origin", f"refs/heads/{branch}"])
    require(remote.split()[:1] == [sha], "Push the exact commit before deploying.")
    data = run_json(["gh", "api", f"repos/vickhunter/affilio/commits/{sha}/check-runs?per_page=100"])
    validate_checks(data.get("check_runs", []))
    return sha


def preflight(host="railway", env_file=None):
    if host == "generic":
        require(env_file is not None, "Generic host checks require --env-file. Names are checked; values are never printed.")
        values = load_env_file(env_file)
        origin = values.get("PUBLIC_BASE_URL", "").strip()
        validate_config(values, origin=origin)
        sha = git_preflight()
        print(f"Preflight passed for {sha[:12]} on {origin}.")
        return sha
    require(host == "railway", "Deploy and the Railway variable check stay on --host railway.")
    values = run_json(["railway", "variables", "--project", PROJECT, "--environment", ENVIRONMENT, "--service", SERVICE, "--json"])
    validate_config(values)
    sha = git_preflight()
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


def referenced_plugin_files(text):
    needed = set()
    for match in ASSET_CONCAT.finditer(text):
        relative = match.group(2).lstrip("/")
        if "." in PurePosixPath(relative).name:
            needed.add(relative)
    if DYNAMIC_BUILD.search(text):
        for match in QUOTED_BUILD.finditer(text):
            needed.add("admin/build/" + match.group(2))
    return needed


def unguarded_init(text):
    marker = "function affilio_init"
    start = 0
    while True:
        index = text.find(marker, start)
        if index < 0:
            return False
        window = text[max(0, index - 400):index]
        if "function_exists('affilio_init')" not in window and 'function_exists("affilio_init")' not in window:
            return True
        start = index + len(marker)


def validate_plugin(data, origin=None):
    del origin  # The plugin no longer bakes a deployment origin into its source.
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        names = archive.namelist()
        archive_names(names)
        require(sum(file.file_size for file in archive.infolist()) <= MAX_DOWNLOAD, "WordPress ZIP expands beyond the release size limit.")
        require(archive.testzip() is None, "WordPress ZIP failed its checksum check.")
        for info in archive.infolist():
            if info.is_dir():
                continue
            require(info.compress_type == zipfile.ZIP_DEFLATED, "WordPress ZIP stores " + info.filename + " uncompressed")
        files = {name: archive.read(name) for name in names if not name.endswith("/")}
        require(PLUGIN_ROOT + "admin/build/analytics.js" in files, "WordPress ZIP is missing admin/build/analytics.js")
        missing = sorted(WP_FILES - set(files))
        require(not missing, "WordPress ZIP is missing " + ", ".join(missing))
        for name, payload in files.items():
            require(name.startswith(PLUGIN_ROOT), "WordPress ZIP top folder must be affilio-wp/")
            require(b"railway.app" not in payload, "WordPress ZIP must not contain railway.app")
            if name.startswith(PLUGIN_ROOT + "admin/build/"):
                file = PurePosixPath(name).name
                require(not file.startswith(("workspace", "landing", "package")) and file not in {"home.html", "agent-poc.html", "setup-controls.js", "THIRD_PARTY_NOTICES.md"}, "WordPress ZIP includes hosted UI or build tools.")
        plugin = files[PLUGIN_ROOT + "affilio.php"].decode("utf8")
        require("Plugin Name:" in plugin, "WordPress plugin header is missing.")
        php = {name: payload.decode("utf8", "replace") for name, payload in files.items() if name.endswith(".php")}
        require(not any(unguarded_init(text) for text in php.values()), "WordPress ZIP has an unguarded function affilio_init")
        require(f"Version: {PLUGIN_VERSION}" in plugin, f"WordPress plugin header is not {PLUGIN_VERSION}")
        require(f"define('AFFILIO_VERSION', '{PLUGIN_VERSION}')" in plugin, f"AFFILIO_VERSION is not {PLUGIN_VERSION}")
        require(f"Stable tag: {PLUGIN_VERSION}".encode() in files.get(PLUGIN_ROOT + "readme.txt", b""), f"readme stable tag is not {PLUGIN_VERSION}")
        for text in php.values():
            for relative in referenced_plugin_files(text):
                require(PLUGIN_ROOT + relative in files, "WordPress ZIP is missing " + relative)


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


def fetch(path, method="GET", data=None, origin=ORIGIN):
    req = request.Request(origin + path, method=method, data=data, headers={"Content-Type": "application/json", "User-Agent": "Distributoor-Release-Check"})
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


def verify(origin=ORIGIN):
    require(https_origin(origin), "--base-url must be an https origin with no path, query, or fragment.")

    def get(path):
        status, headers, body = fetch(path, "GET", None, origin)
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
    validate_plugin(get("/downloads/affilio.zip")[1], origin)
    validate_installer(get("/downloads/" + INSTALLER)[1])
    resource = json.loads(get("/.well-known/oauth-protected-resource/mcp")[1])
    authorization = json.loads(get("/.well-known/oauth-authorization-server")[1])
    require(resource.get("resource") == origin + "/mcp" and resource.get("authorization_servers") == [origin] and set(resource.get("scopes_supported", [])) == {"maker", "publisher"}, "MCP discovery is configured for the wrong resource or scopes.")
    for field, path in {"issuer": "", "authorization_endpoint": "/oauth/authorize", "token_endpoint": "/oauth/token", "registration_endpoint": "/oauth/register"}.items():
        require(authorization.get(field) == origin + path, "OAuth discovery is configured for the wrong origin.")
    require("S256" in authorization.get("code_challenge_methods_supported", []), "OAuth discovery does not advertise S256 PKCE.")
    status, headers, _ = fetch("/owner", "GET", None, origin)
    require(status == 302 and headers.get("Location", "").startswith("/auth/login?"), "Anonymous owner access did not require sign-in.")
    status, headers, _ = fetch("/mcp", "POST", b'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"release-check","version":"1"}}}', origin)
    require(status == 401 and origin + "/.well-known/oauth-protected-resource/mcp" in headers.get("WWW-Authenticate", ""), "Anonymous MCP access did not require OAuth.")
    status, headers, _ = fetch("/auth/login?return_to=/owner", "GET", None, origin)
    target = parse.urlsplit(headers.get("Location", ""))
    params = parse.parse_qs(target.query)
    require(status == 302 and target.scheme == "https" and target.hostname == "accounts.google.com" and params.get("redirect_uri") == [origin + "/auth/callback"] and params.get("code_challenge_method") == ["S256"], "Google login start has the wrong callback or PKCE configuration.")
    cookie = headers.get("Set-Cookie", "")
    require(cookie.startswith("__Host-dtr_login=") and all(flag in cookie for flag in ("Path=/", "Secure", "HttpOnly", "SameSite=Lax")), "Google login attempt cookie is missing security attributes.")
    print("Public delivery checks passed: health, assets, both downloads, OAuth discovery, protected owner/MCP routes and Google login redirect.")
    print("Real Google login, operator access and merchant checkout still require verification.")


def main():
    parser = argparse.ArgumentParser(description="Release only the isolated Distributoor MVP. Configuration values are never printed.")
    parser.add_argument("command", choices=("check", "deploy", "verify", "bundle"), nargs="?", default="check")
    parser.add_argument("path", type=Path, nargs="?", help="Extracted Docker release directory for bundle validation")
    parser.add_argument("--host", choices=("railway", "generic"), default="railway", help="railway reads Railway variables; generic reads --env-file and does not call the Railway CLI")
    parser.add_argument("--env-file", type=Path, help="KEY=VALUE file for --host generic. Values are never printed.")
    parser.add_argument("--base-url", help="https origin for verify, for example https://service.onrender.com")
    args = parser.parse_args()
    try:
        if args.command == "bundle":
            require(args.path is not None, "Provide the extracted release directory.")
            validate_bundle(args.path)
        elif args.command == "verify":
            verify((args.base_url or ORIGIN).rstrip("/"))
        else:
            require(args.command != "deploy" or args.host == "railway", "Deploy stays on the Railway host. Use check --host generic to validate another host.")
            sha = preflight(args.host, args.env_file)
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
