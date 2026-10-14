"""Explicit private foundation stages; local journals precede remote state.

The adapter performs no calls on import. A pending PUT is reconciled by GET;
even an authenticated 404 cannot authorize repeating that PUT.
"""
from datetime import datetime, timezone
import ipaddress
import json
from pathlib import Path
import re
import socket
import ssl
import urllib.error
import urllib.request
import uuid

ARM = "https://management.azure.com"
UUID = r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}"
VNET = re.compile(r"/subscriptions/(" + UUID + r")/resourceGroups/([a-zA-Z0-9_.()-]{1,90})/providers/Microsoft.Network/virtualNetworks/([a-zA-Z0-9_.-]{1,64})\Z", re.I)
PRIVATE = tuple(ipaddress.ip_network(v) for v in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"))
STORAGE_ROLE = "ba92f5b4-2d11-453d-a403-e96b0029c9fe"
VAULT_ROLE = "b86a8fe4-44ce-4948-aee5-eccb2c155cd7"


def now():
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        raise RuntimeError("Azure management redirect refused")


class AzureARM:
    def __init__(self, cli):
        # CLI authentication is user-run. Tokens remain private process data,
        # never command arguments, returned receipts or durable journals.
        value = cli(["account", "get-access-token", "--resource", ARM])
        self.token = value.get("accessToken", "")
        if not isinstance(self.token, str) or not 32 <= len(self.token) <= 16384:
            raise RuntimeError("Azure management authentication unavailable")
        self.client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def __call__(self, method, path, api, body=None, headers=None):
        if method not in ("GET", "PUT", "PATCH", "POST", "DELETE") or not path.startswith("/subscriptions/") or "?" in path or "#" in path:
            raise ValueError("Exact Azure resource operation required")
        request = urllib.request.Request(ARM + path + "?api-version=" + api,
            data=json.dumps(body).encode() if body is not None else None, method=method,
            headers={"Authorization": "Bearer " + self.token, "Content-Type": "application/json", **(headers or {})})
        try:
            with self.client.open(request, timeout=30) as response:
                data = response.read((128 << 10) + 1)
                if len(data) > 128 << 10:
                    raise RuntimeError("Azure management response exceeded bounds")
                return json.loads(data) if data else {}
        except urllib.error.HTTPError as error:
            if method == "GET" and error.code == 404:
                return None
            raise RuntimeError("Azure management operation failed or is uncertain; preserve its journal") from None


def private_network(value):
    network = ipaddress.ip_network(value, strict=True)
    if network.version != 4 or not any(network.subnet_of(parent) for parent in PRIVATE):
        raise ValueError("Selected address space must be entirely RFC1918")
    return network


def unused_subnets(spaces, occupied):
    roots = [private_network(v) for v in spaces]
    used = [private_network(v) for v in occupied]
    candidates = []
    for root in roots:
        if root.prefixlen > 24:
            continue
        for subnet in root.subnets(new_prefix=24):
            if not any(subnet.overlaps(v) for v in used + candidates):
                candidates.append(subnet)
                if len(candidates) == 2:
                    return [str(v) for v in candidates]
    raise ValueError("Existing VNet needs two unused /24 subnets; no connectivity or expansion is created")


def tcp_probe(host, port):
    # Caller selects an existing private listener, never an invented VM/IP.
    with socket.create_connection((host, port), timeout=3):
        return True


def inventory(core, bundle_root, selection_path, vnet_id, endpoint_subnet, route_host,
              route_port, management_cidr, principal_id, arm, account, probe=tcp_probe):
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    if any((parent / n).exists() for n in ("azure-foundation-plan.json", "azure-setup-state.json", "operator-template.json", "azure-image-prep-state.json")):
        raise ValueError("Private inventory is retained; do not change a prepared foundation")
    match = VNET.fullmatch(vnet_id)
    if not match or not re.fullmatch(UUID, principal_id) or account.get("id", "").lower() != match[1].lower() or not re.fullmatch(UUID, account.get("tenantId", "")):
        raise ValueError("Select the authenticated subscription, tenant and operator object ID")
    management = private_network(management_cidr)
    row = arm("GET", vnet_id, "2023-09-01")
    if not row or row.get("id", "").lower() != vnet_id.lower() or row.get("location") != selected["region"]:
        raise ValueError("Exact existing VNet in the selected region is required")
    properties = row.get("properties", {})
    subnets = properties.get("subnets", [])
    if not isinstance(subnets, list) or len(subnets) > 256 or properties.get("provisioningState") != "Succeeded":
        raise ValueError("Existing VNet inventory is incomplete or unsettled")
    occupied, endpoint = [], None
    for subnet in subnets:
        prop = subnet.get("properties", {})
        occupied.extend(prop.get("addressPrefixes", []) or [prop.get("addressPrefix", "")])
        if subnet.get("name") == endpoint_subnet:
            endpoint = subnet
    spaces = properties.get("addressSpace", {}).get("addressPrefixes", [])
    proposed = unused_subnets(spaces, occupied)
    zones = arm('GET', '/subscriptions/' + match[1] + '/providers/Microsoft.Network/privateDnsZones', '2020-06-01')
    if not zones or zones.get('nextLink') or not isinstance(zones.get('value'), list) or len(zones['value']) > 128:
        raise ValueError('Bounded private DNS inventory is required before proposing foundation links')
    linked_zones, dns_dependencies = [], []
    for zone in zones['value']:
        if zone.get('name') not in ('privatelink.blob.core.windows.net', 'privatelink.vaultcore.azure.net'):
            continue
        links = arm('GET', zone['id'] + '/virtualNetworkLinks', '2020-06-01')
        if not links or links.get('nextLink'):
            raise ValueError('Private DNS link inventory is incomplete')
        for link in links.get('value', []):
            if link.get('properties', {}).get('virtualNetwork', {}).get('id', '').lower() == vnet_id.lower():
                linked_zones.append(zone['id'])
                dns_dependencies.extend([{'id': zone['id'], 'apiVersion': '2020-06-01', 'body': {'location': 'global'}, 'externallyOwned': True},
                    {'id': link['id'], 'apiVersion': '2020-06-01', 'body': {'properties': {'virtualNetwork': {'id': vnet_id}, 'virtualNetworkLinkState': 'Completed'}}, 'externallyOwned': True}])
    if not endpoint or endpoint.get("properties", {}).get("privateEndpointNetworkPolicies") != "Disabled" or endpoint.get("properties", {}).get("delegations"):
        raise ValueError("Select an existing nondelegated private-endpoint subnet with endpoint policies disabled")
    address = ipaddress.ip_address(route_host)
    if address.version != 4 or not any(address in private_network(v) for v in occupied) or not 1 <= route_port <= 65535 or not probe(str(address), route_port):
        raise ValueError("An explicit successful probe to an existing private VNet listener is required")
    receipt = {"version": 1, "state": "private_location_observed", "sourceCommit": manifest["luciRevision"],
        "scope": variables["scope"], "generation": variables["generation"], "observedAtUTC": now(),
        "subscriptionID": match[1].lower(), "tenantID": account["tenantId"], "operatorObjectID": principal_id,
        "vnetID": vnet_id, "vnetResourceGroup": match[2], "vnetName": match[3], "vnetETag": row.get("etag"),
        "endpointSubnetID": endpoint["id"], "addressSpaces": spaces, "occupiedSubnets": occupied,
        "existingLinkedPrivateDNSZoneIDs": linked_zones, "dnsServers": properties.get('dhcpOptions', {}).get('dnsServers', []),
        "existingDNSDependencies": dns_dependencies,
        "runtimeSubnetCIDR": proposed[0], "controllerSubnetCIDR": proposed[1], "managementSourceCIDR": str(management),
        "routeProbe": {"host": str(address), "port": route_port, "observed": True}, "connected": False}
    core.atomic_json(parent / "azure-private-location.json", receipt)
    return receipt


def plan(core, bundle_root, selection_path, foundation_group):
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    if (parent / "azure-foundation-plan.json").exists() or not re.fullmatch(r"[a-zA-Z0-9_.()-]{1,90}", foundation_group):
        raise ValueError("Use a new exact foundation resource group and preserve retained plans")
    location = core.private_json(parent / "azure-private-location.json")
    if location.get("state") != "private_location_observed" or location.get("sourceCommit") != manifest["luciRevision"] or location.get("scope") != variables["scope"] or not location.get("routeProbe", {}).get("observed"):
        raise ValueError("Current authenticated private location and route inventory is required")
    subscription = location["subscriptionID"]
    group_id = "/subscriptions/" + subscription + "/resourceGroups/" + foundation_group
    suffix = core.digest([subscription, variables["scope"], foundation_group])[:16]
    storage_name, vault_name = "lucist" + suffix, "luci-kv-" + suffix
    storage = group_id + "/providers/Microsoft.Storage/storageAccounts/" + storage_name
    vault = group_id + "/providers/Microsoft.KeyVault/vaults/" + vault_name
    tags = {"luci-owner": variables["controller_owner_id"], "luci-scope": variables["scope"], "luci-source": manifest["luciRevision"], "luci-purpose": "private-foundation"}
    region = selected["region"]
    resources = []
    dependencies = location.get('existingDNSDependencies', [])
    dns_resource_group = foundation_group
    def add(identifier, api, body, kind):
        resources.append({"id": identifier, "apiVersion": api, "body": body, "kind": kind})
    add(group_id, "2022-09-01", {"location": region, "tags": tags}, "resourceGroup")
    add(storage, "2023-05-01", {"location": region, "tags": tags, "kind": "StorageV2", "sku": {"name": "Standard_LRS"}, "properties": {
        "minimumTlsVersion": "TLS1_2", "allowSharedKeyAccess": False, "allowBlobPublicAccess": False,
        "publicNetworkAccess": "Disabled", "networkAcls": {"bypass": "None", "defaultAction": "Deny"}, "supportsHttpsTrafficOnly": True}}, "storage")
    add(storage + "/blobServices/default", "2023-05-01", {"properties": {"isVersioningEnabled": True,
        "deleteRetentionPolicy": {"enabled": True, "days": 30}, "containerDeleteRetentionPolicy": {"enabled": True, "days": 30}}}, "blobService")
    add(storage + "/blobServices/default/containers/luci-state", "2023-05-01", {"properties": {"publicAccess": "None"}}, "container")
    add(vault, "2023-07-01", {"location": region, "tags": tags, "properties": {"tenantId": location["tenantID"], "sku": {"family": "A", "name": "standard"},
        "enableRbacAuthorization": True, "enableSoftDelete": True, "enablePurgeProtection": True, "softDeleteRetentionInDays": 90,
        "publicNetworkAccess": "Disabled", "networkAcls": {"bypass": "None", "defaultAction": "Deny"}, "accessPolicies": []}}, "vault")
    for identifier, role in ((storage, STORAGE_ROLE), (vault, VAULT_ROLE)):
        role_id = str(uuid.uuid5(uuid.NAMESPACE_URL, identifier.lower() + location["operatorObjectID"] + role))
        add(identifier + "/providers/Microsoft.Authorization/roleAssignments/" + role_id, "2022-04-01", {"properties": {
            "principalId": location["operatorObjectID"], "roleDefinitionId": "/subscriptions/" + subscription + "/providers/Microsoft.Authorization/roleDefinitions/" + role}}, "role")
    for name, target, group in (("blob", storage, "blob"), ("vault", vault, "vault")):
        zone_name = "privatelink.blob.core.windows.net" if name == "blob" else "privatelink.vaultcore.azure.net"
        zone = group_id + "/providers/Microsoft.Network/privateDnsZones/" + zone_name
        existing = [identifier for identifier in location.get('existingLinkedPrivateDNSZoneIDs', []) if identifier.endswith('/' + zone_name)]
        if len(existing) > 1:
            raise ValueError('Existing DNS zone ownership is ambiguous')
        if existing:
            zone = existing[0]
            if name == 'blob': dns_resource_group = zone.split('/')[4]
        else:
            add(zone, "2020-06-01", {"location": "global", "tags": tags}, "dnsZone")
            add(zone + "/virtualNetworkLinks/luci-" + suffix, "2020-06-01", {"location": "global", "tags": tags,
                "properties": {"virtualNetwork": {"id": location["vnetID"]}, "registrationEnabled": False}}, "dnsLink")
        endpoint = group_id + "/providers/Microsoft.Network/privateEndpoints/luci-" + name + "-" + suffix
        add(endpoint, "2023-09-01", {"location": region, "tags": tags, "properties": {"subnet": {"id": location["endpointSubnetID"]},
            "privateLinkServiceConnections": [{"name": "luci-" + name, "properties": {"privateLinkServiceId": target, "groupIds": [group]}}]}}, "endpoint")
        add(endpoint + "/privateDnsZoneGroups/default", "2023-09-01", {"properties": {"privateDnsZoneConfigs": [{"name": name, "properties": {"privateDnsZoneId": zone}}]}}, "dnsGroup")
    result = {"version": 1, "state": "foundation_review_required", "sourceCommit": manifest["luciRevision"], "scope": variables["scope"],
        "generation": variables["generation"], "selectionDigest": core.digest(selected), "locationDigest": core.digest(location),
        "resourceGroup": foundation_group, "dnsResourceGroup": dns_resource_group, "storageID": storage, "storageName": storage_name, "vaultID": vault, "vaultName": vault_name,
        "dependencies": dependencies,
        "resources": resources, "monthlyBudgetUsd": selected["monthlyBudgetUsd"], "priceVerified": False, "finalInvoiceVerified": False,
        "retainedCharges": ["state storage/version retention", "Key Vault/soft-delete retention", "two private endpoints", "private DNS zones/links"],
        "message": "Review actual account pricing and permissions. Budget is a policy limit, not an Azure bill cap. No public connectivity is created."}
    result["planDigest"] = core.digest(result)
    core.atomic_json(parent / "azure-foundation-plan.json", result)
    return result


def matches(row, expected):
    """Exact planned security/identity properties; provider-added fields allowed."""
    if isinstance(expected, dict):
        return isinstance(row, dict) and all(key in row and matches(row[key], value) for key, value in expected.items())
    if isinstance(expected, list):
        return isinstance(row, list) and len(row) == len(expected) and all(matches(a, b) for a, b in zip(row, expected))
    if isinstance(expected, str) and expected.startswith('/subscriptions/'):
        return isinstance(row, str) and row.lower() == expected.lower()
    return row == expected


def reuse(core, bundle_root, selection_path, storage_id, vault_id, dns_group, arm):
    """Existing privately configured resources stay externally owned."""
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    location = core.private_json(parent / 'azure-private-location.json')
    prefix = '/subscriptions/' + location['subscriptionID'] + '/resourceGroups/'
    storage_match = re.fullmatch(re.escape(prefix) + r'([a-zA-Z0-9_.()-]{1,90})/providers/Microsoft.Storage/storageAccounts/([a-z0-9]{3,24})', storage_id, re.I)
    vault_match = re.fullmatch(re.escape(prefix) + r'[a-zA-Z0-9_.()-]{1,90}/providers/Microsoft.KeyVault/vaults/([a-z0-9-]{3,24})', vault_id, re.I)
    if not storage_match or not vault_match or not re.fullmatch(r'[a-zA-Z0-9_.()-]{1,90}', dns_group) or (parent / 'azure-foundation-plan.json').exists():
        raise ValueError('Exact existing private state/vault/DNS resources in the selected subscription are required')
    resources = [
        {'id': storage_id, 'apiVersion': '2023-05-01', 'kind': 'storage', 'body': {'properties': {
            'minimumTlsVersion': 'TLS1_2', 'allowSharedKeyAccess': False, 'allowBlobPublicAccess': False, 'publicNetworkAccess': 'Disabled'}}},
        {'id': storage_id + '/blobServices/default/containers/luci-state', 'apiVersion': '2023-05-01', 'kind': 'container', 'body': {'properties': {'publicAccess': 'None'}}},
        {'id': vault_id, 'apiVersion': '2023-07-01', 'kind': 'vault', 'body': {'properties': {'tenantId': location['tenantID'], 'enableRbacAuthorization': True, 'publicNetworkAccess': 'Disabled'}}}]
    for zone in ('privatelink.blob.core.windows.net', 'privatelink.vaultcore.azure.net'):
        identifier = prefix + dns_group + '/providers/Microsoft.Network/privateDnsZones/' + zone
        resources.append({'id': identifier, 'apiVersion': '2020-06-01', 'kind': 'dnsZone', 'body': {'location': 'global'}})
        links = arm('GET', identifier + '/virtualNetworkLinks', '2020-06-01')
        if not links or links.get('nextLink') or not any(row.get('properties', {}).get('virtualNetwork', {}).get('id', '').lower() == location['vnetID'].lower() and row.get('properties', {}).get('virtualNetworkLinkState') == 'Completed' for row in links.get('value', [])):
            raise ValueError('Existing private DNS zones must already be linked to the selected VNet')
    completed = []
    for resource in resources:
        row = arm('GET', resource['id'], resource['apiVersion'])
        if not row or row.get('id', '').lower() != resource['id'].lower() or not matches(row, resource['body']):
            raise ValueError('Existing resource is not the exact private configuration; no changes made')
        completed.append({'id': resource['id'], 'readbackDigest': core.digest(row), 'observedAtUTC': now()})
    result = {'version': 1, 'state': 'external_private_foundation_reuse', 'sourceCommit': manifest['luciRevision'],
        'scope': variables['scope'], 'generation': variables['generation'], 'selectionDigest': core.digest(selected),
        'locationDigest': core.digest(location), 'resourceGroup': storage_match[1], 'dnsResourceGroup': dns_group,
        'storageID': storage_id, 'storageName': storage_match[2], 'vaultID': vault_id, 'vaultName': vault_match[1],
        'resources': resources, 'externallyOwned': True, 'priceVerified': False, 'finalInvoiceVerified': False,
        'retainedCharges': ['existing state storage', 'existing private Key Vault', 'existing DNS/private endpoints']}
    result['planDigest'] = core.digest(result)
    core.atomic_json(parent / 'azure-foundation-plan.json', result)
    core.atomic_json(parent / 'azure-foundation-state.json', {'version': 1, 'state': 'foundation_observed_pending_private_data_route',
        'planDigest': result['planDigest'], 'completed': completed, 'externallyOwned': True})
    return result


def step(core, bundle_root, selection_path, plan_digest, arm, acknowledge=False, reconcile=False):
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    parent = Path(selection_path).parent
    proposal = core.private_json(parent / "azure-foundation-plan.json")
    if proposal.get('externallyOwned') or proposal["sourceCommit"] != manifest["luciRevision"] or proposal["selectionDigest"] != core.digest(selected) or proposal["planDigest"] != plan_digest or core.digest({k: v for k, v in proposal.items() if k != "planDigest"}) != plan_digest:
        raise ValueError("Foundation plan/source/selection changed")
    journal_path = parent / "azure-foundation-state.json"
    journal = core.private_json(journal_path) if journal_path.exists() else {"version": 1, "state": "not_started", "planDigest": plan_digest, "completed": []}
    if journal["planDigest"] != plan_digest:
        raise ValueError("Retained foundation operation belongs to another plan")
    index = len(journal["completed"])
    if [v.get('id') for v in journal['completed']] != [v['id'] for v in proposal['resources'][:index]]:
        raise ValueError('Foundation completion journal differs from reviewed resource order')
    if index == len(proposal["resources"]):
        return journal
    resource = proposal["resources"][index]
    for dependency in proposal.get('dependencies', []):
        row = arm('GET', dependency['id'], dependency['apiVersion'])
        if not row or not matches(row, dependency['body']):
            raise ValueError('Existing private DNS dependency changed; no mutation permitted')
    if journal.get("pending"):
        if not reconcile or journal["pending"] != resource["id"]:
            raise ValueError("Foundation operation is uncertain; reconcile by GET without repeating PUT")
    else:
        if reconcile or not acknowledge:
            raise ValueError("Review exact foundation costs and acknowledge before one staged operation")
        existing = arm("GET", resource["id"], resource["apiVersion"])
        # BlobServices/default exists with its storage account; configure only
        # this child of our exact already observed owned storage resource.
        if existing is not None and resource["kind"] != "blobService":
            raise ValueError("Planned foundation resource already exists; use verified reuse, never adopt it silently")
        journal = dict(journal, state="foundation_write_pending", pending=resource["id"], sourceCommit=proposal["sourceCommit"], scope=proposal["scope"], generation=proposal["generation"])
        core.atomic_json(journal_path, journal)
        arm("PUT", resource["id"], resource["apiVersion"], resource["body"])
    observed = arm("GET", resource["id"], resource["apiVersion"])
    if not observed or observed.get("id", "").lower() != resource["id"].lower() or not matches(observed, resource["body"]) or observed.get("properties", {}).get("provisioningState", "Succeeded") != "Succeeded":
        raise RuntimeError("Foundation operation remains uncertain or differs from reviewed private properties")
    journal["completed"].append({"id": resource["id"], "readbackDigest": core.digest(observed), "observedAtUTC": now()})
    journal.pop("pending", None)
    journal["state"] = "foundation_observed_pending_private_data_route" if len(journal["completed"]) == len(proposal["resources"]) else "foundation_stage_observed"
    core.atomic_json(journal_path, journal)
    return journal


def bind(core, bundle_root, selection_path, arm, cli, resolver=socket.getaddrinfo):
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    proposal = core.private_json(parent / "azure-foundation-plan.json")
    journal = core.private_json(parent / "azure-foundation-state.json")
    location = core.private_json(parent / "azure-private-location.json")
    if journal.get("state") not in ("foundation_observed_pending_private_data_route", "foundation_bound_private_data_route_observed") or journal.get("planDigest") != proposal["planDigest"] or proposal["sourceCommit"] != manifest["luciRevision"] or proposal['locationDigest'] != core.digest(location):
        raise ValueError("Exact observed private foundation is required")
    # Re-read every exact resource before handing remote state to Terraform.
    for resource in proposal["resources"] + proposal.get('dependencies', []):
        observed = arm("GET", resource["id"], resource["apiVersion"])
        if not observed or not matches(observed, resource["body"]):
            raise ValueError("Foundation private identity/configuration changed")
    vnet = arm('GET', location['vnetID'], '2023-09-01')
    used = [private_network(v) for row in vnet.get('properties', {}).get('subnets', []) for v in
            (row.get('properties', {}).get('addressPrefixes', []) or [row.get('properties', {}).get('addressPrefix', '')])]
    if any(private_network(candidate).overlaps(existing) for candidate in (location['runtimeSubnetCIDR'], location['controllerSubnetCIDR']) for existing in used):
        raise ValueError('Proposed runtime/controller address space was taken; review a new location before application')
    for host in (proposal["storageName"] + ".blob.core.windows.net", proposal["vaultName"] + ".vault.azure.net"):
        ips = {row[4][0] for row in resolver(host, 443, type=socket.SOCK_STREAM)}
        if not ips or any(not any(ipaddress.ip_address(ip) in private_network(space) for space in location["addressSpaces"]) for ip in ips):
            raise ValueError("Private endpoint DNS must resolve only inside the selected VNet")
    # Real authenticated private data-plane reads prove route and operator RBAC.
    container = cli(["storage", "container", "show", "--account-name", proposal["storageName"], "--name", "luci-state", "--auth-mode", "login"])
    if container.get("name") != "luci-state" or container.get("properties", {}).get("publicAccess") not in (None, "None", "off"):
        raise ValueError("Private state container readback unavailable")
    cli(["keyvault", "secret", "list", "--vault-name", proposal["vaultName"], "--maxresults", "1"])
    runtime, controller = location["runtimeSubnetCIDR"], location["controllerSubnetCIDR"]
    variables.update(subscription_id=location["subscriptionID"], resource_group_name=proposal["resourceGroup"],
        key_vault_id=proposal["vaultID"], key_vault_name=proposal["vaultName"], existing_vnet_name=location["vnetName"],
        existing_vnet_resource_group_name=location["vnetResourceGroup"], foundation_private_dns_resource_group_name=proposal.get('dnsResourceGroup', proposal["resourceGroup"]),
        runtime_subnet_cidr=runtime, controller_subnet_cidr=controller, gateway_private_ip=str(private_network(runtime).network_address + 10),
        controller_private_ip=str(private_network(controller).network_address + 10), management_source_cidr=location["managementSourceCIDR"])
    selected["stateBackend"].update(subscriptionId=location["subscriptionID"], resourceGroupName=proposal["resourceGroup"],
        storageAccountName=proposal["storageName"], containerName="luci-state", key=variables["scope"] + "/terraform.tfstate")
    core.atomic_json(selection_path, selected)
    journal.update(state="foundation_bound_private_data_route_observed", selectionDigest=core.digest(selected), observedAtUTC=now(), connected=False, finalInvoiceVerified=False)
    core.atomic_json(parent / "azure-foundation-state.json", journal)
    return journal
