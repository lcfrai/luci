#!/usr/bin/env python3
"""Explicit release-producer build. Never used by ordinary installed setup.

No registry push, credentials, global settings or broad cleanup. Windows cache
assistance is opt-in, bounded and allowed only while every running container is
positively owned by this producer session. Failed/ambiguous work keeps receipts.
"""
from __future__ import annotations

import argparse
import ctypes
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import time
import uuid

import archive
from canonical_inputs import stage
from common import plain_directory, read_json, schema_validate, write_json

SESSION_LABEL = "luci.runtime-payload.producer-session"
RESERVE = 1 << 30
MARGIN = 512 << 20
BUILD_MEMORY = 1 << 30
CACHE_TRIGGER = RESERVE + BUILD_MEMORY + MARGIN
CACHE_INTERVAL_SECONDS = 5
MAX_CACHE_ACTIONS = 36


def command(args, **kwargs):
    result = subprocess.run(args, check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=kwargs.pop("timeout", 15), **kwargs)
    if result.returncode:
        raise ValueError("producer command failed: " + args[0] + " " + args[1] + " (exit " + str(result.returncode) + ")")
    return result.stdout


def clean_source(repo, revision):
    if not re.fullmatch(r"[a-f0-9]{40}", revision) or command(["git", "-C", str(repo), "rev-parse", "HEAD"]).strip().decode("ascii") != revision or command(["git", "-C", str(repo), "status", "--porcelain=v1", "--untracked-files=all"]).strip():
        raise ValueError("producer requires the full clean selected HEAD")


def available_memory():
    if os.name == "nt":
        class Status(ctypes.Structure):
            _fields_ = [("length", ctypes.c_ulong), ("load", ctypes.c_ulong)] + [(name, ctypes.c_ulonglong) for name in ("total", "available", "commit", "commitAvailable", "virtual", "virtualAvailable", "extended")]
        value = Status()
        value.length = ctypes.sizeof(value)
        if not ctypes.windll.kernel32.GlobalMemoryStatusEx(ctypes.byref(value)):
            raise ValueError("physical memory observation unavailable")
        return value.available
    fields = dict(line.split(":", 1) for line in Path("/proc/meminfo").read_text(encoding="ascii").splitlines())
    return int(fields["MemAvailable"].split()[0]) * 1024


def cpu_sample():
    if os.name == 'nt':
        idle, kernel, user = ctypes.c_ulonglong(), ctypes.c_ulonglong(), ctypes.c_ulonglong()
        if not ctypes.windll.kernel32.GetSystemTimes(ctypes.byref(idle), ctypes.byref(kernel), ctypes.byref(user)):
            raise ValueError('host CPU observation unavailable')
        return idle.value, kernel.value + user.value
    values = [int(value) for value in Path('/proc/stat').read_text(encoding='ascii').splitlines()[0].split()[1:]]
    return values[3] + values[4], sum(values[:8])


def cpu_busy():
    first = cpu_sample()
    time.sleep(1)
    second = cpu_sample()
    if second[1] <= first[1]:
        raise ValueError('host CPU observation did not advance')
    return 100 * (1 - (second[0] - first[0]) / (second[1] - first[1]))


def docker_ids(running=True):
    return command(["docker", "ps", "-q" if running else "-aq"]).decode("ascii").split()


def owned_running(session, reject_unrelated=True):
    rows, unrelated = [], []
    for identifier in docker_ids():
        row = json.loads(command(["docker", "inspect", identifier]))[0]
        if row["Config"].get("Labels", {}).get(SESSION_LABEL) != session:
            unrelated.append(row["Id"])
            continue
        rows.append({"id": row["Id"], "memory": row["HostConfig"]["Memory"], "memorySwap": row["HostConfig"]["MemorySwap"],
                     "cpuPeriod": row["HostConfig"]["CpuPeriod"], "cpuQuota": row["HostConfig"]["CpuQuota"], "nanoCPUs": row["HostConfig"].get('NanoCpus', 0), "oomKilled": row["State"]["OOMKilled"]})
    if unrelated and reject_unrelated:
        raise ValueError("unrelated running Docker workload; producer cache assistance refused")
    return rows


