#!/usr/bin/env python3
"""Fence and settle an owned installation; never release a task lease by inference."""
import argparse
import base64
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time

spec = importlib.util.spec_from_file_location('luci_retirement_recovery', Path(__file__).with_name('recover.py'))
recovery = importlib.util.module_from_spec(spec); spec.loader.exec_module(recovery)


def validate(value):
    required = {'version', 'sourceCommit', 'scope', 'ownerID', 'generation', 'diskID', 'filesystemUUID',
        'controllerID', 'recoveryID', 'recoveryClientID', 'fenceBlobURL', 'planDigest', 'toolSelection'}
    if (set(value) != required or value['version'] != 1 or not recovery.HEX.fullmatch(value['planDigest']) or
            not recovery.re.fullmatch(r'[0-9a-f]{40}', value['sourceCommit']) or
            not all(recovery.GEN.fullmatch(value[key]) for key in ('scope', 'ownerID', 'generation')) or
            not all(recovery.VM.fullmatch(value[key]) for key in ('controllerID', 'recoveryID')) or
            not recovery.DISK.fullmatch(value['diskID']) or not recovery.UUID.fullmatch(value['filesystemUUID']) or
            not recovery.UUID.fullmatch(value['recoveryClientID'])):
        raise ValueError('Exact source-bound retirement selection is invalid')
    tool = value['toolSelection']
    if not recovery.re.fullmatch(r'/mnt/[a-z0-9-]+', tool.get('mountPath', '')):
        raise ValueError('Retirement authoritative mount is invalid')
    if any(tool.get(key) != value[target] for key, target in (('sourceCommit', 'sourceCommit'), ('scope', 'scope'),
            ('generation', 'generation'), ('dataDiskID', 'diskID'), ('controllerResourceID', 'controllerID'),
            ('filesystemUUID', 'filesystemUUID'), ('fenceBlobURL', 'fenceBlobURL'), ('retirementPlanDigest', 'planDigest'))):
        raise ValueError('Retirement tool differs from exact owned controller selection')
    url = recovery.urllib.parse.urlsplit(value['fenceBlobURL'])
    if url.scheme != 'https' or not recovery.re.fullmatch(r'[a-z0-9]{3,24}\.blob\.core\.windows\.net', url.netloc) or url.path != '/controller-fence/controller-fence.json' or url.query or url.fragment:
        raise ValueError('Retirement fence URL is outside the exact private resource')


def identity(selection, record):
    if not record or record.get('version') != 1 or any(record.get(key) != selection[target] for key, target in (
            ('scope', 'scope'), ('ownerID', 'ownerID'), ('diskID', 'diskID'), ('filesystemUUID', 'filesystemUUID'),
            ('generation', 'generation'), ('controllerID', 'controllerID'))):
        raise ValueError('Retirement off-disk identity or generation changed')


def write(store, record, etag):
    if len(recovery.canonical(record)) > 8192: raise ValueError('Retirement fence exceeds the durable bound')
    return record, store.write(record, etag)


def tool_script(selection, action, stop=False):
    if action not in ('retirement-preflight', 'retirement-observe'): raise ValueError('Unsupported retirement observation')
    encoded = base64.b64encode(recovery.canonical(selection['toolSelection'])).decode()
    return """set -eu
python3 - <<'PY'
import base64,os,pathlib,stat,tempfile
data=base64.b64decode(%s)
target=pathlib.Path('/etc/luci/azure-retirement-tool.json')
if target.is_symlink() or target.exists():
    if target.is_symlink() or not stat.S_ISREG(target.stat().st_mode) or target.stat().st_mode & 0o077 or target.read_bytes()!=data:
        raise SystemExit('retained retirement tool selection changed')
else:
    fd,name=tempfile.mkstemp(dir=target.parent);os.fchmod(fd,0o600)
    with os.fdopen(fd,'wb') as output: output.write(data);output.flush();os.fsync(output.fileno())
    os.replace(name,target)
PY
%s
receipt=$(/opt/luci/azure-controller/bin/luci-api azure-setup %s /etc/luci/azure-retirement-tool.json)
sync -f %s
printf 'LUCI_AZURE_RETIREMENT_SETTLED=%%s\\n' "$receipt"
""" % (repr(encoded), 'systemctl stop luci-api.service' if stop else ':', action, selection['toolSelection']['mountPath'])


def observation(selection, compute, action, stop=False):
    output = compute.run_command(selection['controllerID'], tool_script(selection, action, stop))
    rows = [line.removeprefix('LUCI_AZURE_RETIREMENT_SETTLED=') for line in output.splitlines() if line.startswith('LUCI_AZURE_RETIREMENT_SETTLED=')]
    if len(rows) != 1: raise RuntimeError('Authoritative retirement observation remains unavailable')
    result = json.loads(rows[0])
    if (result.get('state') != 'controller_setup_readback' or result.get('taskReservationsSettled') is not True or
            not isinstance(result.get('settledJournalDigests'), dict) or any(result.get(key) != selection[target] for key, target in (
                ('sourceCommit', 'sourceCommit'), ('scope', 'scope'), ('generation', 'generation'),
                ('controllerResourceID', 'controllerID'), ('dataDiskID', 'diskID'), ('retirementPlanDigest', 'planDigest')))):
        raise ValueError('Task/provider/reservation readback is not exact settled ownership proof')
    return result


