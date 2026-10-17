#!/usr/bin/env python3
import argparse
import base64
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
from urllib import error, parse, request
import zipfile


ROOT = Path(__file__).resolve().parent.parent
PROJECT = "cf3a1d9d-d8f8-44bf-94e1-fcb590055ae0"
ENVIRONMENT = "2534a1db-d231-45ae-868e-680c48b11dfb"
SERVICE = "c3a5be47-e442-4ff4-8dda-ee808944c233"
ORIGIN = "https://distributoor-production.up.railway.app"
INSTALLER = "offer-rails-0.1.0.tgz"
CHECKS = {"onboarding", "affilio-api-tests", "mvp-release-package"}
ASSETS = (
    "home.html", "workspace.html", "protocol.html", "workspace.css", "workspace.js",
    "setup-controls.js", "landing.css", "landing.js",
    "landing-scene.bundle.js", "landing-scene.bundle.js.LEGAL.txt",
    "THIRD_PARTY_NOTICES.md",
)
TEMPLATE_ASSETS = {"home.html", "workspace.html", "protocol.html"}
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
    "affilio-wp/languages/affilio-it_IT.mo",
    "affilio-wp/readme.txt",
}
PLUGIN_ROOT = "affilio-wp/"
PLUGIN_VERSION = "0.1.32"
TEST_ONLY_PLUGIN_SEAMS = ("affilio_go_utc_date", "affilio_rewrite_requires_wp_rewrite")
PACKAGED_OUT = {"e2e", "tests", "test-results", "playwright-report"}
PACKAGED_FILES = {"phpunit.xml", "phpunit.xml.dist", ".phpunit.result.cache"}
HOSTED_BUILD_NAMES = {"home.html", "makers.html", "publishers.html", "publishers-setup.html", "protocol.html", "agent-poc.html", "setup-controls.js", "THIRD_PARTY_NOTICES.md"}
HOSTED_BUILD_PREFIXES = ("workspace", "landing", "package")
ASSET_CONCAT = re.compile(r"""AFFILIO_PLUGIN_(?:URL|DIR)\s*\.\s*(['"])([^'"]+)\1""")
DYNAMIC_BUILD = re.compile(r"""admin/build/['"]\s*\.\s*\$""")
QUOTED_BUILD = re.compile(r"""(['"])([A-Za-z0-9_.-]+\.(?:js|css))\1""")
JS_DYNAMIC_IMPORT = re.compile(r"""(?<![.\w$])import\s*\(\s*(['"`])([^'"`]+)\1""")
JS_FROM = re.compile(r"""\bfrom\s+(['"`])([^'"`]+)\1""")
JS_SIDE_IMPORT = re.compile(r"""(?<![.\w$])import\s+(['"`])([^'"`]+)\1""")
PHP_LOAD_CONCAT = re.compile(
    r"""(?<![.\w])(?:require_once|include_once|require|include)\b\s*(?:\(\s*)?"""
    r"""(?P<base>AFFILIO_PLUGIN_DIR|__DIR__|dirname\s*\(\s*__DIR__\s*(?:,\s*(?P<dir_levels>\d+)\s*)?\)|dirname\s*\(\s*__FILE__\s*(?:,\s*(?P<file_levels>\d+)\s*)?\))\s*\.\s*"""
    r"""(['"])(?P<path>[^'"]+)\4"""
)
PHP_LOAD_LITERAL = re.compile(
    r"""(?<![.\w])(?:require_once|include_once|require|include)\b\s*(?:\(\s*)?(['"])(?P<path>[^'"]+\.(?:php|inc|js|css))\1"""
)
PHP_ENQUEUE = re.compile(
    r"""wp_enqueue_(?:script|style)\s*\(\s*(?:(['"])[^'"]+\1|\$[A-Za-z_][A-Za-z0-9_]*)\s*,\s*"""
    r"""(?:AFFILIO_PLUGIN_URL\s*\.\s*|plugins_url\s*\(\s*)?(['"])([^'"]+)\2"""
)
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


def database_identified(value):
    if "://" in value:
        database = parse.urlsplit(value)
        return database.scheme in {"postgres", "postgresql"} and bool(database.hostname) and database.path not in {"", "/"}
    fields = {}
    for part in value.split():
        if "=" not in part:
            continue
        key, item = part.split("=", 1)
        fields[key.strip().lower()] = item.strip().strip('"').strip("'")
    host = fields.get("host") or fields.get("hostaddr")
    name = fields.get("dbname") or fields.get("database")
    return bool(host and name)


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
    require(database_identified(values["DATABASE_URL"]), "DATABASE_URL must identify the isolated PostgreSQL database.")


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


