#!/usr/bin/env python3
"""Prepare and bind exact Azure Key Vault versions for one BYO generation.

Generation is local. Publication is an explicit operator effect, journaled
before each write. An unknown acknowledgement is reconciled by read-only
version inspection and is never uploaded again.
"""

import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import stat
import subprocess


HOST = {"teamApiKey": "team-api", "adminToken": "admin-token", "sandboxTokenSeed": "sandbox-seed",
        "gatewayServerCert": "server-cert", "gatewayServerKey": "server-key", "gatewayClientCa": "client-ca"}
CONTROLLER = {"apiKey": "team-api", "adminKey": "admin-token", "attachKey": "attach-key",
              "clientCert": "client-cert", "clientKey": "client-key"}
MATERIAL = {"team-api": "team-api.txt", "admin-token": "admin-token.txt", "sandbox-seed": "sandbox-seed.txt",
            "server-cert": "server.crt", "server-key": "server.key", "client-ca": "ca.crt",
            "attach-key": "attach-key.txt", "client-cert": "client.crt", "client-key": "client.key"}
VERSIONED_ID = re.compile(r"https://([a-z0-9-]+)\.vault\.azure\.net/secrets/([a-zA-Z0-9-]+)/([0-9a-f]{32})\Z")


def team_api_key():
    # The pinned Embed host accepts the e2b_ + 128-bit API-key format.
    # Other independent admin/seed/attach material remains 256-bit hex.
    return "e2b_" + secrets.token_hex(16)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def private_file(path, limit=1 << 20):
    path = Path(path)
    info = path.lstat()
    if path.is_symlink() or not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or not 0 < info.st_size <= limit:
        raise ValueError("Azure credential material is unavailable or public")
    return path.read_bytes()


def save(path, data):
    path = Path(path)
    if path.exists() or path.is_symlink():
        raise ValueError("Azure credential file already exists")
    fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    try:
        with os.fdopen(fd, "wb", closefd=False) as sink:
            sink.write(data)
            sink.flush()
        os.fsync(fd)
    finally:
        os.close(fd)


def command(args):
    result = subprocess.run(args, capture_output=True, timeout=30, check=False)
    if result.returncode or len(result.stdout) > 64 << 10:
        raise RuntimeError("Azure credential preparation command failed")
    return result.stdout


def retained_task_material(selection, predecessor):
    """Keep live task identities while replacing generation-specific TLS only."""
    previous = json.loads(private_file(Path(predecessor) / 'journal.json', 64 << 10))
    variables = selection['terraformVariables']
    if previous.get('state') != 'published' or previous.get('scope') != variables['scope'] or previous.get('generation') == variables['generation']:
        raise ValueError('Successor needs the exact separately published predecessor generation')
    retained = {}
    # The runtime's three stable refs include its sandbox seed. Attach is also
    # retained for existing guest/task reconnection and checked by the signer.
    for kind in ('team-api', 'admin-token', 'sandbox-seed', 'attach-key'):
        data = private_file(Path(predecessor) / MATERIAL[kind])
        row = previous.get('published', {}).get(kind, {})
        match = VERSIONED_ID.fullmatch(row.get('id', ''))
        if (match is None or match.groups() != (variables['key_vault_name'], previous.get('names', {}).get(kind), row.get('version')) or
                row.get('name') != match[2] or row.get('sha256') != sha(data) or previous.get('files', {}).get(kind) != sha(data)):
            raise ValueError('Predecessor task bytes/name/version/hash changed')
        retained[kind] = {'data': data, 'published': row}
    return retained


