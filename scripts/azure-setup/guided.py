"""Small, journaled user-run Azure setup stages. No implicit provider calls."""
import ipaddress
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import tempfile
import time
from datetime import datetime, timezone


RFC1918 = tuple(ipaddress.ip_network(value) for value in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"))


def private_cidr(value):
    try:
        network = ipaddress.ip_network(value, strict=True)
    except ValueError as error:
        raise ValueError("Choose an exact RFC1918 private management CIDR") from error
    if network.version != 4 or not any(network.subnet_of(parent) for parent in RFC1918):
        raise ValueError("Choose an exact RFC1918 private management CIDR")
    return str(network)


def progress(core, bundle_root, selection_path):
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    parent = Path(selection_path).parent
    stages = []
    for identifier, relative in (("private-location", "azure-private-location.json"),
                                 ("foundation", "azure-foundation-state.json"),
                                 ("image-builder", "azure-builder-state.json"),
                                 ("credentials", "azure-credentials/journal.json"),
                                 ("operator-selection", "azure-operator-prep-state.json"),
                                 ("template", "azure-template-state.json"),
                                 ("operator", "azure-operator-install-state.json"),
                                 ("images", "azure-image-prep-state.json"),
                                 ("runtime-gallery", "azure-gallery-runtime.json"),
                                 ("controller-gallery", "azure-gallery-controller.json"),
                                 ("infrastructure", "azure-setup-state.json"),
                                 ("connection", "azure-connection-state.json"),
                                 ("retirement", "azure-retirement-state.json")):
        path = parent / relative
        record = core.private_json(path) if path.exists() else {}
        state = record.get("state", "not_started")
        stages.append({"id": identifier, "state": state,
                       "uncertain": state.endswith("_pending") or bool(record.get("pending")),
                       "receiptPath": str(path) if record else ""})
    return {"version": 1, "sourceCommit": manifest["luciRevision"], "scope": variables["scope"],
            "selectionDigest": core.digest(selection), "stages": stages, "connected": False,
            "priceVerified": False, "finalInvoiceVerified": False,
            "message": "Local stage receipts do not prove a reachable or authorized workspace."}


def wizard(core, bundle_root, selection_path, ask=input, cli=None, foundation=None):
    """Stage normal private setup without hand-written operator/Terraform JSON.

    Discovery is read-only. Newly proposed foundation effects always stop at an
    exact reviewable cost plan; the operator invokes acknowledged stages later.
    """
    cli = cli or az_json
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    journal_path = parent / 'azure-foundation-state.json'
    if (parent / 'azure-foundation-plan.json').exists():
        journal = core.private_json(journal_path) if journal_path.exists() else {}
        if journal.get('state') != 'foundation_bound_private_data_route_observed':
            proposal = core.private_json(parent / 'azure-foundation-plan.json')
            return {'state': journal.get('state', 'foundation_review_required'), 'planDigest': proposal['planDigest'],
                'resources': [row['id'] for row in proposal['resources']], 'retainedCharges': proposal['retainedCharges'],
                'nextAction': 'foundation-reconcile' if journal.get('pending') else 'foundation-bind' if journal.get('state') == 'foundation_observed_pending_private_data_route' else 'foundation-step',
                'requiresExplicitCostAcknowledgment': not journal.get('pending'), 'connected': False, 'priceVerified': False}
    else:
        if foundation is None:
            spec = importlib.util.spec_from_file_location('luci_azure_foundation', Path(bundle_root) / 'azure-setup/foundation.py')
            foundation = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(foundation)
        account = cli(['account', 'show'])
        principal = cli(['ad', 'signed-in-user', 'show', '--query', 'id']) if account.get('user', {}).get('type') == 'user' else cli(['ad', 'sp', 'show', '--id', account.get('user', {}).get('name', ''), '--query', 'id'])
        networks = cli(['network', 'vnet', 'list', '--subscription', account['id'], '--query', "[?location=='" + selected['region'] + "'].{id:id,name:name,subnets:subnets}"])
        if not isinstance(networks, list) or not 1 <= len(networks) <= 128:
            raise ValueError('An existing privately reachable VNet in the selected region is required; no public connectivity is created')
        prompt = 'Existing private location (' + ', '.join(str(i + 1) + ': ' + row['name'] for i, row in enumerate(networks)) + '), number: '
        try:
            index = int(ask(prompt).strip()) - 1
            if index < 0 or index >= len(networks): raise ValueError('selection')
            chosen = networks[index]
        except (ValueError, IndexError):
            raise ValueError('Choose one displayed existing private VNet') from None
        arm = foundation.AzureARM(cli)
        endpoint = ask('Existing private-endpoint subnet name (policies disabled): ').strip()
        route_host = ask('Existing reachable private listener IPv4 address in that VNet: ').strip()
        route_port = int(ask('Listener port [22]: ').strip() or '22')
        source = ask('Your existing private management source CIDR: ').strip()
        foundation.inventory(core, bundle_root, selection_path, chosen['id'], endpoint, route_host,
            route_port, source, principal, arm, account)
        if ask('Reuse existing private state and Key Vault? [yes/no]: ').strip().lower() == 'yes':
            result = foundation.reuse(core, bundle_root, selection_path,
                ask('Existing state storage ARM ID (private luci-state container): ').strip(),
                ask('Existing private Key Vault ARM ID: ').strip(),
                ask('Existing linked private Blob/vault DNS resource group: ').strip(), arm)
            return dict(result, nextAction='foundation-bind', connected=False)
        group = ask('New separately owned foundation resource group [' + selected['name'] + '-foundation]: ').strip() or selected['name'] + '-foundation'
        return foundation.plan(core, bundle_root, selection_path, group)
    backend = selected['stateBackend']
    # Dsv5 is a documented nested-virtualization series. Availability and zones
    # are checked for this account; actual KVM/guest acceptance is still separate.
    runtime_size = ask('Runtime size [Standard_D8s_v5; review its Azure quote]: ').strip() or 'Standard_D8s_v5'
    controller_size = ask('Controller size [Standard_D2s_v5; review its Azure quote]: ').strip() or 'Standard_D2s_v5'
    supported_zones = []
    for size in (runtime_size, controller_size):
        if not re.fullmatch(r'Standard_[A-Za-z0-9_]{2,80}', size): raise ValueError('Invalid selected VM size')
        rows = cli(['vm', 'list-skus', '--location', selected['region'], '--resource-type', 'virtualMachines', '--size', size,
                    '--query', "[?name=='" + size + "' && length(restrictions)==`0`].{name:name,locationInfo:locationInfo}"])
        if not isinstance(rows, list) or len(rows) != 1:
            raise ValueError('Selected VM size is restricted or unavailable in this account/region')
        supported_zones.append({zone for row in rows[0].get('locationInfo', []) for zone in row.get('zones', [])})
    zone = ask('Availability zone [1; must be supported by both selected sizes]: ').strip() or '1'
    if any(zone not in zones for zones in supported_zones):
        raise ValueError('Select a zone supported by both available VM sizes')
    key_path = ask('Absolute SSH public-key file: ').strip()
    return core.configure_account(bundle_root, selection_path, subscription_id=backend['subscriptionId'],
        resource_group=variables['resource_group_name'], state_resource_group=backend['resourceGroupName'],
        state_storage_account=backend['storageAccountName'], state_container=backend['containerName'],
        zone=zone, runtime_size=runtime_size, controller_size=controller_size, ssh_public_key_file=key_path,
        key_vault_id=variables['key_vault_id'], key_vault_name=variables['key_vault_name'],
        fence_storage_account='lucifn' + core.digest([manifest['luciRevision'], variables['scope']])[:16],
        runtime_image_version_id=variables.get('runtime_image_version_id', ''), controller_image_version_id=variables.get('controller_image_version_id', ''))


def configure_private(core, bundle_root, selection_path, network_cidr, management_cidr, acknowledge_route):
    core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    parent = Path(selection_path).parent
    if any((parent / name).exists() for name in ("azure-setup-state.json", "azure-image-prep-state.json", "azure-terraform", "operator-template.json")):
        raise ValueError("Private network selection cannot change after retained preparation")
    network = ipaddress.ip_network(private_cidr(network_cidr), strict=True)
    management = private_cidr(management_cidr)
    if network.prefixlen != 16 or not acknowledge_route:
        raise ValueError("Select an unused private /16 and acknowledge an existing private management route")
    runtime, controller = list(network.subnets(new_prefix=24))[:2]
    variables.update(runtime_subnet_cidr=str(runtime), controller_subnet_cidr=str(controller),
                     gateway_private_ip=str(runtime.network_address + 10),
                     controller_private_ip=str(controller.network_address + 10),
                     management_source_cidr=management)
    core.atomic_json(selection_path, selection)
    result = {"version": 1, "state": "private_route_prerequisite_acknowledged_unverified",
              "scope": variables["scope"], "selectionDigest": core.digest(selection),
              "networkCIDR": str(network), "managementSourceCIDR": management,
              "controllerPrivateIP": variables["controller_private_ip"], "connected": False,
              "message": "An existing private route is required; no VPN, Bastion, public IP or gateway is created."}
    core.atomic_json(parent / "azure-private-location.json", result)
    return result


def connection_script(mount):
    if not re.fullmatch(r"/mnt/[a-z0-9-]+", mount):
        raise ValueError("Exact controller mount is required")
    # This is an authenticated ARM guest read, not keyscan or trust-on-first-use.
    return """set -eu
python3 - <<'PY'
import json, pathlib, urllib.request
root = pathlib.Path(%s)
request = urllib.request.Request('http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01', headers={'Metadata':'true'})
with urllib.request.urlopen(request, timeout=3) as response:
    compute = json.load(response)
receipt = {'vmID': compute['vmId'], 'bootID': pathlib.Path('/proc/sys/kernel/random/boot_id').read_text().strip(),
           'generation': json.loads((root/'luci/generation.json').read_bytes()),
           'disk': json.loads((root/'luci/disk-receipt.json').read_bytes()),
           'hostKey': pathlib.Path('/etc/ssh/ssh_host_ed25519_key.pub').read_text().strip()}
print('LUCI_AZURE_HOST_IDENTITY=' + json.dumps(receipt, separators=(',', ':')))
PY
""" % repr(mount)


def connection_descriptor(core, bundle_root, selection_path, provider=None):
    provider = provider or az_json
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    private_cidr(variables.get("management_source_cidr", ""))
    host = str(ipaddress.ip_address(variables["controller_private_ip"]))
    private_cidr(host + "/32")
    parent = Path(selection_path).parent
    state = core.private_json(parent / "azure-setup-state.json")
    if state.get("state") not in ("controller_bootstrapped_gateway_unverified", "controller_finalized_private_route_unverified") or state.get("selectionDigest") != core.digest(selection):
        raise ValueError("Connection needs exact bootstrapped infrastructure readback")
    identities = state["resourceIDs"]
    controller_id = identities["controller_instance_id"].lower()
    prefix = "/subscriptions/" + selection["stateBackend"]["subscriptionId"] + "/resourcegroups/" + variables["resource_group_name"] + "/providers/microsoft.compute/virtualmachines/"
    if not controller_id.startswith(prefix.lower()):
        raise ValueError("Controller resource differs from the selected account")
    vm = provider(["rest", "--method", "get", "--url", "https://management.azure.com" + controller_id + "?api-version=2023-09-01"])
    vm_id = vm.get("properties", {}).get("vmId", "")
    uuid_pattern = r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}"
    if vm.get("id", "").lower() != controller_id or not re.fullmatch(uuid_pattern, vm_id):
        raise RuntimeError("Exact authenticated controller identity is unavailable")
    journal = {"version": 1, "state": "host_identity_readback_pending", "selectionDigest": core.digest(selection),
               "controllerResourceID": controller_id, "generation": variables["generation"]}
    core.atomic_json(parent / "azure-connection-state.json", journal)
    response = provider(["vm", "run-command", "invoke", "--ids", controller_id,
                         "--command-id", "RunShellScript", "--scripts", connection_script(variables["controller_mount_path"])])
    markers = [line[len("LUCI_AZURE_HOST_IDENTITY="):] for row in response.get("value", [])
               for line in row.get("message", "").splitlines() if line.startswith("LUCI_AZURE_HOST_IDENTITY=")]
    if len(markers) != 1:
        raise RuntimeError("Controller host identity readback is uncertain")
    observed = json.loads(markers[0])
    generation = observed.get("generation", {})
    disk = observed.get("disk", {})
    key = observed.get("hostKey", "").split()
    if (observed.get("vmID") != vm_id or not re.fullmatch(uuid_pattern, observed.get("bootID", "")) or
            generation.get("generation") != variables["generation"] or generation.get("controllerInstanceId", "").lower() != controller_id or
            generation.get("azureDataDiskId", "").lower() != identities["controller_disk_id"].lower() or
            disk.get("scope") != variables["scope"] or disk.get("ownerId") != variables["controller_owner_id"] or
            disk.get("azureDataDiskId", "").lower() != identities["controller_disk_id"].lower() or
            len(key) not in (2, 3) or key[0] != "ssh-ed25519"):
        raise RuntimeError("Controller host key is not bound to the selected resource and fenced generation")
    try:
        raw = base64.b64decode(key[1], validate=True)
    except ValueError as error:
        raise ValueError("Controller SSH public key is malformed") from error
    if len(raw) != 51 or raw[:19] != b'\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20':
        raise ValueError("Controller SSH public key is malformed")
    descriptor = dict(journal, state="host_identity_observed_private_route_unverified",
        observedAtUTC=datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        sourceCommit=manifest["luciRevision"], bundleDigest=core.digest(manifest), scope=variables["scope"],
        vmID=vm_id, bootID=observed["bootID"], host=host, port=22, user="luci",
        dataDiskID=identities["controller_disk_id"].lower(),
        hostKey=" ".join(key[:2]), fingerprint="SHA256:" + base64.b64encode(hashlib.sha256(raw).digest()).decode().rstrip("="),
        productAPI="http://127.0.0.1:8730", readbackDigest=core.digest(observed), connected=False)
    core.atomic_json(parent / "azure-connection.json", descriptor)
    core.atomic_json(parent / "azure-connection-state.json", descriptor)
    return descriptor


