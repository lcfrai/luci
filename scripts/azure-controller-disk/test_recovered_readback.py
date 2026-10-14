"""Current mounted receipt transport, with no provider effects."""
import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('recovered_readback', Path(__file__).with_name('recover.py'))
recover = importlib.util.module_from_spec(spec); spec.loader.exec_module(recover)


class RecoveredReadbackTests(unittest.TestCase):
    def fixture(self):
        host = {'generation': 'gen-current'}
        selected = {'newControllerID': 'owned-controller', 'newGeneration': 'gen-current', 'diskID': 'owned-disk',
            'controllerSuccessor': {'sourceCommit': 'a' * 40},
            'controllerHostDelivery': {'hostConfig': host, 'hostConfigSHA256': recover.digest(host)}}
        record = {'state': 'active', 'generation': 'gen-current', 'controllerID': 'owned-controller', 'diskID': 'owned-disk',
            'generationReceiptSHA256': 'b' * 64, 'fenceReceiptDigest': 'c' * 64, 'hostReceiptSHA256': 'd' * 64}
        return selected, record

    def test_actual_mount_fence_and_delivery_digests_survive_readback(self):
        selected, record = self.fixture()
        result = recover.recovered_readback(selected, record)
        self.assertEqual(result['generationReceiptSHA256'], record['generationReceiptSHA256'])
        self.assertEqual(result['hostConfigSHA256'], selected['controllerHostDelivery']['hostConfigSHA256'])

    def test_stale_identity_or_incomplete_delivery_refuses(self):
        for key in ('generation', 'controllerID', 'diskID', 'generationReceiptSHA256', 'fenceReceiptDigest', 'hostReceiptSHA256'):
            selected, record = self.fixture(); record[key] = 'foreign'
            with self.subTest(key=key), self.assertRaises(ValueError): recover.recovered_readback(selected, record)
        selected, record = self.fixture(); selected['controllerHostDelivery']['hostConfig']['generation'] = 'stale'
        with self.assertRaises(ValueError): recover.recovered_readback(selected, record)


if __name__ == '__main__': unittest.main()