def generate(selection, root, atomic_json, predecessor=None):
    variables = selection["terraformVariables"]
    scope, generation, gateway = variables["scope"], variables["generation"], variables["gateway_zone_name"]
    root = Path(root)
    retained = retained_task_material(selection, predecessor) if predecessor is not None else {}
    prior = json.loads(private_file(Path(predecessor) / 'journal.json', 64 << 10)) if predecessor is not None else None
    if prior is not None:
        for kind, filename in MATERIAL.items():
            row = prior.get('published', {}).get(kind, {})
            match = VERSIONED_ID.fullmatch(row.get('id', ''))
            if (match is None or match.groups() != (variables['key_vault_name'], prior.get('names', {}).get(kind), row.get('version')) or
                    row.get('name') != match[2] or row.get('sha256') != sha(private_file(Path(predecessor) / filename)) or
                    prior.get('files', {}).get(kind) != row.get('sha256')):
                raise ValueError('Predecessor managed-identity secret name/version/bytes changed')
    if root.exists() or root.is_symlink() or not root.parent.is_dir() or root.parent.stat().st_mode & 0o077:
        raise ValueError("Use a new Azure credential directory under an owner-private parent")
    if not re.fullmatch(r"[a-z][a-z0-9-]{1,63}", scope) or not re.fullmatch(r"[a-z][a-z0-9-]{1,63}", generation) or \
            not re.fullmatch(r"[a-z0-9-]+\.internal", gateway):
        raise ValueError("Selected private certificate identities are invalid")
    root.mkdir(mode=0o700)
    atomic_json(root / "journal.json", {"version": 1, "state": "generation_pending", "scope": scope,
                                        "generation": generation})
    original_umask = os.umask(0o077)
    try:
        return _generate_material(root, scope, generation, gateway, atomic_json, retained, prior)
    finally:
        os.umask(original_umask)


def _generate_material(root, scope, generation, gateway, atomic_json, retained=None, predecessor=None):
    ca_key, ca_cert = root / "ca.key", root / "ca.crt"
    command(["openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", str(ca_key)])
    ca_key.chmod(0o600)
    command(["openssl", "req", "-new", "-x509", "-sha256", "-days", "365", "-key", str(ca_key),
             "-subj", "/CN=Luci Azure " + scope + " CA", "-out", str(ca_cert)])
    ca_cert.chmod(0o600)
    for role, san, usage in (("server", "DNS:" + gateway + ",DNS:*." + gateway, "serverAuth"),
                             ("client", "URI:spiffe://luci/controller/" + scope + "/" + generation, "clientAuth")):
        key, csr, cert, ext = (root / (role + suffix) for suffix in (".key", ".csr", ".crt", ".ext"))
        command(["openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", str(key)])
        key.chmod(0o600)
        command(["openssl", "req", "-new", "-sha256", "-key", str(key), "-subj", "/CN=Luci " + role,
                 "-out", str(csr)])
        save(ext, ("basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\n"
                   "extendedKeyUsage=" + usage + "\nsubjectAltName=" + san + "\n").encode())
        command(["openssl", "x509", "-req", "-sha256", "-days", "90", "-in", str(csr),
                 "-CA", str(ca_cert), "-CAkey", str(ca_key), "-set_serial", "0x" + secrets.token_hex(16),
                 "-extfile", str(ext), "-out", str(cert)])
        cert.chmod(0o600)
        csr.unlink()
        ext.unlink()
    for kind in ('team-api', 'admin-token', 'sandbox-seed', 'attach-key'):
        # Select bytes before the exclusive create. A successor never writes
        # a random replacement and then weakens save() to overwrite it.
        if kind in (retained or {}):
            data = retained[kind]['data']
        else:
            data = ((team_api_key() if kind == 'team-api' else secrets.token_hex(32)) + '\n').encode()
        save(root / MATERIAL[kind], data)
    files = {kind: sha(private_file(root / filename)) for kind, filename in MATERIAL.items()}
    manifest = {"version": 1, "state": "generated_unpublished", "scope": scope, "generation": generation,
                "files": files, "names": {kind: scope + "-" + suffix for kind, suffix in
                                   ((kind, kind) for kind in MATERIAL)}, "published": {}}
    if retained:
        manifest['predecessorSecrets'] = predecessor['published']
        manifest['names'] = dict(predecessor['names'])
        manifest['retainedTaskIdentity'] = {kind: {'name': row['published']['name'], 'version': row['published']['version'],
            'sha256': files[kind]} for kind, row in retained.items()}
        for kind, row in retained.items():
            manifest['names'][kind] = row['published']['name']
            manifest['published'][kind] = row['published']
    atomic_json(root / "journal.json", manifest)
    return {"state": manifest["state"], "scope": scope, "generation": generation,
            "fileCount": len(files), "journalSHA256": sha((root / "journal.json").read_bytes())}


def az(args):
    result = subprocess.run(["az", *args, "--output", "json"], capture_output=True, timeout=45, check=False)
    if result.returncode or len(result.stdout) > 1 << 20:
        raise RuntimeError("Azure Key Vault command failed or is uncertain")
    return json.loads(result.stdout)


