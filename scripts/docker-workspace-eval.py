"""Exercise real Linux workspace ownership without a daemon or user-profile mount.

Run once the local Docker engine is available. Every fixture is a disposable,
non-privileged, network-isolated container with only temporary in-memory paths
and a read-only mount of the shipped initialization script. No host socket,
credentials, account or project volume is mounted.
"""
from __future__ import annotations

import json
from pathlib import Path
import subprocess
import uuid

REPO = Path(__file__).resolve().parent.parent
IMAGE = "docker:28.5.1-dind@sha256:ea9d20492ca1caaaba78e68453433895d256173c79281756e88b745647fcbcfd"
LABEL = "ai.lcfr.luci.eval=workspace-identity"


def docker(*arguments: str, timeout: int = 45, required: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(["docker", *arguments], capture_output=True, text=True, timeout=timeout)
    if required and result.returncode:
        # Docker diagnostics can include private host paths. Preserve only the
        # operation and exit status in this evaluator's public receipt.
        raise AssertionError(f"Docker {arguments[0]} exited with status {result.returncode}")
    return result


def fixture(script: str, action=None) -> None:
    name = "luci-workspace-eval-" + uuid.uuid4().hex
    try:
        docker("run", "--detach", "--name", name, "--label", LABEL,
               "--platform", "linux/amd64", "--network", "none", "--read-only",
               "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "DAC_OVERRIDE", "--cap-add", "FOWNER",
               "--security-opt", "no-new-privileges", "--memory", "64m", "--cpus", "0.5", "--pids-limit", "32",
               "--tmpfs", "/luci-workspace:rw,nosuid,nodev,size=8m,mode=0755",
               "--tmpfs", "/run:rw,nosuid,nodev,size=2m,mode=0755",
               "--tmpfs", "/tmp:rw,nosuid,nodev,size=2m,mode=1777",
               "--mount", f"type=bind,src={REPO / 'distribution/docker/workspace-init.sh'},dst=/source/workspace-init.sh,readonly",
               "--entrypoint", "sh", IMAGE, "-c", "exec sleep 300", timeout=120)
        docker("exec", name, "sh", "-eu", "-c", script)
        if action:
            action(name)
    finally:
        # Only remove the exact random fixture if its ownership label still
        # matches. Never enumerate or clean other containers or any volumes.
        metadata = docker("inspect", "--format", '{{index .Config.Labels "ai.lcfr.luci.eval"}}', name, required=False)
        if metadata.returncode == 0 and metadata.stdout.strip() == "workspace-identity":
            docker("rm", "--force", name)


def nonroot_writes_and_retain(name: str) -> None:
    docker("exec", "--user", "1001:1001", name, "sh", "-eu", "-c", """
test "$(id -u):$(id -g)" = 1001:1001
umask 077
printf 'retained-profile' > /luci-workspace/home/profile-test
mkdir /luci-workspace/projects/application
printf 'agent-readable' > /luci-workspace/projects/application/source-test
test "$(stat -c '%u:%g:%a' /luci-workspace/home/profile-test)" = 1001:1001:600
""")
    docker("exec", name, "sh", "-eu", "-c", """
printf 'foreign-child-preserved' > /luci-workspace/projects/foreign-child
chown 2222:2222 /luci-workspace/projects/foreign-child
chmod 0640 /luci-workspace/projects/foreign-child
before=$(sha256sum /luci-workspace/home/profile-test /luci-workspace/projects/application/source-test /luci-workspace/projects/foreign-child)
sh /source/workspace-init.sh
test "$(sha256sum /luci-workspace/home/profile-test /luci-workspace/projects/application/source-test /luci-workspace/projects/foreign-child)" = "$before"
test "$(stat -c '%u:%g:%a' /luci-workspace/projects/foreign-child)" = 2222:2222:640
""")
    docker("exec", "--user", "1001:1001", name, "sh", "-eu", "-c", """
test "$(cat /luci-workspace/projects/application/source-test)" = agent-readable
if sh /source/workspace-init.sh >/dev/null 2>&1; then exit 1; fi
test "$(cat /luci-workspace/home/profile-test)" = retained-profile
""")


def main() -> None:
    engine = docker("info", "--format", "{{.OSType}}", timeout=15)
    if engine.stdout.strip() != "linux":
        raise AssertionError("Use a ready Linux Docker engine for the Linux identity outcome checks")
    checks = []
    fixture("""
sh /source/workspace-init.sh
for directory in /luci-workspace /luci-workspace/home /luci-workspace/state /luci-workspace/projects /luci-workspace/tmp; do
  test "$(stat -c '%u:%g:%a' "$directory")" = 1001:1001:700
done
test "$(stat -c '%u:%g:%a' /run/luci-engine)" = 0:1001:750
""", nonroot_writes_and_retain)
    checks.append("fresh workspace permits real UID 1001 profile/project writes; repeated initialization preserves retained bytes, modes and foreign child ownership")
    fixture("""
printf 'root-retained' > /luci-workspace/sentinel
chmod 0600 /luci-workspace/sentinel
before=$(sha256sum /luci-workspace/sentinel)
if sh /source/workspace-init.sh >/dev/null 2>&1; then exit 1; fi
test "$(sha256sum /luci-workspace/sentinel)" = "$before"
test "$(stat -c '%u:%g:%a' /luci-workspace/sentinel)" = 0:0:600
test ! -e /luci-workspace/home
""")
    checks.append("populated root-owned workspace is refused without changing retained files or creating directories")
    fixture("""
chown 1001:1001 /luci-workspace
chmod 0700 /luci-workspace
mkdir /tmp/linked-target
printf 'outside-preserved' > /tmp/linked-target/sentinel
before=$(sha256sum /tmp/linked-target/sentinel)
ln -s /tmp/linked-target /luci-workspace/state
if sh /source/workspace-init.sh >/dev/null 2>&1; then exit 1; fi
test -L /luci-workspace/state
test "$(stat -c '%u:%g' /tmp/linked-target)" = 0:0
test "$(sha256sum /tmp/linked-target/sentinel)" = "$before"
test ! -e /luci-workspace/home
""")
    checks.append("linked retained directory is refused before any initialization, and its target is untouched")
    fixture("""
chown 1001:1001 /luci-workspace
chmod 0700 /luci-workspace
mkdir -m 0700 /luci-workspace/projects
printf 'foreign-owner-preserved' > /luci-workspace/projects/sentinel
before=$(sha256sum /luci-workspace/projects/sentinel)
if sh /source/workspace-init.sh >/dev/null 2>&1; then exit 1; fi
test "$(sha256sum /luci-workspace/projects/sentinel)" = "$before"
test "$(stat -c '%u:%g:%a' /luci-workspace/projects)" = 0:0:700
test ! -e /luci-workspace/home
""")
    checks.append("foreign-owned retained directory is refused without rewriting ownership or earlier missing directories")
    fixture("""
chown 1001:1001 /luci-workspace
chmod 0700 /luci-workspace
mkdir -m 0755 /luci-workspace/state
chown 1001:1001 /luci-workspace/state
if sh /source/workspace-init.sh >/dev/null 2>&1; then exit 1; fi
test "$(stat -c '%u:%g:%a' /luci-workspace/state)" = 1001:1001:755
test ! -e /luci-workspace/home
""")
    checks.append("incompatible retained directory permissions are refused and preserved")
    print(json.dumps({"passed": True, "image": IMAGE, "checks": checks, "limits": "Initializer outcomes only; no daemon, model sign-in or application deployment was exercised."}))


if __name__ == "__main__":
    main()
