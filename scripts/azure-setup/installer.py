#!/usr/bin/env python3
"""Version-bound BYO Azure Develop plan and explicit-effect entry point.

This is a user-run installer, never an implicit Luci server provisioner.
`plan` performs no account call. `apply` requires the exact displayed plan
digest, saved authority/cost acknowledgement and complete immutable inputs.
"""

import argparse
import base64
import hashlib
import importlib.util
import ipaddress
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import stat
import subprocess
import sys
import tempfile
import uuid


SHA = re.compile(r"[0-9a-f]{64}\Z")
GIT = re.compile(r"[0-9a-f]{40}\Z")
NAME = re.compile(r"[a-z][a-z0-9-]{1,39}\Z")
REGION = re.compile(r"[a-z][a-z0-9-]{1,39}\Z")
MODULE_FILES = {"compute.tf", "fence.tf", "identity.tf", "moved.tf", "network.tf", "outputs.tf", "versions.tf", "variables.tf"}
SCRIPT_FILES = {"azure-setup/installer.py", "azure-setup/credentials.py", "azure-setup/guided.py", "azure-setup/foundation.py", "azure-setup/builder.py", "azure-setup/builder_guest.py", "azure-setup/source_bundle.py", "azure-setup/successor.py", "azure-setup/operator_setup.py", "azure-setup/README.md", "azure-embed-host/package.py",
                "azure-embed-host/service.py", "azure-embed-host/deliver.py", "azure-embed-host/guest_registry.py", "azure-controller-disk/package.py",
                "azure-controller-disk/mount.py", "azure-controller-disk/recover.py"}
SOURCE_FILES = {'builder-source/source.json', 'builder-source/luci.pack', 'builder-source/e2b.pack'}
EXPECTED_FILES = {"LICENSE"} | SCRIPT_FILES | SOURCE_FILES | {"terraform/" + name for name in MODULE_FILES | {".terraform.lock.hcl"}}
PLAN_VERSION = 1
BACKEND_KEYS = {"subscriptionId", "resourceGroupName", "storageAccountName", "containerName", "key"}


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def digest(value):
    return hashlib.sha256(canonical(value)).hexdigest()


def bounded_json(path, limit=128 << 10, private=False):
    path = Path(path)
    info = path.lstat()
    if path.is_symlink() or not stat.S_ISREG(info.st_mode) or private and info.st_mode & 0o077 or info.st_size == 0 or info.st_size > limit:
        raise ValueError("Azure setup input is unavailable or not private when required")
    return json.loads(path.read_bytes())


def private_json(path, limit=128 << 10):
    return bounded_json(path, limit, private=True)


