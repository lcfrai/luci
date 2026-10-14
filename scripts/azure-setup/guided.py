"""Small, journaled user-run Azure setup stages. No implicit provider calls."""
import ipaddress
import json
from pathlib import Path
import re
import subprocess


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
    for identifier, relative in (("credentials", "azure-credentials/journal.json"),
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


def wizard(core, bundle_root, selection_path, ask=input):
    """Ordinary questions only; the existing validator owns account selection."""
    questions = (
        ("subscription_id", "Azure subscription ID"), ("resource_group", "Existing resource group"),
        ("state_resource_group", "Existing Terraform state resource group"),
        ("state_storage_account", "Existing state storage account"),
        ("state_container", "Existing private state Blob container"),
        ("zone", "Availability zone"), ("runtime_size", "KVM-capable runtime VM size"),
        ("controller_size", "Controller VM size"),
        ("ssh_public_key_file", "Absolute SSH public-key file"),
        ("key_vault_id", "Existing Key Vault ARM ID"), ("key_vault_name", "Key Vault name"),
        ("fence_storage_account", "Unused off-disk fence storage account name"))
    choices = {key: ask(label + ": ").strip() for key, label in questions}
    return core.configure_account(bundle_root, selection_path, **choices)


def az_json(arguments):
    result = subprocess.run(["az", *arguments, "--only-show-errors", "--output", "json"],
                            capture_output=True, timeout=300, check=False)
    if result.returncode or len(result.stdout) > 128 << 10:
        raise RuntimeError("Azure stage failed or is uncertain; retain its journal")
    return json.loads(result.stdout)


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
