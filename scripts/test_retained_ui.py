import json
from pathlib import Path
import tempfile
import unittest
from retained_ui import (SHADCN_ROOT, SHADCN_MANIFEST, SHADCN_METADATA, SHADCN_COMPONENTS,
                         read_shadcn, verify_shadcn)


class RetainedUITests(unittest.TestCase):
    def test_packager_verifies_actual_checked_in_component_provenance(self):
        files = read_shadcn(Path(__file__).resolve().parent.parent)
        self.assertEqual(verify_shadcn(files)['adaptedSourceFilesVerified'], 5)
        changed = {**files, SHADCN_ROOT + 'upstream/button.tsx': b'changed'}
        with self.assertRaisesRegex(ValueError, 'original shadcn source'):
            verify_shadcn(changed)
        manifest = json.loads(files[SHADCN_MANIFEST])
        manifest['files'][0]['path'] = '../../outside'
        with self.assertRaisesRegex(ValueError, 'shadcn source entry'):
            verify_shadcn({**files, SHADCN_MANIFEST: json.dumps(manifest).encode()})

    def test_packager_refuses_linked_source_or_ancestor_before_reading(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / 'external'
            target.mkdir()
            parent = root / 'third-party'
            parent.mkdir()
            (parent / 'shadcn-ui').symlink_to(target, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, 'Linked retained shadcn'):
                read_shadcn(root)

    def test_partial_adaptation_cannot_omit_the_original_grant(self):
        with self.assertRaisesRegex(ValueError, 'MIT grant'):
            verify_shadcn({'desktop/src/components/ui/button.tsx': b'// adaptation'})


if __name__ == '__main__':
    unittest.main()
