#!/usr/bin/env python3
"""Deliver one generation to an already-owned Azure E2B VMSS host.

Runs on the active controller against its authoritative ext4 journal. Azure
RunCommand is the protected guest transport; no SSH or E2B port is opened.
An uncertain RunCommand response is retained and is never retried by this
tool. The selected host's own service journals gateway effects separately.
"""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


ARM = "https://management.azure.com"
IMDS = "http://169.254.169.254/metadata/identity/oauth2/token"
HEX = re.compile(r"[0-9a-f]{64}\Z")
UUID = re.compile(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z")
ARM_ID = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft\.compute/virtualmachinescalesets/[a-z0-9_-]+\Z")
VM_ID = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft\.compute/virtualmachines/[a-z0-9_-]+\Z")
DISK_ID = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft\.compute/disks/[a-z0-9_-]+\Z")
GEN = re.compile(r"[a-z0-9][a-z0-9-]{1,127}\Z")
RECEIPT_PREFIX = "LUCI_AZURE_HOST_ROTATION_RECEIPT="


def digest(data):
    return hashlib.sha256(data).hexdigest()


def private_json(path, limit=16384):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or path.is_symlink() or info.st_mode & 0o077 or info.st_size > limit:
        raise ValueError("private delivery input is unavailable")
    return json.loads(path.read_bytes())


def atomic(path, value):
    raw = (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()
    path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix=".delivery-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as out:
            out.write(raw)
            out.flush()
            os.fsync(out.fileno())
        os.replace(name, path)
        parent = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(parent)
        finally:
            os.close(parent)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def bound_generation_receipt(receipt, host, selection):
    same_generation = receipt.get("generation") == host["previousGeneration"]
    fenced_successor = (receipt.get("generation") == host["generation"] and
                        receipt.get("previousGeneration") == host["previousGeneration"] and
                        HEX.fullmatch(receipt.get("previousFenceReceiptDigest", "")) is not None)
    return (receipt.get("controllerInstanceId") == selection["controllerInstanceID"] and
            receipt.get("azureDataDiskId") == selection["azureDataDiskID"] and
            (same_generation or fenced_successor))


def validate(selection, host):
    required = {"version", "controllerClientID", "controllerInstanceID", "azureDataDiskID", "scaleSetID",
                "instanceID", "controllerGenerationReceiptSHA256", "controllerGenerationReceiptPath",
                "journalPath", "controllerMountPath", "controllerFilesystemUUID", "hostConfigSHA256", "hostConfig"}
    if set(selection) != required or selection["version"] != 1 or not UUID.fullmatch(selection["controllerClientID"]):
        raise ValueError("delivery selection fields are invalid")
    for key, pattern in (("controllerInstanceID", VM_ID), ("azureDataDiskID", DISK_ID), ("scaleSetID", ARM_ID)):
        if not pattern.fullmatch(selection[key]) or selection[key] != selection[key].lower():
            raise ValueError("delivery Azure identity is invalid")
    if not re.fullmatch(r"[0-9]+", selection["instanceID"]) or len(selection["instanceID"]) > 12:
        raise ValueError("runtime VMSS instance identity is invalid")
    if not HEX.fullmatch(selection["controllerGenerationReceiptSHA256"]) or not HEX.fullmatch(selection["hostConfigSHA256"]):
        raise ValueError("delivery receipt hash is invalid")
    if host != selection["hostConfig"] or host.get("controllerClientID") != selection["controllerClientID"] or not GEN.fullmatch(host.get("generation", "")) or not GEN.fullmatch(host.get("previousGeneration", "")):
        raise ValueError("host selection does not bind the controller generation")
    if digest(json.dumps(host, sort_keys=True, separators=(",", ":")).encode()) != selection["hostConfigSHA256"]:
        raise ValueError("host selection digest is invalid")
    receipt_path = Path(selection["controllerGenerationReceiptPath"])
    journal_path = Path(selection["journalPath"])
    mount = Path(selection["controllerMountPath"])
    if not UUID.fullmatch(selection["controllerFilesystemUUID"]) or not mount.is_absolute() or not receipt_path.is_absolute() or not journal_path.is_absolute() or receipt_path == journal_path or not journal_path.parent.is_dir():
        raise ValueError("controller delivery paths are invalid")
    if any(path.is_symlink() or mount not in path.parents for path in (receipt_path, journal_path)) or mount.is_symlink() or not os.path.ismount(mount):
        raise ValueError("controller delivery journal is outside the exact disk mount")
    lines = Path("/proc/self/mountinfo").read_text().splitlines()
    if not any(line.partition(" - ")[0].split()[4] == str(mount) and line.partition(" - ")[2].startswith("ext4 ") for line in lines):
        raise ValueError("controller delivery disk is not ext4")
    device = Path("/dev/disk/by-uuid", selection["controllerFilesystemUUID"])
    if not device.exists() or os.stat(device).st_rdev != os.stat(mount).st_dev or os.stat(receipt_path).st_dev != os.stat(mount).st_dev or os.stat(journal_path.parent).st_dev != os.stat(mount).st_dev:
        raise ValueError("controller delivery disk identity changed")
    receipt = private_json(receipt_path, 4096)
    if digest(receipt_path.read_bytes()) != selection["controllerGenerationReceiptSHA256"] or not bound_generation_receipt(receipt, host, selection):
        raise ValueError("current controller generation receipt changed")


class AzureTransport:
    def __init__(self, client_id):
        self.client_id = client_id
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        token_url = IMDS + "?api-version=2018-02-01&resource=" + urllib.parse.quote(ARM, safe="") + "&client_id=" + client_id
        request = urllib.request.Request(token_url, headers={"Metadata": "true"})
        with self.opener.open(request, timeout=8) as response:
            payload = json.load(response)
        self.token = payload["access_token"]

    def request(self, method, url, body=None):
        parts = urllib.parse.urlsplit(url)
        if parts.scheme != "https" or parts.netloc != "management.azure.com" or parts.username or parts.password or parts.fragment or not parts.path.startswith("/subscriptions/"):
            raise ValueError("Azure response URL is outside selected ARM boundary")
        data = None if body is None else json.dumps(body, sort_keys=True, separators=(",", ":")).encode()
        request = urllib.request.Request(url, data=data, method=method, headers={"Authorization": "Bearer " + self.token, "Accept": "application/json", "Content-Type": "application/json"})
        with self.opener.open(request, timeout=20) as response:
            raw = response.read(64 << 10)
            if response.read(1):
                raise ValueError("Azure response exceeds the bound")
            return response.status, dict(response.headers), json.loads(raw or b"{}")


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise ValueError("Azure management redirect refused")


def readback(transport, selection):
    disk = transport.request("GET", ARM + selection["azureDataDiskID"] + "?api-version=2023-10-02")[2]
    extended = [value.lower() for value in disk.get("managedByExtended", [])]
    if disk.get("id", "").lower() != selection["azureDataDiskID"] or disk.get("managedBy", "").lower() != selection["controllerInstanceID"] or disk.get("properties", {}).get("diskState") != "Attached" or extended not in ([], [selection["controllerInstanceID"]]) or (disk.get("properties", {}).get("maxShares") or 1) != 1:
        raise ValueError("controller disk attachment is not current")
    instance_path = selection["scaleSetID"] + "/virtualMachines/" + selection["instanceID"]
    instance = transport.request("GET", ARM + instance_path + "?api-version=2023-09-01")[2]
    if instance.get("id", "").lower() != instance_path.lower() or instance.get("properties", {}).get("provisioningState") != "Succeeded":
        raise ValueError("selected runtime host is not current")


def script(host):
    encoded = base64.b64encode((json.dumps(host, sort_keys=True, separators=(",", ":")) + "\n").encode()).decode()
    config_hash = digest((json.dumps(host, sort_keys=True, separators=(",", ":")) + "\n").encode())
    name = "/run/luci_embed/selection-" + config_hash + ".json"
    # Only base64 data and a hash enter this fixed script. RunCommand is
    # scoped to the selected VMSS instance by Azure RBAC and its guest agent.
    return "\n".join(("set -eu", "umask 077", "test ! -e " + name,
                      "printf '%s' '" + encoded + "' | base64 -d > " + name,
                      "printf '%s  %s\\n' '" + config_hash + "' '" + name + "' | sha256sum -c - >/dev/null",
                      "trap 'rm -f " + name + "' EXIT",
                      "python3 /opt/luci/azure-embed/package/bin/service.py deliver /etc/luci/azure-embed-deployment.json " + name + " | sed 's/^/" + RECEIPT_PREFIX + "/'"))


def exact_receipt(payload, host, expected_hash):
    entries = payload.get("value", [])
    lines = [line[len(RECEIPT_PREFIX):] for item in entries for line in item.get("message", "").splitlines() if line.startswith(RECEIPT_PREFIX)]
    if len(lines) != 1:
        return None
    value = json.loads(lines[0])
    if value != {"state": "active", "generation": host["generation"], "configSHA256": expected_hash}:
        return None
    return value


def run(selection, transport):
    host = selection["hostConfig"]
    validate(selection, host)
    readback(transport, selection)
    journal = Path(selection["journalPath"])
    if journal.exists():
        raise ValueError("previous host delivery requires exact reconciliation")
    record = {"version": 1, "state": "run_command_pending", "scaleSetID": selection["scaleSetID"],
              "instanceID": selection["instanceID"], "controllerInstanceID": selection["controllerInstanceID"],
              "azureDataDiskID": selection["azureDataDiskID"], "previousGeneration": host["previousGeneration"],
              "generation": host["generation"], "hostConfigSHA256": selection["hostConfigSHA256"]}
    atomic(journal, record)
    url = ARM + selection["scaleSetID"] + "/virtualMachines/" + selection["instanceID"] + "/runCommand?api-version=2023-09-01"
    try:
        status, headers, payload = transport.request("POST", url, {"commandId": "RunShellScript", "script": [script(host)]})
    except Exception:
        # The exact operation may have reached the host. No automatic retry.
        raise RuntimeError("Azure RunCommand outcome is unknown; retained journal requires read-only reconciliation") from None
    operation = headers.get("Azure-AsyncOperation") or headers.get("azure-asyncoperation")
    result_url = headers.get("Location") or headers.get("location")
    if operation:
        parts = urllib.parse.urlsplit(operation)
        if parts.scheme != "https" or parts.netloc != "management.azure.com" or not parts.path.startswith("/subscriptions/"):
            raise RuntimeError("Azure operation identity is untrusted; retained delivery is uncertain")
        record["operationURL"] = operation
        if result_url:
            result_parts = urllib.parse.urlsplit(result_url)
            if result_parts.scheme != "https" or result_parts.netloc != "management.azure.com" or not result_parts.path.startswith("/subscriptions/"):
                raise RuntimeError("Azure result identity is untrusted; retained delivery is uncertain")
            record["resultURL"] = result_url
        record["state"] = "operation_acknowledged"
        atomic(journal, record)
    expected_host_hash = digest(json.dumps(host, sort_keys=True, separators=(",", ":")).encode())
    if status == 202 and operation:
        for _ in range(6):
            time.sleep(2)
            try:
                poll_status, _, polled = transport.request("GET", operation)
            except Exception:
                raise RuntimeError("Azure acknowledged RunCommand but operation readback is unavailable; retained delivery is uncertain") from None
            if poll_status == 200 and polled.get("status") == "Succeeded":
                payload = polled
                status = 200
                break
            if polled.get("status") in ("Failed", "Canceled"):
                raise RuntimeError("Azure RunCommand failed; retained host state requires exact readback")
        if status == 200 and result_url and exact_receipt(payload, host, expected_host_hash) is None:
            try:
                status, _, payload = transport.request("GET", result_url)
            except Exception:
                raise RuntimeError("Azure RunCommand settled but exact host output is unavailable; retained delivery is uncertain") from None
    if status != 200 or exact_receipt(payload, host, expected_host_hash) is None:
        raise RuntimeError("Azure delivery response lacks the exact settled host receipt; inspect retained operation/host before any new delivery")
    readback(transport, selection)
    record["state"] = "host_rotation_observed"
    record["hostReceipt"] = exact_receipt(payload, host, expected_host_hash)
    atomic(journal, record)
    return record


def reconcile(selection, transport):
    """Inspect the exact retained host; never repeat its delivery/rotation."""
    validate(selection, selection['hostConfig'])
    path = Path(selection['journalPath'])
    record = private_json(path)
    if any(record.get(key) != selection[key] for key in ('scaleSetID', 'instanceID', 'controllerInstanceID', 'azureDataDiskID', 'hostConfigSHA256')) or record.get('generation') != selection['hostConfig']['generation']:
        raise ValueError('retained host delivery selects another generation or resource')
    readback(transport, selection)
    inspection = "set -eu\npython3 /opt/luci/azure-embed/package/bin/service.py inspect /etc/luci/azure-embed-deployment.json | sed 's/^/" + RECEIPT_PREFIX + "/'"
    status, headers, response = transport.request('POST', ARM + selection['scaleSetID'] + '/virtualMachines/' + selection['instanceID'] + '/runCommand?api-version=2023-09-01',
        {'commandId': 'RunShellScript', 'script': [inspection]})
    if status == 202:
        operation = headers.get('Azure-AsyncOperation') or headers.get('azure-asyncoperation')
        result = headers.get('Location') or headers.get('location')
        prefix = '/subscriptions/' + selection['scaleSetID'].split('/')[2] + '/'
        for url in (operation, result):
            parsed = urllib.parse.urlsplit(url or '')
            if parsed.scheme != 'https' or parsed.netloc != 'management.azure.com' or not parsed.path.lower().startswith(prefix.lower()):
                raise RuntimeError('host inspection has no exact trusted operation/result identity')
        for _ in range(6):
            time.sleep(2)
            polled_status, _, polled = transport.request('GET', operation)
            if polled_status == 200 and polled.get('status') == 'Succeeded':
                status, _, response = transport.request('GET', result)
                break
            if polled.get('status') in ('Failed', 'Canceled'): break
    receipt = exact_receipt(response, selection['hostConfig'], selection['hostConfigSHA256']) if status == 200 else None
    if receipt is None: raise RuntimeError('exact host delivery remains uncertain; no repeated rotation')
    readback(transport, selection)
    record.update(state='host_rotation_observed', hostReceipt=receipt)
    atomic(path, record)
    return record


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("selection", type=Path)
    args = parser.parse_args()
    selection = private_json(args.selection)
    run(selection, AzureTransport(selection["controllerClientID"]))
    print(json.dumps({"state": "host_rotation_observed", "generation": selection["hostConfig"]["generation"]}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, KeyError, urllib.error.URLError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
