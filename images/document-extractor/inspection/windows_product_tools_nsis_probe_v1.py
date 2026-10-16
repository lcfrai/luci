"""Fixed GPG original framing only. No NSIS decoder, acquisition or eligibility."""
import argparse
from contextlib import ExitStack
import hashlib
import io
import json
from pathlib import Path
import sys
import time

ROOT = Path('E:/Luci-local-acceptance/artifacts')
LIBRARY = ROOT / 'windows-product-tools-v1-source-20261017'
INPUT = ROOT / 'windows-product-tools-v1-acquire-20261017'
OUTPUT = ROOT / 'windows-product-tools-nsis-probe-v1-20261017'
BATCH = 'windows-product-tools-nsis-probe-v1'
MANIFEST_SHA256 = '99355cca07585ff558d493372f979bf26beb3152f582b37560389c9d0daca6a5'
FRAMING_SHA256 = '3fc47d45916911a48211f7f2875b9641601d6ad0d3fd9e9cca97430a3c83d75c'
WORKER_LIBRARY_SHA256 = '5fad1e22757cb4974592587b971890e7dfd6fee5aceb97e497115ad8c5ad092e'
PE_SHA256 = '49c0f20562ae8020b9b9d0f90238ba01b107d7ee64a0053dac53cc5e4e09bab6'
DONOR = ROOT / 'extractor-wheels-65a8dafb-source-20261016/inspect_inputs.py'
DONOR_SHA256 = '06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba'
WRAPPER = {'file': 'gnupg-w32-2.5.24_20260923.exe', 'bytes': 5804864,
           'sha256': 'ea6f9dee2cccd83287432625b8b399173805add81d27067fcaf019c0c4da1ad8'}
# Actual future receipts do not exist. Only a new reviewed source successor can
# populate these constants; neither CLI nor input JSON can choose them.
ACQUISITION_PINS = {
    INPUT / 'controller-receipt.json': '',
    INPUT / 'worker-result.json': '',
    ROOT / 'windows-product-tools-v1-outer-acquire-20261017/outer-receipt.json': '',
    ROOT / 'windows-product-tools-v1-acquisition-root-acceptance-20261017.json': '',
}