def assert_settled(session):
    # A successful foreground Docker command and this exact inventory together
    # establish cleanup; missing containers alone never settle a lost builder.
    identifiers = command(['docker', 'ps', '-aq', '--filter', 'label=' + SESSION_LABEL + '=' + session]).decode('ascii').split()
    if identifiers:
        raise ValueError('exact producer containers retained after terminal command: ' + ','.join(identifiers))


def cache_assist(session, output):
    baseline = read_json(output / 'producer-engine.json')
    current = json.loads(command(['docker', 'info', '--format', '{{json .}}']))
    if os.name != 'nt' or current['ID'] != baseline['engineID'] or 'Docker Desktop' not in current.get('OperatingSystem', ''):
        raise ValueError('producer cache assistance requires the same exact Docker Desktop engine')
    owned_running(session)  # Any unrelated live work refuses mutation.
    started, available_before = time.monotonic(), available_memory()
    command(['wsl.exe', '-d', 'docker-desktop', '--', 'sh', '-c', 'sync; echo 3 > /proc/sys/vm/drop_caches'], timeout=10)
    return {'elapsedSeconds': time.monotonic() - started, 'availableBefore': available_before,
            'availableAfter': available_memory(), 'exitCode': 0, 'engineID': current['ID']}


def context_tar(canonical, destination, bases, session):
    dockerfile = (canonical / "images/sandbox/Dockerfile").read_bytes().decode("utf-8")
    lines, found = [], []
    for line in dockerfile.splitlines():
        if line.startswith("FROM "):
            parts = line.split()
            base = next((x for x in bases if x["reference"] == parts[1]), None)
            if base is None:
                raise ValueError("unexpected producer base image")
            found.append(parts[1])
            parts[1] = base["imageID"]
            lines.extend([" ".join(parts), "LABEL " + SESSION_LABEL + '="' + session + '"'])
        else:
            lines.append(line)
    if found != ["golang:1.26-bookworm", "node:24-bookworm-slim"]:
        raise ValueError("producer requires the exact two runtime stages")
    generated = ("\n".join(lines) + "\n").encode("utf-8")
    with tarfile.open(destination, "w", format=tarfile.USTAR_FORMAT) as output:
        for file in sorted(p for p in canonical.rglob("*") if p.is_file()):
            name = file.relative_to(canonical).as_posix()
            raw = generated if name == "images/sandbox/Dockerfile" else file.read_bytes()
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime = len(raw), 0o644, 0
            output.addfile(info, io.BytesIO(raw))
    return archive.file_record(destination)["sha256"]


