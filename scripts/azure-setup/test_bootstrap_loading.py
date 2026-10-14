"""Exercise the actual bundled recovery loader before a mocked guest handoff."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('bootstrap_installer', Path(__file__).with_name('installer.py'))
installer = importlib.util.module_from_spec(spec); spec.loader.exec_module(installer)


class BootstrapLoadingTests(unittest.TestCase):
    def test_actual_recovery_module_loads_validates_then_prepares_handoff(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            prefix = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/'
            recovery = {'version': 1, 'recoveryClientID': '12345678-1234-1234-1234-123456789abc',
                'filesystemUUID': '12345678-1234-1234-1234-123456789abc',
                'oldControllerID': prefix + 'microsoft.compute/virtualmachines/old',
                'newControllerID': prefix + 'microsoft.compute/virtualmachines/new',
                'controllerIdentityID': prefix + 'microsoft.managedidentity/userassignedidentities/controller',
                'recoveryIdentityID': prefix + 'microsoft.managedidentity/userassignedidentities/recovery',
                'diskID': prefix + 'microsoft.compute/disks/owned', 'oldGeneration': 'gen-old', 'newGeneration': 'gen-new',
                'oldGenerationReceiptSHA256': 'b' * 64, 'ownerID': 'owner-one', 'scope': 'scope-one',
                'fenceBlobURL': 'https://owned.blob.core.windows.net/controller-fence/controller-fence.json',
                'mountSelectionPath': str(root / 'mount.json'), 'hostDeliverySelectionPath': str(root / 'host.json'),
                'apiService': 'luci-api.service', 'apiHealthURL': 'http://127.0.0.1:8730/v1/healthz'}
            ids = {'controller_recovery_instance_id': recovery['newControllerID']}
            state = {'state': 'infrastructure_observed_pending_private_handoff', 'planDigest': 'a' * 64, 'resourceIDs': ids}
            selection = {'stateBackend': {'subscriptionId': 'selected'}, 'terraformVariables': {'resource_group_name': 'owned'}}
            receipt = {'state': 'active', 'generation': recovery['oldGeneration'], 'diskID': recovery['diskID']}
            response = {'value': [{'message': 'LUCI_AZURE_INITIALIZE_RECEIPT=' + installer.json.dumps(receipt)}]}
            with patch.object(installer, 'plan_command', return_value={'planDigest': 'a' * 64, 'missing': []}), \
                    patch.object(installer, 'work_paths', return_value=(root / 'state.json', root)), \
                    patch.object(installer, 'private_json', side_effect=[state, selection]), \
                    patch.object(installer, 'initial_recovery_selection', return_value=recovery), \
                    patch.object(installer, 'initial_guest_script', return_value='exact checked handoff') as script, \
                    patch.object(installer.shutil, 'which', return_value='az'), \
                    patch.object(installer, 'invoke_initial_controller', return_value=response) as invoke, \
                    patch.object(installer, 'atomic_json'):
                result = installer.bootstrap(Path(__file__).parents[1], root / 'selection.json', 'a' * 64)
            self.assertEqual(result['bootstrapReceipt'], receipt)
            script.assert_called_once_with(recovery, False)
            invoke.assert_called_once_with('exact checked handoff', 'selected', 'owned', 'new')


if __name__ == '__main__': unittest.main()