def exact_read(vault, name, version, expected_sha, az_call=az):
    row = az_call(["keyvault", "secret", "show", "--vault-name", vault, "--name", name, "--version", version])
    identity = VERSIONED_ID.fullmatch(row.get("id", ""))
    if identity is None or identity.groups() != (vault, name, version) or sha(row.get("value", "").encode()) != expected_sha:
        raise ValueError("Exact Key Vault version or content changed")
    return {"name": name, "version": version, "id": row["id"], "sha256": expected_sha}


def account_preflight(selection, az_call):
    variables = selection["terraformVariables"]
    vault = variables.get("key_vault_name", "")
    account = az_call(["account", "show"])
    selected_vault = az_call(["keyvault", "show", "--name", vault])
    if account.get("id", "").lower() != variables.get("subscription_id", "").lower() or \
            selected_vault.get("id", "").lower() != variables.get("key_vault_id", "").lower():
        raise ValueError("Current Azure account or Key Vault does not match the selected scope")


def listed_names(vault, az_call):
    rows = az_call(["keyvault", "secret", "list", "--vault-name", vault])
    if not isinstance(rows, list) or len(rows) > 1000 or any(not isinstance(row.get("id"), str) for row in rows):
        raise RuntimeError("Exact Key Vault name preflight is incomplete")
    names = set()
    for row in rows:
        parts = row["id"].split("/secrets/")
        if len(parts) != 2 or parts[0].lower() != "https://" + vault + ".vault.azure.net":
            raise RuntimeError("Key Vault preflight returned another vault")
        names.add(parts[1].split("/", 1)[0])
    return names


def publish(selection, root, atomic_json, *, acknowledge=False, az_call=az):
    if not acknowledge:
        raise ValueError("Exact Key Vault publication requires explicit acknowledgement")
    root = Path(root)
    journal = root / "journal.json"
    state = json.loads(private_file(journal, 64 << 10))
    variables = selection["terraformVariables"]
    vault = variables.get("key_vault_name")
    if not vault or not re.fullmatch(r"[a-z0-9-]{3,24}", vault) or \
            state.get("scope") != variables["scope"] or state.get("generation") != variables["generation"] or \
            state.get("state") not in ("generated_unpublished", "publishing", "published"):
        raise ValueError("Azure credential selection or journal is stale")
    account_preflight(selection, az_call)
    names_present = listed_names(vault, az_call)
    for kind, filename in sorted(MATERIAL.items()):
        data = private_file(root / filename)
        expected = state["files"].get(kind)
        name = state["names"].get(kind)
        if sha(data) != expected or not re.fullmatch(r"[a-zA-Z0-9-]{1,127}", name):
            raise ValueError("Azure credential bytes or selected name changed")
        row = state["published"].get(kind)
        if row is not None:
            exact_read(vault, name, row["version"], expected, az_call)
            continue
        # A previous unknown write is *never* repeated. The operator must
        # use a separate read-only reconciliation path for this exact name.
        if state.get("pending"):
            raise RuntimeError("Key Vault publication outcome is unknown; inspect before retry")
        if name in names_present:
            # A successor appends one new TLS version at the exact already
            # owned name, preserving existing name-scoped MI permissions.
            # Ordinary setup still refuses adoption or arbitrary overwrite.
            predecessor = state.get('predecessorSecrets', {}).get(kind)
            identity = VERSIONED_ID.fullmatch((predecessor or {}).get('id', ''))
            if (identity is None or identity.groups() != (vault, name, predecessor.get('version')) or
                    predecessor.get('name') != name or kind in ('team-api', 'admin-token', 'sandbox-seed', 'attach-key')):
                raise ValueError("Selected Key Vault secret name already exists; no overwrite is allowed")
            exact_read(vault, name, predecessor['version'], predecessor['sha256'], az_call)
            latest = az_call(['keyvault', 'secret', 'show', '--vault-name', vault, '--name', name])
            if latest.get('id') != predecessor['id'] or sha(latest.get('value', '').encode()) != predecessor['sha256']:
                raise ValueError('Owned predecessor secret changed before successor version publication')
        state = dict(state, state="publishing", pending={"kind": kind, "name": name, "sha256": expected})
        atomic_json(journal, state)
        try:
            uploaded = az_call(["keyvault", "secret", "set", "--vault-name", vault, "--name", name,
                                "--file", str(root / filename), "--encoding", "utf-8"])
            identity = VERSIONED_ID.fullmatch(uploaded.get("id", ""))
            if identity is None or identity.group(1) != vault or identity.group(2) != name:
                raise RuntimeError("Key Vault write did not return its exact version")
            bound = exact_read(vault, name, identity.group(3), expected, az_call)
        except Exception as error:
            raise RuntimeError("Key Vault write outcome is retained as unknown; read back exact name/version") from error
        state["published"][kind] = bound
        names_present.add(name)
        state.pop("pending")
        atomic_json(journal, state)
    state = dict(state, state="published")
    atomic_json(journal, state)
    return {"state": state["state"], "scope": state["scope"], "generation": state["generation"],
            "versionCount": len(state["published"]), "journalSHA256": sha(journal.read_bytes())}


