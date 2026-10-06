#!/usr/bin/env python3
"""Exercise managed teams through the actual main binary; never print credentials."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import signal
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--binary", default=os.environ.get("LUCI_TEAM_SMOKE_BINARY"),
                    help="Compiled Luci main binary (or LUCI_TEAM_SMOKE_BINARY).")
parser.add_argument("--output", type=Path, default=ROOT / "artifacts/team-main-smoke.json")
parser.add_argument("--fixture-parent", type=Path, default=None,
                    help="Parent for a newly generated private fixture; never reuses existing state.")
parser.add_argument("--keep-fixture", action="store_true",
                    help="Retain private logs/state for diagnosis; contains temporary test credentials.")
parser.add_argument("--docker-host", default=None,
                    help="Optional local Docker endpoint. This evaluator creates no containers.")
args = parser.parse_args()
if not args.binary:
    parser.error("--binary is required; build the main service first")
BINARY = str(Path(args.binary).expanduser().resolve())
if not Path(BINARY).is_file():
    parser.error("--binary must be an existing compiled main service")
if args.fixture_parent is not None and not args.fixture_parent.is_dir():
    parser.error("--fixture-parent must be an existing directory")
OUT = args.output.expanduser().resolve()
OUT.parent.mkdir(parents=True, exist_ok=True)
fixture = Path(tempfile.mkdtemp(prefix="luci-team-main-", dir=args.fixture_parent))
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
base = f"http://127.0.0.1:{port}"
# Inherit only process/toolchain necessities. Never inject the caller's model,
# GitHub, cloud, SSH-agent, managed binding or account credentials into fixtures.
keep_env = {"PATH", "SystemRoot", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT",
            "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE", "TZ"}
env = {key: value for key, value in os.environ.items() if key in keep_env}
env.update(LUCI_ADDR=f"127.0.0.1:{port}", LUCI_STATE_DIR=str(fixture / "state"),
           LUCI_PROJECTS_DIR=str(fixture / "projects"), LUCI_LOG_LEVEL="error",
           HOME=str(fixture / "home"), USERPROFILE=str(fixture / "home"),
           APPDATA=str(fixture / "home/appdata"), LOCALAPPDATA=str(fixture / "home/localappdata"),
           GIT_CONFIG_GLOBAL=str(fixture / "home/.gitconfig"), GIT_CONFIG_NOSYSTEM="1")
if args.docker_host:
    env["LUCI_DOCKER_HOST"] = args.docker_host
for name in ("home", "projects", "home/appdata", "home/localappdata"):
    (fixture / name).mkdir(mode=0o700, parents=True, exist_ok=True)
process = None
hasher = hashlib.sha256()
with open(BINARY, "rb") as binary_file:
    for chunk in iter(lambda: binary_file.read(1024 * 1024), b""):
        hasher.update(chunk)
results = {"binary": BINARY, "binarySHA256": hasher.hexdigest(), "checks": [],
           "externalModelCalls": 0, "cloudResourcesCreated": 0,
           "scope": "Actual main service and supervised team workers with generated accounts and context; no inference, container execution, remote host or cloud provisioning."}

def request(method, path, data=None, token=None, expect=200):
    headers = {"Origin": base, "Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(base + path, data=None if data is None else json.dumps(data).encode(), headers=headers, method=method)
    try:
        response = urllib.request.urlopen(req, timeout=35)
    except urllib.error.HTTPError as error:
        response = error
    body = response.read()
    try:
        decoded = json.loads(body) if body else {}
    except json.JSONDecodeError:
        decoded = {"unparseable": True}
    if response.code != expect:
        raise AssertionError(f"{method} {path}: expected {expect}, got {response.code}: {decoded.get('error', '')} {decoded.get('message', '')}")
    return decoded

def checked(name, condition=True):
    if not condition:
        raise AssertionError(name)
    results["checks"].append({"name": name, "passed": True})

def start():
    global process
    log = open(fixture / "service.log", "ab")
    options = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {"start_new_session": True}
    process = subprocess.Popen([BINARY], cwd=fixture, env=env, stdout=log, stderr=log, **options)
    log.close()
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise AssertionError(f"main service exited with {process.returncode}; use --keep-fixture to retain private startup logs")
        try:
            request("GET", "/v1/control-plane/status")
            return
        except (OSError, AssertionError):
            time.sleep(0.1)
    raise AssertionError("main service did not become ready")

def stop():
    global process
    if process is None:
        return
    process.send_signal(signal.CTRL_BREAK_EVENT if os.name == "nt" else signal.SIGTERM)
    try:
        process.wait(timeout=20)
    except subprocess.TimeoutExpired:
        if os.name == "nt":
            subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"], check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        else:
            os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=5)
        raise AssertionError("main service did not drain team workers")
    checked("main service drains owned workers", process.returncode == 0)
    process = None

try:
    start()
    owner_password = secrets.token_urlsafe(24)
    editor_password = secrets.token_urlsafe(24)
    bootstrap = (fixture / "state/control-plane/bootstrap-token").read_text().strip()
    owner = request("POST", "/v1/auth/bootstrap", {"bootstrapToken": bootstrap, "username": "teamowner", "displayName": "Team Owner", "password": owner_password}, expect=201)
    owner_token = owner["token"]
    editor_user = request("POST", "/v1/admin/users", {"username": "teameditor", "displayName": "Team Editor", "password": editor_password}, owner_token, 201)
    editor = request("POST", "/v1/auth/login", {"username": "teameditor", "password": editor_password})
    editor_token = editor["token"]
    team = request("POST", "/v1/teams", {"name": "Production acceptance team"}, owner_token, 201)
    team_id = team["team"]["id"]
    team_path = "/v1/teams/" + team_id
    workspace = team_path + "/workspace"
    grants = {"execute": True, "secrets": False, "deploy": False, "admin": False}
    team = request("POST", team_path + "/members", {"expectedRevision": team["team"]["revision"], "username": "teameditor", "role": "editor", "grants": grants}, owner_token)
    project = request("POST", workspace + "/v1/product/projects", {"name": "shared-context"}, owner_token, 201)
    project_id = project["id"]
    checked("project resides in dedicated team runtime", (fixture / "state/control-plane/teams" / team_id / "runtime/projects").resolve() in Path(project["path"]).resolve().parents)
    checked("editor catalogue hides project before explicit grant", request("GET", workspace + "/v1/projects", token=editor_token)["projects"] == [])
    brief_path = workspace + "/v1/product/projects/" + project_id + "/brief"
    request("GET", brief_path, token=editor_token, expect=403)
    checked("opaque project route denied before project grant")
    team = request("PUT", team_path + "/projects/" + project_id + "/members/" + editor_user["id"], {"expectedRevision": team["team"]["revision"], "role": "editor", "grants": grants}, owner_token)
    owner_projects = request("GET", workspace + "/v1/projects", token=owner_token)["projects"]
    editor_projects = request("GET", workspace + "/v1/projects", token=editor_token)["projects"]
    checked("two accounts observe identical shared project", [p["id"] for p in owner_projects] == [p["id"] for p in editor_projects] == [project_id])
    draft = {"revision": 0, "purpose": "One durable collaborative context", "audience": "Owner and editor", "constraints": "Preserve explicit access and actor identity", "criteria": ["Both collaborators read the same reviewed context"]}
    brief = request("POST", brief_path, draft, editor_token)
    checked("editor saved shared context", brief["revision"] == 1)
    checked("owner reads editor context", request("GET", brief_path, token=owner_token) == brief)
    request("POST", brief_path, draft, owner_token, 409)
    checked("stale concurrent revision rejected")
    draft["revision"] = 1
    draft["constraints"] += "; second attributed owner revision"
    brief = request("POST", brief_path, draft, owner_token)
    history_path = workspace + "/v1/product/projects/" + project_id + "/context-history"
    events = request("GET", history_path, token=editor_token)["events"]
    checked("shared context retains both real human authors", {e["actor"]["id"] for e in events} == {owner["user"]["id"], editor_user["id"]} and all(e["status"] == "applied" for e in events))
    for label, token in (("owner", owner_token), ("editor", editor_token)):
        catalogue = request("GET", "/v1/projects", token=token)["projects"]
        checked(label + " personal catalogue excludes team project", project_id not in [p["id"] for p in catalogue])
    stop()
    start()
    checked("owner session and team catalogue survive restart", any(row["team"]["id"] == team_id for row in request("GET", "/v1/teams", token=owner_token)["teams"]))
    checked("editor reads identical context after main and worker restart", request("GET", brief_path, token=editor_token) == brief)
    checked("private authorship receipts survive restart", request("GET", history_path, token=owner_token)["events"] == events)
    team = request("GET", team_path, token=owner_token)
    request("DELETE", team_path + "/members/" + editor_user["id"], {"expectedRevision": team["team"]["revision"]}, owner_token)
    request("GET", brief_path, token=editor_token, expect=403)
    checked("removed collaborator loses shared project immediately")
    checked("removing collaborator preserves shared context", request("GET", brief_path, token=owner_token) == brief)
    stop()
    results["passed"] = True
except Exception as error:
    results["passed"] = False
    results["failure"] = str(error)
finally:
    if process is not None:
        try:
            stop()
        except Exception as error:
            results["cleanupFailure"] = str(error)
            results["passed"] = False
    if args.keep_fixture:
        results["privateFixture"] = str(fixture)
    else:
        shutil.rmtree(fixture)
    results["report"] = str(OUT)
    OUT.write_text(json.dumps(results, indent=2) + "\n")
    print(json.dumps(results, indent=2))

raise SystemExit(0 if results.get("passed") else 1)