def monitor_build(process, session, output, assistance):
    log, actions = output / "producer-cache-assistance.jsonl", 0
    last_action, cancelled = 0, False
    deadline, cancel_deadline = time.monotonic() + 30 * 60, None
    while process.poll() is None:
        free = available_memory()
        try:
            containers = owned_running(session)
        except (ValueError, subprocess.SubprocessError):
            # Foreign work disables cache assistance; only exact own labels
            # authorize cancellation. Never terminate a foreign container.
            containers = owned_running(session, reject_unrelated=False)
            assistance = False
            cancelled = True
        row = {"time": time.time(), "availableBytes": free, "reserveBytes": RESERVE, "containers": containers}
        if any(container['memory'] != BUILD_MEMORY or container['memorySwap'] != BUILD_MEMORY or container['cpuPeriod'] != 100000 or container['cpuQuota'] != 100000 for container in containers):
            cancelled = True
            row['limitsMismatch'] = True
        if assistance and free < CACHE_TRIGGER and actions < MAX_CACHE_ACTIONS and time.monotonic() - last_action >= CACHE_INTERVAL_SECONDS:
            if os.name != "nt":
                raise ValueError("WSL cache assistance is Windows producer-only")
            row['cacheResult'] = cache_assist(session, output)
            actions += 1
            last_action = time.monotonic()
            row["cacheAssistance"] = "exact-owned-engine transient caches; no persistent data removed"
        if assistance and free < CACHE_TRIGGER and actions >= MAX_CACHE_ACTIONS:
            row['cacheAssistanceExhausted'] = True
        if free < RESERVE or time.monotonic() > deadline or cancelled:
            for container in row["containers"]:
                try:
                    command(["docker", "kill", "--signal", "KILL", container["id"]])
                except (ValueError, subprocess.SubprocessError):
                    row["killAcknowledgment"] = "unknown; retain terminal stream"
            cancelled = True
            cancel_deadline = cancel_deadline or time.monotonic() + 20
            row["cancellationRequested"] = True
        with (output / "producer-resource-monitor.jsonl").open("a", encoding="utf-8") as stream:
            stream.write(json.dumps(row, sort_keys=True) + "\n")
        if "cacheAssistance" in row:
            with log.open("a", encoding="utf-8") as stream:
                stream.write(json.dumps(row, sort_keys=True) + "\n")
        if cancelled and time.monotonic() >= cancel_deadline:
            write_json(output / "producer-unknown.json", {"session": session, "reason": "No terminal builder response after exact owned cancellation; do not retry or infer completion from absence"})
            process.terminate()  # close only the owned client; daemon outcome stays unknown
            process.wait(timeout=5)
            raise ValueError("producer build outcome unknown; receipts and ownership retained")
        time.sleep(2)
    engine = json.loads(command(['docker', 'info', '--format', '{{json .}}']))
    if engine['ID'] != read_json(output / 'producer-engine.json')['engineID']:
        write_json(output / 'producer-unknown.json', {'session': session, 'reason': 'Builder terminal returned across engine identity change; retain ownership, no absence-only settlement'})
        raise ValueError('producer engine changed before terminal verification')
    assert_settled(session)
    write_json(output / 'producer-terminal.json', {'session': session, 'builderExitCode': process.returncode, 'cancelRequested': cancelled, 'exactOwnedContainersRemaining': [], 'engineID': engine['ID'], 'terminalStreamReturned': True})
    if cancelled or process.returncode:
        raise ValueError("producer build cancelled/failed; inspect retained logs")
    return actions


def prepare_effect(session, output, demand, assistance):
    """A producer-only recovery before the next bounded probe/export."""
    required = RESERVE + demand + MARGIN
    if available_memory() >= required:
        return 0
    if not assistance or os.name != "nt" or owned_running(session):
        raise ValueError("producer next effect lacks normal reserve and margin")
    cache_result = cache_assist(session, output)
    with (output / "producer-cache-assistance.jsonl").open("a", encoding="utf-8") as stream:
        stream.write(json.dumps({"time": time.time(), "phase": "between producer effects", "demandBytes": demand, "scope": "idle exact producer engine transient cache", 'cacheResult': cache_result}, sort_keys=True) + "\n")
    deadline = time.monotonic() + 15
    while available_memory() < required and time.monotonic() < deadline:
        time.sleep(1)
    if available_memory() < required:
        raise ValueError("producer cache assistance did not restore required reserve/margin")
    return 1


