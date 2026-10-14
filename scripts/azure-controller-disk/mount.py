#!/usr/bin/env python3
"""Mount the one fenced Azure controller disk and bind Luci's operator config.

Runs only on the selected private controller VM as root. `initialize` may
format a proven blank LUN0 once; `restore` never formats or copies state.
All application state, journal and task artifacts remain on the same disk.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile


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
    if set(value) not in (required, required | {"previousGeneration", "previousGenerationReceiptSHA256", "previousFenceReceiptDigest"}) or value["version"] != 1 or value["mode"] not in ("initialize", "restore") or value["device"] != str(DEVICE):
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


def bind(value):
    selected(value)
    Path(value["mountPath"]).mkdir(mode=0o700, parents=True, exist_ok=True)
    mount_exact(value)
    disk_path = Path(value["diskReceiptPath"])
    generation_path = Path(value["generationReceiptPath"])
    disk_record = {"version": 1, "azureDataDiskId": value["azureDataDiskId"], "filesystemUuid": value["filesystemUuid"],
                   "ownerId": value["ownerId"], "scope": value["scope"], "controllerHostId": value["controllerHostId"],
                   "runtimeHostId": value["runtimeHostId"]}
    disk_data = canonical(disk_record)
    if value["mode"] == "initialize":
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
    generation = {"version": 1, "generation": value["generation"], "controllerInstanceId": value["controllerInstanceId"],
                  "azureDataDiskId": value["azureDataDiskId"]}
    if old_generation:
        generation["previousGeneration"] = value["previousGeneration"]
        generation["previousFenceReceiptDigest"] = value["previousFenceReceiptDigest"]
    generation_data = canonical(generation)
    # For recovery the on-disk prior receipt is replaced only after the
    # independently durable fence has committed and the disk is attached here.
    if not already_new:
        atomic(generation_path, generation_data)
    template_path = Path(value["operatorTemplatePath"])
    template_bytes = private(template_path)
    if sha(template_bytes) != value["operatorTemplateSHA256"]:
        raise ValueError("controller operator template changed")
    operator = json.loads(template_bytes)
    operator["azureMount"] = {"mountPath": value["mountPath"], "filesystemUuid": value["filesystemUuid"],
                              "receiptPath": str(disk_path), "generation": value["generation"],
                              "controllerInstanceId": value["controllerInstanceId"],
                              "generationReceiptPath": str(generation_path), "generationReceiptDigest": sha(generation_data),
                              "fenceBlobURL": value["fenceBlobURL"]}
    operator["store"]["separationReceiptDigest"] = sha(disk_data)
    operator["deployment"]["hostGeneration"] = value["generation"]
    gateway_contract = ["luci-azure-e2b-gateway-v1", operator["deployment"]["azureGatewayHost"],
                        value["scope"], value["generation"], "api:3000", "guest:3002", "envd:49983", "sandboxd:8080"]
    operator["azureGatewayReceiptDigest"] = sha(canonical(gateway_contract))
    atomic(Path(value["operatorOutputPath"]), canonical(operator))
    return {"state": "mounted", "generation": value["generation"], "diskReceiptSHA256": sha(disk_data),
            "generationReceiptSHA256": sha(generation_data), "operatorSHA256": sha(canonical(operator))}


def inspect(value):
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
    if operator.get("azureMount", {}).get("generationReceiptDigest") != sha(generation_data) or operator.get("store", {}).get("separationReceiptDigest") != sha(disk_data):
        raise ValueError("controller operator selection is stale")
    return {"state": "mounted", "generation": value["generation"], "diskReceiptSHA256": sha(disk_data),
            "generationReceiptSHA256": sha(generation_data), "operatorSHA256": sha(output)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--inspect", action="store_true")
    parser.add_argument("selection", type=Path)
    args = parser.parse_args()
    value = json.loads(private(args.selection))
    print(json.dumps(inspect(value) if args.inspect else bind(value), sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, KeyError, json.JSONDecodeError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
