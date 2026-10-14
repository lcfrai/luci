"""Finite normal setup progression. All provider adapters are offline fixtures."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('journey_guided', Path(__file__).with_name('guided.py'))
guided = importlib.util.module_from_spec(spec); spec.loader.exec_module(guided)


class JourneyTests(unittest.TestCase):
    def ready(self):
        variables = {'scope': 'scope-one', 'generation': 'gen-one', 'controller_admin_ssh_public_key': 'fixture',
            'runtime_image_version_id': 'runtime', 'controller_image_version_id': 'controller',
            'host_secret_refs': {'bound': True}, 'controller_secret_ids': ['bound']}
        completed = ['source:container', 'source:upload', 'source:delete-container']
        for role in ('runtime', 'controller'):
            completed += [role + ':' + key for key in ('nic', 'vm', 'disk-tag', 'grant', 'install',
                'deprovision', 'deallocate', 'generalize', 'capture', 'retired')]
        return {'terraformVariables': variables}, {
            'azure-foundation-state.json': {'state': 'foundation_bound_private_data_route_observed'},
            'azure-builder-source.json': {}, 'azure-builder-plan.json': {'roles': {
                'runtime': {'imageVersionID': 'runtime'}, 'controller': {'imageVersionID': 'controller'}}},
            'azure-builder-state.json': {'completed': completed, 'state': 'private_images_bound_staging_retired'},
            'azure-credentials/journal.json': {'state': 'published'}, 'operator-template.json': {'public': 'source'}}

    def fixture(self, state=None):
        temporary = tempfile.TemporaryDirectory(); self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        def write(path, value):
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_text(json.dumps(value))
        canonical = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
        core = SimpleNamespace(bundle=lambda _: {'luciRevision': 'a' * 40}, requested=lambda s: s['terraformVariables'],
            private_json=lambda path: json.loads(Path(path).read_text()), atomic_json=write,
            digest=lambda value: hashlib.sha256(canonical(value)).hexdigest())
        selected, records = self.ready()
        selected.update(name='example', region='australiaeast')
        if state: records['azure-setup-state.json'] = {'state': state}
        selection = root / 'selection.json'; write(selection, selected)
        for name, value in records.items(): write(root / name, value)
        calls = []
        def set_state(state):
            write(root / 'azure-setup-state.json', {'state': state, 'terraformPlanSHA256': 'b' * 64})
            return {'state': state}
        core.plan_command = lambda *_: {'planDigest': 'c' * 64, 'missing': []}
        core.prepare = lambda *_: set_state('prepared')
        def apply(*args): calls.append('apply'); return set_state('apply_returned_pending_provider_readback')
        def reconcile(*args): calls.append('reconcile'); return set_state('infrastructure_observed_pending_private_handoff')
        def bootstrap(*args, **kwargs): calls.append(('bootstrap', kwargs['reconcile'])); return set_state('controller_bootstrapped_gateway_unverified')
        core.apply, core.observed_outputs, core.bootstrap = apply, reconcile, bootstrap
        def template(*args):
            mode = args[3]; calls.append(('template', mode))
            if mode == 'create': write(root / 'template/create.json.response', {'created': True})
            if mode == 'start': write(root / 'template/build.json', {'started': True})
            result = {'state': 'template_ready_provider_guest_unverified' if mode == 'status' else 'template_' + mode + '_returned'}
            write(root / 'azure-template-state.json', result)
            return result
        def finalize(*_): write(root / 'azure-operator-finalization.json', {'signed': True}); return {'state': 'signed'}
        def install(*args, **kwargs):
            calls.append(('install', kwargs['reconcile']))
            write(root / 'azure-operator-install-state.json', {'state': 'operator_finalized_private_route_unverified'})
            return {'state': 'operator_finalized_private_route_unverified'}
        modules = {'foundation': SimpleNamespace(AzureARM=lambda _: Mock()), 'builder': SimpleNamespace(),
            'operator_setup': SimpleNamespace(controller_step=template, finalize=finalize, install=install)}
        return root, core, selection, modules, calls

    def test_full_normal_tail_has_one_apply_then_readback_and_authenticated_handoff(self):
        root, core, selection, modules, calls = self.fixture()
        approve = lambda text: text.split('type ', 1)[1].split(' (anything', 1)[0]
        with patch.object(guided, 'connection_descriptor', return_value={'state': 'descriptor_private_route_unverified', 'connected': False}):
            result = guided.continue_setup(core, root, selection, ask=approve, emit=lambda _: None, modules=modules, cli=Mock())
        self.assertFalse(result['connected'])
        self.assertEqual(calls, ['apply', 'reconcile', ('bootstrap', False), ('template', 'create'),
            ('template', 'start'), ('template', 'status'), ('install', False)])
        self.assertEqual(len(core.private_json(root / 'azure-guided-review.json')['approvals']), 2)

    def test_refused_exact_cost_review_has_zero_remote_mutations(self):
        root, core, selection, modules, calls = self.fixture('prepared')
        result = guided.continue_setup(core, root, selection, ask=lambda _: 'no', emit=lambda _: None, modules=modules, cli=Mock())
        self.assertEqual(result['state'], 'infrastructure_review_paused')
        self.assertEqual(calls, [])
        self.assertFalse((root / 'azure-guided-review.json').exists())

    def test_restart_unknown_apply_and_bootstrap_choose_only_reconciliation(self):
        selected, records = self.ready()
        for state, expected in (('apply_pending', ('reconcile', '')), ('bootstrap_pending', ('bootstrap', True))):
            records['azure-setup-state.json'] = {'state': state}
            self.assertEqual(guided.journey_action(selected, records), expected)
        records['azure-setup-state.json'] = {'state': 'controller_bootstrapped_gateway_unverified'}
        records['azure-template-state.json'] = {'pendingAction': 'create'}
        self.assertEqual(guided.journey_action(selected, records), ('template', 'reconcile'))
        records['azure-template-state.json']['pendingAction'] = 'start'
        self.assertEqual(guided.journey_action(selected, records), ('template', 'status'))
        records['azure-operator-install-state.json'] = {'state': 'operator_install_pending'}
        self.assertEqual(guided.journey_action(selected, records), ('operator-install', True))

    def test_builder_roles_order_and_pending_substeps_never_repeat_writes(self):
        selected, records = self.ready()
        records['azure-builder-state.json']['completed'] = ['source:container', 'source:upload']
        self.assertEqual(guided.journey_action(selected, records), ('builder', ('allocate', 'runtime', False)))
        records['azure-builder-state.json']['pending'] = 'runtime:nic'
        self.assertEqual(guided.journey_action(selected, records), ('builder', ('allocate', 'runtime', True)))
        records['azure-builder-state.json']['pending'] = 'controller:nic'
        with self.assertRaisesRegex(ValueError, 'differs'): guided.journey_action(selected, records)
        records['azure-builder-state.json']['abortRequested'] = True
        with self.assertRaisesRegex(ValueError, 'abort'): guided.journey_action(selected, records)

    def test_cost_authority_changes_with_source_generation_or_resource_plan(self):
        root, core, selection, _, _ = self.fixture()
        selected = core.private_json(selection)
        approve = lambda text: text.split('type ', 1)[1].split(' (anything', 1)[0]
        manifest = core.bundle(root)
        self.assertTrue(guided.approve_stage(core, root, manifest, selected, 'builder', {'resource': 'owned'}, approve, lambda _: None))
        self.assertTrue(guided.approve_stage(core, root, manifest, selected, 'builder', {'resource': 'owned'}, lambda _: self.fail('same authority reprompted'), lambda _: None))
        for key in ('source', 'generation', 'plan'):
            changed, source, proposal = copy.deepcopy(selected), dict(manifest), {'resource': 'owned'}
            if key == 'source': source['luciRevision'] = 'd' * 40
            if key == 'generation': changed['terraformVariables']['generation'] = 'gen-other'
            if key == 'plan': proposal['resource'] = 'foreign'
            self.assertFalse(guided.approve_stage(core, root, source, changed, 'builder', proposal, lambda _: 'no', lambda _: None))

    def test_waiting_template_stops_without_poll_or_mutation_retry(self):
        root, core, selection, modules, calls = self.fixture('controller_bootstrapped_gateway_unverified')
        core.atomic_json(root / 'template/create.json.response', {})
        core.atomic_json(root / 'template/build.json', {})
        modules['operator_setup'].controller_step = Mock(return_value={'state': 'template_status_returned'})
        result = guided.continue_setup(core, root, selection, ask=lambda _: self.fail('unexpected authority'), emit=lambda _: None, modules=modules, cli=Mock())
        self.assertEqual(result['nextAction'], 'continue')
        self.assertFalse(result['connected'])
        self.assertEqual(modules['operator_setup'].controller_step.call_count, 1)
        self.assertEqual(calls, [])

    def test_generation_interruption_and_unknown_state_refuse_progress(self):
        selected, records = self.ready()
        records['azure-credentials/journal.json']['state'] = 'generation_pending'
        with self.assertRaisesRegex(RuntimeError, 'interrupted'): guided.journey_action(selected, records)
        records['azure-credentials/journal.json']['state'] = 'published'
        records['azure-setup-state.json'] = {'state': 'invented'}
        with self.assertRaisesRegex(ValueError, 'Unknown'): guided.journey_action(selected, records)

    def test_front_stages_resume_foundation_and_finish_image_retirement_receipt(self):
        selected, records = self.ready()
        records.pop('azure-foundation-state.json')
        self.assertEqual(guided.journey_action(selected, records), ('configure', ''))
        records['azure-foundation-plan.json'] = {'planDigest': 'fixture'}
        records['azure-foundation-state.json'] = {'state': 'foundation_write_pending', 'pending': 'owned-resource'}
        self.assertEqual(guided.journey_action(selected, records), ('foundation', 'reconcile'))
        records['azure-foundation-state.json'] = {'state': 'foundation_observed_pending_private_data_route'}
        self.assertEqual(guided.journey_action(selected, records), ('foundation-bind', ''))
        records['azure-foundation-state.json']['state'] = 'foundation_bound_private_data_route_observed'
        records['azure-builder-state.json']['state'] = 'builder_stage_observed'
        self.assertEqual(guided.journey_action(selected, records), ('builder', ('retire-staging', '', False)))
        records['azure-builder-state.json']['state'] = 'private_images_bound_staging_retired'
        self.assertEqual(guided.journey_action(selected, records), ('prepare', ''))


if __name__ == '__main__': unittest.main()