def az_json(arguments, timeout=300):
    # Bound the child output while it runs. A post-hoc capture_output length
    # check would already have buffered an arbitrarily large CLI response.
    with tempfile.TemporaryFile() as output:
        process = subprocess.Popen(["az", *arguments, "--only-show-errors", "--output", "json"],
                                   stdout=output, stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL)
        deadline = time.monotonic() + min(max(timeout, 1), 7200)
        try:
            while process.poll() is None:
                if output.seek(0, 2) > 128 << 10 or time.monotonic() >= deadline:
                    raise RuntimeError("Azure response exceeded setup bounds or timed out; retain its journal")
                time.sleep(0.05)
            if process.returncode or output.seek(0, 2) > 128 << 10:
                raise RuntimeError("Azure stage failed or is uncertain; retain its journal")
            output.seek(0)
            return json.loads(output.read((128 << 10) + 1))
        finally:
            if process.poll() is None:
                process.kill()
            process.wait()


def gallery_plan(core, bundle_root, selection_path, role, image_definition_id, version, managed_image_id):
    manifest = core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    variables = core.requested(selection)
    if role not in ("runtime", "controller") or not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Select a runtime/controller gallery version")
    if (Path(selection_path).parent / ("azure-gallery-" + role + ".json")).exists():
        raise ValueError("Gallery operation is retained; reconcile the exact saved plan")
    prefix = "/subscriptions/" + selection["stateBackend"]["subscriptionId"] + "/resourceGroups/"
    if (not image_definition_id.lower().startswith(prefix.lower()) or
            not re.fullmatch(r"/subscriptions/[0-9a-f-]{36}/resourceGroups/[^/]+/providers/Microsoft.Compute/galleries/[a-zA-Z0-9_.-]+/images/[a-zA-Z0-9_.-]+", image_definition_id, re.I) or
            not managed_image_id.lower().startswith(prefix.lower()) or
            not re.fullmatch(r"/subscriptions/[0-9a-f-]{36}/resourceGroups/[^/]+/providers/Microsoft.Compute/images/[a-zA-Z0-9_.-]+", managed_image_id, re.I)):
        raise ValueError("Use exact selected-subscription image definition and generalized managed image IDs")
    prep = core.private_json(Path(selection_path).parent / "azure-image-prep-state.json")
    package = prep.get(role + "Package", {})
    if prep.get("state") != "image_roots_prepared_pending_gallery_publication" or not core.package_ready(package):
        raise ValueError("Exact prepared image package is required before gallery publication")
    image_id = image_definition_id + "/versions/" + version
    body = {"location": selection["region"], "tags": {"luci-scope": variables["scope"],
             "luci-revision": manifest["luciRevision"], "luci-package-sha256": package["sha256"]},
            "properties": {"storageProfile": {"source": {"id": managed_image_id}},
                           "publishingProfile": {"replicaCount": 1,
                              "targetRegions": [{"name": selection["region"], "regionalReplicaCount": 1}]}}}
    plan = {"version": 1, "role": role, "sourceCommit": manifest["luciRevision"],
            "selectionDigest": core.digest(selection), "scope": variables["scope"],
            "imageVersionID": image_id, "managedImageID": managed_image_id, "body": body,
            "package": package, "priceVerified": False, "finalInvoiceVerified": False,
            "prerequisite": "The generalized managed image must contain the exact prepared root; tags alone do not prove its contents."}
    plan["planDigest"] = core.digest(plan)
    core.atomic_json(Path(selection_path).parent / ("azure-gallery-" + role + "-plan.json"), plan)
    return plan