def scoped_run(args, session, output, stdout_path, stderr_path, memory, timeout, script=None):
    """Observe the complete short-lived producer container, including cleanup."""
    with stdout_path.open('wb') as stdout, stderr_path.open('wb') as stderr:
        incoming = None
        if script is not None:
            if len(script) > 4 << 20:
                raise ValueError('source-controlled licence collector input exceeds bound')
            source = output / 'license-collector-input.py'
            source.write_bytes(script)
            incoming = source.open('rb')
        try:
            process = subprocess.Popen(args, stdin=incoming if incoming is not None else subprocess.DEVNULL, stdout=stdout, stderr=stderr)
        finally:
            if incoming is not None:
                incoming.close()
        deadline = time.monotonic() + timeout
        failure = None
        while process.poll() is None:
            try:
                rows = owned_running(session)
                free = available_memory()
                if free < RESERVE or time.monotonic() > deadline or any(row['memory'] != memory or row['memorySwap'] != memory or not (row['nanoCPUs'] == 1000000000 or row['cpuQuota'] == row['cpuPeriod'] and row['cpuQuota'] > 0) for row in rows):
                    failure = 'version/licence effect exceeded reserve, deadline or enforced limits'
                if stdout_path.stat().st_size > 65 << 20 or stderr_path.stat().st_size > 1 << 20:
                    failure = 'producer effect output bound exceeded'
                with (output / 'producer-resource-monitor.jsonl').open('a', encoding='utf-8') as log:
                    log.write(json.dumps({'time': time.time(), 'phase': 'bounded version/licence container', 'availableBytes': free, 'containers': rows}, sort_keys=True) + '\n')
            except (ValueError, subprocess.SubprocessError):
                failure = 'producer effect inventory unavailable or foreign work appeared'
            if failure:
                for row in owned_running(session, reject_unrelated=False):
                    command(['docker', 'kill', '--signal', 'KILL', row['id']])
                try:
                    process.wait(timeout=20)
                except subprocess.TimeoutExpired:
                    write_json(output / 'producer-unknown.json', {'session': session, 'reason': failure + '; exact container terminal response missing, no retry'})
                    process.terminate()
                    process.wait(timeout=5)
                raise ValueError(failure)
            time.sleep(0.5)
    assert_settled(session)
    return process.returncode


def probe(image_id, executable, version, tool, session, output):
    argv = [executable, "--version"]
    name = "luci-producer-" + session + "-" + tool.lower()
    args = ["docker", "run", "--rm", "--name", name, "--network", "none", "--memory", str(512 << 20), "--memory-swap", str(512 << 20), "--cpus", "1", "--label", SESSION_LABEL + "=" + session,
            "--entrypoint", executable, image_id, "--version"]
    stdout_path, stderr_path = output / (tool + ".out.log"), output / (tool + ".err.log")
    exit_code = scoped_run(args, session, output, stdout_path, stderr_path, 512 << 20, 20)
    actual_stdout = stdout_path.read_text(encoding='utf-8', errors='strict') if stdout_path.stat().st_size <= 1 << 20 else ''
    if exit_code or stdout_path.stat().st_size + stderr_path.stat().st_size > 1 << 20 or not re.search(r'(?<![0-9.])' + re.escape(version) + r'(?![0-9.])', actual_stdout) or tool == 'sandboxd' and actual_stdout != 'sandboxd ' + version + '\n':
        raise ValueError("actual runtime version probe failed: " + tool)
    return {"tool": tool, "argv": argv, "reportedVersion": version, "exitCode": exit_code,
            "stdoutSHA256": archive.file_record(stdout_path)["sha256"], "stderrSHA256": archive.file_record(stderr_path)["sha256"]}


def export_image(image_id, session, output, assistance):
    args = ["docker", "image", "save", "-o", str(output / "docker-save.raw.tar"), image_id]
    actions, last_action, deadline = 0, 0, time.monotonic() + 300
    with (output / "export.out.log").open("wb") as stdout, (output / "export.err.log").open("wb") as stderr:
        process = subprocess.Popen(args, stdout=stdout, stderr=stderr)
        try:
            while process.poll() is None:
                free = available_memory()
                if owned_running(session):
                    raise ValueError("producer export unexpectedly overlaps running containers")
                row = {"time": time.time(), "phase": "read-only image export", "availableBytes": free, "reserveBytes": RESERVE}
                if assistance and os.name == "nt" and free < CACHE_TRIGGER and actions < MAX_CACHE_ACTIONS and time.monotonic() - last_action >= CACHE_INTERVAL_SECONDS:
                    row['cacheResult'] = cache_assist(session, output)
                    actions += 1
                    last_action = time.monotonic()
                    row["cacheAssistance"] = "idle owned producer engine during read-only export"
                    with (output / "producer-cache-assistance.jsonl").open("a", encoding="utf-8") as stream:
                        stream.write(json.dumps(row, sort_keys=True) + "\n")
                if assistance and free < CACHE_TRIGGER and actions >= MAX_CACHE_ACTIONS:
                    row['cacheAssistanceExhausted'] = True
                with (output / "producer-resource-monitor.jsonl").open("a", encoding="utf-8") as stream:
                    stream.write(json.dumps(row, sort_keys=True) + "\n")
                if free < RESERVE or time.monotonic() > deadline:
                    raise ValueError("read-only producer export breached reserve or time budget")
                time.sleep(2)
        finally:
            if process.poll() is None:
                process.terminate()  # read-only export client, no image publication/mutation
                process.wait(timeout=5)
    if process.returncode:
        raise ValueError("producer image export failed; partial owned archive retained")
    return actions


