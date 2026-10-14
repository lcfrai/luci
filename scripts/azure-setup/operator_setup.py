"""Generate source-bound deferred Azure operator inputs without hand-written JSON."""
import base64
import binascii
import hashlib
import json
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import uuid

RUNTIME = "cf536c284434fdd2f09bbab35ec807a5dacc87e4"
OCI = re.compile(r"[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}@sha256:([0-9a-f]{64})\Z")
REFS = {"apiKey": "team-api", "adminKey": "admin-token", "attachKey": "attach-key", "clientCert": "client-cert", "clientKey": "client-key"}


def domains_valid(domains):
    return bool(domains) and len(domains) <= 8 and len(domains) == len(set(domains)) and all(
        4 <= len(name) <= 253 and not name.endswith((".local", ".localhost", ".internal")) and
        len(name.split(".")) >= 2 and not re.fullmatch(r"[0-9.]+", name) and all(
            re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", part) for part in name.split(".")) for name in domains)


def sign(core, payload, credentials):
    with tempfile.TemporaryDirectory(dir=credentials) as directory:
        message, signature = Path(directory) / "message", Path(directory) / "signature"
        message.write_bytes(core.canonical(payload))
        message.chmod(0o600)
        subprocess.run(["openssl", "dgst", "-sha256", "-sign", str(credentials / "ca.key"),
                        "-out", str(signature), str(message)], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        return base64.b64encode(signature.read_bytes()).decode()


def verify_signed_payload(core, envelope, public_key):
    if envelope.get('signatureAlgorithm') != 'ecdsa-sha256' or not isinstance(envelope.get('payload'), dict):
        raise ValueError('prior operator envelope is not signed')
    with tempfile.TemporaryDirectory() as directory:
        key, message, signature = (Path(directory) / name for name in ('key', 'message', 'signature'))
        key.write_text(public_key)
        message.write_bytes(core.canonical(envelope['payload']))
        try:
            signature.write_bytes(base64.b64decode(envelope['signature'], validate=True))
        except (KeyError, ValueError, binascii.Error) as error:
            raise ValueError('prior operator signature is invalid') from error
        result = subprocess.run(['openssl', 'dgst', '-sha256', '-verify', str(key), '-signature', str(signature), str(message)],
                                capture_output=True, timeout=10, check=False)
        if result.returncode:
            raise ValueError('prior operator signature is invalid')


def prepare_successor(core, bundle_root, selection_path, recovery_selection_path, prior_envelope_sha,
                      new_boot_id, new_credentials_dir):
    """Sign a replacement controller without copying old-generation TLS or task state.

    The account-side caller supplies an authenticated target boot observation and
    a separately published new-generation credential journal. The guest still
    checks its own boot before any staged write; no provider effect occurs here.
    """
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    recovery = core.private_json(recovery_selection_path)
    parent = Path(selection_path).parent
    successor_path = parent / ('azure-successor-' + recovery['newGeneration'] + '.json')
    if successor_path.exists():
        raise ValueError('retained signed successor exists; inspect it without resigning')
    old_draft = core.private_json(parent / 'operator-template.json')
    old_envelope = core.private_json(parent / 'azure-operator-finalization.json')
    old_payload = old_envelope.get('payload', {})
    if (not re.fullmatch(r'[0-9a-f]{64}', prior_envelope_sha) or core.digest(old_envelope) != prior_envelope_sha or
            not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', new_boot_id) or
            old_payload.get('sourceCommit') != manifest['luciRevision'] or
            old_payload.get('templateSHA256') != hashlib.sha256((parent / 'operator-template.json').read_bytes()).hexdigest() or
            old_payload.get('scope') != recovery.get('scope') or
            old_payload.get('scope') != variables['scope'] or
            old_payload.get('generation') != recovery.get('oldGeneration') or
            old_payload.get('controllerInstanceId') != recovery.get('oldControllerID') or
            old_payload.get('diskID') != recovery.get('diskID') or
            recovery.get('newControllerID') == recovery.get('oldControllerID') or
            recovery.get('newGeneration') == recovery.get('oldGeneration') or
            not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', old_payload.get('controllerBootID', '')) or
            new_boot_id == old_payload.get('controllerBootID') or
            old_payload.get('operator', {}).get('store', {}).get('ownerID') != recovery.get('ownerID')):
        raise ValueError('signed predecessor or replacement controller selection is stale')
    binding = old_draft.get('setupDeferred', {})
    if binding.get('sourceCommit') != manifest['luciRevision'] or binding.get('generation') != recovery['oldGeneration']:
        raise ValueError('prior source-pinned operator draft changed')
    verify_signed_payload(core, old_envelope, binding['signerPublicKey'])
    credentials = Path(new_credentials_dir)
    journal = core.private_json(credentials / 'journal.json')
    key_path = credentials / 'ca.key'
    key_info = key_path.lstat()
    if key_path.is_symlink() or not stat.S_ISREG(key_info.st_mode) or key_info.st_mode & 0o077 or not 0 < key_info.st_size <= 16384:
        raise ValueError('new-generation private signer is unavailable')
    ca_path = credentials / 'ca.crt'
    ca_info = ca_path.lstat()
    if ca_path.is_symlink() or not ca_path.is_file() or ca_info.st_size < 1 or ca_info.st_size > 16384:
        raise ValueError('new-generation public CA is unavailable')
    with ca_path.open('rb') as source:
        ca_data = source.read(16385)
    if (journal.get('state') != 'published' or journal.get('scope') != recovery['scope'] or
            journal.get('generation') != recovery['newGeneration'] or
            hashlib.sha256(ca_data).hexdigest() != journal.get('files', {}).get('client-ca') or
            len(ca_data) > 16384):
        raise ValueError('new-generation public CA and Key Vault versions are unavailable')
    signer = core.local_command(['openssl', 'x509', '-in', str(credentials / 'ca.crt'), '-pubkey', '-noout'])
    if signer == binding['signerPublicKey']:
        raise ValueError('successor must use new-generation controller mTLS material')
    references = {}
    old_refs = old_payload['operator']['azureSecrets']
    vault = old_refs['vaultUrl']
    for target, kind in REFS.items():
        row = journal.get('published', {}).get(kind, {})
        match = re.fullmatch(r'https://([a-z0-9-]+)\.vault\.azure\.net/secrets/([a-zA-Z0-9-]+)/([0-9a-f]{32})', row.get('id', ''))
        retained_task = target in ('apiKey', 'adminKey', 'attachKey')
        old_ref = old_refs.get(target, {})
        if (match is None or 'https://' + match[1] + '.vault.azure.net' != vault or
                (not retained_task and old_ref.get('version') == match[3]) or
                (retained_task and old_ref != {'name': match[2], 'version': match[3]})):
            raise ValueError('successor Key Vault reference is stale or outside the selected vault')
        if retained_task:
            predecessor_journal = core.private_json(parent / 'azure-credentials/journal.json')
            retained = journal.get('retainedTaskIdentity', {}).get(kind, {})
            old_row = predecessor_journal.get('published', {}).get(kind, {})
            if (row != old_row or retained != {'name': match[2], 'version': match[3], 'sha256': predecessor_journal.get('files', {}).get(kind)} or
                    journal.get('files', {}).get(kind) != retained.get('sha256')):
                raise ValueError('successor task identity differs from exact predecessor bytes/name/version/hash')
            filename = {'team-api': 'team-api.txt', 'admin-token': 'admin-token.txt', 'attach-key': 'attach-key.txt'}[kind]
            for task_root in (parent / 'azure-credentials', credentials):
                task_file = task_root / filename
                task_info = task_file.lstat()
                if (task_file.is_symlink() or not stat.S_ISREG(task_info.st_mode) or task_info.st_mode & 0o077 or
                        not 0 < task_info.st_size <= 4096 or hashlib.sha256(task_file.read_bytes()).hexdigest() != retained['sha256']):
                    raise ValueError('successor retained task bytes changed')
        references[target] = {'name': match[2], 'version': match[3]}
    draft = json.loads(json.dumps(old_draft))
    draft['setupDeferred'].update(generation=recovery['newGeneration'], signerPublicKey=signer)
    draft['deployment']['hostGeneration'] = recovery['newGeneration']
    draft['azureSecrets'].update(references)
    operator = json.loads(json.dumps(old_payload['operator']))
    operator['deployment'].update(hostGeneration=recovery['newGeneration'], managementReceiptDigest='', restoreReceiptDigest='')
    operator['azureSecrets'].update(references)
    payload = dict(old_payload, templateSHA256=core.digest(draft), generation=recovery['newGeneration'],
                   controllerInstanceId=recovery['newControllerID'], controllerBootID=new_boot_id,
                   operator=operator, previousFinalizationSHA256=prior_envelope_sha,
                   previousControllerInstanceID=recovery['oldControllerID'], previousGeneration=recovery['oldGeneration'],
                   previousBootID=old_payload['controllerBootID'])
    envelope = {'payload': payload, 'signatureAlgorithm': 'ecdsa-sha256',
                'signature': sign(core, payload, credentials)}
    env = ('LUCI_ADDR=127.0.0.1:8730\nLUCI_BACKING=azure\n'
           'LUCI_AZURE_OPERATOR_TEMPLATE=/etc/luci/operator.json\n'
           'LUCI_STATE_DIR=' + variables['controller_mount_path'] + '/luci/control\n'
           'LUCI_PROJECTS_DIR=' + variables['controller_mount_path'] + '/luci/projects\n'
           'LUCI_EXECUTION_STATE_DIR=' + variables['controller_mount_path'] + '/luci/execution\n')
    files = []
    for path, content in (('/etc/luci/operator-template.json', core.canonical(draft).decode()),
                          ('/etc/luci/azure-private-ca.crt', ca_data.decode()),
                          ('/etc/luci/azure-controller.env', env),
                          ('/etc/luci/azure-operator-finalization.json', core.canonical(envelope).decode())):
        if not 0 < len(content) <= 16384:
            raise ValueError('bounded successor public configuration is unavailable')
        files.append({'path': path, 'content': content, 'sha256': hashlib.sha256(content.encode()).hexdigest()})
    mount_root = variables['controller_mount_path']
    mount_template = {'version': 1, 'mode': 'restore', 'device': '/dev/disk/azure/scsi1/lun0',
                      'mountPath': mount_root, 'filesystemUuid': recovery['filesystemUUID'],
                      'azureDataDiskId': recovery['diskID'], 'ownerId': recovery['ownerID'], 'scope': recovery['scope'],
                      'controllerHostId': variables['controller_host_id'], 'runtimeHostId': variables['runtime_host_id'],
                      'controllerInstanceId': recovery['newControllerID'], 'generation': recovery['newGeneration'],
                      'diskReceiptPath': mount_root + '/luci/disk-receipt.json',
                      'generationReceiptPath': mount_root + '/luci/generation.json',
                      'operatorTemplatePath': variables['controller_operator_template_path'],
                      'operatorTemplateSHA256': core.digest(draft),
                      'operatorOutputPath': variables['controller_operator_output_path'],
                      'fenceBlobURL': recovery['fenceBlobURL'],
                      'previousGeneration': recovery['oldGeneration'],
                      'previousGenerationReceiptSHA256': recovery['oldGenerationReceiptSHA256'],
                      'previousControllerInstanceID': recovery['oldControllerID'],
                      'previousFinalizationSHA256': prior_envelope_sha,
                      'previousBootID': old_payload['controllerBootID'], 'targetBootID': new_boot_id,
                      'finalizationSHA256': core.digest(envelope)}
    successor = {'version': 1, 'sourceCommit': manifest['luciRevision'], 'scope': recovery['scope'],
                 'ownerID': recovery['ownerID'], 'diskID': recovery['diskID'],
                 'oldControllerID': recovery['oldControllerID'], 'newControllerID': recovery['newControllerID'],
                 'oldGeneration': recovery['oldGeneration'], 'newGeneration': recovery['newGeneration'],
                 'oldBootID': old_payload['controllerBootID'], 'newBootID': new_boot_id,
                 'oldGenerationReceiptSHA256': recovery['oldGenerationReceiptSHA256'],
                 'priorEnvelopeSHA256': prior_envelope_sha, 'newEnvelopeSHA256': core.digest(envelope),
                 'newTemplateSHA256': core.digest(draft), 'newPublicCASHA256': hashlib.sha256(ca_data).hexdigest(),
                 'files': files, 'mountTemplate': mount_template}
    core.atomic_json(successor_path, successor)
    return {'state': 'successor_signed_pending_fence', 'successorPath': str(successor_path),
            'successorDigest': core.digest(successor), 'sourceCommit': manifest['luciRevision'],
            'oldGeneration': recovery['oldGeneration'], 'newGeneration': recovery['newGeneration']}


def deferred(core, bundle_root, selection_path, from_image, cpu_count, memory_mib,
             task_max_seconds, task_cost_micros, model_domains):
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    parent = Path(selection_path).parent
    if not from_image or from_image.startswith('127.0.0.1:5055/'):
        produced = core.private_json(parent / 'azure-builder-state.json')
        measured = core.private_json(parent / 'azure-guest-source.json')
        retained = produced.get('receipts', {}).get('runtime:install', {}).get('guestSource')
        if (measured != retained or measured.get('sourceCommit') != manifest['luciRevision'] or
                not re.fullmatch(r'[0-9a-f]{64}', measured.get('imageDigest', '')) or
                measured.get('fromImage') != '127.0.0.1:5055/luci/guest@sha256:' + measured['imageDigest'] or
                not re.fullmatch(r'[0-9a-f]{64}', measured.get('receiptSHA256', '')) or
                'runtime:retired' not in produced.get('completed', []) or produced.get('abortRequested')):
            raise ValueError('Normal setup requires the exact observed immutable guest producer receipt')
        if from_image and from_image != measured['fromImage']:
            raise ValueError('Guest reference differs from observed source bytes')
        from_image = measured['fromImage']
    if any((parent / name).exists() for name in ("azure-setup-state.json", "azure-image-prep-state.json", "azure-terraform", "operator-template.json")):
        raise ValueError("Operator preparation is retained; do not replace its source binding")
    if (not OCI.fullmatch(from_image) or isinstance(cpu_count, bool) or not 1 <= cpu_count <= 16 or
            isinstance(memory_mib, bool) or not 128 <= memory_mib <= 32768 or
            not 1 <= task_max_seconds <= 86400 or not 1 <= task_cost_micros <= int(selection["monthlyBudgetUsd"] * 1000000) or
            not domains_valid(model_domains)):
        raise ValueError("Select a digest-pinned guest, bounded task resources/cost estimate and exact model DNS domains")
    credentials = parent / "azure-credentials"
    journal = core.private_json(credentials / "journal.json")
    if journal.get("state") != "published" or journal.get("scope") != variables["scope"] or journal.get("generation") != variables["generation"]:
        raise ValueError("Exact published private credential versions are required")
    ca = credentials / "ca.crt"
    if ca.is_symlink() or hashlib.sha256(ca.read_bytes()).hexdigest() != journal["files"]["client-ca"]:
        raise ValueError("Selected private root CA changed")
    public_key = core.local_command(["openssl", "x509", "-in", str(ca), "-pubkey", "-noout"])
    references = {}
    for target, kind in REFS.items():
        row = journal.get("published", {}).get(kind, {})
        match = re.fullmatch(r"https://([a-z0-9-]+)\.vault\.azure\.net/secrets/([a-zA-Z0-9-]+)/([0-9a-f]{32})", row.get("id", ""))
        if match is None or match[1] != variables["key_vault_name"]:
            raise ValueError("Controller credentials need exact selected-vault version readback")
        references[target] = {"name": match[2], "version": match[3]}
    guest = {"version": 1, "kind": "luci-azure-guest-selection", "sourceCommit": manifest["luciRevision"],
             "runtimeRevision": RUNTIME, "scope": variables["scope"], "generation": variables["generation"],
             "fromImage": from_image, "imageDigest": OCI.fullmatch(from_image)[1],
             "cpuCount": cpu_count, "memoryMiB": memory_mib,
             "taskMaxSeconds": task_max_seconds, "taskEstimatedCostMicros": task_cost_micros,
             "modelEgressDomains": sorted(set(model_domains))}
    prep_path = parent / "azure-operator-prep-state.json"
    if prep_path.exists():
        raise ValueError("Operator signing preparation is retained; inspect its exact local files")
    core.atomic_json(prep_path, {"version": 1, "state": "operator_signing_pending", "sourceCommit": manifest["luciRevision"],
        "scope": variables["scope"], "generation": variables["generation"], "guestSelectionDigest": core.digest(guest)})
    signed_guest = {"selection": guest, "signatureAlgorithm": "ecdsa-sha256", "signature": sign(core, guest, credentials), "signerPublicKey": public_key}
    core.atomic_json(parent / "signed-guest-selection.json", signed_guest)
    mount, gateway = variables["controller_mount_path"], variables["gateway_zone_name"]
    operator = {"sourceCommit": manifest["luciRevision"], "endpoint": "https://" + gateway, "apiKeyFile": "", "adminApiKeyFile": "", "attachKeyFile": "",
        "clientCertFile": "", "clientKeyFile": "", "rootCaFile": "/etc/luci/azure-private-ca.crt",
        "azureSecrets": {"vaultUrl": "https://" + variables["key_vault_name"] + ".vault.azure.net", "clientId": "",
                         "directory": "/run/luci", **references},
        "hostNodeId": "", "controllerStateDir": mount + "/luci/controller", "stagingDir": mount + "/luci/staging",
        "envdOriginTemplate": "https://49983-{sandboxID}." + gateway,
        "sandboxOriginTemplate": "https://8080-{sandboxID}." + gateway,
        "deployment": {"provider": "azure", "region": selection["region"], "instanceType": variables["runtime_vm_size"],
            "scope": variables["scope"], "hostGeneration": variables["generation"], "runtimeRevision": RUNTIME,
            "managementReceiptDigest": "", "restoreReceiptDigest": "", "azureSubscriptionId": variables["subscription_id"],
            "azureResourceGroup": variables["resource_group_name"].lower(), "azureScaleSetId": "", "azureInstanceId": "",
            "azureDataDiskId": "", "azureZone": variables["zone"], "azureGatewayHost": gateway},
        "template": {"id": "", "buildId": "", "buildDigest": "", "memoryMiB": memory_mib, "cpuMillis": cpu_count * 1000},
        "templateCreateJournalPath": mount + "/luci/template/create.json", "templateBuildJournalPath": mount + "/luci/template/build.json",
        "templateFromImage": from_image, "taskMaxSeconds": task_max_seconds, "taskEstimatedCostMicros": task_cost_micros,
        "modelEgressDomains": guest["modelEgressDomains"],
        "limits": {"MaxConcurrent": selection["maxConcurrentVMs"], "MaxPerProject": selection["maxConcurrentVMs"],
            "MemoryMiB": memory_mib * selection["maxConcurrentVMs"], "CPUMillis": cpu_count * 1000 * selection["maxConcurrentVMs"],
            "EstimatedCostMicros": int(selection["monthlyBudgetUsd"] * 1000000), "MaxTaskSeconds": task_max_seconds},
        "store": {"directory": mount + "/luci/transfers", "ownerID": variables["controller_owner_id"],
            "runtimeScope": variables["scope"], "controllerHostID": variables["controller_host_id"],
            "runtimeHostID": variables["runtime_host_id"], "separationReceiptDigest": "", "azureDataDiskId": "",
            "maxBytes": 1073741824, "maxObjects": 16384},
        "setupDeferred": {"version": 1, "sourceCommit": manifest["luciRevision"], "scope": variables["scope"],
            "generation": variables["generation"], "signerPublicKey": public_key, "guestSelectionDigest": core.digest(guest)}}
    target = parent / "operator-template.json"
    core.atomic_json(target, operator)
    variables["controller_operator_template_sha256"] = hashlib.sha256(target.read_bytes()).hexdigest()
    core.atomic_json(selection_path, selection)
    template_dir = parent / "template"
    template_dir.mkdir(mode=0o700)
    core.atomic_json(template_dir / "operator.json", {"endpoint": "https://" + gateway,
        "apiKeyFile": str(credentials / "team-api.txt"), "rootCaFile": str(ca),
        "clientCertFile": str(credentials / "client.crt"), "clientKeyFile": str(credentials / "client.key"),
        "name": "luci-" + selection["name"][:30] + "-" + uuid.uuid4().hex,
        "cpuCount": cpu_count, "memoryMB": memory_mib, "templateId": "", "buildId": "", "fromImage": from_image,
        "createJournalPath": str(template_dir / "create.json"), "journalPath": str(template_dir / "create.json")})
    core.atomic_json(prep_path, {"version": 1, "state": "operator_deferred_pending_provider_and_template_readback",
        "sourceCommit": manifest["luciRevision"], "scope": variables["scope"], "generation": variables["generation"],
        "guestSelectionDigest": core.digest(guest), "templateSHA256": variables["controller_operator_template_sha256"]})
    return {"state": "operator_deferred_pending_provider_and_template_readback", "operatorTemplatePath": str(target),
            "templateSHA256": variables["controller_operator_template_sha256"], "guestSelectionPath": str(parent / "signed-guest-selection.json"),
            "sourceCommit": manifest["luciRevision"], "connected": False,
            "message": "The signed guest selection binds a chosen OCI digest, not a running guest or a completed provider build."}


def controller_step(core, bundle_root, selection_path, mode, provider, acknowledge=False):
    manifest = core.bundle(bundle_root)
    if mode not in ("create", "created", "reconcile", "start", "status") or mode in ("create", "start") and not acknowledge:
        raise ValueError("Template mutation requires explicit publication/cost acknowledgement")
    parent = Path(selection_path).parent
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    infrastructure = core.private_json(parent / "azure-setup-state.json")
    if infrastructure.get("state") not in ("controller_bootstrapped_gateway_unverified", "controller_finalized_private_route_unverified") or infrastructure.get("selectionDigest") != core.digest(selection):
        raise ValueError("Template preparation needs exact fenced management-only controller initialization")
    identities = infrastructure["resourceIDs"]
    local_config = core.private_json(parent / "template/operator.json")
    config = {"version": 1, "sourceCommit": manifest["luciRevision"], "scope": variables["scope"], "generation": variables["generation"],
        "controllerResourceID": identities["controller_instance_id"], "runtimeScaleSetID": identities["runtime_scale_set_id"],
        "dataDiskID": identities["controller_disk_id"], "controllerClientID": identities["controller_identity_client_id"],
        "fenceBlobURL": identities["controller_fence_blob_url"], "filesystemUUID": variables["controller_filesystem_uuid"],
        "mountPath": variables["controller_mount_path"], "operatorTemplatePath": variables["controller_operator_template_path"],
        "operatorTemplateSHA256": variables["controller_operator_template_sha256"], "templateName": local_config["name"],
        "cpuCount": local_config["cpuCount"], "memoryMiB": local_config["memoryMB"], "fromImage": local_config["fromImage"]}
    journal_path = parent / "azure-template-state.json"
    previous = core.private_json(journal_path) if journal_path.exists() else {}
    if mode in ("create", "start") and (parent / "template" / ("create.json" if mode == "create" else "build.json")).exists():
        raise ValueError("Template effect already has a retained intent; use reconcile/status")
    if previous.get("pendingAction") and mode in ("create", "start"):
        raise ValueError("Template publication is uncertain; use reconcile/status without repeating its mutation")
    pending = dict(previous, version=1, state="template_readback_pending", sourceCommit=manifest["luciRevision"],
        scope=variables["scope"], generation=variables["generation"], selectionDigest=core.digest(selection), configDigest=core.digest(config))
    if mode in ("create", "start"):
        pending["pendingAction"] = mode
    core.atomic_json(journal_path, pending)
    action = {"create": "template-create", "created": "observe", "reconcile": "template-reconcile", "start": "template-start", "status": "template-status"}[mode]
    script = """set -eu
python3 - <<'PY'
import base64, os, pathlib, tempfile
data = base64.b64decode(%s)
target = pathlib.Path('/etc/luci/azure-setup-operator.json')
if target.exists() or target.is_symlink():
    if target.is_symlink() or target.read_bytes() != data:
        raise SystemExit('retained controller setup selection differs')
else:
    fd, name = tempfile.mkstemp(dir=target.parent); os.fchmod(fd, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(data); output.flush(); os.fsync(output.fileno())
    os.replace(name, target)
PY
receipt=$(/opt/luci/azure-controller/bin/luci-api azure-setup %s /etc/luci/azure-setup-operator.json)
printf 'LUCI_AZURE_SETUP_RECEIPT=%%s\\n' "$receipt"
""" % (repr(base64.b64encode(core.canonical(config)).decode()), action)
    response = provider(["vm", "run-command", "invoke", "--ids", identities["controller_instance_id"],
                         "--command-id", "RunShellScript", "--scripts", script])
    markers = [line[len("LUCI_AZURE_SETUP_RECEIPT="):] for row in response.get("value", [])
               for line in row.get("message", "").splitlines() if line.startswith("LUCI_AZURE_SETUP_RECEIPT=")]
    if len(markers) != 1:
        raise RuntimeError("Controller template operation is uncertain; retain its durable journal")
    observed = json.loads(markers[0])
    if observed.get("version") != 1 or observed.get("state") != "controller_setup_readback" or observed.get("microVMExecutionVerified") is not False:
        raise RuntimeError("Exact nonsecret controller setup receipt is unavailable")
    if any(observed.get(key) != config[key] for key in ("sourceCommit", "scope", "generation", "controllerResourceID", "dataDiskID")):
        raise RuntimeError("Controller setup readback changed the selected source, resource or generation")
    if mode in ("created", "create", "reconcile", "start", "status") and not observed.get("createResponse"):
        raise RuntimeError("Exact template creation acknowledgment is still unavailable")
    for name, target in (("createJournal", "create.json"), ("createResponse", "create.json.response"), ("buildJournal", "build.json")):
        if name in observed:
            path = parent / "template" / target
            if path.exists() and core.private_json(path) != observed[name]:
                raise RuntimeError("Retained local template mirror differs from authenticated controller readback")
            core.atomic_json(path, observed[name])
    state = "template_ready_provider_guest_unverified" if observed.get("templateStatus") == "ready" else "template_" + mode + "_returned"
    receipt = dict(pending, state=state, readbackDigest=core.digest(observed),
        message="Provider build readiness does not prove guest boot, isolation or a successful task.")
    receipt.pop("pendingAction", None)
    core.atomic_json(parent / "azure-controller-setup-readback.json", observed)
    core.atomic_json(journal_path, receipt)
    return receipt


def finalize(core, bundle_root, selection_path, provider, controller_readback=None):
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    parent = Path(selection_path).parent
    if (parent / "azure-operator-install-state.json").exists():
        raise ValueError("Operator installation is retained; preserve and reconcile its exact finalization")
    state = core.private_json(parent / "azure-setup-state.json")
    template_state = core.private_json(parent / "azure-template-state.json")
    if (state.get("state") != "controller_bootstrapped_gateway_unverified" or
            state.get("selectionDigest") != core.digest(selection) or
            template_state.get("state") != "template_ready_provider_guest_unverified" or
            template_state.get("selectionDigest") != core.digest(selection)):
        raise ValueError("Finalize requires exact infrastructure and template readback, without stale selections")
    identities = state["resourceIDs"]
    template_path = parent / "operator-template.json"
    if hashlib.sha256(template_path.read_bytes()).hexdigest() != variables["controller_operator_template_sha256"]:
        raise ValueError("Deferred operator source changed")
    operator = core.private_json(template_path)
    deferred_binding = operator.pop("setupDeferred")
    if deferred_binding["sourceCommit"] != manifest["luciRevision"] or deferred_binding["generation"] != variables["generation"]:
        raise ValueError("Deferred operator source or generation is stale")
    create = core.private_json(parent / "template/create.json")
    response = core.private_json(parent / "template/create.json.response")
    build = core.private_json(parent / "template/build.json")
    if (create["runtimeRevision"] != RUNTIME or any(response.get(key) != value for key, value in create.items()) or
            build.get("runtimeRevision") != RUNTIME or build.get("templateId") != response.get("templateId") or
            build.get("buildId") != response.get("buildId") or build.get("fromImage") != operator["templateFromImage"] or
            build.get("imageDigest") != OCI.fullmatch(operator["templateFromImage"])[1]):
        raise ValueError("Template journals differ from the selected signed guest")
    if controller_readback is None:
        controller_step(core, bundle_root, selection_path, "status", provider)
        controller_readback = core.private_json(parent / "azure-controller-setup-readback.json")
    if any(controller_readback.get(key) != value for key, value in (("sourceCommit", manifest["luciRevision"]),
            ("scope", variables["scope"]), ("generation", variables["generation"]),
            ("controllerResourceID", identities["controller_instance_id"]), ("dataDiskID", identities["controller_disk_id"]))):
        raise RuntimeError("Authenticated controller readback changed the source or fenced generation")
    if controller_readback.get("templateStatus") != "ready" or any(controller_readback.get(key) != value for key, value in (
            ("createJournal", create), ("createResponse", response), ("buildJournal", build))):
        raise RuntimeError("Current controller template readiness or durable journal readback changed")
    boot_id = controller_readback.get("bootID", "")
    if not re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", boot_id):
        raise RuntimeError("Current controller boot identity is unavailable")
    scale_set = identities["runtime_scale_set_id"]
    instances = [controller_readback.get("runtimeInstance", {})]
    if instances[0].get("properties", {}).get("provisioningState") != "Succeeded":
        raise RuntimeError("Initial runtime host identity requires one exact settled scale-set instance")
    instance_id = instances[0].get("instanceId", "")
    if not re.fullmatch(r"[0-9]{1,12}", instance_id) or instances[0].get("id", "").lower() != (scale_set + "/virtualMachines/" + instance_id).lower():
        raise RuntimeError("Runtime scale-set instance readback changed")
    client_id = identities["controller_identity_client_id"]
    if not re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", client_id):
        raise ValueError("Controller managed identity readback is incomplete")
    gateway_host = variables["gateway_zone_name"]
    expected_gateway = core.digest(["luci-azure-e2b-gateway-v1", gateway_host, variables["scope"], variables["generation"], "api:3000", "guest:3002", "envd:49983", "sandboxd:8080"])
    if controller_readback.get("gatewayReceiptDigest") != expected_gateway:
        raise RuntimeError("Private gateway generation readback is stale")
    nodes = controller_readback.get("nodes")
    if (not isinstance(nodes, list) or len(nodes) != 1 or nodes[0].get("status") != "ready" or
            not re.fullmatch(r"[a-zA-Z0-9_-]{1,128}", nodes[0].get("id", ""))):
        raise RuntimeError("Exact initial private Embed node is not ready")
    operator["hostNodeId"] = nodes[0]["id"]
    operator["azureSecrets"]["clientId"] = client_id
    operator["template"].update(id=response["templateId"], buildId=response["buildId"],
        buildDigest=hashlib.sha256(json.dumps({key: build[key] for key in ("runtimeRevision", "templateId", "buildId", "fromImage", "imageDigest")}, separators=(",", ":")).encode()).hexdigest())
    operator["deployment"].update(azureScaleSetId=scale_set.lower(), azureInstanceId=instance_id,
        azureDataDiskId=identities["controller_disk_id"].lower(), managementReceiptDigest=core.digest({"instance": instances[0], "nodes": nodes, "gateway": expected_gateway}),
        restoreReceiptDigest=state["readbackPlanSHA256"])
    operator["store"]["azureDataDiskId"] = identities["controller_disk_id"].lower()
    payload = {"version": 1, "kind": "luci-azure-operator-finalization", "sourceCommit": manifest["luciRevision"],
        "templateSHA256": variables["controller_operator_template_sha256"], "scope": variables["scope"], "generation": variables["generation"],
        "controllerInstanceId": identities["controller_instance_id"].lower(), "diskID": identities["controller_disk_id"].lower(),
        "controllerBootID": boot_id,
        "operator": operator, "createJournal": create, "createResponse": response, "buildJournal": build}
    envelope = {"payload": payload, "signatureAlgorithm": "ecdsa-sha256", "signature": sign(core, payload, parent / "azure-credentials")}
    core.atomic_json(parent / "azure-operator-finalization.json", envelope)
    return {"state": "operator_finalized_pending_authenticated_install", "sourceCommit": manifest["luciRevision"],
            "finalizationPath": str(parent / "azure-operator-finalization.json"), "finalizationDigest": core.digest(envelope), "connected": False}


def install(core, bundle_root, selection_path, finalization_digest, provider, reconcile=False):
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    parent = Path(selection_path).parent
    envelope = core.private_json(parent / "azure-operator-finalization.json")
    payload = envelope["payload"]
    state = core.private_json(parent / "azure-setup-state.json")
    if (core.digest(envelope) != finalization_digest or payload["sourceCommit"] != manifest["luciRevision"] or
            payload["generation"] != variables["generation"] or payload["scope"] != variables["scope"] or
            payload["templateSHA256"] != variables["controller_operator_template_sha256"] or
            state.get("selectionDigest") != core.digest(selection) or
            state.get("state") not in ("controller_bootstrapped_gateway_unverified", "controller_finalized_private_route_unverified") or
            payload["controllerInstanceId"] != state["resourceIDs"]["controller_instance_id"]):
        raise ValueError("Finalized operator installation selection is stale")
    journal_path = parent / "azure-operator-install-state.json"
    if journal_path.exists():
        journal = core.private_json(journal_path)
        if not reconcile or journal.get("finalizationDigest") != finalization_digest:
            raise ValueError("Operator installation is retained; reconcile without repeating its write")
    elif reconcile:
        raise ValueError("No retained operator installation to reconcile")
    data = core.canonical(envelope)
    file_sha = hashlib.sha256(data).hexdigest()
    # Reconciliation invokes only the readback branch. Never rewrite unknown
    # remote data or overwrite a changed operator selection.
    script = """set -eu
python3 - <<'PY'
import base64, hashlib, json, os, pathlib, subprocess, tempfile
target = pathlib.Path('/etc/luci/azure-operator-finalization.json')
data = base64.b64decode(%s)
if not %s:
    if target.exists() or target.is_symlink():
        raise SystemExit('retained finalization must be reconciled')
    fd, name = tempfile.mkstemp(dir=target.parent)
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(data); output.flush(); os.fsync(output.fileno())
    os.replace(name, target)
if target.is_symlink() or target.stat().st_mode & 0o077 or hashlib.sha256(target.read_bytes()).hexdigest() != %s:
    raise SystemExit('exact private operator finalization unavailable')
mode = '--inspect' if %s else '--finalize'
subprocess.run(['python3','/opt/luci/azure-controller/bin/mount.py',mode,'/etc/luci/azure-controller-disk-initial.json'], check=True, stdout=subprocess.DEVNULL)
operator = json.loads(pathlib.Path('/etc/luci/operator.json').read_bytes())
envelope = json.loads(data)
if 'setupDeferred' in operator or operator.get('sourceCommit') != envelope['payload']['sourceCommit'] or operator.get('template') != envelope['payload']['operator']['template'] or operator.get('azureMount',{}).get('generation') != envelope['payload']['generation']:
    raise SystemExit('signed task operator finalization is incomplete')
print('LUCI_AZURE_OPERATOR_INSTALLED=' + json.dumps({'sha256': %s, 'generation': %s, 'sourceCommit': %s}, separators=(',', ':')))
PY
""" % (repr(base64.b64encode(data).decode()), repr(reconcile), repr(file_sha), repr(reconcile), repr(file_sha), repr(variables["generation"]), repr(manifest["luciRevision"]))
    if not reconcile:
        core.atomic_json(journal_path, {"version": 1, "state": "operator_install_pending", "sourceCommit": manifest["luciRevision"],
            "scope": variables["scope"], "generation": variables["generation"], "finalizationDigest": finalization_digest})
    response = provider(["vm", "run-command", "invoke", "--ids", payload["controllerInstanceId"],
                         "--command-id", "RunShellScript", "--scripts", script])
    markers = [line[len("LUCI_AZURE_OPERATOR_INSTALLED="):] for row in response.get("value", [])
               for line in row.get("message", "").splitlines() if line.startswith("LUCI_AZURE_OPERATOR_INSTALLED=")]
    expected = {"sha256": file_sha, "generation": variables["generation"], "sourceCommit": manifest["luciRevision"]}
    if len(markers) != 1 or json.loads(markers[0]) != expected:
        raise RuntimeError("Operator installation outcome is uncertain; use read-only reconciliation")
    receipt = {"version": 1, "state": "operator_finalized_private_route_unverified", "sourceCommit": manifest["luciRevision"],
               "scope": variables["scope"], "generation": variables["generation"],
               "finalizationDigest": finalization_digest, "readback": expected, "connected": False}
    core.atomic_json(journal_path, receipt)
    core.atomic_json(parent / "azure-setup-state.json", dict(state, state="controller_finalized_private_route_unverified"))
    return receipt
