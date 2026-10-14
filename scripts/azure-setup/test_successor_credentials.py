"""Exact retained task identity, distinct generation TLS; no provider calls."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('successor_credentials', Path(__file__).with_name('credentials.py'))
credentials = importlib.util.module_from_spec(spec); spec.loader.exec_module(credentials)


class SuccessorCredentialTests(unittest.TestCase):
    def fixture(self):
        selected = {'terraformVariables': {'scope': 'scope-one', 'generation': 'gen-two', 'key_vault_name': 'owned-vault'}}
        prior = {'state': 'published', 'scope': 'scope-one', 'generation': 'gen-one', 'names': {}, 'published': {}, 'files': {}}
        files = {}
        for kind in ('team-api', 'admin-token', 'sandbox-seed', 'attach-key'):
            data = ('private-fixture-' + kind).encode(); sha = hashlib.sha256(data).hexdigest()
            files[credentials.MATERIAL[kind]] = data
            prior['files'][kind] = sha; prior['names'][kind] = 'scope-one-' + kind
            prior['published'][kind] = {'name': prior['names'][kind], 'version': 'a' * 32, 'sha256': sha,
                'id': 'https://owned-vault.vault.azure.net/secrets/' + prior['names'][kind] + '/' + 'a' * 32}
        return selected, prior, files

    def test_preserves_all_runtime_refs_and_attach_name_version_and_bytes(self):
        selected, prior, files = self.fixture()
        with patch.object(credentials, 'private_file', side_effect=lambda path, *args: json.dumps(prior).encode() if path.name == 'journal.json' else files[path.name]):
            retained = credentials.retained_task_material(selected, Path('predecessor'))
        self.assertEqual(len(retained), 4)
        for kind, row in retained.items():
            self.assertEqual(row['published'], prior['published'][kind])
            self.assertEqual(row['data'], files[credentials.MATERIAL[kind]])

    def test_scope_generation_state_bytes_and_version_substitution_refuse(self):
        for change in ('scope', 'generation', 'state', 'bytes', 'version', 'name', 'vault'):
            selected, prior, files = self.fixture()
            if change == 'scope': prior['scope'] = 'other'
            if change == 'generation': prior['generation'] = 'gen-two'
            if change == 'state': prior['state'] = 'publishing'
            if change == 'bytes': files['attach-key.txt'] = b'foreign'
            if change == 'version': prior['published']['team-api']['version'] = 'b' * 32
            if change == 'name': prior['names']['team-api'] = 'foreign'
            if change == 'vault': selected['terraformVariables']['key_vault_name'] = 'other-vault'
            with self.subTest(change=change), patch.object(credentials, 'private_file', side_effect=lambda path, *args: json.dumps(prior).encode() if path.name == 'journal.json' else files[path.name]), self.assertRaisesRegex(ValueError, 'predecessor|Predecessor'):
                credentials.retained_task_material(selected, Path('predecessor'))


if __name__ == '__main__': unittest.main()