def collect_licenses(repo, image_id, session, output):
    target = output / "licenses.tar"
    from grant_fallbacks import load
    script = ('SOURCE_GRANT_FALLBACKS = ' + repr(load(repo)) + '\n').encode('utf-8') + Path(__file__).with_name("image_licenses.py").read_bytes()
    args = ["docker", "run", "--rm", "--network", "none", "--memory", str(256 << 20), "--memory-swap", str(256 << 20), "--cpus", "1", "--label", SESSION_LABEL + "=" + session, "-i", "--entrypoint", "python3", image_id, "-"]
    exit_code = scoped_run(args, session, output, target, output / 'licenses.err.log', 256 << 20, 90, script)
    if exit_code or target.stat().st_size > 65 << 20:
        raise ValueError("runtime licence collection failed; inspect licenses.err.log")
    with tarfile.open(target, "r:") as source:
        members = archive.scan(source, normalized=True)
        _, rows = archive.read_metadata(source, members, "runtime-license-inventory.json")
        if set(members) != {"runtime-license-inventory.json", *(row["path"] for row in rows)}:
            raise ValueError("runtime licence inventory differs from tar")
        for row in rows:
            if not row["path"].startswith("licenses/") or row["bytes"] > 16 << 20:
                raise ValueError("runtime licence path/bound mismatch")
            with source.extractfile(members[row["path"]]) as stream:
                data = stream.read((16 << 20) + 1)
            if len(data) != row["bytes"] or hashlib.sha256(data).hexdigest() != row["sha256"]:
                raise ValueError("runtime licence byte mismatch")
            destination = output / row["path"]
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(data)
    # The sandbox binary is statically linked. Its vendored grants are source
    # inputs, not necessarily retained as files in the final Linux image.
    canonical = output / 'canonical/sandboxd/vendor'
    modules = (canonical / 'modules.txt').read_text(encoding='utf-8')
    for component, version in re.findall(r'^# (\S+) (v\S+)$', modules, re.M):
        directory = canonical / component
        notices = [file for file in directory.iterdir() if file.is_file() and re.match(r'^(licen[cs]e|copying|notice|copyright)([._-].*)?$', file.name, re.I)]
        if not notices:
            raise ValueError('vendored sandbox component lacks retained grant: ' + component)
        for file in sorted(notices):
            archive.regular(file)
            name = 'licenses/runtime-image/go-' + re.sub(r'[^A-Za-z0-9._-]', '_', component + '-' + version) + '/' + file.name
            destination = output / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(file.read_bytes())
            record = archive.file_record(destination, name)
            if not 0 < record['bytes'] <= 16 << 20:
                raise ValueError('vendored runtime grant exceeds bounds')
            rows.append(dict(record, component='go:' + component, version=version, grant='Upstream grant retained verbatim', provenance='Canonical sandboxd/vendor/' + component + '/' + file.name))
    if len(rows) > 4096 or sum(row['bytes'] for row in rows) > 64 << 20:
        raise ValueError('complete runtime grant inventory exceeds bounds')
    return sorted(rows, key=lambda row: row['path'])


