#!/usr/bin/env python3
"""Run Go tests with preserved exit status, bounded check annotations and full logs."""
from __future__ import annotations
import argparse
from collections import deque
import html
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ANNOTATION_LIMIT = 10
DIAGNOSTIC_LIMIT = 3500
LOCATION = re.compile(r'(?:^|\s)([^\s:]+\.go):(\d+)(?::\d+)?:?')
NOISE = re.compile(r'^\s*(?:=== (?:RUN|PAUSE|CONT)|--- (?:PASS|FAIL|SKIP)|PASS\s*$|FAIL\s*$)')


def escape_data(value):
    return str(value).replace('%', '%25').replace('\r', '%0D').replace('\n', '%0A')


def escape_property(value):
    return escape_data(value).replace(':', '%3A').replace(',', '%2C')


class Output:
    def __init__(self):
        self.tail = deque(maxlen=60)
        self.important = []

    def add(self, value):
        for line in value.splitlines():
            if NOISE.match(line):
                continue
            # A panic or first assertion can precede a long stack/log tail.
            if len(self.important) < 12 and (LOCATION.search(line) or re.search(r'panic:|fatal error:|WARNING: DATA RACE', line)):
                self.important.append(line)
            self.tail.append(line)

    def diagnostic(self):
        lines = list(self.important)
        lines.extend(line for line in list(self.tail)[-28:] if line not in self.important)
        text = '\n'.join(lines).strip()
        if len(text) > DIAGNOSTIC_LIMIT:
            text = text[:1600] + '\n[… full diagnostic retained in output.log …]\n' + text[-1800:]
        return text


class Report:
    def __init__(self, name, root):
        self.name, self.root = name, Path(root).resolve()
        self.packages, self.tests = {}, {}
        self.failed_tests, self.failed_packages = {}, set()
        self.counts = {'passed': 0, 'failed': 0, 'skipped': 0}
        self.raw = Output()
        self.modules = []
        for file in [self.root / 'go.mod', self.root / 'control-plane/go.mod', self.root / 'sandboxd/go.mod']:
            if file.is_file():
                match = re.search(r'^module\s+(\S+)', file.read_text(), re.M)
                if match:
                    self.modules.append((match[1], file.parent))

    def add(self, line):
        try:
            event = json.loads(line)
        except (ValueError, UnicodeError):
            self.raw.add(line)
            return line
        if not isinstance(event, dict):
            self.raw.add(line)
            return line
        package = event.get('Package') or event.get('ImportPath') or ''
        test = event.get('Test') or ''
        action, output = event.get('Action'), event.get('Output')
        if isinstance(output, str):
            self.raw.add(output)
            if package:
                self.packages.setdefault(package, Output()).add(output)
            if package and test:
                self.tests.setdefault((package, test), Output()).add(output)
        if action in ('pass', 'fail', 'skip') and package and test:
            self.counts[{'pass': 'passed', 'fail': 'failed', 'skip': 'skipped'}[action]] += 1
            if action == 'fail':
                self.failed_tests[(package, test)] = self.tests.get((package, test), Output()).diagnostic()
            self.tests.pop((package, test), None)
        elif action in ('fail', 'build-fail') and package:
            self.failed_packages.add(package)
        return output if isinstance(output, str) else ''

    def location(self, package, diagnostic):
        module_dir = next((directory / package[len(module):].lstrip('/') for module, directory in self.modules if package == module or package.startswith(module + '/')), None)
        for match in LOCATION.finditer(diagnostic):
            file = Path(match[1])
            candidates = [file] if file.is_absolute() else ([module_dir / file] if module_dir else []) + [self.root / file]
            for candidate in candidates:
                path = candidate.resolve()
                if path.is_relative_to(self.root) and path.is_file():
                    return {'file': path.relative_to(self.root).as_posix(), 'line': int(match[2])}
        return {}

    def finish(self, exit_code, command):
        failures = []
        tested = {package for package, _ in self.failed_tests}
        for (package, test), diagnostic in self.failed_tests.items():
            failures.append({'package': package, 'test': test, 'diagnostic': diagnostic or 'Test failed without output; see events.jsonl.', **self.location(package, diagnostic)})
        for package in sorted(self.failed_packages - tested):
            diagnostic = self.packages.get(package, self.raw).diagnostic() or self.raw.diagnostic()
            failures.append({'package': package, 'test': '', 'diagnostic': diagnostic or 'Package failed without output; see events.jsonl.', **self.location(package, diagnostic)})
        if exit_code and not failures:
            failures.append({'package': '', 'test': '', 'diagnostic': self.raw.diagnostic() or 'Go exited without a test event. Inspect output.log for launch, build or toolchain errors.'})
        return {'name': self.name, 'exitCode': exit_code, 'command': command, 'testEvents': self.counts, 'failedPackages': sorted(self.failed_packages | tested), 'failures': failures}


