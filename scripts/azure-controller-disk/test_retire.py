"""Authoritative retirement fence transitions, provider/ledger fixtures only."""
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('controller_retire', Path(__file__).with_name('retire.py'))
retire = importlib.util.module_from_spec(spec); spec.loader.exec_module(retire)
PREFIX = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/microsoft.compute/'
UUID = '12345678-1234-1234-1234-123456789abc'


class Store:
    def __init__(self, record): self.record, self.etag, self.writes = record, '0', 0
    def read(self): return dict(self.record), self.etag
    def write(self, record, etag):
        if etag != self.etag: raise RuntimeError('CAS collision')
        self.record = record; self.writes += 1; self.etag = str(self.writes); return self.etag


class RetirementFenceTests(unittest.TestCase):
    def fixture(self):
        selected = {'version': 1, 'sourceCommit': 'a' * 40, 'scope': 'scope-one', 'ownerID': 'owner-one', 'generation': 'gen-one',
            'diskID': PREFIX + 'disks/controller', 'filesystemUUID': UUID, 'controllerID': PREFIX + 'virtualmachines/controller',
            'recoveryID': PREFIX + 'virtualmachines/standby', 'recoveryClientID': UUID, 'planDigest': 'b' * 64,
            'fenceBlobURL': 'https://owned.blob.core.windows.net/controller-fence/controller-fence.json'}
        selected['toolSelection'] = {key: selected[value] for key, value in (('sourceCommit', 'sourceCommit'), ('scope', 'scope'),
            ('generation', 'generation'), ('dataDiskID', 'diskID'), ('controllerResourceID', 'controllerID'),
            ('filesystemUUID', 'filesystemUUID'), ('fenceBlobURL', 'fenceBlobURL'), ('retirementPlanDigest', 'planDigest'))}
        selected['toolSelection']['mountPath'] = '/mnt/luci-controller'
        record = {key: selected[key] for key in ('version', 'scope', 'ownerID', 'generation', 'diskID', 'filesystemUUID', 'controllerID')}
        record.update(state='active', generationReceiptSHA256='c' * 64)
        store = Store(record)
        class Compute:
            def __init__(self): self.calls, self.stopped, self.lost_stop, self.settled, self.lost_start = [], False, False, True, False
            def disk_owner(self, _): return selected['controllerID'], 'Attached'
            def run_command(self, vm, script):
                self.calls.append(script)
                if 'systemctl stop' in script:
                    self.stopped = True
                    if self.lost_stop: raise TimeoutError('lost stop acknowledgment')
                if 'systemctl start' in script:
                    self.stopped = False
                    if self.lost_start: raise TimeoutError('lost start acknowledgment')
                if 'retirement-observe' in script and not self.stopped: raise RuntimeError('account still active')
                if not self.settled and 'retirement-resume-observe' not in script: raise RuntimeError('held task reservation')
                if 'retirement-resume-observe' in script and self.stopped: raise RuntimeError('account still stopped')
                value = {key: selected[value] for key, value in (('sourceCommit', 'sourceCommit'), ('scope', 'scope'),
                    ('generation', 'generation'), ('controllerResourceID', 'controllerID'), ('dataDiskID', 'diskID'), ('retirementPlanDigest', 'planDigest'))}
                value.update(state='controller_setup_readback', taskReservationsSettled=True, taskReservationsReleased=False, settledJournalDigests={'owned-ledger': 'd' * 64}, observedAtUTC='2026-10-14T12:00:00Z')
                return 'LUCI_AZURE_RETIREMENT_SETTLED=' + json.dumps(value)
        return selected, store, Compute()

    def test_preflight_fence_stop_and_actual_settlement_precede_closure(self):
        selected, store, compute = self.fixture()
        result = retire.quiesce(selected, store, compute)
        self.assertEqual(result['state'], 'retirement_settled'); self.assertTrue(result['taskReservationsSettled'])
        self.assertEqual(sum('systemctl stop' in script for script in compute.calls), 1)
        self.assertIn('retirement-preflight', compute.calls[0])
        self.assertEqual(store.record['retirementPredecessor']['state'], 'active')
        result = retire.close(selected, store, 'e' * 64)
        self.assertEqual(result['state'], 'retirement_closed_pending_compute')
        self.assertFalse(result['computeDeallocationVerified']); self.assertFalse(result['providerInvoiceVerified'])

    def test_held_reservation_refuses_before_fence_or_stop(self):
        selected, store, compute = self.fixture(); compute.settled = False
        with self.assertRaisesRegex(RuntimeError, 'held task'): retire.quiesce(selected, store, compute)
        self.assertEqual(store.writes, 0); self.assertFalse(compute.stopped)

    def test_unknown_stop_restart_only_inspects_and_preserves_original_receipts(self):
        selected, store, compute = self.fixture(); compute.lost_stop = True
        with self.assertRaisesRegex(RuntimeError, 'uncertain'): retire.quiesce(selected, store, compute)
        self.assertEqual(store.record['state'], 'retirement_quiesce_pending')
        result = retire.quiesce(selected, store, compute, inspect=True)
        self.assertEqual(result['state'], 'retirement_settled')
        self.assertEqual(sum('systemctl stop' in script for script in compute.calls), 1)

    def test_absent_pending_effect_and_stale_generation_have_zero_mutations(self):
        selected, store, compute = self.fixture()
        with self.assertRaisesRegex(RuntimeError, 'no repeated stop'): retire.quiesce(selected, store, compute, inspect=True)
        selected['generation'] = 'gen-other'; selected['toolSelection']['generation'] = 'gen-other'
        with self.assertRaisesRegex(ValueError, 'generation changed'): retire.quiesce(selected, store, compute)
        self.assertEqual(store.writes, 0); self.assertEqual(compute.calls, [])

    def test_close_needs_exact_settlement_backups_and_no_unknown_replay(self):
        selected, store, compute = self.fixture()
        with self.assertRaisesRegex(ValueError, 'source or plan changed'): retire.close(selected, store, 'e' * 64)
        retire.quiesce(selected, store, compute)
        before = store.writes
        with self.assertRaisesRegex(RuntimeError, 'no repeated fence'): retire.close(selected, store, 'e' * 64, inspect=True)
        self.assertEqual(store.writes, before)
        retire.close(selected, store, 'e' * 64)
        self.assertEqual(retire.close(selected, store, 'e' * 64, inspect=True)['state'], 'retirement_closed_pending_compute')
        with self.assertRaisesRegex(ValueError, 'handoff changed'): retire.close(selected, store, 'f' * 64, inspect=True)

    def test_actor_cancel_restores_same_generation_and_preserves_held_reservations(self):
        selected, store, compute = self.fixture()
        retire.quiesce(selected, store, compute)
        predecessor = dict(store.record['retirementPredecessor'])
        compute.settled = False
        result = retire.cancel(selected, store, compute)
        self.assertFalse(result['taskReservationsReleased']); self.assertEqual(store.record['state'], 'active')
        for key, value in predecessor.items(): self.assertEqual(store.record[key], value)
        self.assertEqual(store.record['retirementCancelledPlanDigest'], selected['planDigest'])
        self.assertEqual(sum('systemctl start' in script for script in compute.calls), 1)
        with self.assertRaisesRegex(ValueError, 'Actor cancelled'): retire.quiesce(selected, store, compute)
        with self.assertRaises(ValueError): retire.close(selected, store, 'e' * 64)

    def test_cancel_unknown_start_inspects_without_repeating_or_releasing_leases(self):
        selected, store, compute = self.fixture()
        compute.lost_stop = True
        with self.assertRaises(RuntimeError): retire.quiesce(selected, store, compute)
        compute.settled = False; compute.lost_start = True
        with self.assertRaises(TimeoutError): retire.cancel(selected, store, compute)
        self.assertTrue(store.record['retirementCancelStartPending'])
        result = retire.cancel(selected, store, compute, inspect=True)
        self.assertFalse(result['taskReservationsReleased'])
        self.assertEqual(sum('systemctl start' in script for script in compute.calls), 1)
        self.assertFalse(store.record['retirementCancelStartPending'])

    def test_cancel_closed_or_stale_plan_has_zero_effects(self):
        selected, store, compute = self.fixture()
        retire.quiesce(selected, store, compute); retire.close(selected, store, 'e' * 64)
        calls, writes = len(compute.calls), store.writes
        with self.assertRaisesRegex(ValueError, 'cannot be cancelled'): retire.cancel(selected, store, compute)
        self.assertEqual((len(compute.calls), store.writes), (calls, writes))

    def test_fresh_actor_can_retire_and_cancel_but_all_cancelled_plans_stay_revoked(self):
        selected, store, compute = self.fixture()
        retire.quiesce(selected, store, compute); retire.cancel(selected, store, compute)
        first = selected['planDigest']
        selected['planDigest'] = 'f' * 64; selected['toolSelection']['retirementPlanDigest'] = selected['planDigest']
        retire.quiesce(selected, store, compute); retire.cancel(selected, store, compute)
        self.assertEqual(set(store.record['retirementCancelledPlans']), {first, 'f' * 64})
        for digest in (first, 'f' * 64):
            selected['planDigest'] = digest; selected['toolSelection']['retirementPlanDigest'] = digest
            calls, writes = len(compute.calls), store.writes
            with self.assertRaisesRegex(ValueError, 'Actor cancelled'): retire.quiesce(selected, store, compute)
            self.assertEqual((len(compute.calls), store.writes), (calls, writes))


if __name__ == '__main__': unittest.main()