def strip_code_comments(text):
    kept = []
    index = 0
    length = len(text)
    while index < length:
        character = text[index]
        if character in "'\"`":
            kept.append(character)
            index += 1
            while index < length:
                kept.append(text[index])
                if text[index] == "\\":
                    index += 1
                    if index < length:
                        kept.append(text[index])
                        index += 1
                    continue
                if text[index] == character:
                    index += 1
                    break
                index += 1
            continue
        if text.startswith("/*", index):
            end = text.find("*/", index + 2)
            kept.append(" ")
            index = length if end < 0 else end + 2
            continue
        if text.startswith("//", index):
            end = text.find("\n", index)
            index = length if end < 0 else end
            continue
        kept.append(character)
        index += 1
    return "".join(kept)


def _packaged_reference(base_parts, spec):
    spec = spec.strip().split("?", 1)[0].split("#", 1)[0].strip()
    if not spec or spec.startswith("//") or re.match(r"^[a-z][a-z0-9+.-]*:", spec, re.I):
        return ""
    if base_parts is None:
        return None
    collapsed = []
    for part in list(base_parts) + [part for part in PurePosixPath(spec).parts if part != "/"]:
        if part in ("", "."):
            continue
        if part == "..":
            if not collapsed:
                return None
            collapsed.pop()
            continue
        collapsed.append(part)
    if not collapsed or collapsed[0] != "affilio-wp":
        return None
    relative = "/".join(collapsed[1:])
    if not relative or "." not in PurePosixPath(relative).name:
        return ""
    return relative


def _remember_reference(needed, outside, relative, label):
    if relative is None:
        outside.append(label.strip() or "..")
    elif relative:
        needed.add(relative)


def _js_references(source_name, text, needed, outside):
    base = list(PurePosixPath(source_name).parent.parts)
    specs = []
    for pattern in (JS_DYNAMIC_IMPORT, JS_FROM, JS_SIDE_IMPORT):
        specs.extend(match.group(2) for match in pattern.finditer(text))
    for spec in specs:
        cleaned = spec.strip()
        if not cleaned.startswith(("./", "../")):
            continue
        _remember_reference(needed, outside, _packaged_reference(base, cleaned), cleaned)


def _php_load_base(source_name, match):
    kind = match.group("base")
    if kind == "AFFILIO_PLUGIN_DIR":
        return ["affilio-wp"]
    parent = list(PurePosixPath(source_name).parent.parts)
    if kind == "__DIR__":
        return parent
    if match.group("file_levels") is not None or "FILE" in kind:
        levels = int(match.group("file_levels") or 1) - 1
    else:
        levels = int(match.group("dir_levels") or 1)
    base = parent[:]
    for _ in range(max(levels, 0)):
        if not base:
            return None
        base.pop()
    return base


def referenced_plugin_files(source_name, text):
    needed = set()
    outside = []
    suffix = PurePosixPath(source_name).suffix.lower()
    source = strip_code_comments(text)
    if suffix in {".js", ".mjs"}:
        _js_references(source_name, source, needed, outside)
        return needed, outside
    if suffix != ".php":
        return needed, outside
    for match in ASSET_CONCAT.finditer(source):
        _remember_reference(needed, outside, _packaged_reference(["affilio-wp"], match.group(2)), match.group(2))
    if DYNAMIC_BUILD.search(source):
        for match in QUOTED_BUILD.finditer(source):
            needed.add("admin/build/" + match.group(2))
    for match in PHP_LOAD_CONCAT.finditer(source):
        _remember_reference(needed, outside, _packaged_reference(_php_load_base(source_name, match), match.group("path")), match.group("path"))
    for match in PHP_LOAD_LITERAL.finditer(source):
        path = match.group("path")
        base = list(PurePosixPath(source_name).parent.parts) if path.startswith(("./", "../")) else ["affilio-wp"]
        _remember_reference(needed, outside, _packaged_reference(base, path), path)
    for match in PHP_ENQUEUE.finditer(source):
        _remember_reference(needed, outside, _packaged_reference(["affilio-wp"], match.group(3)), match.group(3))
    return needed, outside


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


def packaged_out(name):
    path = PurePosixPath(name)
    if any(part in PACKAGED_OUT for part in path.parts):
        return True
    file = path.name
    return file in PACKAGED_FILES or file.startswith(".phpunit.")


def hosted_build_file(name):
    return name in HOSTED_BUILD_NAMES or name.startswith(HOSTED_BUILD_PREFIXES)