def annotate(report, stream):
    if not report['exitCode']:
        return
    failures = report['failures']
    shown = failures[:ANNOTATION_LIMIT if len(failures) <= ANNOTATION_LIMIT else ANNOTATION_LIMIT - 1]
    for failure in shown:
        title = report['name'] + ': ' + (failure['test'] or failure['package'] or 'command failed')
        properties = {'title': title[:160]}
        properties.update({key: failure[key] for key in ('file', 'line') if key in failure})
        attributes = ','.join(f'{key}={escape_property(value)}' for key, value in properties.items())
        message = f"{failure['package']}\n{failure['diagnostic']}\nExit {report['exitCode']}; full events/output/report are in the luci-go-tests artifact."
        print(f'::error {attributes}::{escape_data(message)}', file=stream)
    if len(shown) < len(failures):
        print(f"::error title={escape_property(report['name'] + ': additional failures')}::{len(failures) - len(shown)} additional failures are retained in report.json and output.log in the luci-go-tests artifact.", file=stream)


def summary(report):
    count = report['testEvents']
    lines = [f"### {html.escape(report['name'])}", '', f"Exit code **{report['exitCode']}**. Test events: {count['passed']} passed, {count['failed']} failed, {count['skipped']} skipped.", '', 'Full `events.jsonl`, readable `output.log` and structured `report.json` are retained in the `luci-go-tests` artifact for this commit.', '']
    for failure in report['failures'][:ANNOTATION_LIMIT]:
        name = failure['test'] or failure['package'] or 'Command failure'
        lines.append(f"<details><summary>{html.escape(name)}</summary><pre>{html.escape(failure['diagnostic'])}</pre></details>\n")
    if len(report['failures']) > ANNOTATION_LIMIT:
        lines.append('Additional failures are retained in the artifact.\n')
    return '\n'.join(lines) + '\n'


def run(name, directory, command, root=None):
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    report = Report(name, root or Path.cwd())
    code = 127
    with (directory / 'events.jsonl').open('wb', buffering=0) as events, (directory / 'output.log').open('w', encoding='utf-8', buffering=1) as output:
        try:
            process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        except OSError as error:
            message = f'Could not launch Go test command: {error}\n'
            events.write(message.encode())
            output.write(report.add(message))
        else:
            try:
                for raw in process.stdout:
                    events.write(raw)
                    output.write(report.add(raw.decode('utf-8', errors='replace')))
                code = process.wait()
                if code < 0:
                    code = 128 - code
            finally:
                process.stdout.close()
                if process.poll() is None:
                    process.terminate()
                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()
    result = report.finish(code, command)
    (directory / 'report.json').write_text(json.dumps(result, indent=2) + '\n', encoding='utf-8')
    markdown = summary(result)
    (directory / 'summary.md').write_text(markdown, encoding='utf-8')
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as stream:
            stream.write(markdown)
    annotate(result, sys.stdout)
    print(f"{name}: exit {code}; {result['testEvents']}; logs retained at {directory}")
    return code


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--name', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if not command:
        parser.error('A Go test command is required after --.')
    return run(args.name, args.output, command)

if __name__ == '__main__':
    sys.exit(main())
