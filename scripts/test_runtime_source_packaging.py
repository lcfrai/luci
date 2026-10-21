"""Source-only checks for the sandbox inputs retained by both release paths."""
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[1]


class RuntimeSourcePackagingTests(unittest.TestCase):
    def test_release_paths_retain_current_sandbox_dockerfile_inputs(self):
        dockerfile = (ROOT / 'images/sandbox/Dockerfile').read_text(encoding='utf-8')
        sources = [line.split()[1] for line in dockerfile.splitlines()
                   if line.startswith('COPY ') and not line.startswith('COPY --from=')]
        self.assertIn('sandboxd', sources)
        self.assertIn('images/sandbox/codex-native-task.cjs', sources)

        windows = (ROOT / 'scripts/build-release.ps1').read_text(encoding='utf-8')
        files = re.search(r'foreach \(\$luciFile in @\(([^)]*)\)\)', windows)
        trees = re.search(r'foreach \(\$luciTree in @\(([^)]*)\)\)', windows)
        self.assertIsNotNone(files)
        self.assertIsNotNone(trees)
        copied_files = set(re.findall(r"'([^']+)'", files.group(1)))
        copied_trees = set(re.findall(r"'([^']+)'", trees.group(1)))
        for source in sources:
            with self.subTest(source=source):
                if source == 'sandboxd':
                    for item in (ROOT / source).iterdir():
                        relative = item.relative_to(ROOT).as_posix()
                        self.assertIn(relative, copied_trees if item.is_dir() else copied_files)
                else:
                    self.assertTrue((ROOT / source).is_file())
                    self.assertIn(source, copied_files)

        unix = (ROOT / 'scripts/build-release.sh').read_text(encoding='utf-8')
        self.assertIn('cp -R "$root/images/sandbox" "$package/runtime/images/sandbox"', unix)
        loop = re.search(r'for source in ([^;]+); do', unix)
        self.assertIsNotNone(loop)
        self.assertEqual(set(loop.group(1).split()),
                         {item.name for item in (ROOT / 'sandboxd').iterdir()})
        self.assertIn('cp -R "$root/sandboxd/$source" "$package/runtime/sandboxd/$source"', unix)


if __name__ == '__main__':
    unittest.main()