def produce(repo, revision, output, assistance=False):
    repo, output = Path(repo).absolute(), Path(output).absolute()
    clean_source(repo, revision)
    plain_directory(output.parent)
    if output.exists() or output.is_relative_to(repo):
        raise ValueError("choose a fresh private producer output outside source")
    if docker_ids() or available_memory() < RESERVE + BUILD_MEMORY + MARGIN:
        raise ValueError("producer requires idle Docker and measured reserve plus build demand and 512 MiB margin")
    busy = cpu_busy()
    if (os.cpu_count() or 0) < 2 or busy > 90:
        raise ValueError('producer requires observed host CPU margin for one bounded CPU')
    # Raw export plus normalized output may coexist; retain a disk reserve.
    if shutil.disk_usage(output.parent).free < 2 * archive.MAX_ARCHIVE + (2 << 30):
        raise ValueError('producer output lacks raw+normalized archive disk budget')
    if os.name == 'nt' and shutil.disk_usage(os.environ.get('SystemDrive', 'C:') + '/').free < archive.MAX_ARCHIVE + (2 << 30):
        raise ValueError('Docker Desktop host storage lacks build disk budget')
    output.mkdir(mode=0o700)
    session = uuid.uuid4().hex
    engine = json.loads(command(['docker', 'info', '--format', '{{json .}}']))
    write_json(output / 'producer-engine.json', {'engineID': engine['ID'], 'operatingSystem': engine.get('OperatingSystem', ''), 'session': session, 'availableBytes': available_memory(), 'cpuBusyPercent': busy})
    inputs = stage(repo, revision, output / "canonical")
    write_json(output / "canonical-inputs.json", inputs)
    bases = []
    for reference in ("golang:1.26-bookworm", "node:24-bookworm-slim"):
        row = json.loads(command(["docker", "image", "inspect", reference]))[0]
        if row.get("Os") != "linux" or row.get("Architecture") != "amd64" or not row.get("RepoDigests"):
            raise ValueError("producer base requires an inspected Linux amd64 immutable repository digest")
        bases.append({"reference": reference, "imageID": row["Id"], "repoDigest": sorted(row["RepoDigests"])[0]})
    generated = context_tar(output / "canonical", output / "context.tar", bases, session)
    argv = ["docker", "build", "--memory", str(BUILD_MEMORY), "--memory-swap", str(BUILD_MEMORY), "--cpu-period", "100000", "--cpu-quota", "100000", "--rm", "--force-rm", "--iidfile", str(output / "image.id"), "-f", "images/sandbox/Dockerfile", "-"]
    write_json(output / "producer-pending.json", {"sourceCommit": revision, "canonicalInputDigest": inputs["canonicalInputDigest"], "session": session, "argv": argv, "effect": "builder pending; no automatic retry"})
    environment = dict(os.environ, DOCKER_BUILDKIT="0")
    with (output / "context.tar").open("rb") as stdin, (output / "build.out.log").open("wb") as stdout, (output / "build.err.log").open("wb") as stderr:
        process = subprocess.Popen(argv, stdin=stdin, stdout=stdout, stderr=stderr, env=environment)
        try:
            actions = monitor_build(process, session, output, assistance)
        except BaseException:
            if process.poll() is None:
                for row in owned_running(session, reject_unrelated=False):
                    try:
                        command(["docker", "kill", "--signal", "KILL", row["id"]])
                    except (ValueError, subprocess.SubprocessError):
                        pass
                try:
                    process.wait(timeout=20)
                except subprocess.TimeoutExpired:
                    process.terminate()
                    process.wait(timeout=5)
                write_json(output / "producer-unknown.json", {"session": session, "reason": "Interrupted producer monitor; keep exact daemon ownership for supported retirement, no automatic retry"})
            raise
    assert_settled(session)
    image_id = (output / "image.id").read_text(encoding="utf-8").strip()
    return finalize_image(repo, revision, output, session, inputs, bases, generated, argv, image_id, actions, assistance)