def omit_from_plugin_zip(path):
    if packaged_out(path):
        return True
    if path.startswith(PLUGIN_ROOT + "admin/build/landing-v9/"):
        return True
    return path.startswith(PLUGIN_ROOT + "admin/build/") and hosted_build_file(PurePosixPath(path).name)


_canonical_admin_js_ok = False
_JS_ESCAPES = {"n": "\n", "r": "\r", "t": "\t", "'": "'", '"': '"', "\\": "\\", "`": "`"}
_HEX = set("0123456789abcdefABCDEF")
NPM_CI_STDERR_TAIL = 20
REQUIRED_LABELS = ROOT / "affilio-wp" / "admin" / "build" / "required-labels.json"


def _js_hex(value, index, count):
    end = index + count
    if end > len(value) or any(char not in _HEX for char in value[index:end]):
        return None
    return int(value[index:end], 16)


def _js_unescape(value):
    """Decode JavaScript string escapes, including \\u, \\u{...}, and \\x."""
    chars = []
    index = 0
    length = len(value)
    while index < length:
        if value[index] != "\\" or index + 1 >= length:
            chars.append(value[index])
            index += 1
            continue
        nxt = value[index + 1]
        if nxt == "u" and index + 2 < length and value[index + 2] == "{":
            end = value.find("}", index + 3)
            body = value[index + 3:end] if end > index + 2 else ""
            if end != -1 and body and all(char in _HEX for char in body):
                point = int(body, 16)
                if point <= 0x10FFFF:
                    chars.append(chr(point))
                    index = end + 1
                    continue
        if nxt == "u":
            point = _js_hex(value, index + 2, 4)
            if point is not None:
                chars.append(chr(point))
                index += 6
                continue
        if nxt == "x":
            point = _js_hex(value, index + 2, 2)
            if point is not None:
                chars.append(chr(point))
                index += 4
                continue
        chars.append(_JS_ESCAPES.get(nxt, nxt))
        index += 2
    return "".join(chars)


def js_string_literals(text):
    """Decoded whole string literals, including each static part of a template."""
    found = []
    length = len(text)
    index = 0
    frames = [("code", 0)]

    def push(raw):
        if raw:
            found.append(_js_unescape(raw))

    while index < length:
        mode, data = frames[-1]
        char = text[index]
        if mode == "line":
            if char == "\n":
                frames.pop()
            index += 1
            continue
        if mode == "block":
            if text.startswith("*/", index):
                frames.pop()
                index += 2
                continue
            index += 1
            continue
        if mode in ("squote", "dquote"):
            quote = "'" if mode == "squote" else '"'
            if char == "\\":
                data.append(char)
                if index + 1 < length:
                    data.append(text[index + 1])
                    index += 2
                else:
                    index += 1
                continue
            if char == quote:
                push("".join(data))
                frames.pop()
                index += 1
                continue
            if char == "\n":
                frames.pop()
                index += 1
                continue
            data.append(char)
            index += 1
            continue
        if mode == "template":
            if char == "\\":
                data.append(char)
                if index + 1 < length:
                    data.append(text[index + 1])
                    index += 2
                else:
                    index += 1
                continue
            if char == "`":
                push("".join(data))
                frames.pop()
                index += 1
                continue
            if text.startswith("${", index):
                push("".join(data))
                data.clear()
                frames.append(("code", 1))
                index += 2
                continue
            data.append(char)
            index += 1
            continue
        depth = data
        if depth and char == "{":
            frames[-1] = ("code", depth + 1)
            index += 1
            continue
        if depth and char == "}":
            depth -= 1
            if depth == 0:
                frames.pop()
            else:
                frames[-1] = ("code", depth)
            index += 1
            continue
        if text.startswith("//", index):
            frames.append(("line", None))
            index += 2
            continue
        if text.startswith("/*", index):
            frames.append(("block", None))
            index += 2
            continue
        if char == "'":
            frames.append(("squote", []))
            index += 1
            continue
        if char == '"':
            frames.append(("dquote", []))
            index += 1
            continue
        if char == "`":
            frames.append(("template", []))
            index += 1
            continue
        index += 1
    return found


def user_visible_string_literals(text):
    """UI copy a built bundle has to keep, such as an Italian label."""
    found = []
    seen = set()
    for value in js_string_literals(text):
        visible = any(char.isalpha() for char in value) and (
            any(char.isspace() for char in value) or any(ord(char) > 127 and char.isalpha() for char in value)
        )
        if visible and value not in seen:
            seen.add(value)
            found.append(value)
    return found


