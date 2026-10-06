import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).with_name('ci-go-tests.py').resolve()
spec = importlib.util.spec_from_file_location('ci_go_tests', SCRIPT)
reporter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reporter)

class GoTestReporting(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='luci-go-report-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'go.mod').write_text('module example.com/fixture\n\ngo 1.25.0\n')
        (self.root / 'fixture_test.go').write_text('package fixture\nimport "testing"\nfunc TestFailure(t *testing.T) { t.Fatal("expected annotated failure") }\n')

    def event(self, report, action, test='', output=''):
        return report.add(json.dumps({'Action': action, 'Package': 'example.com/fixture', 'Test': test, 'Output': output}) + '\n')

    def test_test_failure_points_to_checkout_source_and_keeps_assertion_before_long_tail(self):
        report = reporter.Report('Go services', self.root)
        self.event(report, 'output', 'TestFailure', '    fixture_test.go:3: expected annotated failure\n')
        for index in range(100):
            self.event(report, 'output', 'TestFailure', f'additional diagnostic {index}\n')
        self.event(report, 'fail', 'TestFailure')
        self.event(report, 'fail')
        result = report.finish(1, ['go', 'test'])
        self.assertEqual(len(result['failures']), 1, 'Do not duplicate package failure when test failure is available.')
        failure = result['failures'][0]
        self.assertEqual((failure['file'], failure['line']), ('fixture_test.go', 3))
        self.assertIn('expected annotated failure', failure['diagnostic'])
        self.assertIn('additional diagnostic 99', failure['diagnostic'])

    def test_compile_and_raw_toolchain_errors_are_annotated_without_test_events(self):
        report = reporter.Report('Go services', self.root)
        report.add(json.dumps({'Action': 'build-output', 'ImportPath': 'example.com/fixture', 'Output': './fixture_test.go:3:7: undefined: broken\n'}) + '\n')
        report.add(json.dumps({'Action': 'build-fail', 'ImportPath': 'example.com/fixture'}) + '\n')
        result = report.finish(1, [])
        self.assertIn('undefined: broken', result['failures'][0]['diagnostic'])
        self.assertEqual(result['failures'][0]['file'], 'fixture_test.go')
        raw = reporter.Report('Go services', self.root)
        raw.add('go: failed to load toolchain\n')
        self.assertIn('failed to load toolchain', raw.finish(2, [])['failures'][0]['diagnostic'])

    def test_annotations_escape_commands_and_cap_count_while_report_retains_all_failures(self):
        report = reporter.Report('Go: services, 100%', self.root)
        for index in range(15):
            self.event(report, 'output', f'TestFailure{index}', 'assertion failed\n::error title=Injected::untrusted\n100%\rnext')
            self.event(report, 'fail', f'TestFailure{index}')
        result = report.finish(7, [])
        output = io.StringIO()
        reporter.annotate(result, output)
        commands = output.getvalue().splitlines()
        self.assertEqual(len(commands), reporter.ANNOTATION_LIMIT)
        self.assertTrue(all(line.startswith('::error title=Go%3A services%2C 100%25') for line in commands))
        self.assertIn('%0A::error title=Injected::untrusted', commands[0])
        self.assertIn('100%25', commands[0])
        self.assertEqual(len(result['failures']), 15)
        self.assertIn('6 additional failures', commands[-1])

    def test_success_has_no_failure_annotation_and_summary_escapes_markup(self):
        report = reporter.Report('Go services', self.root)
        self.event(report, 'pass', 'TestPassed')
        self.event(report, 'skip', 'TestSkipped')
        result = report.finish(0, [])
        output = io.StringIO()
        reporter.annotate(result, output)
        self.assertEqual(output.getvalue(), '')
        self.assertEqual(result['testEvents'], {'passed': 1, 'failed': 0, 'skipped': 1})
        result['failures'] = [{'test': '<script>name</script>', 'package': '', 'diagnostic': '<script>diagnostic</script>'}]
        summary = reporter.summary(result)
        self.assertNotIn('<script>', summary)
        self.assertIn('&lt;script&gt;', summary)

    def test_exit_status_raw_bytes_readable_log_report_and_summary_survive_child_failure(self):
        output = self.root / 'logs'
        summary = self.root / 'step-summary.md'
        child = 'import os,sys,json; print(json.dumps({"Package":"example.com/fixture","Test":"TestBroken","Action":"output","Output":"fixture_test.go:3: failure marker\\n"}),flush=True); os.write(2,b"raw build diagnostic\\xff\\n"); print(json.dumps({"Package":"example.com/fixture","Test":"TestBroken","Action":"fail"}),flush=True); sys.exit(7)'
        with mock.patch.dict(os.environ, {'GITHUB_STEP_SUMMARY': str(summary)}), contextlib.redirect_stdout(io.StringIO()) as console:
            code = reporter.run('Go services', output, [sys.executable, '-c', child], self.root)
        self.assertEqual(code, 7)
        self.assertIn(b'raw build diagnostic\xff', (output / 'events.jsonl').read_bytes())
        self.assertIn('failure marker', (output / 'output.log').read_text())
        self.assertEqual(json.loads((output / 'report.json').read_text())['exitCode'], 7)
        self.assertIn('Exit code **7**', summary.read_text())
        self.assertIn('::error', console.getvalue())

    def test_signal_and_launch_failure_cannot_report_success(self):
        with mock.patch.dict(os.environ, {}, clear=False), contextlib.redirect_stdout(io.StringIO()):
            os.environ.pop('GITHUB_STEP_SUMMARY', None)
            code = reporter.run('Go services', self.root / 'signal', [sys.executable, '-c', 'import os,signal; os.kill(os.getpid(),signal.SIGTERM)'])
            missing = reporter.run('Go services', self.root / 'missing', ['/definitely-missing-luci-go-test-command'])
        self.assertEqual(code, 143)
        self.assertEqual(missing, 127)
        self.assertIn('Could not launch', (self.root / 'missing/output.log').read_text())

    @unittest.skipUnless(shutil.which('go'), 'Go is required for the actual compiler/runner integration.')
    def test_actual_go_assertion_and_build_failure_preserve_nonzero_exit_with_source_annotations(self):
        environment = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local', 'GOPROXY': 'off'}
        environment.pop('GITHUB_STEP_SUMMARY', None)
        for name, content, marker in [('assertion', None, 'expected annotated failure'), ('compile', 'package fixture\nvar broken = absentIdentifier\n', 'undefined: absentIdentifier')]:
            with self.subTest(case=name):
                if content:
                    (self.root / 'fixture_test.go').write_text(content)
                destination = self.root / name
                completed = subprocess.run([sys.executable, str(SCRIPT), '--name', 'Go integration', '--output', str(destination), '--', 'go', 'test', '-json', './...'], cwd=self.root, env=environment, text=True, capture_output=True, timeout=60)
                self.assertEqual(completed.returncode, 1, completed.stderr)
                self.assertIn('::error', completed.stdout)
                self.assertIn(marker, completed.stdout)
                self.assertIn('file=fixture_test.go', completed.stdout)
                self.assertEqual(json.loads((destination / 'report.json').read_text())['exitCode'], 1)

if __name__ == '__main__':
    unittest.main()