def bootstrap_library(deadline):
    """One measured source prelude; seed the existing Budget once, never reset."""
    original = calls = 0

    def check():
        if time.monotonic() >= deadline:
            raise ValueError('fixed_source_prelude_deadline')
    check()
    calls += 1  # Exact source open, before the effect.
    path = LIBRARY / 'windows_product_tools_v1.py'
    with path.open('rb', buffering=0) as stream:
        blocks, size = [], 0
        while True:
            check()
            amount = min(65536, 65537 - size)
            if amount <= 0 or calls >= 8192 or original > 33554432 - amount:
                raise ValueError('fixed_source_prelude_bound')
            original += amount
            calls += 1
            # Failed/unknown reads leave this whole pending debit. No subsequent
            # effect occurs after an exception; no caller supplies these totals.
            block = stream.read(amount)
            if not isinstance(block, bytes) or len(block) > amount:
                raise ValueError('fixed_source_prelude_ack')
            check()
            original -= amount - max(1, len(block))
            size += len(block)
            if size > 65536:
                raise ValueError('fixed_source_prelude_bound')
            blocks.append(block)
            if not block:
                break
    raw = b''.join(blocks)
    if hashlib.sha256(raw).hexdigest() != WORKER_LIBRARY_SHA256:
        raise ValueError('fixed_source_library_pin')
    namespace = {'__name__': 'luci_fixed_nsis_source_library', '__file__': str(path)}
    exec(compile(raw, str(path), 'exec'), namespace)
    budget = namespace['Budget'](deadline)
    budget.original, budget.calls = original, calls
    return namespace, budget


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--phase', choices=('probe',), required=True)
    parser.add_argument('--batch', choices=(BATCH,), required=True)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--deadline', type=float, required=True)
    args = parser.parse_args()
    if args.input != INPUT or args.output != OUTPUT or not time.monotonic() < args.deadline <= time.monotonic() + 300:
        raise ValueError('fixed_probe_arguments')
    # Refuse before wrapper/open/parser/decoder effects while evidence is absent.
    if any(len(pin) != 64 or any(c not in '0123456789abcdef' for c in pin) for pin in ACQUISITION_PINS.values()):
        raise ValueError('future_acquisition_pins_unset')
    library, budget = bootstrap_library(args.deadline)
    load, read_source = library['load_source'], library['source_bytes']
    donor = load(DONOR, DONOR_SHA256, budget=budget)
    here = Path(__file__).parent
    output = donor['regular'](OUTPUT, directory=True)
    intent = library['bounded_json'](output / 'intent.json', budget)
    raw_self = read_source(Path(__file__), intent.get('workerSha256'), budget=budget)
    if intent.get('phase') != 'probe' or intent.get('batch') != BATCH or intent.get('manifestSha256') != MANIFEST_SHA256:
        budget.fail('probe_owned_intent')
    manifest = json.loads(read_source(here / 'windows_product_tools_nsis_probe_v1_manifest.json', MANIFEST_SHA256, budget=budget))
    if manifest['input'] != WRAPPER or manifest['batch'] != BATCH:
        budget.fail('probe_closed_manifest')
    framing = load(here / 'windows_product_tools_nsis_framing_v1.py', FRAMING_SHA256, budget=budget)
    pe = load(LIBRARY / 'windows_product_tools_pe_v1.py', PE_SHA256, budget=budget)
    receipt = {'schema': 'luci-windows-nsis-framing-observation-v1', 'phase': 'probe', 'batch': BATCH,
               'state': 'incomplete_no_replay', 'manifestSha256': MANIFEST_SHA256,
               'workerSha256': hashlib.sha256(raw_self).hexdigest(), 'input': WRAPPER,
               'variantIdentified': False, 'codecIdentified': False, 'payloadDecoded': False,
               'authenticated': False, 'productProfileAvailable': False}
    try:
        for path, pin in ACQUISITION_PINS.items():
            if hashlib.sha256(read_source(path, pin, budget=budget)).hexdigest() != pin:
                budget.fail('probe_prior_acquisition_pin')
        prior = library['bounded_json'](INPUT / 'controller-receipt.json', budget)
        transfers = library['bounded_json'](INPUT / 'worker-result.json', budget)
        matches = [row for row in transfers.get('inputs', []) if row == WRAPPER]
        if prior.get('state') != 'completed_owned_worker_retired' or prior.get('phase') != 'acquire' or prior.get('batch') != 'windows-product-tools-v1' or prior.get('workerSha256') != WORKER_LIBRARY_SHA256 or prior.get('controllerSha256') != '3f75d9d3f53e06323fa74a03f5a9d58dae1100afb83dd1bf28d8cb1e3562685a' or not prior.get('treeSettled') or not prior.get('pdhClosed') or transfers.get('state') != 'all_transfers_exactly_verified' or len(matches) != 1:
            budget.fail('probe_prior_acquisition_identity')
        source = donor['regular'](INPUT, directory=True)
        with ExitStack() as stack:
            stream = stack.enter_context(donor['locked_input'](donor['regular'](source / WRAPPER['file'])))
            if not isinstance(stream.raw, io.FileIO):
                budget.fail('probe_unbuffered_original_required')
            original = library['Original'](stream.raw, WRAPPER, budget)
            data = original.verify(keep=True)
            pe_observation = pe['inspect_pe_v1'](data, budget)
            observation = framing['inspect_nsis_framing_v1'](data, pe_observation, budget)
            original.current()
            receipt.update(state='static_nsis_framing_complete_variant_unidentified', observation=observation,
                           pe=pe_observation)
        budget.check()
        if budget.expanded != 0:
            budget.fail('probe_no_expansion')
    except BaseException:
        budget.failed = True
        receipt['state'] = 'incomplete_no_replay'
        receipt.pop('observation', None)
        receipt.pop('pe', None)
        receipt['reason'] = 'fixed_nsis_probe_refused'
    receipt['accounting'] = {'originalBytes': budget.original, 'expandedBytes': budget.expanded,
                             'calls': budget.calls, 'stickyFailure': budget.failed}
    library['json_bound'](receipt, 65535)
    encoded = json.dumps(receipt, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('utf-8') + b'\n'
    if len(encoded) > 65536:
        raise ValueError('probe_observation_bound')
    with (output / 'worker-result.json').open('xb') as target:
        if target.write(encoded) != len(encoded):
            raise ValueError('probe_receipt_write_ack')
    if budget.failed:
        sys.stderr.write('fixed NSIS framing probe refused\n')
        raise SystemExit(2)


if __name__ == '__main__':
    try:
        main()
    except BaseException:
        # Never expose an arbitrary exception/argument/member/path traceback.
        sys.stderr.write('fixed NSIS framing probe did not complete\n')
        raise SystemExit(2)
