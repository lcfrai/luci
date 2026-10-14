#!/usr/bin/env python3
"""Mount the one fenced Azure controller disk and bind Luci's operator config.

Runs only on the selected private controller VM as root. `initialize` may
format a proven blank LUN0 once; `restore` never formats or copies state.
All application state, journal and task artifacts remain on the same disk.
"""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys
import tempfile
import urllib.request


UUID = re.compile(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z")
HEX = re.compile(r"[0-9a-f]{64}\Z")
GEN = re.compile(r"[a-z0-9][a-z0-9-]{1,127}\Z")
DEVICE = Path("/dev/disk/azure/scsi1/lun0")


def sha(data):
    return hashlib.sha256(data).hexdigest()


def private(path, limit=16384):
    info = path.lstat()
    if path.is_symlink() or not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or info.st_size < 1 or info.st_size > limit:
        raise ValueError("private controller input is unavailable")
    return path.read_bytes()


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def atomic(path, data):
    path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix=".controller-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.replace(name, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def call(args):
    result = subprocess.run(args, capture_output=True, text=True, timeout=30, check=False)
    return result.returncode, result.stdout.strip()


def selected(value):
    required = {"version", "mode", "device", "mountPath", "filesystemUuid", "azureDataDiskId", "ownerId", "scope",
                "controllerHostId", "runtimeHostId", "controllerInstanceId", "generation", "diskReceiptPath",
                "generationReceiptPath", "operatorTemplatePath", "operatorTemplateSHA256", "operatorOutputPath", "fenceBlobURL"}
    predecessor = {"previousGeneration", "previousGenerationReceiptSHA256", "previousFenceReceiptDigest"}
    successor = {"previousControllerInstanceID", "previousFinalizationSHA256", "previousBootID", "targetBootID", "finalizationSHA256"}
    if set(value) not in (required, required | predecessor, required | predecessor | successor) or value["version"] != 1 or value["mode"] not in ("initialize", "restore") or value["device"] != str(DEVICE):
        raise ValueError("controller disk selection is invalid")
    if not UUID.fullmatch(value["filesystemUuid"]) or not GEN.fullmatch(value["generation"]) or not HEX.fullmatch(value["operatorTemplateSHA256"]):
        raise ValueError("controller disk generation or UUID is invalid")
    if not re.fullmatch(r"https://[a-z0-9]{3,24}\.blob\.core\.windows\.net/controller-fence/controller-fence\.json", value["fenceBlobURL"]):
        raise ValueError("controller fence Blob selection is invalid")
    if value["mode"] == "restore" and (not GEN.fullmatch(value.get("previousGeneration", "")) or
         value["previousGeneration"] == value["generation"] or not HEX.fullmatch(value.get("previousGenerationReceiptSHA256", "")) or
         not HEX.fullmatch(value.get("previousFenceReceiptDigest", ""))):
        raise ValueError("controller predecessor fence is incomplete")
    if value["mode"] == "initialize" and set(value) != required:
        raise ValueError("initial controller disk cannot claim a predecessor")
    if successor.issubset(value) and (value["mode"] != "restore" or
            value["previousControllerInstanceID"] == value["controllerInstanceId"] or
            not all(UUID.fullmatch(value[key]) for key in ("previousBootID", "targetBootID")) or
            not all(HEX.fullmatch(value[key]) for key in ("previousFinalizationSHA256", "finalizationSHA256"))):
        raise ValueError("signed successor mount selection is incomplete")
    mount = Path(value["mountPath"])
    if not re.fullmatch(r"/mnt/[a-z0-9-]+", str(mount)) or mount.is_symlink() or mount.exists() and not mount.is_dir():
        raise ValueError("controller mount point is invalid")
    for key in ("diskReceiptPath", "generationReceiptPath"):
        path = Path(value[key])
        if not path.is_absolute() or mount not in path.parents or any(part.is_symlink() for part in (path, *path.parents) if part == mount or mount in part.parents):
            raise ValueError("controller receipt is outside the exact mount")
    for key in ("operatorTemplatePath", "operatorOutputPath"):
        path = Path(value[key])
        if not path.is_absolute() or path.is_symlink() or key == "operatorTemplatePath" and not path.is_file():
            raise ValueError("controller operator config path is invalid")
    if value["operatorTemplatePath"] == value["operatorOutputPath"]:
        raise ValueError("controller operator template cannot be overwritten")


def mount_exact(value):
    device = Path(value["device"])
    if not device.exists() or not stat.S_ISBLK(device.stat().st_mode):
        raise ValueError("selected Azure LUN0 block device is unavailable")
    filesystem = call(["blkid", "-p", "-o", "value", "-s", "TYPE", str(device)])
    observed_uuid = call(["blkid", "-p", "-o", "value", "-s", "UUID", str(device)])
    if value["mode"] == "initialize":
        signatures = call(["wipefs", "-n", str(device)])
        if filesystem[0] == 0 or observed_uuid[0] == 0 or signatures[0] != 0 or signatures[1]:
            raise ValueError("controller disk is not proven blank; formatting refused")
        if call(["mkfs.ext4", "-U", value["filesystemUuid"], "-L", "luci-controller", str(device)])[0] != 0:
            raise RuntimeError("controller disk format did not settle")
    elif filesystem != (0, "ext4") or observed_uuid != (0, value["filesystemUuid"]):
        raise ValueError("existing controller filesystem identity changed")
    mount = Path(value["mountPath"])
    if os.path.ismount(mount):
        if os.stat(mount).st_dev != os.stat(device).st_rdev:
            raise ValueError("controller mount is a different device")
    elif call(["mount", "-t", "ext4", "-o", "rw,nosuid,nodev", str(device), str(mount)])[0] != 0:
        raise RuntimeError("controller disk mount did not settle")
    if os.stat(mount).st_dev != os.stat(device).st_rdev:
        raise ValueError("controller mounted device changed")


def bound_operator(operator, value, disk_path, generation_path, disk_data, generation_data):
    operator = json.loads(json.dumps(operator))
    operator["azureMount"] = {"mountPath": value["mountPath"], "filesystemUuid": value["filesystemUuid"],
                              "receiptPath": str(disk_path), "generation": value["generation"],
                              "controllerInstanceId": value["controllerInstanceId"],
                              "generationReceiptPath": str(generation_path), "generationReceiptDigest": sha(generation_data),
                              "fenceBlobURL": value["fenceBlobURL"]}
    operator["store"]["separationReceiptDigest"] = sha(disk_data)
    operator["deployment"]["hostGeneration"] = value["generation"]
    if value['mode'] == 'restore':
        operator['deployment']['restoreReceiptDigest'] = value['previousFenceReceiptDigest']
    gateway_contract = ["luci-azure-e2b-gateway-v1", operator["deployment"]["azureGatewayHost"],
                        value["scope"], value["generation"], "api:3000", "guest:3002", "envd:49983", "sandboxd:8080"]
    operator["azureGatewayReceiptDigest"] = sha(canonical(gateway_contract))
    return operator


def bind(value, finalize_only=False):
    selected(value)
    template_bytes = private(Path(value["operatorTemplatePath"]))
    if sha(template_bytes) != value["operatorTemplateSHA256"]:
        raise ValueError("controller operator template changed")
    operator, journals = finalized_operator(value, json.loads(template_bytes), allow_deferred=not finalize_only and value["mode"] == "initialize")
    Path(value["mountPath"]).mkdir(mode=0o700, parents=True, exist_ok=True)
    # Finalization uses the existing-filesystem branch, never mkfs.
    mount_exact(dict(value, mode="restore") if finalize_only else value)
    disk_path = Path(value["diskReceiptPath"])
    generation_path = Path(value["generationReceiptPath"])
    disk_record = {"version": 1, "azureDataDiskId": value["azureDataDiskId"], "filesystemUuid": value["filesystemUuid"],
                   "ownerId": value["ownerId"], "scope": value["scope"], "controllerHostId": value["controllerHostId"],
                   "runtimeHostId": value["runtimeHostId"]}
    disk_data = canonical(disk_record)
    if finalize_only:
        if private(disk_path, 4096) != disk_data:
            raise ValueError("finalization controller disk receipt changed")
    elif value["mode"] == "initialize":
        if disk_path.exists() or generation_path.exists() or any(Path(value["mountPath"]).iterdir()):
            # Newly formatted ext4 normally has lost+found; exclude only it.
            contents = {p.name for p in Path(value["mountPath"]).iterdir()}
            if disk_path.exists() or generation_path.exists() or contents != {"lost+found"}:
                raise ValueError("new controller disk has unexpected content")
        atomic(disk_path, disk_data)
    elif private(disk_path, 4096) != disk_data:
        raise ValueError("authoritative controller disk receipt changed")
    old_generation = None
    already_new = False
    if value["mode"] == "restore":
        old_bytes = private(generation_path, 4096)
        old_generation = json.loads(old_bytes)
        if (old_generation.get("generation") == value["generation"] and
                old_generation.get("controllerInstanceId") == value["controllerInstanceId"] and
                old_generation.get("previousGeneration") == value["previousGeneration"] and
                old_generation.get("previousFenceReceiptDigest") == value["previousFenceReceiptDigest"] and
                old_generation.get("azureDataDiskId") == value["azureDataDiskId"]):
            already_new = True
        elif sha(old_bytes) != value["previousGenerationReceiptSHA256"] or old_generation.get("generation") != value["previousGeneration"] or old_generation.get("azureDataDiskId") != value["azureDataDiskId"]:
            raise ValueError("controller predecessor generation changed")
    if "previousFinalizationSHA256" in value:
        prior = json.loads(private(Path(value["mountPath"]) / "luci/operator-finalization-receipt.json", 4096))
        expected = ({"version": 1, "envelopeSHA256": value["finalizationSHA256"],
                     "generation": value["generation"], "controllerInstanceId": value["controllerInstanceId"],
                     "previousEnvelopeSHA256": value["previousFinalizationSHA256"]} if already_new else
                    {"version": 1, "envelopeSHA256": value["previousFinalizationSHA256"],
                     "generation": value["previousGeneration"],
                     "controllerInstanceId": value["previousControllerInstanceID"]})
        if prior != expected:
            raise ValueError("signed predecessor/operator receipt differs from the fenced generation")
    generation = {"version": 1, "generation": value["generation"], "controllerInstanceId": value["controllerInstanceId"],
                  "azureDataDiskId": value["azureDataDiskId"]}
    if old_generation:
        generation["previousGeneration"] = value["previousGeneration"]
        generation["previousFenceReceiptDigest"] = value["previousFenceReceiptDigest"]
    generation_data = canonical(generation)
    # For recovery the on-disk prior receipt is replaced only after the
    # independently durable fence has committed and the disk is attached here.
    if finalize_only:
        if private(generation_path, 4096) != generation_data:
            raise ValueError("finalization fenced generation changed")
    elif not already_new:
        atomic(generation_path, generation_data)
    for path, journal in journals:
        data = canonical(journal)
        if path.exists():
            if json.loads(private(path)) != journal:
                raise ValueError("retained template journal differs from signed finalization")
        else:
            atomic(path, data)
    if finalize_only and journals:
        envelope_data = private(Path('/etc/luci/azure-operator-finalization.json'), 64 << 10)
        atomic(Path(value['mountPath']) / 'luci/operator-finalization-receipt.json', canonical({
            'version': 1, 'envelopeSHA256': sha(envelope_data), 'generation': value['generation'],
            'controllerInstanceId': value['controllerInstanceId']}))
    operator = bound_operator(operator, value, disk_path, generation_path, disk_data, generation_data)
    atomic(Path(value["operatorOutputPath"]), canonical(operator))
    if "previousFinalizationSHA256" in value:
        envelope_data = private(Path('/etc/luci/azure-operator-finalization.json'), 64 << 10)
        atomic(Path(value['mountPath']) / 'luci/operator-finalization-receipt.json', canonical({
            'version': 1, 'envelopeSHA256': sha(envelope_data), 'generation': value['generation'],
            'controllerInstanceId': value['controllerInstanceId'],
            'previousEnvelopeSHA256': value['previousFinalizationSHA256']}))
    atomic(Path('/etc/luci/azure-controller-disk-active.json'), canonical(value))
    return {"state": "mounted", "generation": value["generation"], "diskReceiptSHA256": sha(disk_data),
            "generationReceiptSHA256": sha(generation_data), "operatorSHA256": sha(canonical(operator))}


def finalized_operator(value, draft, allow_deferred=False, retained=False):
    """Verify the image-pinned source and operator signature before formatting.

    Legacy complete operator selections retain their existing behavior. A
    deferred selection is never runnable without exact signed finalization.
    """
    binding = draft.get("setupDeferred")
    if binding is None:
        return draft, []
    if allow_deferred and not Path("/etc/luci/azure-operator-finalization.json").exists():
        # Account management may initialize its owned disk, while the trusted
        # worker launcher explicitly refuses this deferred task operator.
        return draft, []
    envelope = json.loads(private(Path("/etc/luci/azure-operator-finalization.json"), 64 << 10))
    payload = envelope.get("payload", {})
    if (envelope.get("signatureAlgorithm") != "ecdsa-sha256" or
            payload.get("version") != 1 or payload.get("kind") != "luci-azure-operator-finalization" or
            payload.get("sourceCommit") != binding.get("sourceCommit") or
            payload.get("templateSHA256") != value["operatorTemplateSHA256"] or
            payload.get("scope") != value["scope"] or payload.get("generation") != value["generation"] or
            not UUID.fullmatch(payload.get("controllerBootID", "")) or
            not retained and payload.get("controllerBootID") != boot_identity() or
            payload.get("controllerInstanceId") != value["controllerInstanceId"] or payload.get("diskID") != value["azureDataDiskId"]):
        raise ValueError("deferred operator finalization is not bound to the selected source, disk and generation")
    if "previousFinalizationSHA256" in value and (
            sha(canonical(envelope)) != value["finalizationSHA256"] or
            payload.get("previousFinalizationSHA256") != value["previousFinalizationSHA256"] or
            payload.get("previousControllerInstanceID") != value["previousControllerInstanceID"] or
            payload.get("previousGeneration") != value["previousGeneration"] or
            payload.get("previousBootID") != value["previousBootID"] or
            payload.get("controllerBootID") != value["targetBootID"]):
        raise ValueError("signed successor does not bind the predecessor envelope and target boot")
    with tempfile.TemporaryDirectory() as directory:
        key, message, signature = (Path(directory) / name for name in ("key", "message", "signature"))
        key.write_text(binding["signerPublicKey"])
        message.write_bytes(canonical(payload))
        signature.write_bytes(base64.b64decode(envelope.get("signature", ""), validate=True))
        if call(["openssl", "dgst", "-sha256", "-verify", str(key), "-signature", str(signature), str(message)])[0] != 0:
            raise ValueError("deferred operator signature is invalid")
    operator = payload["operator"]
    if ("setupDeferred" in operator or operator.get("deployment", {}).get("scope") != value["scope"] or
            operator.get("sourceCommit") != binding.get("sourceCommit") or
            operator.get("deployment", {}).get("azureDataDiskId") != value["azureDataDiskId"] or
            operator.get("store", {}).get("ownerID") != value["ownerId"]):
        raise ValueError("finalized operator changes the selected controller authority")
    expected_create = PurePosixPath(value["mountPath"]) / "luci/template/create.json"
    expected_build = PurePosixPath(value["mountPath"]) / "luci/template/build.json"
    if operator.get("templateCreateJournalPath") != str(expected_create) or operator.get("templateBuildJournalPath") != str(expected_build):
        raise ValueError("finalized template journals are outside the exact durable location")
    return operator, [(Path(expected_create), payload["createJournal"]),
                      (Path(str(expected_create) + ".response"), payload["createResponse"]),
                      (Path(expected_build), payload["buildJournal"])]


def current_controller():
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    request = urllib.request.Request('http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01', headers={'Metadata': 'true'})
    with opener.open(request, timeout=3) as response:
        data = response.read(8193)
    if len(data) > 8192:
        raise ValueError('controller metadata exceeded bounds')
    return json.loads(data)['resourceId']


def resume(value):
    """Explicit same-controller/generation reboot; no format, writes or resign.

    The original signed boot is installation evidence. A retained receipt on
    the authoritative disk permits remount after reboot; the account API still
    checks the off-disk writer fence and the desktop must import its new boot pin.
    Replacement controller/generation uses the separate fenced recovery path.
    """
    selected(value)
    if current_controller().lower() != value['controllerInstanceId'].lower():
        raise ValueError('resume is only for the exact existing controller')
    template_data = private(Path(value['operatorTemplatePath']))
    if sha(template_data) != value['operatorTemplateSHA256']:
        raise ValueError('resume operator template source changed')
    mount_exact(dict(value, mode='restore'))
    installed = json.loads(private(Path(value['operatorOutputPath'])))
    operator, journals = finalized_operator(value, json.loads(template_data), allow_deferred='setupDeferred' in installed, retained=True)
    if journals:
        envelope_data = private(Path('/etc/luci/azure-operator-finalization.json'), 64 << 10)
        expected = {'version': 1, 'envelopeSHA256': sha(envelope_data), 'generation': value['generation'],
                    'controllerInstanceId': value['controllerInstanceId']}
        if 'previousFinalizationSHA256' in value:
            expected['previousEnvelopeSHA256'] = value['previousFinalizationSHA256']
        receipt = json.loads(private(Path(value['mountPath']) / 'luci/operator-finalization-receipt.json'))
        if receipt != expected:
            raise ValueError('resume requires the retained exact signed installation receipt')
        if installed.get('template') != operator.get('template') or installed.get('sourceCommit') != operator.get('sourceCommit') or 'setupDeferred' in installed:
            raise ValueError('retained operator changed after signed installation')
    return inspect(value, retained=True)


def boot_identity():
    return Path("/proc/sys/kernel/random/boot_id").read_text().strip()


def inspect(value, retained=False):
    """Read back a prior attach/mount without formatting or writing a byte."""
    selected(value)
    mount = Path(value["mountPath"])
    device = Path(value["device"])
    if not os.path.ismount(mount) or not device.exists() or not stat.S_ISBLK(device.stat().st_mode) or os.stat(mount).st_dev != os.stat(device).st_rdev:
        raise ValueError("controller disk is not the selected mounted block device")
    if call(["blkid", "-p", "-o", "value", "-s", "TYPE", str(device)]) != (0, "ext4") or call(["blkid", "-p", "-o", "value", "-s", "UUID", str(device)]) != (0, value["filesystemUuid"]):
        raise ValueError("controller filesystem changed")
    disk_data = private(Path(value["diskReceiptPath"]), 4096)
    disk = json.loads(disk_data)
    if disk != {"version": 1, "azureDataDiskId": value["azureDataDiskId"], "filesystemUuid": value["filesystemUuid"],
                "ownerId": value["ownerId"], "scope": value["scope"], "controllerHostId": value["controllerHostId"],
                "runtimeHostId": value["runtimeHostId"]}:
        raise ValueError("controller disk receipt changed")
    generation_data = private(Path(value["generationReceiptPath"]), 4096)
    generation = json.loads(generation_data)
    if generation.get("generation") != value["generation"] or generation.get("controllerInstanceId") != value["controllerInstanceId"] or generation.get("azureDataDiskId") != value["azureDataDiskId"]:
        raise ValueError("controller generation receipt changed")
    if value["mode"] == "restore" and (generation.get("previousGeneration") != value["previousGeneration"] or generation.get("previousFenceReceiptDigest") != value["previousFenceReceiptDigest"]):
        raise ValueError("controller predecessor fence changed")
    output = private(Path(value["operatorOutputPath"]))
    operator = json.loads(output)
    if "previousFinalizationSHA256" in value:
        if not retained and boot_identity() != value["targetBootID"]:
            raise ValueError("successor controller boot changed")
        envelope = private(Path('/etc/luci/azure-operator-finalization.json'), 64 << 10)
        if sha(envelope) != value["finalizationSHA256"]:
            raise ValueError("successor signed envelope changed")
        receipt = json.loads(private(mount / 'luci/operator-finalization-receipt.json', 4096))
        if receipt != {"version": 1, "envelopeSHA256": value["finalizationSHA256"],
                       "generation": value["generation"], "controllerInstanceId": value["controllerInstanceId"],
                       "previousEnvelopeSHA256": value["previousFinalizationSHA256"]}:
            raise ValueError("successor installation receipt is absent or changed")
        template = private(Path(value['operatorTemplatePath']))
        if sha(template) != value['operatorTemplateSHA256']:
            raise ValueError('successor operator template changed')
        signed_operator, journals = finalized_operator(value, json.loads(template), retained=retained)
        expected_operator = bound_operator(signed_operator, value, Path(value['diskReceiptPath']),
                                           Path(value['generationReceiptPath']), disk_data, generation_data)
        if output != canonical(expected_operator):
            raise ValueError('successor installed operator differs from signed source and fence')
        for path, expected in journals:
            if json.loads(private(path)) != expected:
                raise ValueError('successor retained task template journal changed')
    if operator.get("azureMount", {}).get("generationReceiptDigest") != sha(generation_data) or operator.get("store", {}).get("separationReceiptDigest") != sha(disk_data):
        raise ValueError("controller operator selection is stale")
    return {"state": "mounted", "generation": value["generation"], "diskReceiptSHA256": sha(disk_data),
            "generationReceiptSHA256": sha(generation_data), "operatorSHA256": sha(output)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--inspect", action="store_true")
    parser.add_argument("--finalize", action="store_true")
    parser.add_argument("--resume", action="store_true")
    parser.add_argument("selection", type=Path)
    args = parser.parse_args()
    active = Path('/etc/luci/azure-controller-disk-active.json')
    value = json.loads(private(active if args.resume and active.exists() else args.selection))
    if sum((args.inspect, args.finalize, args.resume)) > 1:
        raise ValueError("choose inspection, explicit resume or signed finalization")
    print(json.dumps(resume(value) if args.resume else inspect(value) if args.inspect else bind(value, args.finalize), sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, KeyError, json.JSONDecodeError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