def finalize_image(repo, revision, output, session, inputs, bases, generated, argv, image_id, actions, assistance, license_repo=None):
    inspected = json.loads(command(["docker", "image", "inspect", image_id]))[0]
    archive.config_policy({"os": inspected["Os"], "architecture": inspected["Architecture"], "config": inspected["Config"], "rootfs": {"type": "layers", "diff_ids": inspected["RootFS"]["Layers"]}})
    probes = []
    for tool, exe, version in (("sandboxd", "sandboxd", "0.5.0"), ("codex", "codex", "0.160.0"), ("claudeCode", "claude", "2.1.274"), ("openCode", "opencode", "1.18.34")):
        actions += prepare_effect(session, output, 512 << 20, assistance)
        probes.append(probe(image_id, exe, version, tool, session, output))
    actions += prepare_effect(session, output, 256 << 20, assistance)
    licenses = collect_licenses(license_repo or repo, image_id, session, output)
    assert_settled(session)
    actions += prepare_effect(session, output, BUILD_MEMORY, assistance)
    actions += export_image(image_id, session, output, assistance)
    normalized = archive.normalize(output / "docker-save.raw.tar", output / "runtime-image-linux-amd64.tar", image_id)
    assert_settled(session)
    log = output / "producer-cache-assistance.jsonl"
    receipt = {"version": 1, "kind": "luci-runtime-producer-build", "buildSourceCommit": revision, "canonicalInputAlgorithm": inputs["algorithm"], "canonicalInputDigest": inputs["canonicalInputDigest"], "target": {"os": "linux", "architecture": "amd64"}, "generatedContextSHA256": generated, "baseImages": bases,
               "builder": {"backend": "docker-builder-v1", "version": command(["docker", "version", "--format", "{{.Server.Version}}"] ).strip().decode("ascii"), "argv": argv, "memoryBytes": BUILD_MEMORY, "cpuQuotaMicros": 100000, "exitCode": 0, "stdoutSHA256": archive.file_record(output / "build.out.log")["sha256"], "stderrSHA256": archive.file_record(output / "build.err.log")["sha256"], "cacheAssistance": actions > 0, "cacheAssistanceLogSHA256": archive.file_record(log)["sha256"] if log.exists() else hashlib.sha256(b"").hexdigest()}, "imageID": image_id, "archive": {key: normalized["archive"][key] for key in ("sha256", "bytes")}, "probes": probes}
    schema_validate(receipt, "runtime-oci-build-receipt-v1.schema.json")
    write_json(output / "runtime-build-receipt.json", receipt)
    manifest = {"version": 1, "kind": "luci-runtime-docker-save", "sourceCommit": revision, "canonicalInputAlgorithm": inputs["algorithm"], "canonicalInputDigest": inputs["canonicalInputDigest"], "target": receipt["target"], "sourceFiles": inputs["sourceFiles"], "archive": {"fileName": "runtime-image-linux-amd64.tar", "sha256": normalized["archive"]["sha256"], "bytes": normalized["archive"]["bytes"], "members": normalized["members"]}, "image": normalized["image"], "runtime": archive.RUNTIME, "licenses": licenses, "producer": {"dockerfileSHA256": next(x["sha256"] for x in inputs["sourceFiles"] if x["path"] == "images/sandbox/Dockerfile"), "buildReceiptSHA256": archive.file_record(output / "runtime-build-receipt.json")["sha256"], "buildSourceCommit": revision}}
    schema_validate(manifest, "runtime-oci-manifest-v1.schema.json")
    clean_source(repo, revision)
    write_json(output / "runtime-oci-manifest-v1.json", manifest)
    write_json(output / "producer-complete.json", {"sourceCommit": revision, "session": session, "imageID": image_id, "manifestSHA256": archive.file_record(output / "runtime-oci-manifest-v1.json")["sha256"], "producerOnly": True, "cacheAssistance": actions > 0, "ordinaryInstalledAcceptance": False})
    return manifest


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--revision", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--producer-wsl-cache-assistance", action="store_true")
    args = parser.parse_args()
    try:
        result = produce(args.repo, args.revision, args.output, args.producer_wsl_cache_assistance)
    except (ValueError, OSError, subprocess.SubprocessError) as exc:
        parser.exit(1, "Producer refused/failed; preserve the owned output and logs: " + str(exc) + "\n")
    print(json.dumps({"imageID": result["image"]["id"], "archiveBytes": result["archive"]["bytes"], "output": str(args.output)}, sort_keys=True))
