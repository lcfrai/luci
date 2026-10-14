#!/usr/bin/env python3
"""Dedicated-host Azure Embed install and generation rotation controller.

Only this host's boot/rotate command is effectful. Every uncertain external
effect is journaled before it starts, and an interrupted effect is not replayed.
The controller's own durable disk/generation fence remains authoritative.
"""

import argparse
import base64
import fcntl
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
from urllib.parse import urlsplit


PACKAGE = Path("/opt/luci/azure-embed/package")
PACKAGE_HASH = Path("/opt/luci/azure-embed/package.sha256")
PACKAGE_ARCHIVE = Path("/opt/luci/azure-embed/package.tar")
PRIVATE = Path("/run/luci_embed")
JOURNAL = Path("/var/lib/luci/azure-embed/host-state.json")
HOST_CONFIG = Path("/etc/luci/azure-embed-deployment.json")
HEX = re.compile(r"[0-9a-f]{64}\Z")
IDENTITY = re.compile(r"[a-z0-9][a-z0-9-]{1,127}\Z")
REFS = ("teamApiKey", "adminToken", "sandboxTokenSeed", "gatewayServerCert", "gatewayServerKey", "gatewayClientCa")
REQUIRED_RUNNING = {"api", "client-proxy", "orchestrator"}
RFC1918 = tuple(ipaddress.ip_network(prefix) for prefix in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"))


def sha(data):
    return hashlib.sha256(data).hexdigest()


def command(argv, timeout=30):
    result = subprocess.run(argv, check=False, capture_output=True, text=True, timeout=timeout)
    if result.returncode:
        # Never echo stdout/stderr: Compose or provider tools might include a token.
        raise RuntimeError("owned host command failed: " + argv[0])
    return result.stdout.strip()


def strict_json(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or path.is_symlink() or info.st_mode & 0o077 or info.st_size > 16 << 10:
        raise ValueError("private host configuration is unavailable")
    return json.loads(path.read_text())


def config(path):
    value = strict_json(path)
    required = {
        "version", "scope", "generation", "runtimePackageSHA256", "vaultURL", "gatewayHost",
        "gatewayPrivateIP", "controllerPrivateIP", "runtimeSubnetCIDR", "hostClientID",
        "controllerClientID", "secretRefs",
    }
    if not required.issubset(value) or not set(value).issubset(required | {"previousGeneration", "controllerSuccessorLink"}):
        raise ValueError("host configuration fields are invalid")
    if value["version"] != 1 or not all(IDENTITY.fullmatch(value[k]) for k in ("scope", "generation")):
        raise ValueError("host generation is invalid")
    if "previousGeneration" in value and not IDENTITY.fullmatch(value["previousGeneration"]):
        raise ValueError("previous host generation is invalid")
    if not HEX.fullmatch(value["runtimePackageSHA256"]):
        raise ValueError("host package binding is invalid")
    addresses = [ipaddress.ip_address(value[key]) for key in ("gatewayPrivateIP", "controllerPrivateIP")]
    if addresses[0] == addresses[1] or any(
        address.version != 4 or not any(address in block for block in RFC1918) for address in addresses
    ):
        raise ValueError("host network boundary is invalid")
    subnet = ipaddress.ip_network(value["runtimeSubnetCIDR"], strict=True)
    if subnet.version != 4 or not any(subnet.subnet_of(block) for block in RFC1918) or addresses[0] not in subnet or addresses[1] in subnet:
        raise ValueError("runtime subnet is invalid")
    if set(value["secretRefs"]) != set(REFS) or any(
        set(ref) != {"name", "version"} or not re.fullmatch(r"[0-9a-f]{32}", ref["version"])
        for ref in value["secretRefs"].values()
    ):
        raise ValueError("exact host secret references are invalid")
    vault = urlsplit(value["vaultURL"])
    if vault.scheme != "https" or vault.username or vault.password or vault.port or vault.path or vault.query or vault.fragment or not vault.hostname or not re.fullmatch(r"[a-z0-9-]+\.vault\.azure\.net", vault.hostname):
        raise ValueError("private Key Vault address is invalid")
    if not re.fullmatch(r"[a-z0-9][a-z0-9.-]{2,252}", value["gatewayHost"]):
        raise ValueError("gateway host is invalid")
    for key in ("hostClientID", "controllerClientID"):
        if not re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", value[key]):
            raise ValueError("host identity is invalid")
    if value["hostClientID"] == value["controllerClientID"]:
        raise ValueError("host and controller identities must differ")
    return value


def private_tmpfs():
    if PRIVATE.is_symlink() or not PRIVATE.is_dir() or PRIVATE.stat().st_mode & 0o077:
        raise ValueError("host private tmpfs is unavailable")
    for line in Path("/proc/self/mountinfo").read_text().splitlines():
        before, _, after = line.partition(" - ")
        if before.split()[4] == str(PRIVATE) and after.startswith("tmpfs "):
            return
    raise ValueError("host private directory is not a dedicated tmpfs mount")


def pinned_package(c):
    if PACKAGE_HASH.read_text().strip() != c["runtimePackageSHA256"]:
        raise ValueError("image package does not match deployment generation")
    if PACKAGE_ARCHIVE.is_symlink() or sha(PACKAGE_ARCHIVE.read_bytes()) != c["runtimePackageSHA256"]:
        raise ValueError("installed host package archive changed")
    manifest_bytes = (PACKAGE / "manifest.json").read_bytes()
    with tarfile.open(PACKAGE_ARCHIVE, mode="r:") as archive:
        entry = archive.extractfile("manifest.json")
        if entry is None or entry.read() != manifest_bytes:
            raise ValueError("installed host manifest changed")
    manifest = json.loads(manifest_bytes)
    if manifest.get("version") != 1 or manifest.get("e2bRevision") != "cf536c284434fdd2f09bbab35ec807a5dacc87e4":
        raise ValueError("host package source binding is invalid")
    for name, record in manifest["files"].items():
        path = PACKAGE / name
        if path.is_symlink() or not path.is_file() or path.stat().st_size != record["size"] or sha(path.read_bytes()) != record["sha256"]:
            raise ValueError("host package file changed")
    for required in ("compose/compose.yaml", "compose/.env", "compose/image-lock.yaml", "bin/azure-embed-host-bootstrap", "bin/azure-gateway-config", "bin/envoy"):
        if required not in manifest["files"]:
            raise ValueError("host package is incomplete")
    return manifest


def host_ip(c):
    network = ipaddress.ip_network(c["runtimeSubnetCIDR"], strict=True)
    interfaces = json.loads(command(["ip", "-j", "-4", "addr", "show"]))
    found = {
        detail["local"] for iface in interfaces for detail in iface.get("addr_info", [])
        if detail.get("family") == "inet" and ipaddress.ip_address(detail["local"]) in network
        and detail["local"] not in (c["gatewayPrivateIP"], c["controllerPrivateIP"])
    }
    if len(found) != 1:
        raise ValueError("runtime host private interface is ambiguous")
    return found.pop()


def atomic_json(path, value):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    temp = path.with_name(path.name + ".next")
    with temp.open("x") as out:
        os.fchmod(out.fileno(), 0o600)
        json.dump(value, out, sort_keys=True, separators=(",", ":"))
        out.flush()
        os.fsync(out.fileno())
    os.replace(temp, path)
    parent = os.open(path.parent, os.O_RDONLY)
    try:
        os.fsync(parent)
    finally:
        os.close(parent)


def write_private(path, data):
    if path.exists() or path.is_symlink():
        raise ValueError("private host output already exists")
    with path.open("xb") as out:
        os.fchmod(out.fileno(), 0o600)
        out.write(data)
        out.flush()
        os.fsync(out.fileno())


def journal():
    if not JOURNAL.exists():
        return None
    return strict_json(JOURNAL)


def save(record):
    atomic_json(JOURNAL, record)


def compose_args(stage):
    return ["docker", "compose", "-p", "e2b", "--env-file", str(stage / "runtime.env"),
            "-f", str(PACKAGE / "compose/compose.yaml"), "-f", str(PACKAGE / "compose/image-lock.yaml"),
            "-f", str(PACKAGE / "compose/azure-services.yaml")]


def guest_source_ready(manifest):
    if 'guest-source-receipt.json' not in manifest['files']:
        return  # Explicit existing-registry compatibility package.
    expected = json.loads((PACKAGE / 'guest-source-receipt.json').read_bytes())
    observed = json.loads(command(['python3', str(PACKAGE / 'bin/guest_registry.py'), 'ready',
                                  '--source-commit', manifest['luciRevision']], 180))
    if observed != expected:
        raise ValueError('Installed/served guest source differs from pinned host package')


def running(stage):
    names = set(command(compose_args(stage) + ["ps", "--status", "running", "--services"]).splitlines())
    return REQUIRED_RUNNING.issubset(names)


def running_by_label():
    names = set(command(["docker", "ps", "--filter", "label=com.docker.compose.project=e2b",
                         "--format", '{{.Label "com.docker.compose.service"}}']).splitlines())
    return REQUIRED_RUNNING.issubset(names)


def has_kvm():
    try:
        return stat.S_ISCHR(Path("/dev/kvm").stat().st_mode)
    except OSError:
        return False


def host_input(c, manifest, address):
    fields = {
        "vaultUrl": c["vaultURL"], "gatewayHost": c["gatewayHost"], "privateListenIp": address,
        "controllerPrivateIp": c["controllerPrivateIP"], "scope": c["scope"], "generation": c["generation"],
        "hostClientId": c["hostClientID"], "controllerClientId": c["controllerClientID"],
        "privateDirectory": str(PRIVATE), "composePath": str(PACKAGE / "compose/compose.yaml"),
        "composeSha256": manifest["files"]["compose/compose.yaml"]["sha256"],
        "composeEnvBase": str(PACKAGE / "compose/.env"),
        "composeEnvSha256": manifest["files"]["compose/.env"]["sha256"],
    }
    fields.update(c["secretRefs"])
    return fields


def prepare(c, manifest, address):
    suffix = c["generation"]
    source = PRIVATE / ("host-" + suffix + ".json")
    write_private(source, (json.dumps(host_input(c, manifest, address), sort_keys=True) + "\n").encode())
    stage = None
    try:
        stage = Path(command([str(PACKAGE / "bin/azure-embed-host-bootstrap"), str(source)], 120))
        if stage.parent != PRIVATE or not stage.name.startswith("luci-embed-"):
            raise ValueError("host bootstrap returned an unowned stage")
        gateway = stage / "gateway.json"
        envoy = PRIVATE / ("envoy-" + suffix + ".yaml")
        nft = PRIVATE / ("firewall-" + suffix + ".nft")
        write_private(envoy, command([str(PACKAGE / "bin/azure-gateway-config"), str(gateway)]).encode())
        write_private(nft, command([str(PACKAGE / "bin/azure-gateway-config"), "--nftables", str(gateway)]).encode())
        command([str(PACKAGE / "bin/envoy"), "--mode", "validate", "-c", str(envoy)])
        command(["nft", "-c", "-f", str(nft)])
        command(compose_args(stage) + ["config", "--quiet"], 90)
        return {"stage": str(stage), "source": str(source), "envoy": str(envoy), "nft": str(nft)}
    except Exception:
        # No runtime effect started. Retire only a returned exact owned stage;
        # an ambiguous bootstrap response remains visible and is never guessed.
        if stage is not None and stage.parent == PRIVATE and stage.name.startswith("luci-embed-"):
            command([str(PACKAGE / "bin/azure-embed-host-bootstrap"), "--retire", str(source), str(stage)])
        for path in (PRIVATE / ("envoy-" + suffix + ".yaml"), PRIVATE / ("firewall-" + suffix + ".nft"), source):
            if path.exists():
                path.unlink()
        raise


def switch_gateway(prepared):
    link = PRIVATE / "active-envoy.yaml"
    next_link = PRIVATE / "active-envoy.yaml.next"
    if next_link.exists() or next_link.is_symlink():
        raise ValueError("gateway switch has an unresolved prior attempt")
    next_link.symlink_to(prepared["envoy"])
    os.replace(next_link, link)
    command(["systemctl", "restart", "luci-azure-embed-gateway.service"], 45)
    if command(["systemctl", "is-active", "luci-azure-embed-gateway.service"]) != "active":
        raise RuntimeError("private gateway did not become active")


def boot(c):
    private_tmpfs()
    manifest = pinned_package(c)
    guest_source_ready(manifest)
    address = host_ip(c)
    if not has_kvm():
        raise RuntimeError("dedicated runtime host has no KVM device")
    old = journal()
    if old:
        if old.get("state") == "active" and old.get("scope") == c["scope"] and old.get("generation") == c["generation"] and old.get("packageSHA256") == c["runtimePackageSHA256"] and old.get("secretRefs") == c["secretRefs"] and old.get("hostIP") == address:
            stage = Path(old["prepared"]["stage"])
            if stage.exists() and running(stage) and command(["systemctl", "is-active", "luci-azure-embed-gateway.service"]) == "active":
                return
            if not stage.exists() and running_by_label():
                # VM reboot removed tmpfs, while Docker restarted its exact
                # already-owned services. Restore the same secret versions and
                # gateway only; never rerun Compose up or allocate guests.
                prepared = prepare(c, manifest, address)
                old["state"] = "gateway_restage_pending"
                old["nextPrepared"] = prepared
                save(old)
                switch_gateway(prepared)
                old["prepared"] = prepared
                old.pop("nextPrepared")
                old["state"] = "active"
                save(old)
                return
        raise RuntimeError("retained host state needs exact reconciliation; no startup replay")
    if command(["docker", "ps", "--filter", "label=com.docker.compose.project=e2b", "--format", "{{.ID}}"]):
        raise RuntimeError("unowned E2B Compose containers are present")
    if command(["nft", "list", "tables"]).find("table inet luci_embed") >= 0:
        raise RuntimeError("unowned dedicated-host firewall table is present")
    prepared = prepare(c, manifest, address)
    record = {"version": 1, "state": "firewall_pending", "scope": c["scope"], "generation": c["generation"],
              "packageSHA256": c["runtimePackageSHA256"], "hostIP": address, "prepared": prepared,
              "secretRefs": c["secretRefs"]}
    save(record)
    command(["nft", "-f", prepared["nft"]])
    record["state"] = "compose_pending"
    save(record)
    command(compose_args(Path(prepared["stage"])) + ["up", "-d"], 150)
    record["state"] = "gateway_pending"
    save(record)
    switch_gateway(prepared)
    record["state"] = "active"
    save(record)


def rotate(c):
    private_tmpfs()
    manifest = pinned_package(c)
    guest_source_ready(manifest)
    old = journal()
    if not old or old.get("state") != "active" or old.get("scope") != c["scope"] or old.get("generation") != c.get("previousGeneration") or old.get("packageSHA256") != c["runtimePackageSHA256"]:
        raise RuntimeError("host rotation requires the exact settled previous generation")
    if any(old.get("secretRefs", {}).get(key) != c["secretRefs"][key] for key in REFS[:3]):
        raise RuntimeError("E2B task keys require a drained, fenced replacement host")
    address = host_ip(c)
    if address != old["hostIP"] or not running(Path(old["prepared"]["stage"])):
        raise RuntimeError("owned E2B services are not settled on the selected host")
    prepared = prepare(c, manifest, address)
    old["state"] = "rotation_pending"
    old["nextGeneration"] = c["generation"]
    old["nextPrepared"] = prepared
    save(old)
    switch_gateway(prepared)
    old_prepared = old["prepared"]
    old["generation"] = c["generation"]
    old["secretRefs"] = c["secretRefs"]
    old["prepared"] = prepared
    old["oldPrepared"] = old_prepared
    old["state"] = "retirement_pending"
    old.pop("nextGeneration")
    old.pop("nextPrepared")
    save(old)
    # Only the old stage's five known private files are removed by the pinned
    # bootstrap. A failure retains the journal and does not claim retirement.
    command([str(PACKAGE / "bin/azure-embed-host-bootstrap"), "--retire", old_prepared["source"], old_prepared["stage"]])
    for key in ("source", "envoy", "nft"):
        Path(old_prepared[key]).unlink()
    old.pop("oldPrepared")
    old["state"] = "active"
    save(old)


def deliver(current_path, incoming_path):
    """Apply a protected RunCommand selection to this exact running host.

    The caller's Azure role authorizes RunCommand; this host still checks the
    selected previous generation and immutable package/identity boundary.
    The host journal records the rotation before its gateway effect. A lost
    response is read back, never retried as another delivery.
    """
    if current_path != HOST_CONFIG:
        raise ValueError("host configuration destination is not owned")
    private_tmpfs()
    current = config(current_path)
    next_config = config(incoming_path)
    old = journal()
    if old is None or old.get("state") != "active" or old.get("generation") != current["generation"]:
        raise RuntimeError("host generation is not settled")
    if next_config.get("previousGeneration") != current["generation"] or next_config["generation"] == current["generation"]:
        raise ValueError("host rotation generation is not consecutive")
    for field in ("scope", "runtimePackageSHA256", "vaultURL", "gatewayHost", "gatewayPrivateIP",
                  "runtimeSubnetCIDR", "hostClientID", "controllerClientID"):
        if next_config[field] != current[field]:
            raise ValueError("host deployment boundary changed; replace a drained host")
    if next_config['controllerPrivateIP'] != current['controllerPrivateIP'] or 'controllerSuccessorLink' in next_config:
        verify_successor_link(current, next_config, old)
    if any(next_config["secretRefs"][key] != current["secretRefs"][key] for key in REFS[:3]):
        raise ValueError("task credentials changed; replace a drained host")
    # rotate() journals each uncertain effect. The persistent selection is
    # replaced only after rotation has settled; a crash between these steps
    # leaves a visible mismatch and boot refuses replay.
    rotate(next_config)
    atomic_json(current_path, next_config)
    updated = journal()
    updated["configSHA256"] = sha(current_path.read_bytes())
    save(updated)
    return {"state": updated["state"], "generation": updated["generation"],
            "configSHA256": updated["configSHA256"]}


def verify_successor_link(current, incoming, record):
    """Only the predecessor signer may authorize the observed replacement IP.

    ARM NIC/boot/fence observations are checked by the existing recovery owner
    before this protected RunCommand, without adding runtime identity grants.
    The runtime independently checks the predecessor public CA and exact next
    config, rather than accepting an arbitrary address in a private subnet.
    """
    link = incoming.get('controllerSuccessorLink', {})
    payload = link.get('payload', {})
    required = {'version', 'sourceCommit', 'scope', 'ownerID', 'oldGeneration', 'newGeneration', 'oldControllerID',
        'newControllerID', 'newBootID', 'diskID', 'oldPrivateIP', 'newPrivateIP', 'newNICID', 'newNICETag',
        'controllerSubnetID', 'predecessorFenceETag', 'predecessorFenceSHA256', 'hostConfigSHA256'}
    unsigned = {key: value for key, value in incoming.items() if key != 'controllerSuccessorLink'}
    manifest = pinned_package(current)
    if (set(link) != {'payload', 'signature', 'signatureAlgorithm'} or link.get('signatureAlgorithm') != 'ecdsa-sha256' or
            set(payload) != required or payload.get('version') != 1 or payload.get('sourceCommit') != manifest['luciRevision'] or
            payload.get('scope') != current['scope'] or payload.get('oldGeneration') != current['generation'] or
            payload.get('newGeneration') != incoming['generation'] or payload.get('oldPrivateIP') != current['controllerPrivateIP'] or
            payload.get('newPrivateIP') != incoming['controllerPrivateIP'] or
            payload.get('hostConfigSHA256') != sha(json.dumps(unsigned, sort_keys=True, separators=(',', ':')).encode()) or
            not HEX.fullmatch(payload.get('predecessorFenceSHA256', '')) or
            not all(isinstance(payload.get(key), str) and 1 <= len(payload[key]) <= 1024 for key in ('predecessorFenceETag', 'newNICETag', 'newNICID', 'controllerSubnetID'))):
        raise ValueError('controller successor link differs from exact source, predecessor or host configuration')
    stage = Path(record['prepared']['stage'])
    if stage.parent != PRIVATE or not stage.name.startswith('luci-embed-') or stage.is_symlink():
        raise ValueError('predecessor public trust stage is not owned')
    ca = stage / 'client-ca.crt'
    if ca.is_symlink() or not ca.is_file() or not 0 < ca.stat().st_size <= 16384:
        raise ValueError('predecessor public trust is unavailable')
    try:
        signature = base64.b64decode(link['signature'], validate=True)
    except (ValueError, TypeError) as error:
        raise ValueError('controller successor signature is malformed') from error
    if not 1 <= len(signature) <= 256: raise ValueError('controller successor signature exceeds bound')
    with tempfile.TemporaryDirectory(dir=PRIVATE) as directory:
        root = Path(directory)
        (root / 'key').write_text(command(['openssl', 'x509', '-in', str(ca), '-pubkey', '-noout']))
        (root / 'message').write_bytes(json.dumps(payload, sort_keys=True, separators=(',', ':')).encode())
        (root / 'signature').write_bytes(signature)
        command(['openssl', 'dgst', '-sha256', '-verify', str(root / 'key'), '-signature', str(root / 'signature'), str(root / 'message')])


def inspect(c):
    manifest = pinned_package(c)
    guest_source_ready(manifest)
    record = journal()
    expected = sha(json.dumps(c, sort_keys=True, separators=(',', ':')).encode())
    if (not record or record.get('state') != 'active' or record.get('scope') != c['scope'] or
            record.get('generation') != c['generation'] or record.get('packageSHA256') != c['runtimePackageSHA256'] or
            record.get('configSHA256') != expected or not running(Path(record['prepared']['stage']))):
        raise RuntimeError('exact owned host generation remains uncertain')
    return {'state': 'active', 'generation': c['generation'], 'configSHA256': expected}


def retirement(c, plan_digest, inspect_only=False):
    """Stop only this owned runtime stack; preserve all task/template volumes."""
    if not HEX.fullmatch(plan_digest): raise ValueError('Exact retirement plan digest is required')
    manifest = pinned_package(c); guest_source_ready(manifest)
    record = journal()
    if (not record or record.get('scope') != c['scope'] or record.get('generation') != c['generation'] or
            record.get('packageSHA256') != c['runtimePackageSHA256'] or record.get('secretRefs') != c['secretRefs']):
        raise ValueError('Retirement runtime source or owned generation changed')
    if record.get('state') == 'active':
        if record.get('retirementCancelledPlanDigest') == plan_digest:
            raise ValueError('Actor cancelled this runtime retirement plan; no late stop')
        if inspect_only: raise RuntimeError('Runtime stop remains absent/unknown; no repeated stop')
        if not running(Path(record['prepared']['stage'])): raise ValueError('Owned runtime stack is not settled before retirement')
        record.update(state='retirement_stop_pending', retirementPlanDigest=plan_digest,
            retirementConfigSHA256=sha(json.dumps(c, sort_keys=True, separators=(',', ':')).encode()))
        save(record)
        command(['systemctl', 'stop', 'luci-azure-embed-gateway.service'])
        command(compose_args(Path(record['prepared']['stage'])) + ['stop', '--timeout', '30'], 90)
    if record.get('state') not in ('retirement_stop_pending', 'retirement_stopped') or record.get('retirementPlanDigest') != plan_digest or record.get('retirementConfigSHA256') != sha(json.dumps(c, sort_keys=True, separators=(',', ':')).encode()):
        raise ValueError('Runtime retirement differs from retained source/plan')
    if command(['systemctl', 'show', 'luci-azure-embed-gateway.service', '--property=ActiveState', '--value']) != 'inactive':
        raise RuntimeError('Owned gateway stop is uncertain; no repeated stop')
    if Path(record['prepared']['stage']).exists() and command(compose_args(Path(record['prepared']['stage'])) + ['ps', '--status', 'running', '--services']):
        raise RuntimeError('Owned runtime containers still run; no repeated stop')
    if command(['docker', 'ps', '--format', '{{.ID}}']):
        raise RuntimeError('Other running containers are present; preserve this host instead of deallocating unrelated work')
    if command(['docker', 'info', '--format', '{{.DockerRootDir}}']) != '/var/lib/docker':
        raise ValueError('Runtime durable Docker data is outside the selected OS-disk backup')
    root_device = os.stat('/').st_dev
    if any(Path(name).is_symlink() or not Path(name).is_dir() or os.stat(name).st_dev != root_device for name in ('/var/lib/docker', '/var/lib/e2b/storage/templates', '/var/lib/e2b/storage/build-cache')):
        raise ValueError('Runtime task/template storage is not on the selected source OS disk')
    record['state'] = 'retirement_stopped'; save(record)
    command(['sync', '-f', '/'])
    return {'state': record['state'], 'sourceCommit': manifest['luciRevision'], 'scope': c['scope'],
        'generation': c['generation'], 'planDigest': plan_digest, 'runtimePackageSHA256': c['runtimePackageSHA256'],
        'taskAndTemplateVolumesPreserved': True, 'runtimeStorageOnOSDiskVerified': True, 'computeDeallocationVerified': False}


def retirement_resume(c, plan_digest, inspect_only=False):
    """Resume only retained owned containers, without creating guests or clearing leases."""
    if not HEX.fullmatch(plan_digest): raise ValueError('Exact retirement cancellation plan is required')
    manifest = pinned_package(c); guest_source_ready(manifest)
    record = journal()
    expected = sha(json.dumps(c, sort_keys=True, separators=(',', ':')).encode())
    if (not record or record.get('scope') != c['scope'] or record.get('generation') != c['generation'] or
            record.get('packageSHA256') != c['runtimePackageSHA256'] or record.get('secretRefs') != c['secretRefs']):
        raise ValueError('Runtime cancellation source or owned generation changed')
    cancelled = record.get('retirementCancelledPlanDigest')
    if cancelled and cancelled != plan_digest: raise ValueError('Runtime cancellation belongs to another exact actor plan')
    if not cancelled:
        if inspect_only: raise RuntimeError('Runtime cancellation intent is absent/unknown; no repeated start')
        if record.get('state') not in ('active', 'retirement_stop_pending', 'retirement_stopped'):
            raise ValueError('Runtime has another pending transition; preserve it')
        if record['state'] != 'active' and (record.get('retirementPlanDigest') != plan_digest or record.get('retirementConfigSHA256') != expected):
            raise ValueError('Runtime cancellation differs from exact stopped plan/configuration')
        prior = record['state']
        record.update(state='retirement_resume_pending', retirementCancelledPlanDigest=plan_digest,
            retirementResumePriorState=prior, retirementResumeConfigSHA256=expected)
        save(record)
        stage = Path(record['prepared']['stage'])
        if not stage.exists():
            private_tmpfs()
            # Rehydrate the exact existing generation/versioned credentials;
            # Compose start cannot create containers or task microVMs.
            record['prepared'] = prepare(c, manifest, host_ip(c)); save(record)
        if prior != 'active':
            command(compose_args(Path(record['prepared']['stage'])) + ['start'], 90)
            switch_gateway(record['prepared'])
    if record.get('state') not in ('retirement_resume_pending', 'active') or record.get('retirementResumeConfigSHA256') != expected:
        raise ValueError('Runtime cancellation configuration or pending intent changed')
    if not running(Path(record['prepared']['stage'])) or command(['systemctl', 'is-active', 'luci-azure-embed-gateway.service']) != 'active':
        raise RuntimeError('Owned runtime resume remains uncertain; inspect only, no second start')
    record['state'] = 'active'; save(record)
    return {'state': 'retirement_cancelled_active', 'sourceCommit': manifest['luciRevision'], 'scope': c['scope'],
        'generation': c['generation'], 'planDigest': plan_digest, 'runtimePackageSHA256': c['runtimePackageSHA256'],
        'taskReservationsReleased': False, 'taskAndTemplateVolumesPreserved': True}


def main():
    p = argparse.ArgumentParser()
    p.add_argument("action", choices=("boot", "rotate", "status", "deliver", "inspect", "retirement-stop", "retirement-inspect", "retirement-resume", "retirement-resume-inspect"))
    p.add_argument("config")
    p.add_argument("incoming", nargs="?")
    args = p.parse_args()
    needs_argument = args.action in ('deliver', 'retirement-stop', 'retirement-inspect', 'retirement-resume', 'retirement-resume-inspect')
    if needs_argument != bool(args.incoming):
        raise ValueError("host delivery input is invalid")
    c = config(Path(args.config))
    if args.action == 'inspect':
        print(json.dumps(inspect(c), sort_keys=True))
        return
    JOURNAL.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if JOURNAL.parent.is_symlink() or JOURNAL.parent.stat().st_mode & 0o077:
        raise ValueError("owned host journal directory is not private")
    with (JOURNAL.parent / "host.lock").open("a+b") as lock:
        os.fchmod(lock.fileno(), 0o600)
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise RuntimeError("owned host lifecycle is already running") from error
        if args.action == "boot":
            boot(c)
        elif args.action == "rotate":
            rotate(c)
        elif args.action == "deliver":
            print(json.dumps(deliver(Path(args.config), Path(args.incoming)), sort_keys=True))
        elif args.action in ('retirement-stop', 'retirement-inspect'):
            print(json.dumps(retirement(c, args.incoming, args.action == 'retirement-inspect'), sort_keys=True))
        elif args.action in ('retirement-resume', 'retirement-resume-inspect'):
            print(json.dumps(retirement_resume(c, args.incoming, args.action == 'retirement-resume-inspect'), sort_keys=True))
        else:
            record = journal()
            print(json.dumps({"state": record.get("state", "absent") if record else "absent",
                              "generation": record.get("generation") if record else None,
                              "configSHA256": record.get("configSHA256") if record else None}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, subprocess.TimeoutExpired, json.JSONDecodeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