def quiesce(selection, store, compute, inspect=False):
    validate(selection)
    record, etag = store.read(); identity(selection, record)
    if compute.disk_owner(selection['diskID']) != (selection['controllerID'], 'Attached'):
        raise ValueError('Retirement authoritative disk attachment changed')
    if record['state'] == 'active':
        if inspect: raise RuntimeError('Retirement fence effect remains absent/unknown; no repeated stop')
        observation(selection, compute, 'retirement-preflight')
        predecessor = dict(record)
        record, etag = write(store, dict(record, state='retirement_quiesce_pending', retirementPlanDigest=selection['planDigest'],
            retirementSourceCommit=selection['sourceCommit'], retirementPredecessor=predecessor,
            retirementPredecessorSHA256=recovery.digest(predecessor)), etag)
        try:
            observed = observation(selection, compute, 'retirement-observe', stop=True)
        except Exception as error:
            raise RuntimeError('Account stop/settlement uncertain; inspect exact journals without repeating stop') from error
    else:
        if record.get('retirementPlanDigest') != selection['planDigest'] or record.get('retirementSourceCommit') != selection['sourceCommit']:
            raise ValueError('Retirement belongs to another exact plan/source')
        if record['state'] not in ('retirement_quiesce_pending', 'retirement_settled'):
            raise ValueError('Retirement is already closed or has another pending authority transition')
        observed = observation(selection, compute, 'retirement-observe')
    record, etag = write(store, dict(record, state='retirement_settled', retirementSettlementSHA256=recovery.digest(observed),
        retirementObservedAtUTC=observed['observedAtUTC']), etag)
    return {'state': record['state'], 'sourceCommit': selection['sourceCommit'], 'scope': selection['scope'],
        'generation': selection['generation'], 'planDigest': selection['planDigest'], 'diskID': selection['diskID'],
        'settlementSHA256': record['retirementSettlementSHA256'], 'taskReservationsSettled': True,
        'settledJournalDigests': observed['settledJournalDigests'], 'observedAtUTC': observed['observedAtUTC'],
        'providerInvoiceVerified': False}


def close(selection, store, backups_sha, inspect=False):
    validate(selection)
    if not recovery.HEX.fullmatch(backups_sha): raise ValueError('Exact observed backup receipt digest is required')
    record, etag = store.read(); identity(selection, record)
    if record.get('retirementPlanDigest') != selection['planDigest'] or record.get('retirementSourceCommit') != selection['sourceCommit']:
        raise ValueError('Retirement source or plan changed before closure')
    if record['state'] == 'retirement_closed_pending_compute':
        if record.get('retirementBackupsSHA256') != backups_sha: raise ValueError('Retained backup handoff changed')
    elif record['state'] == 'retirement_settled':
        if inspect: raise RuntimeError('Retirement closure remains absent/unknown; no repeated fence write')
        record, etag = write(store, dict(record, state='retirement_closed_pending_compute', retirementBackupsSHA256=backups_sha), etag)
    else: raise ValueError('Settled authoritative retirement proof is required before compute closure')
    return {'state': record['state'], 'planDigest': selection['planDigest'], 'scope': selection['scope'],
        'generation': selection['generation'], 'backupsSHA256': backups_sha, 'computeDeallocationVerified': False,
        'providerInvoiceVerified': False}


class LocalCompute(recovery.Compute):
    def __init__(self, transport, local_id):
        super().__init__(transport); self.local_id = local_id

    def run_command(self, vm_id, script):
        if vm_id != self.local_id: return super().run_command(vm_id, script)
        # Recovered controller also owns the recovery MI. Avoid recursive ARM
        # RunCommand on the VM already executing this exact retirement script.
        with tempfile.TemporaryFile() as output:
            process = subprocess.Popen(['sh', '-c', script], stdout=output, stderr=subprocess.DEVNULL)
            deadline = time.monotonic() + 120
            try:
                while process.poll() is None:
                    if output.tell() > 65536 or time.monotonic() >= deadline:
                        raise RuntimeError('Local retirement observation exceeded its bound')
                    time.sleep(.2)
                if process.returncode or output.tell() > 65536: raise RuntimeError('Local retirement observation failed or remains unknown')
                output.seek(0); return output.read(65536).decode()
            finally:
                if process.poll() is None: process.kill(); process.wait(timeout=5)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=('quiesce', 'inspect', 'close'))
    parser.add_argument('selection', type=Path)
    parser.add_argument('--backups-sha256', default='')
    parser.add_argument('--inspect', action='store_true')
    args = parser.parse_args()
    selection = recovery.private(args.selection, 16 << 10); validate(selection)
    transport = recovery.AzureHTTP(selection['recoveryClientID'])
    local_id = transport.local_instance_id()
    if local_id != selection['recoveryID']: raise ValueError('Retirement must run on the exact selected recovery identity host')
    store = recovery.FenceBlob(transport, selection['fenceBlobURL'])
    compute = LocalCompute(transport, local_id)
    result = close(selection, store, args.backups_sha256, args.inspect) if args.action == 'close' else quiesce(selection, store, compute, args.action == 'inspect')
    print(json.dumps(result, sort_keys=True))


if __name__ == '__main__':
    try: main()
    except (ValueError, RuntimeError, OSError, KeyError) as error:
        print(str(error), file=sys.stderr); sys.exit(1)