def gallery_publish(core, bundle_root, selection_path, role, plan_digest, acknowledge=False, reconcile=False, provider=az_json):
    if role not in ("runtime", "controller"):
        raise ValueError("Select a runtime/controller gallery version")
    core.bundle(bundle_root)
    parent = Path(selection_path).parent
    plan = core.private_json(parent / ("azure-gallery-" + role + "-plan.json"))
    expected = dict(plan)
    expected.pop("planDigest", None)
    if plan.get("planDigest") != plan_digest or core.digest(expected) != plan_digest or core.digest(core.private_json(selection_path)) != plan["selectionDigest"]:
        raise ValueError("Gallery publication plan is stale")
    journal_path = parent / ("azure-gallery-" + role + ".json")
    url = "https://management.azure.com" + plan["imageVersionID"] + "?api-version=2023-07-03"
    if not reconcile:
        if not acknowledge or journal_path.exists():
            raise ValueError("A new gallery publication needs cost acknowledgement and no retained operation")
        # List first: do not treat an authentication/network error as absence.
        inventory = provider(["rest", "--method", "get", "--url", url.rsplit("/versions/", 1)[0] + "/versions?api-version=2023-07-03"])
        if not isinstance(inventory.get("value"), list) or inventory.get("nextLink") or any(row.get("id", "").lower() == plan["imageVersionID"].lower() for row in inventory["value"]):
            raise ValueError("Gallery version exists or bounded absence cannot be established")
        core.atomic_json(journal_path, {"state": "publication_pending", "planDigest": plan_digest, "imageVersionID": plan["imageVersionID"]})
        provider(["rest", "--method", "put", "--url", url, "--body", json.dumps(plan["body"])])
    else:
        journal = core.private_json(journal_path)
        if journal.get("planDigest") != plan_digest or journal.get("imageVersionID") != plan["imageVersionID"]:
            raise ValueError("Gallery readback differs from the retained operation")
    observed = provider(["rest", "--method", "get", "--url", url])
    props = observed.get("properties", {})
    if (observed.get("id", "").lower() != plan["imageVersionID"].lower() or
            observed.get("tags") != plan["body"]["tags"] or
            props.get("storageProfile", {}).get("source", {}).get("id", "").lower() != plan["managedImageID"].lower()):
        raise RuntimeError("Gallery identity/source readback is uncertain; do not replay publication")
    state = "gallery_observed" if props.get("provisioningState") == "Succeeded" else "publication_pending"
    receipt = {"state": state, "planDigest": plan_digest, "imageVersionID": plan["imageVersionID"],
               "readbackDigest": core.digest(observed), "sourceCommit": plan["sourceCommit"],
               "priceVerified": False, "finalInvoiceVerified": False}
    core.atomic_json(journal_path, receipt)
    return receipt


def bind_images(core, bundle_root, selection_path):
    core.bundle(bundle_root)
    selection = core.private_json(selection_path)
    core.requested(selection)
    parent = Path(selection_path).parent
    if any((parent / name).exists() for name in ("azure-setup-state.json", "azure-terraform")):
        raise ValueError("Image selection cannot change after retained infrastructure preparation")
    original_digest = core.digest(selection)
    for role in ("runtime", "controller"):
        plan = core.private_json(parent / ("azure-gallery-" + role + "-plan.json"))
        receipt = core.private_json(parent / ("azure-gallery-" + role + ".json"))
        if (plan.get("selectionDigest") != original_digest or receipt.get("state") != "gallery_observed" or
                receipt.get("planDigest") != plan.get("planDigest") or receipt.get("imageVersionID") != plan.get("imageVersionID")):
            raise ValueError("Both exact gallery publications need authenticated source readback before binding")
        selection["terraformVariables"][role + "_image_version_id"] = receipt["imageVersionID"]
    core.atomic_json(selection_path, selection)
    return {"state": "gallery_versions_bound_private_infrastructure_pending", "selectionDigest": core.digest(selection), "connected": False}