def reconcile_pending(selection, root, atomic_json, az_call=az):
    journal = Path(root) / "journal.json"
    state = json.loads(private_file(journal, 64 << 10))
    pending = state.get("pending")
    if state.get("state") != "publishing" or not isinstance(pending, dict) or \
            state.get("scope") != selection["terraformVariables"]["scope"] or \
            state.get("generation") != selection["terraformVariables"]["generation"]:
        raise ValueError("No exact pending Key Vault write exists")
    account_preflight(selection, az_call)
    kind, name, expected = pending["kind"], pending["name"], pending["sha256"]
    if kind not in MATERIAL or name != state["names"][kind] or expected != state["files"][kind] or \
            sha(private_file(Path(root) / MATERIAL[kind])) != expected:
        raise ValueError("Pending Key Vault identity or local bytes changed")
    rows = az_call(["keyvault", "secret", "list-versions", "--vault-name",
                    selection["terraformVariables"]["key_vault_name"], "--name", name])
    if not isinstance(rows, list) or len(rows) > 25:
        raise RuntimeError("Pending Key Vault versions cannot be bounded")
    matches = []
    for row in rows:
        identity = VERSIONED_ID.fullmatch(row.get("id", ""))
        if identity and identity.group(1) == selection["terraformVariables"]["key_vault_name"] and identity.group(2) == name:
            try:
                matches.append(exact_read(identity.group(1), name, identity.group(3), expected, az_call))
            except ValueError:
                pass
    if len(matches) != 1:
        raise RuntimeError("Pending Key Vault write remains uncertain; no automatic retry")
    state["published"][kind] = matches[0]
    state.pop("pending")
    atomic_json(journal, state)
    return {"state": "reconciled", "name": name, "version": matches[0]["version"]}


def bind_selection(selection, root, atomic_json, selection_path):
    state = json.loads(private_file(Path(root) / "journal.json", 64 << 10))
    if state.get("state") != "published" or set(state.get("published", {})) != set(MATERIAL):
        raise ValueError("Exact Key Vault versions are not all published")
    variables = selection["terraformVariables"]
    if state.get("scope") != variables["scope"] or state.get("generation") != variables["generation"]:
        raise ValueError("Credential generation belongs to another Luci scope")
    refs = state["published"]
    variables["host_secret_refs"] = {key: {"name": refs[kind]["name"], "version": refs[kind]["version"]}
                                    for key, kind in HOST.items()}
    for kind, row in refs.items():
        identity = VERSIONED_ID.fullmatch(row.get("id", ""))
        if identity is None or identity.groups() != (variables["key_vault_name"], row.get("name"), row.get("version")) or \
                row.get("sha256") != state["files"].get(kind):
            raise ValueError("Exact published Key Vault identity changed")
    secret_id = lambda kind: variables["key_vault_id"] + "/secrets/" + refs[kind]["name"]
    variables["host_secret_ids"] = sorted({secret_id(kind) for kind in HOST.values()})
    variables["controller_secret_ids"] = sorted({secret_id(kind) for kind in CONTROLLER.values()})
    atomic_json(selection_path, selection)
    return {"bound": True, "hostSecretCount": len(variables["host_secret_ids"]),
            "controllerSecretCount": len(variables["controller_secret_ids"])}