def required_label_manifest():
    """Hand-maintained admin scripts and the user-facing strings each one must contain."""
    try:
        data = json.loads(REQUIRED_LABELS.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        raise ReleaseError("required-labels.json is missing or invalid.") from None
    require(isinstance(data, dict) and data, "required-labels.json must map build files to labels.")
    cleaned = {}
    for name, labels in data.items():
        require(isinstance(name, str) and name.endswith(".js") and "/" not in name and name not in cleaned, "required-labels.json has an invalid file name.")
        require(isinstance(labels, list) and labels and all(isinstance(item, str) and item for item in labels), "required-labels.json labels for " + name + " must be strings.")
        cleaned[name] = list(labels)
    return cleaned


def required_label_script(labels):
    return "".join("void %s;\n" % json.dumps(label, ensure_ascii=False) for label in labels)


def _built_admin_files(build_dir):
    files = {}
    if not build_dir.is_dir():
        return files
    for path in sorted(build_dir.iterdir()):
        if not path.is_file():
            continue
        if path.suffix == ".js" or path.name.endswith(".js.LEGAL.txt"):
            files[path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
    return files


def _npm(build_dir, args):
    try:
        result = subprocess.run(args, cwd=build_dir, capture_output=True, text=True, timeout=180)
    except (OSError, subprocess.TimeoutExpired):
        raise ReleaseError("Admin JS rebuild could not run; check node, npm, and the admin/build toolchain.") from None
    if result.returncode == 0:
        return
    if list(args[:2]) == ["npm", "ci"]:
        lines = (result.stderr or "").splitlines()
        tail = "\n".join(lines[-NPM_CI_STDERR_TAIL:])
        raise ReleaseError("npm ci failed:\n" + (tail or "npm ci produced no stderr."))
    raise ReleaseError("Admin JS rebuild failed; inspect npm locally. Command output was withheld.")


def _fresh_admin_hashes(plugin_dir):
    package_path = plugin_dir / "admin" / "build" / "package.json"
    if not package_path.is_file():
        return None
    try:
        manifest = json.loads(package_path.read_text())
    except ValueError:
        return None
    if "build" not in manifest.get("scripts", {}):
        return None
    with tempfile.TemporaryDirectory() as directory:
        copy = Path(directory) / "affilio-wp"
        shutil.copytree(plugin_dir, copy, ignore=shutil.ignore_patterns("node_modules"))
        build_dir = copy / "admin" / "build"
        _npm(build_dir, ["npm", "ci", "--ignore-scripts"])
        _npm(build_dir, ["npm", "run", "build"])
        first = _built_admin_files(build_dir)
        _npm(build_dir, ["npm", "run", "build"])
        second = _built_admin_files(build_dir)
    if not first or first != second:
        return None
    return first


def _require_same_file_literals(literals, bundle, source_name):
    require(isinstance(bundle, str), "Built admin JS is missing admin/build/" + source_name)
    present = set(js_string_literals(bundle))
    for literal in literals:
        require(literal in present, "Built admin JS is missing %r from admin/build/%s" % (literal, source_name))


def _require_admin_src_strings(plugin_dir):
    src = plugin_dir / "admin" / "src"
    require(src.is_dir(), "Admin JS rebuild is not deterministic and admin/src is missing.")
    sources = sorted(path for path in src.rglob("*.js") if path.is_file())
    require(sources, "Admin JS rebuild is not deterministic and admin/src has no scripts.")
    bundles = {}
    build_dir = plugin_dir / "admin" / "build"
    if build_dir.is_dir():
        for path in sorted(build_dir.glob("*.js")):
            bundles[path.name] = path.read_text(encoding="utf-8", errors="replace")
    for source in sources:
        literals = user_visible_string_literals(source.read_text(encoding="utf-8", errors="replace"))
        _require_same_file_literals(literals, bundles.get(source.name), source.name)


def _admin_build_scripts(files):
    prefix = PLUGIN_ROOT + "admin/build/"
    scripts = {}
    for name, payload in files.items():
        if not name.startswith(prefix):
            continue
        relative = name[len(prefix):]
        if "/" in relative or not relative.endswith(".js"):
            continue
        scripts[relative] = payload.decode("utf-8", "replace")
    return scripts


def _require_zip_admin_labels(files):
    """Require each hand-maintained build file in this ZIP to contain its own labels."""
    scripts = _admin_build_scripts(files)
    for name, labels in required_label_manifest().items():
        require(name in scripts, "WordPress ZIP is missing admin/build/" + name)
        present = set(js_string_literals(scripts[name]))
        for label in labels:
            require(label in present, "admin/build/%s is missing the label %r" % (name, label))


def _require_zip_admin_src_strings(files):
    prefix = PLUGIN_ROOT + "admin/src/"
    sources = sorted(name for name in files if name.startswith(prefix) and name.endswith(".js"))
    if not sources:
        return
    bundles = _admin_build_scripts(files)
    for name in sources:
        source_name = PurePosixPath(name).name
        literals = user_visible_string_literals(files[name].decode("utf-8", "replace"))
        _require_same_file_literals(literals, bundles.get(source_name), source_name)


def validate_built_admin_js(plugin_dir):
    """Fail when admin/build scripts do not match a fresh rebuild of the plugin."""
    global _canonical_admin_js_ok
    plugin_dir = Path(plugin_dir).resolve()
    canonical = (ROOT / "affilio-wp").resolve()
    if plugin_dir == canonical and _canonical_admin_js_ok:
        return
    require((plugin_dir / "admin" / "build").is_dir(), "WordPress plugin is missing admin/build.")
    fresh = _fresh_admin_hashes(plugin_dir)
    if fresh is None:
        _require_admin_src_strings(plugin_dir)
    else:
        committed = _built_admin_files(plugin_dir / "admin" / "build")
        for name, digest in sorted(fresh.items()):
            require(committed.get(name) == digest, "Built admin JS does not match its source: admin/build/" + name)
        for name in sorted(set(committed) - set(fresh)):
            require(False, "Fresh admin build dropped admin/build/" + name)
        if (plugin_dir / "admin" / "src").is_dir():
            _require_admin_src_strings(plugin_dir)
    if plugin_dir == canonical:
        _canonical_admin_js_ok = True


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
        for name in names:
            require(not packaged_out(name), "WordPress ZIP includes tests or test results: " + name)
        for name, payload in files.items():
            require(name.startswith(PLUGIN_ROOT), "WordPress ZIP top folder must be affilio-wp/")
            require(b"railway.app" not in payload, "WordPress ZIP must not contain railway.app")
            if name.startswith(PLUGIN_ROOT + "admin/build/") and hosted_build_file(PurePosixPath(name).name):
                raise ReleaseError("WordPress ZIP includes hosted UI or build tools.")
        plugin = files[PLUGIN_ROOT + "affilio.php"].decode("utf8")
        require("Plugin Name:" in plugin, "WordPress plugin header is missing.")
        php = {name: payload.decode("utf8", "replace") for name, payload in files.items() if name.endswith(".php")}
        for name, text in php.items():
            for seam in TEST_ONLY_PLUGIN_SEAMS:
                require(seam not in text, "WordPress ZIP ships a test-only seam " + seam + " in " + name)
        require(not any(unguarded_init(text) for text in php.values()), "WordPress ZIP has an unguarded function affilio_init")
        require(f"Version: {PLUGIN_VERSION}" in plugin, f"WordPress plugin header is not {PLUGIN_VERSION}")
        require(f"define('AFFILIO_VERSION', '{PLUGIN_VERSION}')" in plugin, f"AFFILIO_VERSION is not {PLUGIN_VERSION}")
        require(f"Stable tag: {PLUGIN_VERSION}".encode() in files.get(PLUGIN_ROOT + "readme.txt", b""), f"readme stable tag is not {PLUGIN_VERSION}")
        for name, payload in files.items():
            if PurePosixPath(name).suffix.lower() not in {".php", ".js", ".mjs"}:
                continue
            needed, outside = referenced_plugin_files(name, payload.decode("utf8", "replace"))
            if outside:
                raise ReleaseError("WordPress ZIP references a path outside the plugin: " + outside[0])
            for relative in sorted(needed):
                require(PLUGIN_ROOT + relative in files, "WordPress ZIP is missing " + relative)
        _require_zip_admin_labels(files)
        _require_zip_admin_src_strings(files)


def validate_installer(data):
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        members = archive.getmembers()
        archive_names([member.name for member in members])
        require(all(member.isfile() or member.isdir() for member in members), "Installer includes non-regular files.")
        require(sum(member.size for member in members) <= MAX_DOWNLOAD, "Installer expands beyond the release size limit.")
        require(SKILLS | {"package/bin/setup.mjs", "package/lib/setup.mjs", "package/package.json"} <= set(archive.getnames()), "Installer is missing its command or skills.")
        package = json.load(archive.extractfile("package/package.json"))
        bins = package.get("bin") or {}
        require(
            package.get("name") == "offer-rails"
            and package.get("version") == "0.1.0"
            and bins.get("offer-rails") == "bin/setup.mjs"
            and bins.get("affilio") == "bin/setup.mjs"
            and bins.get("distributoor") == "bin/setup.mjs"
            and bins.get("distributoor-setup") == "bin/setup.mjs",
            "Installer package metadata is incorrect.",
        )


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
        if name not in TEMPLATE_ASSETS:
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
