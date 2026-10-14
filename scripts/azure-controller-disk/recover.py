#!/usr/bin/env python3
"""Fence a lost Azure Luci controller and restore its exact managed disk.

Runs on the separate private recovery VM with the recovery managed identity.
The off-disk Blob ETag is the single-writer transition journal. Every Azure
effect is preceded by a conditional durable state change; uncertain effects
are read back and never blindly replayed. This tool neither releases Luci VM
reservations nor claims a final Azure billing receipt.
"""

import argparse
import base64
from datetime import datetime, timezone
import hashlib
import ipaddress
import importlib.util
import json
import os
from pathlib import Path
import re
import socket
import ssl
import stat
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


ARM = "https://management.azure.com"
STORAGE_SCOPE = "https://storage.azure.com/"
IMDS = "http://169.254.169.254/metadata/identity/oauth2/token"
PRIVATE_V4 = tuple(ipaddress.ip_network(prefix) for prefix in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"))
HEX = re.compile(r"[0-9a-f]{64}\Z")
UUID = re.compile(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z")
GEN = re.compile(r"[a-z0-9][a-z0-9-]{1,127}\Z")
VM = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft\.compute/virtualmachines/[a-z0-9_-]+\Z")
DISK = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft\.compute/disks/[a-z0-9_-]+\Z")
IDENTITY = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourcegroups/[a-z0-9_-]+/providers/microsoft\.managedidentity/userassignedidentities/[a-z0-9_-]+\Z")


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def digest(value):
    return hashlib.sha256(canonical(value)).hexdigest()


def private(path, limit=16 << 10):
    info = path.lstat()
    if path.is_symlink() or not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or info.st_size < 1 or info.st_size > limit:
        raise ValueError("private recovery selection is unavailable")
    with path.open('rb') as source:
        data = source.read(limit + 1)
    if len(data) > limit:
        raise ValueError("private recovery selection exceeded its bound")
    return json.loads(data)


def validate(value):
    required = {"version", "recoveryClientID", "oldControllerID", "newControllerID", "diskID", "fenceBlobURL",
                "controllerIdentityID", "recoveryIdentityID",
                "oldGeneration", "newGeneration", "filesystemUUID", "oldGenerationReceiptSHA256",
                "mountSelectionPath", "hostDeliverySelectionPath", "apiService", "apiHealthURL", "ownerID", "scope"}
    if set(value) not in (required, required | {'controllerSetupFiles'}, required | {'controllerSuccessor'}, required | {'controllerSuccessor', 'controllerHostDelivery'}) or value["version"] != 1 or not UUID.fullmatch(value["recoveryClientID"]) or not UUID.fullmatch(value["filesystemUUID"]):
        raise ValueError("controller recovery selection fields are invalid")
    if not VM.fullmatch(value["oldControllerID"]) or not VM.fullmatch(value["newControllerID"]) or value["oldControllerID"] == value["newControllerID"] or not DISK.fullmatch(value["diskID"]):
        raise ValueError("controller recovery ARM identities are invalid")
    if not IDENTITY.fullmatch(value["controllerIdentityID"]) or not IDENTITY.fullmatch(value["recoveryIdentityID"]) or value["controllerIdentityID"] == value["recoveryIdentityID"]:
        raise ValueError("controller and recovery managed identities are invalid")
    if not GEN.fullmatch(value["oldGeneration"]) or not GEN.fullmatch(value["newGeneration"]) or value["oldGeneration"] == value["newGeneration"] or not HEX.fullmatch(value["oldGenerationReceiptSHA256"]):
        raise ValueError("controller recovery generation is invalid")
    if not GEN.fullmatch(value["ownerID"]) or not GEN.fullmatch(value["scope"]):
        raise ValueError("controller recovery owner scope is invalid")
    if value["apiService"] != "luci-api.service":
        raise ValueError("controller API service identity is unsupported")
    health = urllib.parse.urlsplit(value["apiHealthURL"])
    if health.scheme != "http" or health.hostname != "127.0.0.1" or not health.port or health.path != "/v1/healthz" or health.query or health.fragment or health.username:
        raise ValueError("controller API health endpoint is not private")
    for key in ("mountSelectionPath", "hostDeliverySelectionPath"):
        if not Path(value[key]).is_absolute():
            raise ValueError("controller recovery input path is invalid")
    url = urllib.parse.urlsplit(value["fenceBlobURL"])
    if url.scheme != "https" or not re.fullmatch(r"[a-z0-9]{3,24}\.blob\.core\.windows\.net", url.netloc) or url.path != "/controller-fence/controller-fence.json" or url.query or url.fragment:
        raise ValueError("controller fence blob is not the selected private resource")
    if 'controllerSetupFiles' in value:
        validate_setup_files(value['controllerSetupFiles'])
    if 'controllerSuccessor' in value:
        validate_successor(value, value['controllerSuccessor'])


def validate_successor(selection, successor):
    required = {'version', 'sourceCommit', 'scope', 'ownerID', 'diskID', 'oldControllerID', 'newControllerID',
                'oldGeneration', 'newGeneration', 'oldBootID', 'newBootID', 'oldGenerationReceiptSHA256',
                'priorEnvelopeSHA256', 'newEnvelopeSHA256', 'newTemplateSHA256', 'newPublicCASHA256', 'files',
                'mountTemplate'}
    if not isinstance(successor, dict) or set(successor) != required or successor['version'] != 1 or \
            not re.fullmatch(r'[0-9a-f]{40}', successor['sourceCommit']) or \
            not all(UUID.fullmatch(successor[key]) for key in ('oldBootID', 'newBootID')) or \
            successor['oldBootID'] == successor['newBootID'] or \
            not all(HEX.fullmatch(successor[key]) for key in ('priorEnvelopeSHA256', 'newEnvelopeSHA256',
                                                           'newTemplateSHA256', 'newPublicCASHA256')) or \
            any(successor[key] != selection[key] for key in ('scope', 'ownerID', 'diskID', 'oldControllerID',
                                                               'newControllerID', 'oldGeneration', 'newGeneration',
                                                               'oldGenerationReceiptSHA256')):
        raise ValueError('signed successor differs from the exact recovery selection')
    files = successor['files']
    expected = {'/etc/luci/operator-template.json', '/etc/luci/azure-private-ca.crt',
                '/etc/luci/azure-controller.env', '/etc/luci/azure-operator-finalization.json'}
    if not isinstance(files, list) or len(files) != 4 or {row.get('path') for row in files if isinstance(row, dict)} != expected:
        raise ValueError('signed successor public file inventory differs')
    contents = {}
    for row in files:
        if set(row) != {'path', 'content', 'sha256'} or not isinstance(row['content'], str) or \
                not 0 < len(row['content'].encode()) <= 16384 or 'PRIVATE KEY-----' in row['content'] or \
                hashlib.sha256(row['content'].encode()).hexdigest() != row['sha256']:
            raise ValueError('signed successor public file is not bounded and exact')
        contents[row['path']] = row['content']
    draft = json.loads(contents['/etc/luci/operator-template.json'])
    envelope = json.loads(contents['/etc/luci/azure-operator-finalization.json'])
    payload = envelope.get('payload', {})
    if digest(draft) != successor['newTemplateSHA256'] or digest(envelope) != successor['newEnvelopeSHA256'] or \
            hashlib.sha256(contents['/etc/luci/azure-private-ca.crt'].encode()).hexdigest() != successor['newPublicCASHA256'] or \
            envelope.get('signatureAlgorithm') != 'ecdsa-sha256' or \
            any(payload.get(key) != successor[value] for key, value in (
                ('sourceCommit', 'sourceCommit'), ('scope', 'scope'), ('generation', 'newGeneration'),
                ('controllerInstanceId', 'newControllerID'), ('controllerBootID', 'newBootID'),
                ('diskID', 'diskID'), ('templateSHA256', 'newTemplateSHA256'),
                ('previousFinalizationSHA256', 'priorEnvelopeSHA256'),
                ('previousControllerInstanceID', 'oldControllerID'),
                ('previousGeneration', 'oldGeneration'), ('previousBootID', 'oldBootID'))) or \
            draft.get('setupDeferred', {}).get('sourceCommit') != successor['sourceCommit'] or \
            draft.get('setupDeferred', {}).get('generation') != successor['newGeneration'] or \
            not contents['/etc/luci/azure-controller.env'].startswith(
                'LUCI_ADDR=127.0.0.1:8730\nLUCI_BACKING=azure\nLUCI_AZURE_OPERATOR_TEMPLATE=/etc/luci/operator.json\n'):
        raise ValueError('signed successor source, public configuration or predecessor binding changed')
    mount = successor['mountTemplate']
    mount_keys = {'version', 'mode', 'device', 'mountPath', 'filesystemUuid', 'azureDataDiskId', 'ownerId',
                  'scope', 'controllerHostId', 'runtimeHostId', 'controllerInstanceId', 'generation',
                  'diskReceiptPath', 'generationReceiptPath', 'operatorTemplatePath', 'operatorTemplateSHA256',
                  'operatorOutputPath', 'fenceBlobURL', 'previousGeneration', 'previousGenerationReceiptSHA256',
                  'previousControllerInstanceID', 'previousFinalizationSHA256', 'previousBootID', 'targetBootID',
                  'finalizationSHA256'}
    if not isinstance(mount, dict) or set(mount) != mount_keys or mount['version'] != 1 or mount['mode'] != 'restore' or \
            mount['device'] != '/dev/disk/azure/scsi1/lun0' or \
            not re.fullmatch(r'/mnt/[a-z0-9-]+', mount['mountPath']) or \
            not re.fullmatch(r'/etc/luci/[a-z0-9.-]+', selection['mountSelectionPath']) or \
            not re.fullmatch(r'/etc/luci/[a-z0-9.-]+', mount['operatorTemplatePath']) or \
            not re.fullmatch(r'/etc/luci/[a-z0-9.-]+', mount['operatorOutputPath']) or \
            mount['operatorTemplatePath'] != '/etc/luci/operator-template.json' or \
            mount['operatorOutputPath'] != '/etc/luci/operator.json' or \
            not all(GEN.fullmatch(mount[key]) for key in ('controllerHostId', 'runtimeHostId')) or \
            any(mount[key] != selection[selection_key] for key, selection_key in (
                ('filesystemUuid', 'filesystemUUID'), ('azureDataDiskId', 'diskID'), ('ownerId', 'ownerID'),
                ('scope', 'scope'), ('controllerInstanceId', 'newControllerID'), ('generation', 'newGeneration'),
                ('fenceBlobURL', 'fenceBlobURL'), ('previousGeneration', 'oldGeneration'),
                ('previousGenerationReceiptSHA256', 'oldGenerationReceiptSHA256'))) or \
            any(mount[key] != successor[successor_key] for key, successor_key in (
                ('operatorTemplateSHA256', 'newTemplateSHA256'), ('previousControllerInstanceID', 'oldControllerID'),
                ('previousFinalizationSHA256', 'priorEnvelopeSHA256'), ('previousBootID', 'oldBootID'),
                ('targetBootID', 'newBootID'), ('finalizationSHA256', 'newEnvelopeSHA256'))) or \
            mount['diskReceiptPath'] != mount['mountPath'] + '/luci/disk-receipt.json' or \
            mount['generationReceiptPath'] != mount['mountPath'] + '/luci/generation.json':
        raise ValueError('signed successor mount template is not exact source-bound restore input')


def validate_setup_files(files):
    paths = {'/etc/luci/operator-template.json', '/etc/luci/azure-private-ca.crt', '/etc/luci/azure-controller.env'}
    if not isinstance(files, list) or len(files) != 3 or {row.get('path') for row in files} != paths:
        raise ValueError('Controller post-create setup inventory differs')
    for row in files:
        if set(row) != {'path', 'content', 'sha256'} or not isinstance(row['content'], str) or not 0 < len(row['content']) <= 16384 or \
                hashlib.sha256(row['content'].encode()).hexdigest() != row['sha256'] or 'PRIVATE KEY-----' in row['content']:
            raise ValueError('Controller setup is not exact bounded nonsecret input')
    operator = json.loads(next(row['content'] for row in files if row['path'].endswith('operator-template.json')))
    if not operator.get('setupDeferred') or any(operator.get(key) for key in ('apiKeyFile', 'adminApiKeyFile', 'attachKeyFile', 'clientCertFile', 'clientKeyFile')):
        raise ValueError('Controller setup must remain deferred with managed identity references')
    environment = next(row['content'] for row in files if row['path'].endswith('.env'))
    if not environment.startswith('LUCI_ADDR=127.0.0.1:8730\nLUCI_BACKING=azure\nLUCI_AZURE_OPERATOR_TEMPLATE=/etc/luci/operator.json\n'):
        raise ValueError('Controller management setup must retain account gateway and loopback API')


def setup_script(files, inspect=False):
    validate_setup_files(files)
    return """python3 - <<'PY'
import base64, hashlib, json, os, pathlib, tempfile
files = json.loads(base64.b64decode(%s))
for row in files:
    target = pathlib.Path(row['path'])
    data = row['content'].encode()
    if target.exists() or target.is_symlink():
        if target.is_symlink() or target.stat().st_mode & 0o077 or target.read_bytes() != data:
            raise SystemExit('retained post-create controller configuration changed')
    elif %s:
        raise SystemExit('post-create controller setup remains uncertain')
    else:
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        fd, name = tempfile.mkstemp(dir=target.parent)
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, 'wb') as output:
            output.write(data); output.flush(); os.fsync(output.fileno())
        os.replace(name, target)
PY
""" % (repr(base64.b64encode(canonical(files)).decode()), repr(inspect))


def successor_script(successor, fence_digest, inspect=False, mount_path='/etc/luci/azure-controller-disk-restore.json', host_delivery=None):
    """Bounded public staging on the selected boot; inspect never writes."""
    if not HEX.fullmatch(fence_digest) or not re.fullmatch(r'/etc/luci/[a-z0-9.-]+', mount_path):
        raise ValueError('successor staging needs exact durable fence and mount path')
    mount = dict(successor['mountTemplate'], previousFenceReceiptDigest=fence_digest)
    mount_bytes = canonical(mount)
    files = successor['files'] + [{'path': mount_path, 'content': mount_bytes.decode(),
                                   'sha256': hashlib.sha256(mount_bytes).hexdigest()}]
    service_mount_path = '/etc/luci/azure-controller-disk-initial.json'
    if mount_path != service_mount_path:
        files.append({'path': service_mount_path, 'content': mount_bytes.decode(),
                      'sha256': hashlib.sha256(mount_bytes).hexdigest()})
    if host_delivery is not None:
        delivery_bytes = canonical(host_delivery)
        if len(delivery_bytes) > 16384: raise ValueError('successor host delivery exceeds the public bound')
        files.append({'path': '/etc/luci/azure-host-delivery.json', 'content': delivery_bytes.decode(),
            'sha256': hashlib.sha256(delivery_bytes).hexdigest()})
    payload = base64.b64encode(canonical({'boot': successor['newBootID'], 'files': files,
                                         'privateIP': host_delivery['hostConfig']['controllerPrivateIP'] if host_delivery else '',
                                         'digest': digest(successor)})).decode()
    return """python3 - <<'PY'
import base64, hashlib, json, os, pathlib, stat, tempfile, urllib.request
item = json.loads(base64.b64decode(%s))
if pathlib.Path('/proc/sys/kernel/random/boot_id').read_text().strip() != item['boot']:
    raise SystemExit('replacement controller boot differs from signed selection')
if item.get('privateIP'):
    request=urllib.request.Request('http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2021-02-01&format=text',headers={'Metadata':'true'})
    with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request,timeout=3) as response:
        if response.read(64).decode().strip()!=item['privateIP']:
            raise SystemExit('replacement private IP differs from signed ARM NIC observation')
for row in item['files']:
    target = pathlib.Path(row['path'])
    data = row['content'].encode()
    if hashlib.sha256(data).hexdigest() != row['sha256']:
        raise SystemExit('signed successor public file changed')
    if target.parent.is_symlink() or target.parent.exists() and not target.parent.is_dir():
        raise SystemExit('replacement configuration parent is unsafe')
    if target.is_symlink() or target.exists():
        if target.is_symlink() or not stat.S_ISREG(target.stat().st_mode) or target.stat().st_mode & 0o077 or target.stat().st_size != len(data) or target.read_bytes() != data:
            raise SystemExit('retained replacement controller configuration changed')
    elif %s:
        raise SystemExit('replacement controller staging remains uncertain')
    else:
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        if target.parent.is_symlink():
            raise SystemExit('replacement configuration parent changed')
        fd, name = tempfile.mkstemp(dir=target.parent)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, 'wb') as output:
                output.write(data); output.flush(); os.fsync(output.fileno())
            os.replace(name, target)
            parent = os.open(target.parent, os.O_RDONLY)
            try: os.fsync(parent)
            finally: os.close(parent)
        finally:
            if os.path.exists(name): os.unlink(name)
print('LUCI_AZURE_SUCCESSOR_STAGED=' + item['digest'])
PY
""" % (repr(payload), repr(inspect))


def successor_receipt(output, successor):
    rows = [line.split('=', 1)[1] for line in output.splitlines() if line.startswith('LUCI_AZURE_SUCCESSOR_STAGED=')]
    if rows != [digest(successor)]:
        raise RuntimeError('successor public staging has no exact boot-bound readback')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise ValueError("Azure recovery redirect refused")


class AzureHTTP:
    def __init__(self, client_id):
        self.client_id = client_id
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        self.tokens = {}

    def token(self, resource):
        if resource not in self.tokens:
            url = IMDS + "?api-version=2018-02-01&resource=" + urllib.parse.quote(resource, safe="") + "&client_id=" + self.client_id
            request = urllib.request.Request(url, headers={"Metadata": "true"})
            with self.opener.open(request, timeout=8) as response:
                self.tokens[resource] = json.load(response)["access_token"]
        return self.tokens[resource]

    def local_instance_id(self):
        request = urllib.request.Request("http://169.254.169.254/metadata/instance?api-version=2021-02-01", headers={"Metadata": "true"})
        with self.opener.open(request, timeout=8) as response:
            return json.load(response).get("compute", {}).get("resourceId", "").lower()

    def request(self, method, url, body=None, headers=None, storage=False):
        parsed = urllib.parse.urlsplit(url)
        if parsed.scheme != "https" or parsed.fragment or (storage and not re.fullmatch(r"[a-z0-9]{3,24}\.blob\.core\.windows\.net", parsed.netloc)) or (not storage and parsed.netloc != "management.azure.com"):
            raise ValueError("Azure recovery transport URL is invalid")
        if storage:
            addresses = socket.getaddrinfo(parsed.hostname, 443, type=socket.SOCK_STREAM)
            if not addresses or any(ipaddress.ip_address(row[4][0]).version != 4 or
                                    not any(ipaddress.ip_address(row[4][0]) in block for block in PRIVATE_V4)
                                    for row in addresses):
                raise ValueError("fence Blob DNS is not private")
        request_headers = {"Authorization": "Bearer " + self.token(STORAGE_SCOPE if storage else ARM),
                           "Accept": "application/json", "Content-Type": "application/json"}
        if storage:
            request_headers.update({"x-ms-version": "2023-11-03", "x-ms-date": datetime.now(timezone.utc).strftime("%a, %d %b %Y %H:%M:%S GMT")})
        request_headers.update(headers or {})
        data = body if isinstance(body, bytes) else None if body is None else canonical(body)
        request = urllib.request.Request(url, data=data, method=method, headers=request_headers)
        try:
            response = self.opener.open(request, timeout=20)
        except urllib.error.HTTPError as error:
            if storage and error.code == 404:
                return 404, {}, b""
            raise
        with response:
            data = response.read(64 << 10)
            if response.read(1):
                raise ValueError("Azure recovery response exceeds the bound")
            return response.status, dict(response.headers), data


class FenceBlob:
    def __init__(self, transport, url):
        self.transport, self.url = transport, url

    def read(self):
        status, headers, data = self.transport.request("GET", self.url, storage=True)
        if status == 404:
            return None, None
        etag = headers.get("ETag") or headers.get("Etag") or headers.get("etag")
        if status != 200 or not etag or len(data) > 8192:
            raise RuntimeError("off-disk controller fence read is incomplete")
        return json.loads(data), etag

    def write(self, record, etag):
        headers = {"x-ms-blob-type": "BlockBlob", "If-Match" if etag else "If-None-Match": etag or "*"}
        try:
            status, result, _ = self.transport.request("PUT", self.url, canonical(record), headers=headers, storage=True)
        except Exception:
            # A conditional write may have committed. Read it back; do not
            # issue a second PUT on an ambiguous response.
            current, current_etag = self.read()
            if current != record:
                raise RuntimeError("off-disk fence write outcome is unknown") from None
            return current_etag
        result_etag = result.get("ETag") or result.get("Etag") or result.get("etag")
        if status != 201 or not result_etag:
            raise RuntimeError("off-disk fence write was not acknowledged")
        return result_etag


class Compute:
    def __init__(self, transport):
        self.transport = transport

    def get(self, resource, version="2023-09-01"):
        status, headers, body = self.transport.request("GET", ARM + resource + "?api-version=" + version)
        if status != 200:
            raise RuntimeError("selected Azure resource read is incomplete")
        result = json.loads(body)
        if result.get("id", "").lower() != resource:
            raise ValueError("selected Azure resource identity changed")
        return result, headers.get("ETag") or result.get("etag")

    def power(self, vm_id):
        status, _, body = self.transport.request("GET", ARM + vm_id + "/instanceView?api-version=2023-09-01")
        if status != 200:
            raise RuntimeError("controller power readback is unavailable")
        states = [item.get("code") for item in json.loads(body).get("statuses", [])]
        return "PowerState/deallocated" in states

    def disk_owner(self, disk_id):
        record, _ = self.get(disk_id, "2023-10-02")
        owner = record.get("managedBy", "").lower()
        extended = [value.lower() for value in record.get("managedByExtended", [])]
        if extended not in ([], [owner]) or (record.get("properties", {}).get("maxShares") or 1) != 1:
            raise ValueError("controller disk became shared")
        return owner, record.get("properties", {}).get("diskState")

    def vm_disks(self, vm_id):
        record, etag = self.get(vm_id)
        return record.get("properties", {}).get("storageProfile", {}).get("dataDisks", []), etag

    def vm_identities(self, vm_id):
        record, etag = self.get(vm_id)
        return {key.lower() for key in record.get("identity", {}).get("userAssignedIdentities", {})}, etag

    def deallocate(self, vm_id):
        self.transport.request("POST", ARM + vm_id + "/deallocate?api-version=2023-09-01", {})

    def patch_disks(self, vm_id, disks, etag):
        if not etag:
            raise ValueError("controller VM ETag is missing")
        self.transport.request("PATCH", ARM + vm_id + "?api-version=2023-09-01",
                               {"properties": {"storageProfile": {"dataDisks": disks}}}, {"If-Match": etag})

    def patch_identities(self, vm_id, identities, etag):
        if not etag:
            raise ValueError("controller VM ETag is missing")
        self.transport.request("PATCH", ARM + vm_id + "?api-version=2023-09-01",
                               {"identity": {"type": "UserAssigned" if identities else "None", "userAssignedIdentities": {key: {} for key in identities}}},
                               {"If-Match": etag})

    def run_command(self, vm_id, script):
        status, headers, body = self.transport.request("POST", ARM + vm_id + "/runCommand?api-version=2023-09-01",
                                                       {"commandId": "RunShellScript", "script": [script]})
        if status == 202:
            operation = headers.get("Azure-AsyncOperation") or headers.get("azure-asyncoperation")
            result_url = headers.get("Location") or headers.get("location")
            if not operation or urllib.parse.urlsplit(operation).netloc != "management.azure.com" or not urllib.parse.urlsplit(operation).path.startswith("/subscriptions/"):
                raise RuntimeError("controller guest RunCommand has no trusted operation identity")
            for _ in range(8):
                time.sleep(2)
                poll_status, _, poll_body = self.transport.request("GET", operation)
                if poll_status == 200 and json.loads(poll_body).get("status") == "Succeeded":
                    status, body = poll_status, poll_body
                    break
                if json.loads(poll_body).get("status") in ("Failed", "Canceled"):
                    raise RuntimeError("controller guest RunCommand failed with retained exact operation")
            if status == 200 and result_url and urllib.parse.urlsplit(result_url).netloc == "management.azure.com" and urllib.parse.urlsplit(result_url).path.startswith("/subscriptions/"):
                status, _, body = self.transport.request("GET", result_url)
        if status != 200:
            # The command may be running, or its ACK was lost. A future
            # invocation may only issue the read-only inspection variant.
            raise RuntimeError("controller guest RunCommand has no exact settled response")
        result = json.loads(body)
        return "\n".join(item.get("message", "") for item in result.get("value", []))


def exact_disk_row(row, disk_id):
    return (len(row) == 1 and row[0].get("lun") == 0 and
            row[0].get("managedDisk", {}).get("id", "").lower() == disk_id)


def mount_receipt_from_output(output, generation):
    prefix = "LUCI_AZURE_CONTROLLER_MOUNT_RECEIPT="
    lines = [line[len(prefix):] for line in output.splitlines() if line.startswith(prefix)]
    if len(lines) != 1:
        raise RuntimeError("controller mount has no exact guest receipt")
    value = json.loads(lines[0])
    if value.get("state") != "mounted" or value.get("generation") != generation or not HEX.fullmatch(value.get("generationReceiptSHA256", "")) or not HEX.fullmatch(value.get("diskReceiptSHA256", "")):
        raise RuntimeError("controller mount receipt is not bound to the selected generation")
    return value


def initial_guest_script(health_url, inspect=False, setup_files=None):
    selection = "/etc/luci/azure-controller-disk-initial.json"
    command = "python3 /opt/luci/azure-controller/bin/mount.py " + ("--inspect " if inspect else "") + selection
    health = "python3 -c 'import urllib.request; assert urllib.request.urlopen(\"" + health_url + "\", timeout=3).status == 200'"
    return ("set -eu\n" + (setup_script(setup_files, inspect) if setup_files is not None else '') + "receipt=$(" + command + ")\n" + ("" if inspect else "systemctl start luci-api.service\n") +
            "systemctl is-active --quiet luci-api.service\n" + health + "\n" +
            "printf '%s\\n' 'LUCI_AZURE_CONTROLLER_MOUNT_RECEIPT='\"$receipt\"\n")


def initialize(selection, store, compute, reconcile=False):
    """Initial nonshared disk attach/format and off-disk authority creation.

    The first effect is recorded in the same private Blob used for recovery.
    A lost formatting response advances only through read-only guest inspect.
    """
    validate(selection)
    record, etag = store.read()
    if record is None:
        if reconcile:
            raise RuntimeError('initial fence write remains absent or uncertain; no repeated bootstrap effect')
        if compute.disk_owner(selection["diskID"]) != ("", "Unattached"):
            raise ValueError("initial controller disk is not unattached")
        vm, vm_etag = compute.get(selection["oldControllerID"])
        if vm.get("properties", {}).get("provisioningState") != "Succeeded" or vm.get("properties", {}).get("storageProfile", {}).get("dataDisks", []) or compute.vm_identities(selection["oldControllerID"])[0] != {selection["controllerIdentityID"]}:
            raise ValueError("initial controller disk or identity is not selected")
        record = {"version": 1, "state": "bootstrap_attach_pending", "diskID": selection["diskID"],
                  "filesystemUUID": selection["filesystemUUID"], "controllerID": selection["oldControllerID"],
                  "generation": selection["oldGeneration"], "ownerID": selection["ownerID"], "scope": selection["scope"]}
        etag = store.write(record, None)
        disk = {"lun": 0, "name": selection["diskID"].rsplit("/", 1)[-1], "createOption": "Attach",
                "caching": "None", "managedDisk": {"id": selection["diskID"]}}
        try:
            compute.patch_disks(selection["oldControllerID"], [disk], vm_etag)
        except Exception:
            pass
    if record.get("controllerID") != selection["oldControllerID"] or record.get("generation") != selection["oldGeneration"] or record.get("diskID") != selection["diskID"] or record.get("ownerID") != selection["ownerID"] or record.get("scope") != selection["scope"]:
        raise ValueError("initial controller fence belongs to a different deployment")
    if record["state"] == "bootstrap_attach_pending":
        rows, _ = compute.vm_disks(selection["oldControllerID"])
        if not exact_disk_row(rows, selection["diskID"]) or compute.disk_owner(selection["diskID"]) != (selection["oldControllerID"], "Attached"):
            raise RuntimeError("initial disk attach is unknown; no duplicate attach")
        record, etag = transition(store, record, etag, "bootstrap_mount_pending")
        try:
            output = compute.run_command(selection["oldControllerID"], initial_guest_script(selection["apiHealthURL"], setup_files=selection.get('controllerSetupFiles')))
        except Exception:
            raise RuntimeError("initial mount or format is uncertain; inspect the exact guest before retry") from None
        mount_receipt_from_output(output, selection["oldGeneration"])
    if record["state"] == "bootstrap_mount_pending":
        # Inspection reads exact block/receipt/config only. It cannot rerun
        # mkfs or create a second Luci controller from an uncertain POST.
        output = compute.run_command(selection["oldControllerID"], initial_guest_script(selection["apiHealthURL"], inspect=True, setup_files=selection.get('controllerSetupFiles')))
        guest = mount_receipt_from_output(output, selection["oldGeneration"])
        if guest["generationReceiptSHA256"] != selection["oldGenerationReceiptSHA256"]:
            raise ValueError("initial controller generation differs from the selected immutable receipt")
        record, etag = transition(store, record, etag, "active", generationReceiptSHA256=guest["generationReceiptSHA256"], diskReceiptSHA256=guest["diskReceiptSHA256"])
    return record


def transition(store, record, etag, state, **changes):
    updated = dict(record, state=state, **changes)
    return updated, store.write(updated, etag)


def restore_local(selection, fence_digest, inspect=False):
    spec = private(Path(selection["mountSelectionPath"]))
    if spec.get("mode") != "restore" or spec.get("azureDataDiskId") != selection["diskID"] or spec.get("controllerInstanceId") != selection["newControllerID"] or spec.get("generation") != selection["newGeneration"] or spec.get("ownerId") != selection["ownerID"] or spec.get("scope") != selection["scope"] or spec.get("previousGeneration") != selection["oldGeneration"] or spec.get("previousGenerationReceiptSHA256") != selection["oldGenerationReceiptSHA256"] or spec.get("previousFenceReceiptDigest") != fence_digest:
        raise ValueError("controller mount selection does not bind the off-disk fence")
    successor = selection.get('controllerSuccessor')
    if successor and any(spec.get(key) != expected for key, expected in (
            ('previousControllerInstanceID', successor['oldControllerID']),
            ('previousFinalizationSHA256', successor['priorEnvelopeSHA256']),
            ('previousBootID', successor['oldBootID']), ('targetBootID', successor['newBootID']),
            ('finalizationSHA256', successor['newEnvelopeSHA256']),
            ('operatorTemplateSHA256', successor['newTemplateSHA256']))):
        raise ValueError('controller mount selection does not bind the signed successor')
    module_path = Path(__file__).with_name("mount.py")
    spec_module = importlib.util.spec_from_file_location("luci_azure_mount", module_path)
    module = importlib.util.module_from_spec(spec_module)
    spec_module.loader.exec_module(module)
    return module.inspect(spec) if inspect else module.bind(spec)


def target_identity_script():
    return """set -eu
python3 - <<'PY'
import pathlib, urllib.request
request=urllib.request.Request('http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2021-02-01&format=text',headers={'Metadata':'true'})
with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request,timeout=3) as response:
    address=response.read(64).decode().strip()
print('LUCI_AZURE_SUCCESSOR_TARGET='+pathlib.Path('/proc/sys/kernel/random/boot_id').read_text().strip()+'|'+address)
PY
"""


def run(selection, store, compute, host_delivery=None, service=None, reconcile=False):
    validate(selection)
    successor = selection.get('controllerSuccessor')
    successor_hash = digest(successor) if successor else None
    record, etag = store.read()
    if record is None or record.get("version") != 1 or record.get("diskID") != selection["diskID"] or record.get("filesystemUUID") != selection["filesystemUUID"] or record.get("ownerID") != selection["ownerID"] or record.get("scope") != selection["scope"]:
        raise ValueError("selected authoritative fence is absent or foreign")
    if record["state"] != "active" and (record.get("targetControllerID") != selection["newControllerID"] or record.get("targetGeneration") != selection["newGeneration"] or record.get("controllerID") != selection["oldControllerID"] or record.get("generation") != selection["oldGeneration"]):
        raise ValueError("pending recovery belongs to another exact controller generation")
    if record['state'] != 'active' and record.get('successorDigest') != successor_hash:
        raise ValueError('pending recovery belongs to another signed successor')
    delivery = selection.get('controllerHostDelivery')
    if delivery:
        link = delivery.get('hostConfig', {}).get('controllerSuccessorLink', {}).get('payload', {})
        predecessor_hash = digest(record) if record['state'] == 'active' and record.get('generation') == selection['oldGeneration'] else record.get('predecessorFenceSHA256')
        predecessor_etag = etag if record['state'] == 'active' and record.get('generation') == selection['oldGeneration'] else record.get('predecessorFenceETag')
        if (any(link.get(key) != selection[target] for key, target in (('scope', 'scope'), ('ownerID', 'ownerID'), ('diskID', 'diskID'),
                ('oldControllerID', 'oldControllerID'), ('newControllerID', 'newControllerID'), ('oldGeneration', 'oldGeneration'), ('newGeneration', 'newGeneration'))) or
                link.get('sourceCommit') != successor['sourceCommit'] or link.get('newBootID') != successor['newBootID'] or
                link.get('predecessorFenceSHA256') != predecessor_hash or link.get('predecessorFenceETag') != predecessor_etag or
                delivery.get('controllerInstanceID') != selection['newControllerID'] or delivery.get('azureDataDiskID') != selection['diskID'] or
                delivery.get('hostConfigSHA256') != digest(delivery.get('hostConfig')) or
                link.get('hostConfigSHA256') != digest({key: value for key, value in delivery['hostConfig'].items() if key != 'controllerSuccessorLink'})):
            raise ValueError('successor host delivery differs from exact predecessor fence, boot or configuration')
    if record['state'] == 'active' and record.get('controllerID') == selection['newControllerID'] and reconcile:
        if (record.get('generation') != selection['newGeneration'] or record.get('successorDigest') != successor_hash or
                not compute.power(selection['oldControllerID']) or compute.vm_identities(selection['oldControllerID'])[0] or compute.disk_owner(selection['diskID']) != (selection['newControllerID'], 'Attached') or
                compute.vm_identities(selection['newControllerID'])[0] != {selection['controllerIdentityID'], selection['recoveryIdentityID']}):
            raise ValueError('retained recovered generation is no longer exact')
        mounted = restore_local(selection, record['fenceReceiptDigest'], inspect=True)
        if mounted.get('generationReceiptSHA256') != record.get('generationReceiptSHA256') or service is None or not service.ready():
            raise RuntimeError('retained recovered controller is not healthy at the exact generation')
        return record
    if record["state"] == "active":
        if reconcile: raise RuntimeError('no retained recovery effect exists; no repeated mutation')
        if record.get("controllerID") != selection["oldControllerID"] or record.get("generation") != selection["oldGeneration"] or record.get("generationReceiptSHA256") != selection["oldGenerationReceiptSHA256"]:
            raise ValueError("old controller generation does not match the durable fence")
        if compute.disk_owner(selection["diskID"]) != (selection["oldControllerID"], "Attached"):
            raise ValueError("old controller disk ownership is not current")
        if compute.vm_identities(selection["oldControllerID"])[0] not in ({selection["controllerIdentityID"]}, {selection["controllerIdentityID"], selection["recoveryIdentityID"]}):
            raise ValueError("old controller identity is not current")
        new_vm, _ = compute.get(selection["newControllerID"])
        if new_vm.get("properties", {}).get("provisioningState") != "Succeeded" or new_vm.get("properties", {}).get("storageProfile", {}).get("dataDisks", []) or compute.vm_identities(selection["newControllerID"])[0] != {selection["recoveryIdentityID"]}:
            raise ValueError("replacement controller disk or identity is not selected")
        if delivery:
            expected = successor['newBootID'] + '|' + delivery['hostConfig']['controllerPrivateIP']
            output = compute.run_command(selection['newControllerID'], target_identity_script())
            if [line.removeprefix('LUCI_AZURE_SUCCESSOR_TARGET=') for line in output.splitlines() if line.startswith('LUCI_AZURE_SUCCESSOR_TARGET=')] != [expected]:
                raise ValueError('successor actual boot/private IP changed before predecessor fencing')
        record, etag = transition(store, record, etag, "deallocate_pending", targetControllerID=selection["newControllerID"], targetGeneration=selection["newGeneration"], successorDigest=successor_hash,
            predecessorFenceETag=etag, predecessorFenceSHA256=digest(record))
        try:
            compute.deallocate(selection["oldControllerID"])
        except Exception:
            pass  # readback below settles only an observed deallocation
    if record["state"] == "deallocate_pending":
        if not compute.power(selection["oldControllerID"]):
            raise RuntimeError("old controller deallocation is unknown; no repeated stop or disk mutation")
        record, etag = transition(store, record, etag, 'identity_revoke_pending')
        old_identities, vm_etag = compute.vm_identities(selection['oldControllerID'])
        if old_identities != {selection['controllerIdentityID']}:
            raise ValueError('old controller identity changed before exact revocation')
        try:
            compute.patch_identities(selection['oldControllerID'], [], vm_etag)
        except Exception:
            pass
    if record['state'] == 'identity_revoke_pending':
        if not compute.power(selection['oldControllerID']) or compute.vm_identities(selection['oldControllerID'])[0]:
            raise RuntimeError('old controller identity revocation is unknown; no repeated identity write')
        record, etag = transition(store, record, etag, "detach_pending")
        rows, vm_etag = compute.vm_disks(selection["oldControllerID"])
        if not exact_disk_row(rows, selection["diskID"]):
            raise ValueError("old controller disk LUN changed")
        try:
            compute.patch_disks(selection["oldControllerID"], [], vm_etag)
        except Exception:
            pass  # never replay; exact disk and VM readback below
    if record['state'] not in ('active', 'deallocate_pending', 'identity_revoke_pending') and compute.vm_identities(selection['oldControllerID'])[0]:
        raise RuntimeError('old controller authority was reattached; successor progress refused')
    if record["state"] == "detach_pending":
        rows, _ = compute.vm_disks(selection["oldControllerID"])
        if rows or compute.disk_owner(selection["diskID"]) != ("", "Unattached") or not compute.power(selection["oldControllerID"]):
            raise RuntimeError("old disk detach is unknown; no replacement attachment")
        detached = dict(record, state="disk_detached", oldControllerDeallocated=True, diskUnattached=True)
        detached["fenceReceiptDigest"] = digest(detached)
        record, etag = detached, store.write(detached, etag)
    if record["state"] == "disk_detached":
        if compute.disk_owner(selection["diskID"])[0] != "" or not compute.power(selection["oldControllerID"]):
            raise RuntimeError("controller fence changed before replacement attach")
        rows, vm_etag = compute.vm_disks(selection["newControllerID"])
        if rows:
            raise ValueError("replacement controller already owns a data disk")
        record, etag = transition(store, record, etag, "attach_pending")
        name = selection["diskID"].rsplit("/", 1)[-1]
        disk = {"lun": 0, "name": name, "createOption": "Attach", "caching": "None", "managedDisk": {"id": selection["diskID"]}}
        try:
            compute.patch_disks(selection["newControllerID"], [disk], vm_etag)
        except Exception:
            pass
    if record["state"] == "attach_pending":
        rows, _ = compute.vm_disks(selection["newControllerID"])
        if not exact_disk_row(rows, selection["diskID"]) or compute.disk_owner(selection["diskID"]) != (selection["newControllerID"], "Attached") or not compute.power(selection["oldControllerID"]):
            raise RuntimeError("replacement disk attachment is unknown; mount refused")
        identities, vm_etag = compute.vm_identities(selection["newControllerID"])
        if identities != {selection["recoveryIdentityID"]}:
            raise ValueError("replacement controller identity changed before fencing")
        record, etag = transition(store, record, etag, "identity_pending")
        try:
            compute.patch_identities(selection["newControllerID"],
                                     [selection["recoveryIdentityID"], selection["controllerIdentityID"]], vm_etag)
        except Exception:
            pass
    if record["state"] == "identity_pending":
        identities, _ = compute.vm_identities(selection["newControllerID"])
        if identities != {selection["recoveryIdentityID"], selection["controllerIdentityID"]} or not compute.power(selection["oldControllerID"]):
            raise RuntimeError("controller identity grant is unknown; restoration refused")
        record, etag = transition(store, record, etag, "successor_stage_pending" if successor else "restore_pending")
        if successor:
            try:
                output = compute.run_command(selection['newControllerID'], successor_script(
                    successor, record['fenceReceiptDigest'], mount_path=selection['mountSelectionPath'], host_delivery=delivery))
                successor_receipt(output, successor)
            except Exception as error:
                raise RuntimeError('signed successor staging uncertain; inspect exact target without replay') from error
    if record["state"] in ("successor_stage_pending", "restore_pending", "restore_observe_pending", "gateway_pending", "api_start_pending") and (
            compute.disk_owner(selection["diskID"]) != (selection["newControllerID"], "Attached") or
            not compute.power(selection["oldControllerID"])):
        raise RuntimeError("single-writer controller fence changed during restore")
    if record['state'] == 'successor_stage_pending':
        try:
            output = compute.run_command(selection['newControllerID'], successor_script(
                successor, record['fenceReceiptDigest'], inspect=True, mount_path=selection['mountSelectionPath'], host_delivery=delivery))
            successor_receipt(output, successor)
        except Exception as error:
            raise RuntimeError('signed successor staging remains uncertain; no repeat write') from error
        record, etag = transition(store, record, etag, 'restore_pending')
    if record["state"] == "restore_pending":
        if successor:
            record, etag = transition(store, record, etag, 'restore_observe_pending')
            try:
                restore_local(selection, record['fenceReceiptDigest'])
            except Exception as error:
                raise RuntimeError('successor restore uncertain; inspect exact mounted receipts') from error
    if record['state'] == 'restore_observe_pending':
        try:
            mounted = restore_local(selection, record['fenceReceiptDigest'], inspect=True)
        except Exception as error:
            raise RuntimeError('successor mount remains uncertain; no repeat restore') from error
    elif record['state'] == 'restore_pending':
        mounted = restore_local(selection, record['fenceReceiptDigest'])
    if record['state'] in ('restore_pending', 'restore_observe_pending'):
        if mounted["generation"] != selection["newGeneration"]:
            raise ValueError("mounted controller generation changed")
        record, etag = transition(store, record, etag, "gateway_pending", generationReceiptSHA256=mounted["generationReceiptSHA256"], operatorSHA256=mounted["operatorSHA256"])
    if record["state"] == "gateway_pending":
        # `host_delivery` uses the controller MI and the durable on-disk
        # delivery journal. A pending lost response is never sent again.
        if host_delivery is None:
            raise RuntimeError("protected host delivery is unavailable")
        host_receipt = host_delivery(selection, record["generationReceiptSHA256"])
        if host_receipt.get("state") != "host_rotation_observed" or host_receipt.get("generation") != selection["newGeneration"]:
            raise RuntimeError("gateway rotation has no exact settled receipt")
        record, etag = transition(store, record, etag, "api_start_pending", hostReceiptSHA256=digest(host_receipt))
        if service is None:
            raise RuntimeError("controller API service observation is unavailable")
        service.start()
    if record["state"] == "api_start_pending":
        if service is None or not service.ready():
            raise RuntimeError("restored controller API is not healthy; no fresh dispatch claim")
        if not compute.power(selection["oldControllerID"]) or compute.disk_owner(selection["diskID"]) != (selection["newControllerID"], "Attached"):
            raise RuntimeError("old controller fence or new disk ownership changed")
        active = dict(record, state="active", controllerID=selection["newControllerID"], generation=selection["newGeneration"])
        active.pop("targetControllerID", None)
        active.pop("targetGeneration", None)
        record, etag = active, store.write(active, etag)
    return record


class Service:
    def __init__(self, health_url):
        self.health_url = health_url

    def start(self):
        result = subprocess.run(["systemctl", "start", "luci-api.service"], capture_output=True, timeout=30, check=False)
        if result.returncode:
            raise RuntimeError("restored controller service start is uncertain")

    def ready(self):
        result = subprocess.run(["systemctl", "is-active", "luci-api.service"], capture_output=True, timeout=8, check=False)
        if result.returncode or result.stdout.strip() != b"active":
            return False
        try:
            with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(self.health_url, timeout=3) as response:
                return response.status == 200
        except Exception:
            return False


def deliver_host(selection, generation_digest):
    path = Path(selection["hostDeliverySelectionPath"])
    host_selection = private(path)
    host_selection["controllerGenerationReceiptSHA256"] = generation_digest
    if host_selection.get("controllerInstanceID") != selection["newControllerID"] or host_selection.get("azureDataDiskID") != selection["diskID"]:
        raise ValueError("protected host delivery selects a different controller")
    source = Path(__file__).with_name("deliver.py")
    spec = importlib.util.spec_from_file_location("luci_azure_delivery", source)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    method = module.reconcile if Path(host_selection['journalPath']).exists() else module.run
    return method(host_selection, module.AzureTransport(host_selection["controllerClientID"]))


def recovered_readback(selection, record):
    """Expose actual mounted/fenced digests needed by a later replacement."""
    fields = ('generationReceiptSHA256', 'fenceReceiptDigest', 'hostReceiptSHA256')
    if (record.get('state') != 'active' or record.get('controllerID') != selection['newControllerID'] or
            record.get('generation') != selection['newGeneration'] or record.get('diskID') != selection['diskID'] or
            not all(HEX.fullmatch(record.get(key, '')) for key in fields)):
        raise ValueError('Exact recovered generation/delivery receipt is unavailable')
    delivery = selection['controllerHostDelivery']
    if delivery.get('hostConfigSHA256') != digest(delivery.get('hostConfig')):
        raise ValueError('Exact recovered host configuration is unavailable')
    return dict({key: record[key] for key in ('state', 'generation', 'diskID', 'controllerID') + fields},
        sourceCommit=selection['controllerSuccessor']['sourceCommit'], hostConfigSHA256=delivery['hostConfigSHA256'])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=("initialize", "reconcile-initialize", "recover", "reconcile-recover"))
    parser.add_argument("selection", type=Path)
    args = parser.parse_args()
    selected = private(args.selection, 96 << 10)
    validate(selected)
    if args.action in ('recover', 'reconcile-recover') and 'controllerSuccessor' not in selected:
        raise ValueError('replacement recovery requires a source-bound signed successor')
    transport = AzureHTTP(selected["recoveryClientID"])
    if transport.local_instance_id() != selected["newControllerID"]:
        raise ValueError("disk lifecycle must run on the selected private recovery VM")
    store = FenceBlob(transport, selected["fenceBlobURL"])
    compute = Compute(transport)
    record = initialize(selected, store, compute, args.action == 'reconcile-initialize') if args.action in ('initialize', 'reconcile-initialize') else run(
        selected, store, compute, deliver_host, Service(selected["apiHealthURL"]), reconcile=args.action == 'reconcile-recover')
    result = recovered_readback(selected, record) if args.action in ('recover', 'reconcile-recover') else {
        "state": record["state"], "generation": record.get("generation"), "diskID": record["diskID"]}
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, KeyError, urllib.error.URLError, json.JSONDecodeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