def atomic_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, name = tempfile.mkstemp(prefix=".azure-setup-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as output:
            output.write(canonical(value) + b"\n")
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


def bundle(root):
    root = Path(root)
    if root.is_symlink() or not root.is_dir():
        raise ValueError("Azure setup bundle root is unsafe")
    manifest = bounded_json(root / "manifest.json")
    if manifest.get("version") != 1 or not GIT.fullmatch(manifest.get("luciRevision", "")):
        raise ValueError("Azure setup bundle source is not pinned")
    files = manifest.get("files", {})
    if not isinstance(files, dict) or set(files) != EXPECTED_FILES:
        raise ValueError("Azure setup bundle is incomplete")
    if any(not isinstance(row, dict) or set(row) != {'sha256','size'} or not isinstance(row['size'], int) or isinstance(row['size'], bool) or not 0 < row['size'] <= 4 << 20 for row in files.values()):
        raise ValueError('Azure setup bundle member bound is invalid')
    if sum(row.get('size', 8 << 20) for row in files.values()) > 8 << 20:
        raise ValueError('Azure setup bundle exceeds its bounded source inventory')
    for name, binding in files.items():
        path = root / name
        if (name.startswith("/") or "\\" in name or any(part in ("", ".", "..") for part in name.split("/")) or
                any(parent.is_symlink() for parent in (path, *path.parents) if parent == root or root in parent.parents) or
                not path.is_file() or not 0 < path.stat().st_size <= 4 << 20 or path.stat().st_size != binding.get("size") or
                hashlib.sha256(path.read_bytes()).hexdigest() != binding.get("sha256")):
            raise ValueError("Azure setup bundle member changed")
    source = bounded_json(root / 'builder-source/source.json', limit=2 << 20)
    if source.get('luci', {}).get('revision') != manifest['luciRevision'] or source.get('e2b', {}).get('revision') != 'cf536c284434fdd2f09bbab35ec807a5dacc87e4':
        raise ValueError('Embedded source revision differs from setup bundle')
    for role in ('luci', 'e2b'):
        binding = files['builder-source/' + role + '.pack']
        if source.get(role, {}).get('pack') != {'sha256': binding['sha256'], 'bytes': binding['size']}:
            raise ValueError('Embedded source pack differs from setup bundle')
    return manifest


def requested(selection):
    required = {"version", "mode", "name", "region", "monthlyBudgetUsd", "maxConcurrentVMs",
                "terraformVariables", "runtimePackage", "controllerPackage", "stateBackend", "controllerHealthURL", "nextGeneration"}
    if set(selection) != required or selection["version"] != 1 or selection["mode"] != "byo-azure-develop" or not NAME.fullmatch(selection["name"]) or not REGION.fullmatch(selection["region"]):
        raise ValueError("Azure Develop selection is invalid")
    budget = selection["monthlyBudgetUsd"]
    concurrency = selection["maxConcurrentVMs"]
    if isinstance(budget, bool) or not isinstance(budget, (int, float)) or budget <= 0 or budget > 100000 or isinstance(concurrency, bool) or not isinstance(concurrency, int) or concurrency < 1 or concurrency > 1024:
        raise ValueError("Azure spend or concurrency policy is invalid")
    if selection["controllerHealthURL"] != "http://127.0.0.1:8730/v1/healthz" or not isinstance(selection["nextGeneration"], str) or \
            selection["nextGeneration"] and not NAME.fullmatch(selection["nextGeneration"]):
        raise ValueError("Private controller health and successor generation are invalid")
    variables = selection["terraformVariables"]
    if not isinstance(variables, dict) or variables.get("name") != selection["name"] or variables.get("location") != selection["region"]:
        raise ValueError("Azure Terraform selection does not match the requested name and region")
    backend = selection["stateBackend"]
    if not isinstance(backend, dict) or set(backend) != BACKEND_KEYS or not all(isinstance(value, str) for value in backend.values()):
        raise ValueError("Azure setup needs a separate durable Azure Blob state backend")
    if backend["subscriptionId"] and not re.fullmatch(r"[0-9a-f-]{36}", backend["subscriptionId"]):
        raise ValueError("Azure state backend subscription is invalid")
    if backend["storageAccountName"] and not re.fullmatch(r"[a-z0-9]{3,24}", backend["storageAccountName"]):
        raise ValueError("Azure state backend storage account is invalid")
    if backend["containerName"] and not re.fullmatch(r"[a-z0-9-]{3,63}", backend["containerName"]):
        raise ValueError("Azure state backend container is invalid")
    if backend["key"] and (not re.fullmatch(r"[a-zA-Z0-9_./-]{1,160}", backend["key"]) or
                           any(part in ("", ".", "..") for part in backend["key"].split("/"))):
        raise ValueError("Azure state backend key is invalid")
    if variables.get("subscription_id", "") not in ("", backend["subscriptionId"]):
        raise ValueError("Azure provider subscription differs from the durable state selection")
    for key in ("runtimePackage", "controllerPackage"):
        value = selection[key]
        if (not isinstance(value, dict) or set(value) != {"path", "sha256"} or
                not isinstance(value["path"], str) or not isinstance(value["sha256"], str) or
                (value["sha256"] and not SHA.fullmatch(value["sha256"])) or
                (value["path"] and not Path(value["path"]).is_absolute())):
            raise ValueError("Azure package input requires an absolute file and SHA-256 when supplied")
    return variables


def initial_selection(bundle_root, selection_path, name, region, budget, concurrency, private_network_cidr=""):
    """Create the owner-private advanced selection from four ordinary choices.

    Empty account-specific fields are deliberately *not* invented.  A later
    plan lists them before any provider effect can be attempted.
    """
    bundle(bundle_root)
    if not NAME.fullmatch(name) or not REGION.fullmatch(region):
        raise ValueError("Choose a short Azure setup name and region")
    path = Path(selection_path).absolute()
    if path.exists() or path.is_symlink() or not path.parent.is_dir() or path.parent.is_symlink() or \
            path.parent.stat().st_mode & 0o077:
        raise ValueError("Use a new file in an existing owner-private directory")
    variables = {name: "" for name in re.findall(
        r'^variable "([a-z0-9_]+)"', (Path(bundle_root) / "terraform/variables.tf").read_text(), re.M)}
    scope = name + "-" + uuid.uuid4().hex[:12]
    variables.update(name=name, location=region, scope=scope,
                     generation="gen-" + uuid.uuid4().hex[:12],
                     controller_owner_id=str(uuid.uuid4()),
                     controller_host_id=str(uuid.uuid4()), runtime_host_id=str(uuid.uuid4()),
                     controller_filesystem_uuid=str(uuid.uuid4()),
                     controller_disk_size_gb=64, controller_mount_path="/mnt/luci-controller",
                     controller_operator_template_path="/etc/luci/operator-template.json",
                     controller_operator_output_path="/etc/luci/operator.json",
                     gateway_zone_name=scope + ".internal")
    if private_network_cidr:
        try:
            network = ipaddress.ip_network(private_network_cidr, strict=True)
        except ValueError as error:
            raise ValueError("Choose an unused private /16 network for Azure Develop") from error
        if network.version != 4 or network.prefixlen != 16 or not any(network.subnet_of(ipaddress.ip_network(parent)) for parent in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")):
            raise ValueError("Choose an unused private /16 network for Azure Develop")
        children = list(network.subnets(new_prefix=24))
        variables.update(runtime_subnet_cidr=str(children[0]), controller_subnet_cidr=str(children[1]),
                         controller_private_ip=str(children[1].network_address + 10),
                         gateway_private_ip=str(children[0].network_address + 10))
    variables.pop("previous_generation", None)
    variables.pop("tags", None)
    selection = {"version": 1, "mode": "byo-azure-develop", "name": name, "region": region,
                 "monthlyBudgetUsd": budget, "maxConcurrentVMs": concurrency,
                 "controllerHealthURL": "http://127.0.0.1:8730/v1/healthz",
                 "nextGeneration": "gen-" + uuid.uuid4().hex[:12],
                 "runtimePackage": {"path": "", "sha256": ""},
                 "controllerPackage": {"path": "", "sha256": ""},
                 "stateBackend": {name: "" for name in BACKEND_KEYS},
                 "terraformVariables": variables}
    requested(selection)
    atomic_json(path, selection)
    plan = make_plan(selection, bundle(bundle_root), path, bundle_root)
    return {"selectionPath": str(path), "selectionDigest": plan["selectionDigest"],
            "planDigest": plan["planDigest"], "missing": plan["missing"],
            "providerEffects": False}


def initial_image_inputs(bundle_root, target, *, luci_source, e2b_source, image_digests,
                         envoy_binary, envoy_license, runtime_image_root, controller_image_root,
                         operator_template):
    """Bind selected local image inputs without asking for hand-authored JSON."""
    bundle(bundle_root)
    target = Path(target).absolute()
    if target.exists() or target.is_symlink() or not target.parent.is_dir() or \
            target.parent.stat().st_mode & 0o077:
        raise ValueError("Use a new file in an existing owner-private directory")
    inputs = {"luciSource": luci_source, "e2bSource": e2b_source,
              "imageDigests": image_digests, "envoyBinary": envoy_binary,
              "envoyLicense": envoy_license, "runtimeImageRoot": runtime_image_root,
              "controllerImageRoot": controller_image_root, "operatorTemplate": operator_template}
    if any(not isinstance(value, str) or not Path(value).is_absolute() or Path(value).is_symlink()
           for value in inputs.values()):
        raise ValueError("Image inputs must be absolute nonlinked paths")
    if runtime_image_root == controller_image_root:
        raise ValueError("Runtime and controller image roots must be separate")
    atomic_json(target, inputs)
    return {"imageInputsPath": str(target), "inputsDigest": digest(inputs), "providerEffects": False}


def configure_account(bundle_root, selection_path, *, subscription_id, resource_group,
                      state_resource_group, state_storage_account, state_container,
                      zone, runtime_size, controller_size, ssh_public_key_file,
                      key_vault_id="", key_vault_name="", fence_storage_account="",
                      runtime_image_version_id="", controller_image_version_id=""):
    """Fill ordinary account/host fields without exposing selection internals."""
    bundle(bundle_root)
    selected = private_json(selection_path)
    requested(selected)
    parent = Path(selection_path).resolve().parent
    if any((parent / name).exists() for name in ("azure-setup-state.json", "azure-image-prep-state.json", "azure-terraform", "operator-template.json")):
        raise ValueError("Azure setup has retained work; selection cannot change until exact review")
    if not re.fullmatch(r"[0-9a-f-]{36}", subscription_id) or not all(
            re.fullmatch(r"[a-zA-Z0-9_.()-]{1,90}", value) for value in (resource_group, state_resource_group)) or \
            not re.fullmatch(r"[a-z0-9]{3,24}", state_storage_account) or \
            not re.fullmatch(r"[a-z0-9-]{3,63}", state_container) or \
            not re.fullmatch(r"[1-9][0-9]?", zone) or \
            not all(re.fullmatch(r"Standard_[A-Za-z0-9_]{2,80}", value) for value in (runtime_size, controller_size)):
        raise ValueError("Azure account, state, zone or VM size selection is invalid")
    key_path = Path(ssh_public_key_file)
    if not key_path.is_absolute() or key_path.is_symlink() or not key_path.is_file() or key_path.stat().st_size > 4096:
        raise ValueError("Use an existing absolute SSH public-key file")
    key = key_path.read_text().strip()
    if not re.fullmatch(r"(?:ssh-ed25519|ssh-rsa) [A-Za-z0-9+/=]{32,4096}(?: [^\r\n]{0,128})?", key):
        raise ValueError("SSH public key is not a supported public-key line")
    arm_image = re.compile(r"/subscriptions/[0-9a-f-]{36}/resourceGroups/[a-zA-Z0-9_.()-]+/providers/Microsoft.Compute/galleries/[a-zA-Z0-9_.-]+/images/[a-zA-Z0-9_.-]+/versions/[a-zA-Z0-9_.-]+", re.I)
    if any(value and not arm_image.fullmatch(value) for value in (runtime_image_version_id, controller_image_version_id)) or \
            key_vault_name and not re.fullmatch(r"[a-z0-9-]{3,24}", key_vault_name) or \
            fence_storage_account and not re.fullmatch(r"[a-z0-9]{3,24}", fence_storage_account) or \
            key_vault_id and not re.fullmatch(r"/subscriptions/[0-9a-f-]{36}/resourceGroups/[^/]+/providers/Microsoft.KeyVault/vaults/[^/]+", key_vault_id, re.I):
        raise ValueError("Selected image, Key Vault or fence storage identity is invalid")
    variables = selected["terraformVariables"]
    variables.update(subscription_id=subscription_id, resource_group_name=resource_group, zone=zone,
                     runtime_vm_size=runtime_size, controller_vm_size=controller_size,
                     controller_admin_ssh_public_key=key, runtime_admin_ssh_public_key=key,
                     runtime_image_version_id=runtime_image_version_id,
                     controller_image_version_id=controller_image_version_id,
                     key_vault_id=key_vault_id, key_vault_name=key_vault_name,
                     fence_storage_account_name=fence_storage_account)
    selected["stateBackend"] = {"subscriptionId": subscription_id, "resourceGroupName": state_resource_group,
                                "storageAccountName": state_storage_account, "containerName": state_container,
                                "key": "luci/" + variables["scope"] + ".tfstate"}
    requested(selected)
    atomic_json(selection_path, selected)
    plan = make_plan(selected, bundle(bundle_root), selection_path, bundle_root)
    return {"selectionDigest": plan["selectionDigest"], "planDigest": plan["planDigest"],
            "missing": plan["missing"], "nextSteps": plan["nextSteps"], "providerEffects": False}


def package_ready(value):
    if not value["path"] or not SHA.fullmatch(value["sha256"]):
        return False
    path = Path(value["path"])
    if not path.is_file() or path.is_symlink() or path.stat().st_size > 256 << 20:
        return False
    with path.open("rb") as source:
        measured = hashlib.file_digest(source, "sha256").hexdigest()
    return measured == value["sha256"]


def make_plan(selection, manifest, selection_path, bundle_root):
    variables = requested(selection)
    missing = []
    for name, value in selection["stateBackend"].items():
        if not value:
            missing.append("state_backend_" + name)
    if not selection["nextGeneration"] or selection["nextGeneration"] == variables.get("generation"):
        missing.append("distinct_next_controller_generation")
    declared = set(re.findall(r'^variable "([a-z0-9_]+)"', (Path(bundle_root) / "terraform/variables.tf").read_text(), re.M)) - {"previous_generation", "tags", "management_source_cidr", "existing_vnet_name", "existing_vnet_resource_group_name", "foundation_private_dns_resource_group_name"}
    for name in sorted(declared):
        if variables.get(name) in (None, "", [], {}):
            missing.append("terraform_" + name)
    builder_ready = False
    if (Path(selection_path).parent / 'azure-builder-state.json').exists():
        spec = importlib.util.spec_from_file_location('luci_azure_builder', Path(bundle_root) / 'azure-setup/builder.py')
        builder_module = importlib.util.module_from_spec(spec); spec.loader.exec_module(builder_module)
        builder_ready = builder_module.observed_packages(sys.modules[__name__], selection_path, manifest)
    for key, source in (("runtime_package", "runtimePackage"), ("controller_package", "controllerPackage")):
        if not builder_ready and not package_ready(selection[source]):
            missing.append(key + "_exact_archive")
    if variables.get("runtime_package_sha256") != selection["runtimePackage"]["sha256"]:
        missing.append("runtime_package_terraform_binding")
    for key in ("runtime_image_version_id", "controller_image_version_id", "key_vault_id", "key_vault_name", "controller_operator_template_sha256"):
        if not variables.get(key):
            missing.append(key)
    if not variables.get("host_secret_refs") or not variables.get("controller_secret_ids") or not variables.get("host_secret_ids"):
        missing.append("exact_key_vault_secret_references")
    plan = {"version": PLAN_VERSION, "provider": "azure", "purpose": "develop-microvm", "luciRevision": manifest["luciRevision"],
            "bundleDigest": digest(manifest), "selectionDigest": digest(selection), "selectionPath": str(Path(selection_path).resolve()),
            "bundlePath": str(Path(bundle_root).resolve()), "name": selection["name"], "region": selection["region"],
            "maxConcurrentVMs": selection["maxConcurrentVMs"], "monthlyBudgetUsd": selection["monthlyBudgetUsd"],
            "resources": ["private controller and separate recovery VM", "one protected authoritative managed disk and external Blob fence",
                          "dedicated E2B runtime host and private mTLS gateway", "one newly allocated microVM per running agent"],
            "missing": sorted(set(missing)), "priceVerified": False,
            "priceNote": "Account, region, quota, image and recurring price are not queried by this offline plan; review Azure's current quote before applying."}
    plan["nextSteps"] = [
        "Select the Azure subscription, existing resource group, durable Blob state and an unused private network.",
        "Select KVM-capable host/controller sizes and prepare exact runtime/controller gallery image versions.",
        "Generate credentials locally, then explicitly publish exact versions to the selected private Key Vault.",
        "Select a signed E2B guest template, model egress and reviewed task/concurrency limits.",
        "Review the account's actual monthly quote and quota before the explicit provider plan/apply.",
    ] if missing else ["Review the exact Terraform plan and account price before any effect."]
    plan["planDigest"] = digest(plan)
    script = Path(bundle_root).resolve() / "azure-setup/installer.py"
    common = ["--bundle", str(Path(bundle_root).resolve()), "--selection", str(Path(selection_path).resolve())]
    plan["commands"] = [shlex.join(["python3", str(script), "continue", *common]),
                        shlex.join(["python3", str(script), "status", *common])]
    return plan


def plan_command(bundle_root, selection_path):
    manifest = bundle(bundle_root)
    selection = private_json(selection_path)
    return make_plan(selection, manifest, selection_path, bundle_root)


def credential_step(bundle_root, selection_path, action, *, acknowledge=False):
    bundle(bundle_root)
    selected = private_json(selection_path)
    requested(selected)
    root = Path(selection_path).resolve().parent / "azure-credentials"
    module_path = Path(bundle_root) / "azure-setup/credentials.py"
    module_spec = importlib.util.spec_from_file_location("luci_azure_setup_credentials", module_path)
    module = importlib.util.module_from_spec(module_spec)
    module_spec.loader.exec_module(module)
    if action == "generate-secrets":
        if any((root.parent / name).exists() for name in ("azure-setup-state.json", "azure-image-prep-state.json", "azure-terraform")):
            raise ValueError("Azure setup already has retained work; credentials cannot be regenerated")
        return module.generate(selected, root, atomic_json)
    if action == "publish-secrets":
        return module.publish(selected, root, atomic_json, acknowledge=acknowledge)
    if action == "reconcile-secrets":
        return module.reconcile_pending(selected, root, atomic_json)
    if action == "bind-secrets":
        if any((root.parent / name).exists() for name in ("azure-setup-state.json", "azure-image-prep-state.json", "azure-terraform")):
            raise ValueError("Azure setup already has retained work; selection cannot be rebound")
        return module.bind_selection(selected, root, atomic_json, selection_path)
    raise ValueError("Unknown Azure credential action")


def work_paths(selection_path):
    root = Path(selection_path).resolve().parent
    return root / "azure-setup-state.json", root / "azure-terraform"


def selected_workspace(bundle_root, selection_path, plan):
    recovery_path = Path(selection_path).parent / 'azure-successor-state.json'
    if recovery_path.exists():
        raise ValueError('Predecessor Terraform authority is retained for signed recovery; no stale apply or attachment replay')
    state_path, work = work_paths(selection_path)
    source = Path(bundle_root) / "terraform"
    if work.is_symlink():
        raise ValueError("Azure work directory cannot be a symlink")
    work.mkdir(mode=0o700, exist_ok=True)
    for name in MODULE_FILES | {".terraform.lock.hcl"}:
        original = source / name
        target = work / name
        if not original.is_file() or original.is_symlink():
            raise ValueError("Azure Terraform source is incomplete")
        data = original.read_bytes()
        if target.exists():
            if target.is_symlink() or target.read_bytes() != data:
                raise ValueError("Azure Terraform work copy changed")
        else:
            target.write_bytes(data)
            target.chmod(0o600)
    tfvars = work / "selection.auto.tfvars.json"
    encoded = canonical(private_json(selection_path)["terraformVariables"])
    if tfvars.exists() and (tfvars.is_symlink() or tfvars.read_bytes() != encoded):
        raise ValueError("Azure Terraform inputs changed after plan selection")
    if not tfvars.exists():
        tfvars.write_bytes(encoded)
        tfvars.chmod(0o600)
    backend = private_json(selection_path)["stateBackend"]
    values = {"subscription_id": backend["subscriptionId"], "resource_group_name": backend["resourceGroupName"],
              "storage_account_name": backend["storageAccountName"], "container_name": backend["containerName"],
              "key": backend["key"], "use_azuread_auth": True}
    backend_file = work / "backend.hcl"
    backend_data = "".join(key + " = " + json.dumps(value) + "\n" for key, value in values.items()).encode()
    if backend_file.exists() and (backend_file.is_symlink() or backend_file.read_bytes() != backend_data):
        raise ValueError("Azure durable state backend changed after plan selection")
    if not backend_file.exists():
        backend_file.write_bytes(backend_data)
        backend_file.chmod(0o600)
    if (work / "terraform.tfstate").exists() or (work / "terraform.tfstate.backup").exists():
        raise ValueError("local Terraform state is refused; configure the durable Blob backend")
    return state_path, work


def terraform(work, *args, timeout=120):
    command = ["terraform", *args]
    result = subprocess.run(command, cwd=work, capture_output=True, timeout=timeout, check=False,
                            env={**os.environ, "TF_IN_AUTOMATION": "1", "TF_INPUT": "0"})
    if result.returncode:
        # The original stdout/stderr stay in the person's private work area
        # and may include provider metadata. The UI gets only a bounded stage.
        raise RuntimeError("Terraform stage failed or is uncertain: " + args[0])
    if len(result.stdout) > 2 << 20:
        raise RuntimeError("Terraform output exceeds the bounded setup receipt")
    return result.stdout


def local_command(arguments, cwd=None, timeout=600):
    result = subprocess.run(arguments, cwd=cwd, capture_output=True, timeout=timeout, check=False)
    if result.returncode or len(result.stdout) > 4096:
        raise RuntimeError("Local Azure image preparation failed at " + Path(arguments[0]).name)
    return result.stdout.decode().strip()


def prepare_images(bundle_root, selection_path, inputs_path, acknowledge_roots=False):
    manifest = bundle(bundle_root)
    requested(private_json(selection_path))
    inputs = private_json(inputs_path)
    required = {"luciSource", "e2bSource", "imageDigests", "envoyBinary", "envoyLicense",
                "runtimeImageRoot", "controllerImageRoot", "operatorTemplate"}
    if set(inputs) != required or not acknowledge_roots or importlib.util.find_spec("yaml") is None:
        raise ValueError("Image preparation needs exact source inputs, PyYAML and acknowledged isolated image roots")
    if not all(isinstance(value, str) for value in inputs.values()):
        raise ValueError("Image preparation paths are invalid")
    paths = {key: Path(value) for key, value in inputs.items()}
    if any(not path.is_absolute() or path.is_symlink() for path in paths.values()):
        raise ValueError("Image preparation inputs require absolute nonlinked paths")
    source = paths["luciSource"]
    if local_command(["git", "-C", str(source), "rev-parse", "HEAD"]) != manifest["luciRevision"] or \
            local_command(["git", "-C", str(source), "status", "--porcelain"]):
        raise ValueError("Luci image source is not the exact clean setup revision")
    for key in ("runtimeImageRoot", "controllerImageRoot"):
        root = paths[key]
        if not root.is_dir() or root == paths["runtimeImageRoot"] and root == paths["controllerImageRoot"]:
            raise ValueError("Runtime and controller need distinct existing isolated image roots")
    template = paths["operatorTemplate"]
    template_bytes = template.read_bytes()
    if len(template_bytes) == 0 or len(template_bytes) > 16384:
        raise ValueError("Controller operator template is missing or oversized")
    operator = json.loads(template_bytes)
    deferred = operator.get("setupDeferred")
    if deferred is not None and (not isinstance(deferred, dict) or deferred.get("version") != 1 or
            deferred.get("sourceCommit") != manifest["luciRevision"] or
            deferred.get("scope") != private_json(selection_path)["terraformVariables"]["scope"] or
            deferred.get("generation") != private_json(selection_path)["terraformVariables"]["generation"] or
            not deferred.get("signerPublicKey", "").startswith("-----BEGIN PUBLIC KEY-----")):
        raise ValueError("Deferred operator selection differs from the exact setup source and generation")
    secrets = operator.get("azureSecrets", {})
    refs = ("apiKey", "adminKey", "attachKey", "clientCert", "clientKey")
    if (operator.get("deployment", {}).get("provider") != "azure" or
            any(operator.get(name) for name in ("apiKeyFile", "adminApiKeyFile", "attachKeyFile", "clientCertFile", "clientKeyFile")) or
            not isinstance(secrets, dict) or set(secrets) != {"vaultUrl", "clientId", "directory", *refs} or
            not isinstance(secrets["vaultUrl"], str) or not re.fullmatch(r"https://[a-z0-9-]+\.vault\.azure\.net", secrets["vaultUrl"]) or
            not isinstance(secrets["clientId"], str) or not (deferred and secrets["clientId"] == "" or re.fullmatch(r"[0-9a-f-]{36}", secrets["clientId"])) or
            not isinstance(secrets["directory"], str) or not secrets["directory"].startswith("/run/") or
            any(not isinstance(secrets[name], dict) or set(secrets[name]) != {"name", "version"} or
                not isinstance(secrets[name]["name"], str) or not isinstance(secrets[name]["version"], str) or
                not re.fullmatch(r"[a-zA-Z0-9-]{1,127}", secrets[name]["name"]) or
                not re.fullmatch(r"[0-9a-f]{32}", secrets[name]["version"]) for name in refs) or
            not isinstance(operator.get("store"), dict)):
        raise ValueError("Controller image template needs Azure Key Vault references, not inline credentials")
    variables = private_json(selection_path)["terraformVariables"]
    template_target = variables.get("controller_operator_template_path")
    if not isinstance(template_target, str) or not re.fullmatch(r"/etc/luci/[a-z0-9.-]+", template_target) or \
            hashlib.sha256(template_bytes).hexdigest() != variables.get("controller_operator_template_sha256"):
        raise ValueError("Controller image template differs from Terraform selection")
    if deferred and any((paths["controllerImageRoot"] / "etc/luci" / name).exists() or
                        (paths["controllerImageRoot"] / "etc/luci" / name).is_symlink()
                        for name in ("azure-private-ca.crt", "azure-controller.env")):
        raise ValueError("Controller image already has private trust or service configuration")
    state_path = Path(selection_path).parent / "azure-image-prep-state.json"
    if state_path.exists():
        raise ValueError("Previous image preparation is retained; inspect before a new build")
    work = Path(selection_path).parent / "image-prep"
    if work.exists():
        raise ValueError("Image preparation work directory already exists")
    work.mkdir(mode=0o700)
    binding = {"version": 1, "state": "image_prep_pending", "luciRevision": manifest["luciRevision"],
               "selectionDigest": digest(private_json(selection_path)), "inputsDigest": digest(inputs),
               "runtimeRoot": str(paths["runtimeImageRoot"]), "controllerRoot": str(paths["controllerImageRoot"])}
    atomic_json(state_path, binding)
    binaries = {"azure-embed-host-bootstrap": "./control-plane/cmd/azure-embed-host-bootstrap",
                "azure-gateway-config": "./control-plane/cmd/azure-gateway-config",
                "luci-api": "./control-plane/cmd/api"}
    for name, source_path in binaries.items():
        local_command(["go", "build", "-trimpath", "-o", str(work / name), source_path], cwd=source, timeout=900)
    host_script = Path(bundle_root) / "azure-embed-host/package.py"
    controller_script = Path(bundle_root) / "azure-controller-disk/package.py"
    host_tar = work / "azure-embed-host.tar"
    controller_tar = work / "azure-controller.tar"
    host_sha = local_command(["python3", str(host_script), "build", "--upstream", str(paths["e2bSource"]),
                              "--luci", str(source), "--luci-revision", manifest["luciRevision"],
                              "--image-digests", str(paths["imageDigests"]), "--bootstrap", str(work / "azure-embed-host-bootstrap"),
                              "--gateway", str(work / "azure-gateway-config"), "--envoy", str(paths["envoyBinary"]),
                              "--envoy-license", str(paths["envoyLicense"]), "--output", str(host_tar)], timeout=300)
    controller_sha = local_command(["python3", str(controller_script), "build", "--source", str(source),
                                    "--revision", manifest["luciRevision"], "--api", str(work / "luci-api"),
                                    "--output", str(controller_tar)], timeout=300)
    if not SHA.fullmatch(host_sha) or not SHA.fullmatch(controller_sha):
        raise RuntimeError("Azure image package identity was not returned")
    for script, archive, sha, root, mount in (
            (host_script, host_tar, host_sha, paths["runtimeImageRoot"], None),
            (controller_script, controller_tar, controller_sha, paths["controllerImageRoot"], variables.get("controller_mount_path"))):
        local_command(["python3", str(script), "verify", "--archive", str(archive), "--sha256", sha])
        args = ["python3", str(script), "install-image", "--archive", str(archive), "--sha256", sha, "--root", str(root)]
        if mount is not None:
            args += ["--mount", mount]
        local_command(args)
    template_destination = paths["controllerImageRoot"] / template_target.lstrip("/")
    if template_destination.exists() or template_destination.is_symlink():
        raise ValueError("Controller image already has an operator template; retained image preparation needs review")
    if not deferred:
        # Legacy advanced image path; normal guided images stay generic.
        template_destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        template_destination.write_bytes(template_bytes)
        template_destination.chmod(0o600)
    if deferred:
        credentials = Path(selection_path).parent / "azure-credentials"
        journal = private_json(credentials / "journal.json")
        ca_data = (credentials / "ca.crt").read_bytes()
        if hashlib.sha256(ca_data).hexdigest() != journal.get("files", {}).get("client-ca"):
            raise ValueError("Deferred controller root CA changed")
        # Source-bound public trust and references are staged only after the
        # concrete controller exists. Captured generic images carry no active
        # account/generation selection, key material, projects or journals.
    result = dict(binding, state="image_roots_prepared_pending_gallery_publication",
                  runtimePackage={"path": str(host_tar), "sha256": host_sha},
                  controllerPackage={"path": str(controller_tar), "sha256": controller_sha},
                  templateSHA256=hashlib.sha256(template_bytes).hexdigest())
    atomic_json(state_path, result)
    selected = private_json(selection_path)
    selected["runtimePackage"] = result["runtimePackage"]
    selected["controllerPackage"] = result["controllerPackage"]
    selected["terraformVariables"]["runtime_package_sha256"] = result["runtimePackage"]["sha256"]
    atomic_json(selection_path, selected)
    return result


def prepare(bundle_root, selection_path, selected_digest):
    plan = plan_command(bundle_root, selection_path)
    if plan["planDigest"] != selected_digest or plan["missing"]:
        raise ValueError("Azure plan is stale or prerequisites are incomplete: " + ", ".join(plan["missing"]))
    state_path, work = selected_workspace(bundle_root, selection_path, plan)
    if state_path.exists():
        state = private_json(state_path)
        if state.get("state") not in ("prepared",) or state.get("planDigest") != selected_digest:
            raise ValueError("Azure setup has a retained operation; inspect before preparing again")
        return state
    terraform(work, "init", "-backend-config=backend.hcl", "-lockfile=readonly", "-input=false", "-no-color", timeout=180)
    terraform(work, "validate", "-no-color")
    terraform(work, "plan", "-out=selected.tfplan", "-input=false", "-no-color", timeout=300)
    raw = terraform(work, "show", "-json", "selected.tfplan")
    shown = json.loads(raw)
    changes = shown.get("resource_changes", [])
    managed = [row for row in changes if row.get('mode', 'managed') == 'managed']
    reads = [row for row in changes if row.get('mode') == 'data']
    if not managed or len(managed) + len(reads) != len(changes) or any(row.get('change', {}).get('actions') != ['create'] for row in managed) or any(row.get('change', {}).get('actions') not in (['read'], ['no-op']) for row in reads):
        raise ValueError("Azure setup needs a fresh empty durable state; existing or changed resources require exact recovery review")
    tfplan = work / "selected.tfplan"
    if tfplan.is_symlink() or not tfplan.is_file() or tfplan.stat().st_size > 16 << 20:
        raise ValueError("Azure provider plan identity is incomplete")
    record = {"version": 1, "state": "prepared", "planDigest": selected_digest,
              "selectionDigest": plan["selectionDigest"], "sourceCommit": plan["luciRevision"],
              "terraformPlanSHA256": hashlib.sha256(tfplan.read_bytes()).hexdigest(),
              "resourceCreates": len(managed),
              "resourceNoops": len(reads),
              "resourceName": plan["name"], "region": plan["region"], "monthlyBudgetUsd": plan["monthlyBudgetUsd"],
              "priceVerified": False}
    atomic_json(state_path, record)
    return record


def apply(bundle_root, selection_path, plan_digest, terraform_digest, costs, unverified_price):
    plan = plan_command(bundle_root, selection_path)
    if plan["planDigest"] != plan_digest or not costs or not unverified_price or not SHA.fullmatch(terraform_digest):
        raise ValueError("Azure apply requires the exact current plan and explicit recurring-cost acknowledgement")
    if plan["missing"]:
        raise ValueError("Azure prerequisites are incomplete: " + ", ".join(plan["missing"]))
    state_path, work = selected_workspace(bundle_root, selection_path, plan)
    state = private_json(state_path)
    if state.get("state") != "prepared" or state.get("planDigest") != plan_digest or state.get("terraformPlanSHA256") != terraform_digest:
        raise ValueError("Azure Terraform plan is not the exact prepared operation")
    tfplan = work / "selected.tfplan"
    if not tfplan.is_file() or tfplan.is_symlink() or hashlib.sha256(tfplan.read_bytes()).hexdigest() != terraform_digest:
        raise ValueError("Azure Terraform plan bytes changed")
    pending = dict(state, state="apply_pending")
    atomic_json(state_path, pending)
    # A lost Terraform apply response may have created billed resources.
    # Never replay the saved plan from apply_pending, including after restart.
    terraform(work, "apply", "-input=false", "-no-color", "selected.tfplan", timeout=1800)
    # A successful Terraform return still needs a separate read-only refresh
    # before Luci may treat these ARM identities as present. No second apply.
    result = dict(pending, state="apply_returned_pending_provider_readback")
    atomic_json(state_path, result)
    return result


def observed_outputs(bundle_root, selection_path, plan_digest):
    plan = plan_command(bundle_root, selection_path)
    if plan["planDigest"] != plan_digest or plan["missing"]:
        raise ValueError("Azure setup readback selection changed or is incomplete")
    state_path, work = selected_workspace(bundle_root, selection_path, plan)
    state = private_json(state_path)
    if state.get("planDigest") != plan_digest or state.get("state") not in (
            "apply_pending", "apply_returned_pending_provider_readback", "infrastructure_observed_pending_private_handoff"):
        raise ValueError("No exact retained Azure apply exists to reconcile")
    refresh = work / "reconcile.tfplan"
    if refresh.exists() and (refresh.is_symlink() or not refresh.is_file()):
        raise ValueError("Azure readback plan path changed")
    terraform(work, "plan", "-refresh-only", "-out=reconcile.tfplan", "-input=false", "-no-color", timeout=300)
    shown = json.loads(terraform(work, "show", "-json", "reconcile.tfplan"))
    if any(row.get("change", {}).get("actions") not in (["no-op"],) for row in shown.get("resource_drift", [])):
        raise RuntimeError("Azure readback found absent or changed infrastructure; retain the apply outcome for review")
    outputs = shown.get("planned_values", {}).get("outputs", {})
    required = ("controller_instance_id", "controller_recovery_instance_id", "controller_disk_id",
                "controller_fence_blob_url", "runtime_scale_set_id", "runtime_package_sha256",
                "runtime_host_config_sha256", "runtime_image_version_id", "controller_identity_client_id",
                "controller_recovery_identity_client_id", "controller_identity_resource_id",
                "controller_recovery_identity_resource_id", "private_gateway_host")
    identities = {name: outputs.get(name, {}).get("value") for name in required}
    if not all(isinstance(value, str) and value for value in identities.values()):
        raise RuntimeError("Azure readback does not contain every exact selected resource identity")
    variables = private_json(selection_path)["terraformVariables"]
    prefix = ("/subscriptions/" + variables["subscription_id"] + "/resourcegroups/" +
              variables["resource_group_name"] + "/providers/microsoft.compute/").lower()
    for key, kind in (("controller_instance_id", "virtualmachines/"),
                      ("controller_recovery_instance_id", "virtualmachines/"),
                      ("runtime_scale_set_id", "virtualmachinescalesets/")):
        if not identities[key].lower().startswith(prefix + kind):
            raise RuntimeError("Azure readback belongs to another subscription or resource group")
    if (identities["runtime_package_sha256"] != private_json(selection_path)["runtimePackage"]["sha256"] or
            identities["runtime_image_version_id"] != variables["runtime_image_version_id"] or
            identities["controller_disk_id"].lower().split("/providers/")[0] != prefix.split("/providers/")[0]):
        raise RuntimeError("Azure readback changed the selected package, image or disk scope")
    receipt = dict(state, state="infrastructure_observed_pending_private_handoff", resourceIDs=identities,
                   selectionDigest=plan["selectionDigest"], sourceCommit=plan["luciRevision"],
                   scope=variables["scope"], generation=variables["generation"],
                   readbackPlanSHA256=hashlib.sha256(refresh.read_bytes()).hexdigest())
    atomic_json(state_path, receipt)
    return receipt


def initial_recovery_selection(selection, identities):
    variables = selection["terraformVariables"]
    generation = {"version": 1, "generation": variables["generation"],
                  "controllerInstanceId": identities["controller_instance_id"],
                  "azureDataDiskId": identities["controller_disk_id"]}
    return {"version": 1, "recoveryClientID": identities["controller_recovery_identity_client_id"],
            "oldControllerID": identities["controller_instance_id"],
            "newControllerID": identities["controller_recovery_instance_id"],
            "diskID": identities["controller_disk_id"],
            "fenceBlobURL": identities["controller_fence_blob_url"],
            "controllerIdentityID": identities["controller_identity_resource_id"],
            "recoveryIdentityID": identities["controller_recovery_identity_resource_id"],
            "oldGeneration": variables["generation"], "newGeneration": selection["nextGeneration"],
            "filesystemUUID": variables["controller_filesystem_uuid"],
            "oldGenerationReceiptSHA256": digest(generation),
            "mountSelectionPath": "/etc/luci/azure-controller-disk-restore.json",
            "hostDeliverySelectionPath": "/etc/luci/azure-host-delivery.json",
            "apiService": "luci-api.service", "apiHealthURL": selection["controllerHealthURL"],
            "ownerID": variables["controller_owner_id"], "scope": variables["scope"]}


def controller_setup_files(selection_path):
    selected = private_json(selection_path)
    variables = requested(selected)
    parent = Path(selection_path).parent
    template = (parent / 'operator-template.json').read_bytes()
    if hashlib.sha256(template).hexdigest() != variables['controller_operator_template_sha256']:
        raise ValueError('Post-create operator source differs from exact selection')
    operator = json.loads(template)
    if 'setupDeferred' not in operator:
        return None
    if any(operator.get(key) for key in ('apiKeyFile', 'adminApiKeyFile', 'attachKeyFile', 'clientCertFile', 'clientKeyFile')):
        raise ValueError('Controller initialization accepts managed identity references only')
    credentials = parent / 'azure-credentials'
    ca_data = (credentials / 'ca.crt').read_bytes()
    if hashlib.sha256(ca_data).hexdigest() != private_json(credentials / 'journal.json').get('files', {}).get('client-ca'):
        raise ValueError('Post-create public trust changed')
    environment = ('LUCI_ADDR=127.0.0.1:8730\nLUCI_BACKING=azure\n'
        'LUCI_AZURE_OPERATOR_TEMPLATE=/etc/luci/operator.json\n'
        'LUCI_STATE_DIR=' + variables['controller_mount_path'] + '/luci/control\n'
        'LUCI_PROJECTS_DIR=' + variables['controller_mount_path'] + '/luci/projects\n'
        'LUCI_EXECUTION_STATE_DIR=' + variables['controller_mount_path'] + '/luci/execution\n')
    return [{'path': path, 'content': data.decode(), 'sha256': hashlib.sha256(data).hexdigest()} for path, data in (
        (variables['controller_operator_template_path'], template), ('/etc/luci/azure-private-ca.crt', ca_data),
        ('/etc/luci/azure-controller.env', environment.encode()))]


def initial_guest_script(recovery, reconcile=False):
    # Only source-selected resource names and nonsecret Key Vault references
    # enter ARM RunCommand. The script writes an owner-private exact selection
    # and invokes the already-installed recovery managed identity adapter.
    encoded = base64.b64encode(canonical(recovery)).decode()
    action = 'reconcile-initialize' if reconcile else 'initialize'
    return """set -eu
install -d -m 700 /etc/luci
python3 - <<'PY'
import base64, os, pathlib, tempfile
data = base64.b64decode('%s')
target = pathlib.Path('/etc/luci/azure-controller-recovery.json')
if target.exists():
    if target.is_symlink() or target.read_bytes() != data:
        raise SystemExit('retained recovery selection differs')
else:
    fd, name = tempfile.mkstemp(dir=target.parent)
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())
    os.replace(name, target)
PY
receipt=$(python3 /opt/luci/azure-controller/bin/recover.py %s /etc/luci/azure-controller-recovery.json)
printf 'LUCI_AZURE_INITIALIZE_RECEIPT=%%s\\n' "$receipt"
""" % (encoded, action)


def invoke_initial_controller(script, subscription, group, vm_name):
    if shutil.which("az") is None:
        raise ValueError("Azure CLI is required for the protected private guest handoff")
    spec = importlib.util.spec_from_file_location('luci_azure_guided', Path(__file__).with_name('guided.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.az_json(['vm', 'run-command', 'invoke', '--subscription', subscription,
        '--resource-group', group, '--name', vm_name, '--command-id', 'RunShellScript', '--scripts', script])


def bootstrap(bundle_root, selection_path, plan_digest, reconcile=False):
    plan = plan_command(bundle_root, selection_path)
    if plan["planDigest"] != plan_digest or plan["missing"]:
        raise ValueError("Azure controller bootstrap selection is stale or incomplete")
    state_path, _ = work_paths(selection_path)
    state = private_json(state_path)
    expected = 'bootstrap_pending' if reconcile else 'infrastructure_observed_pending_private_handoff'
    if state.get("state") != expected or state.get("planDigest") != plan_digest:
        raise ValueError("Azure infrastructure needs exact provider readback before controller handoff")
    selection = private_json(selection_path)
    recovery = initial_recovery_selection(selection, state["resourceIDs"])
    if (Path(selection_path).parent / 'operator-template.json').exists():
        files = controller_setup_files(selection_path)
        if files is not None:
            recovery['controllerSetupFiles'] = files
    if reconcile and state.get('recoverySelectionSHA256') != digest(recovery):
        raise ValueError('Retained bootstrap source or exact recovery selection changed')
    # Validate the static format with the shipped recovery adapter before any
    # ARM effect. It checks exact immutable IDs, fence and private paths.
    import importlib.util as util
    spec = util.spec_from_file_location("luci_azure_recovery", Path(bundle_root) / "azure-controller-disk/recover.py")
    module = util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.validate(recovery)
    script = initial_guest_script(recovery, reconcile)
    if shutil.which("az") is None:
        raise ValueError("Azure CLI is required before a protected controller handoff can be attempted")
    vm_name = state["resourceIDs"]["controller_recovery_instance_id"].rsplit("/", 1)[-1]
    pending = dict(state, state="bootstrap_pending", recoverySelectionSHA256=digest(recovery),
                   guestScriptSHA256=hashlib.sha256(script.encode()).hexdigest())
    if not reconcile:
        atomic_json(state_path, pending)
    try:
        response = invoke_initial_controller(script, selection["stateBackend"]["subscriptionId"],
                                             selection["terraformVariables"]["resource_group_name"], vm_name)
        messages = [row.get("message", "") for row in response.get("value", [])]
        markers = [line.removeprefix("LUCI_AZURE_INITIALIZE_RECEIPT=")
                   for message in messages for line in message.splitlines()
                   if line.startswith("LUCI_AZURE_INITIALIZE_RECEIPT=")]
        if len(markers) != 1:
            raise RuntimeError("Exact controller bootstrap receipt is unavailable")
        receipt = json.loads(markers[0])
        if receipt != {"state": "active", "generation": recovery["oldGeneration"], "diskID": recovery["diskID"]}:
            raise RuntimeError("Controller bootstrap result does not match the selected fence")
    except Exception as error:
        raise RuntimeError("Controller bootstrap outcome is uncertain; inspect exact Azure and fence identities without replay") from error
    result = dict(pending, state="controller_bootstrapped_gateway_unverified", bootstrapReceipt=receipt)
    atomic_json(state_path, result)
    return result


def main():
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="action", required=True)
    for action in ("init-selection", "configure-account", "init-image-inputs", "plan", "generate-secrets",
                   "publish-secrets", "reconcile-secrets", "bind-secrets", "prepare-images", "prepare", "apply",
                   "reconcile", "bootstrap", "status", "continue", "recover-controller", "prepare-operator", "template-step", "finalize-operator", "install-operator", "reconcile-operator", "bind-images", "configure-guided", "configure-private", "connection-descriptor", "gallery-plan", "gallery-publish", "gallery-reconcile", "foundation-inventory", "foundation-plan", "foundation-step", "foundation-reconcile", "foundation-bind", "foundation-reuse", "builder-source", "builder-plan", "builder-step"):
        row = commands.add_parser(action)
        row.add_argument("--bundle", type=Path, required=True)
        row.add_argument("--selection", type=Path, required=True)
        if action == 'builder-source':
            for key in ('luci-source', 'runtime-source', 'envoy-binary', 'envoy-license'):
                row.add_argument('--' + key, default='', help='Explicit compatibility input; normal setup uses packaged source')
        if action == 'builder-plan':
            row.add_argument('--base-image', required=True, help='Exact Canonical:ubuntu-24_04-lts:server:VERSION')
            row.add_argument('--builder-size', default='Standard_D4s_v5')
            row.add_argument('--gallery-name', required=True)
            row.add_argument('--image-version', required=True)
            row.add_argument('--lifetime-seconds', type=int, default=7200)
        if action == 'builder-step':
            row.add_argument('--plan-digest', required=True)
            row.add_argument('--stage', choices=('staging', 'allocate', 'grant', 'install', 'deprovision', 'deallocate', 'generalize', 'capture', 'retire', 'abort', 'bind', 'retire-staging'), required=True)
            row.add_argument('--role', choices=('runtime', 'controller'), default='runtime')
            row.add_argument('--acknowledge-costs', action='store_true')
            row.add_argument('--reconcile', action='store_true')
        if action == 'foundation-inventory':
            for key in ('vnet-id', 'endpoint-subnet', 'route-host', 'management-source-cidr'):
                row.add_argument('--' + key, required=True)
            row.add_argument('--route-port', type=int, default=22)
            row.add_argument('--operator-object-id', default='')
        if action == 'foundation-plan':
            row.add_argument('--foundation-resource-group', required=True)
        if action in ('foundation-step', 'foundation-reconcile'):
            row.add_argument('--plan-digest', required=True)
            row.add_argument('--acknowledge-costs', action='store_true')
        if action == 'foundation-reuse':
            row.add_argument('--storage-id', required=True)
            row.add_argument('--vault-id', required=True)
            row.add_argument('--private-dns-resource-group', required=True)
        if action == "template-step":
            row.add_argument("--mode", choices=("create", "created", "reconcile", "start", "status"), required=True)
            row.add_argument("--acknowledge-template-publication", action="store_true")
        if action in ("install-operator", "reconcile-operator"):
            row.add_argument("--finalization-digest", required=True)
        if action == "prepare-operator":
            row.add_argument("--guest-image", default="", help="Optional verified existing registry image; normal setup derives measured source")
            row.add_argument("--cpu-count", type=int, required=True)
            row.add_argument("--memory-mib", type=int, required=True)
            row.add_argument("--task-max-seconds", type=int, required=True)
            row.add_argument("--task-estimated-cost-micros", type=int, required=True)
            row.add_argument("--model-domain", action="append", required=True)
        if action == "configure-private":
            row.add_argument("--private-network-cidr", required=True)
            row.add_argument("--management-source-cidr", required=True)
            row.add_argument("--acknowledge-existing-private-route", action="store_true")
        if action.startswith("gallery-"):
            row.add_argument("--role", choices=("runtime", "controller"), required=True)
            if action == "gallery-plan":
                row.add_argument("--image-definition-id", required=True)
                row.add_argument("--version", required=True)
                row.add_argument("--managed-image-id", required=True)
            else:
                row.add_argument("--plan-digest", required=True)
                row.add_argument("--acknowledge-costs", action="store_true")
        if action == "init-selection":
            row.add_argument("--name", required=True)
            row.add_argument("--region", required=True)
            row.add_argument("--monthly-budget-usd", type=float, required=True)
            row.add_argument("--max-concurrent-vms", type=int, required=True)
            row.add_argument("--private-network-cidr", default="")
        if action == "init-image-inputs":
            for key in ("luci-source", "e2b-source", "image-digests", "envoy-binary", "envoy-license",
                        "runtime-image-root", "controller-image-root", "operator-template"):
                row.add_argument("--" + key, required=True)
        if action == "configure-account":
            for key in ("subscription-id", "resource-group", "state-resource-group", "state-storage-account",
                        "state-container", "zone", "runtime-size", "controller-size", "ssh-public-key-file"):
                row.add_argument("--" + key, required=True)
            for key in ("key-vault-id", "key-vault-name", "fence-storage-account",
                        "runtime-image-version-id", "controller-image-version-id"):
                row.add_argument("--" + key, default="")
        if action == "prepare-images":
            row.add_argument("--image-inputs", type=Path, required=True)
            row.add_argument("--acknowledge-image-roots", action="store_true")
        if action == "publish-secrets":
            row.add_argument("--acknowledge-credential-publication", action="store_true")
        if action in ("prepare", "apply", "reconcile", "bootstrap"):
            row.add_argument("--plan-digest", required=True)
        if action == "apply":
            row.add_argument("--tfplan-digest", required=True)
            row.add_argument("--acknowledge-costs", action="store_true")
            row.add_argument("--acknowledge-unverified-price", action="store_true")
    args = parser.parse_args()
    if args.action == 'recover-controller':
        bundle(args.bundle)
        modules = {}
        for name in ('guided', 'successor'):
            spec = importlib.util.spec_from_file_location('luci_azure_' + name, args.bundle / ('azure-setup/' + name + '.py'))
            module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module); modules[name] = module
        core = sys.modules[__name__]
        journal = modules['successor'].run(core, args.bundle, args.selection, modules['guided'].az_json)
        print(json.dumps(modules['successor'].finish(core, args.bundle, args.selection, journal, modules['guided'].az_json), sort_keys=True))
        return
    if args.action == 'continue':
        bundle(args.bundle)
        spec = importlib.util.spec_from_file_location('luci_azure_guided', args.bundle / 'azure-setup/guided.py')
        guided_module = importlib.util.module_from_spec(spec); spec.loader.exec_module(guided_module)
        print(json.dumps(guided_module.continue_setup(sys.modules[__name__], args.bundle, args.selection), sort_keys=True))
        return
    if args.action.startswith('builder-'):
        bundle(args.bundle)
        modules = []
        for name in ('guided', 'foundation', 'builder'):
            spec = importlib.util.spec_from_file_location('luci_azure_' + name, args.bundle / ('azure-setup/' + name + '.py'))
            module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module); modules.append(module)
        guided_module, foundation_module, builder_module = modules
        core = sys.modules[__name__]
        if args.action == 'builder-source':
            result = builder_module.source_payload(core, args.bundle, args.selection, args.luci_source, args.runtime_source, args.envoy_binary, args.envoy_license)
        elif args.action == 'builder-plan':
            pieces = args.base_image.split(':')
            if len(pieces) != 4: raise ValueError('Exact publisher:offer:sku:version base image required')
            result = builder_module.plan(core, args.bundle, args.selection, dict(zip(('publisher', 'offer', 'sku', 'version'), pieces)), args.builder_size, args.gallery_name, args.image_version, args.lifetime_seconds)
        else:
            arm = foundation_module.AzureARM(guided_module.az_json)
            cli = lambda arguments: guided_module.az_json(arguments, timeout=7200)
            common = (core, args.bundle, args.selection, args.plan_digest)
            if args.stage == 'bind': result = builder_module.bind_images(*common)
            elif args.stage in ('staging', 'retire-staging'):
                method = builder_module.staging if args.stage == 'staging' else builder_module.retire_staging
                result = method(*common, arm, cli, args.acknowledge_costs, args.reconcile)
            elif args.stage in ('install', 'deprovision'):
                result = builder_module.install(*common, args.role, arm, cli, args.acknowledge_costs, args.reconcile, args.stage == 'deprovision')
            else:
                method = {'allocate': builder_module.allocate, 'grant': builder_module.grant, 'deallocate': builder_module.deallocate,
                    'generalize': builder_module.generalize, 'capture': builder_module.capture, 'retire': builder_module.retire, 'abort': builder_module.abort}[args.stage]
                result = method(*common, args.role, arm, args.acknowledge_costs, args.reconcile)
        print(json.dumps(result, sort_keys=True))
        return
    if args.action.startswith('foundation-'):
        bundle(args.bundle)
        modules = []
        for name in ('guided', 'foundation'):
            spec = importlib.util.spec_from_file_location('luci_azure_' + name, args.bundle / ('azure-setup/' + name + '.py'))
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            modules.append(module)
        guided_module, foundation_module = modules
        core = sys.modules[__name__]
        # Local proposal does not need account authentication.
        if args.action == 'foundation-plan':
            result = foundation_module.plan(core, args.bundle, args.selection, args.foundation_resource_group)
        else:
            arm = foundation_module.AzureARM(guided_module.az_json)
            if args.action == 'foundation-inventory':
                account = guided_module.az_json(['account', 'show'])
                principal = args.operator_object_id
                if not principal:
                    principal = guided_module.az_json(['ad', 'signed-in-user', 'show', '--query', 'id']) if account.get('user', {}).get('type') == 'user' else guided_module.az_json(['ad', 'sp', 'show', '--id', account.get('user', {}).get('name', ''), '--query', 'id'])
                result = foundation_module.inventory(core, args.bundle, args.selection, args.vnet_id,
                    args.endpoint_subnet, args.route_host, args.route_port, args.management_source_cidr,
                    principal, arm, account)
            elif args.action == 'foundation-reuse':
                result = foundation_module.reuse(core, args.bundle, args.selection, args.storage_id,
                    args.vault_id, args.private_dns_resource_group, arm)
            elif args.action == 'foundation-bind':
                result = foundation_module.bind(core, args.bundle, args.selection, arm, guided_module.az_json)
            else:
                result = foundation_module.step(core, args.bundle, args.selection, args.plan_digest, arm,
                    args.acknowledge_costs, args.action == 'foundation-reconcile')
        print(json.dumps(result, sort_keys=True))
        return
    if args.action in ("prepare-operator", "template-step", "finalize-operator", "install-operator", "reconcile-operator"):
        bundle(args.bundle)
        spec = importlib.util.spec_from_file_location("luci_azure_operator", args.bundle / "azure-setup/operator_setup.py")
        operator_module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(operator_module)
        core = sys.modules[__name__]
        if args.action == "prepare-operator":
            result = operator_module.deferred(core, args.bundle, args.selection,
                args.guest_image, args.cpu_count, args.memory_mib, args.task_max_seconds,
                args.task_estimated_cost_micros, args.model_domain)
        elif args.action == "template-step":
            spec = importlib.util.spec_from_file_location("luci_azure_guided", args.bundle / "azure-setup/guided.py")
            guided_module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(guided_module)
            result = operator_module.controller_step(core, args.bundle, args.selection,
                args.mode, guided_module.az_json, args.acknowledge_template_publication)
        else:
            spec = importlib.util.spec_from_file_location("luci_azure_guided", args.bundle / "azure-setup/guided.py")
            guided_module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(guided_module)
            if args.action == "finalize-operator":
                result = operator_module.finalize(core, args.bundle, args.selection, guided_module.az_json)
            else:
                result = operator_module.install(core, args.bundle, args.selection,
                    args.finalization_digest, guided_module.az_json, args.action == "reconcile-operator")
        print(json.dumps(result, sort_keys=True))
        return
    if args.action in ("configure-guided", "configure-private", "connection-descriptor", "bind-images", "status", "gallery-plan", "gallery-publish", "gallery-reconcile"):
        manifest = bundle(args.bundle)
        spec = importlib.util.spec_from_file_location("luci_azure_guided", args.bundle / "azure-setup/guided.py")
        guided = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(guided)
        core = sys.modules[__name__]
        if args.action == "configure-guided":
            result = guided.wizard(core, args.bundle, args.selection)
        elif args.action == "configure-private":
            result = guided.configure_private(core, args.bundle, args.selection,
                args.private_network_cidr, args.management_source_cidr, args.acknowledge_existing_private_route)
        elif args.action == "connection-descriptor":
            result = guided.connection_descriptor(core, args.bundle, args.selection)
        elif args.action == "bind-images":
            result = guided.bind_images(core, args.bundle, args.selection)
        elif args.action == "status":
            result = guided.progress(core, args.bundle, args.selection)
        elif args.action == "gallery-plan":
            result = guided.gallery_plan(core, args.bundle, args.selection, args.role,
                                         args.image_definition_id, args.version, args.managed_image_id)
        else:
            result = guided.gallery_publish(core, args.bundle, args.selection, args.role,
                args.plan_digest, args.acknowledge_costs, args.action == "gallery-reconcile")
        print(json.dumps(result, sort_keys=True))
        return
    if args.action == "init-selection":
        print(json.dumps(initial_selection(args.bundle, args.selection, args.name, args.region,
                                           args.monthly_budget_usd, args.max_concurrent_vms,
                                           args.private_network_cidr), sort_keys=True))
        return
    if args.action == "init-image-inputs":
        print(json.dumps(initial_image_inputs(args.bundle, args.selection,
            luci_source=args.luci_source, e2b_source=args.e2b_source,
            image_digests=args.image_digests, envoy_binary=args.envoy_binary,
            envoy_license=args.envoy_license, runtime_image_root=args.runtime_image_root,
            controller_image_root=args.controller_image_root,
            operator_template=args.operator_template), sort_keys=True))
        return
    if args.action == "configure-account":
        print(json.dumps(configure_account(args.bundle, args.selection,
            subscription_id=args.subscription_id, resource_group=args.resource_group,
            state_resource_group=args.state_resource_group,
            state_storage_account=args.state_storage_account, state_container=args.state_container,
            zone=args.zone, runtime_size=args.runtime_size, controller_size=args.controller_size,
            ssh_public_key_file=args.ssh_public_key_file, key_vault_id=args.key_vault_id,
            key_vault_name=args.key_vault_name, fence_storage_account=args.fence_storage_account,
            runtime_image_version_id=args.runtime_image_version_id,
            controller_image_version_id=args.controller_image_version_id), sort_keys=True))
        return
    if args.action == "plan":
        print(json.dumps(plan_command(args.bundle, args.selection), sort_keys=True))
        return
    if args.action in ("generate-secrets", "publish-secrets", "reconcile-secrets", "bind-secrets"):
        print(json.dumps(credential_step(args.bundle, args.selection, args.action,
                                         acknowledge=getattr(args, "acknowledge_credential_publication", False)), sort_keys=True))
        return
    if args.action == "prepare":
        print(json.dumps(prepare(args.bundle, args.selection, args.plan_digest), sort_keys=True))
    elif args.action == "prepare-images":
        print(json.dumps(prepare_images(args.bundle, args.selection, args.image_inputs,
                                        args.acknowledge_image_roots), sort_keys=True))
    elif args.action == "reconcile":
        print(json.dumps(observed_outputs(args.bundle, args.selection, args.plan_digest), sort_keys=True))
    elif args.action == "bootstrap":
        print(json.dumps(bootstrap(args.bundle, args.selection, args.plan_digest), sort_keys=True))
    else:
        print(json.dumps(apply(args.bundle, args.selection, args.plan_digest, args.tfplan_digest,
                               args.acknowledge_costs, args.acknowledge_unverified_price), sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, OSError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
