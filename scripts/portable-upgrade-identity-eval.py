#!/usr/bin/env python3
"""Actual Linux package refusal/restart using only newly owned private state.

No Docker, browser, model or cloud operations. The old package is checked against
retained binary/index provenance; the candidate's entire archive is reverified.
"""
import argparse
import hashlib
import http.client
import json
import os
from pathlib import Path, PurePosixPath
import secrets
import shutil
import signal
import socket
import subprocess
import sys
import tarfile
import tempfile
import time


def sha(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def verify_package(package, metadata, archive=False):
    value = json.loads(metadata.read_text())
    assert value['sourceDirty'] is False
    assert sha(package / 'luci') == value['binarySHA256']
    assert sha(package / 'web/index.html') == value['webIndexSHA256']
    result = {key: value[key] for key in ('sourceCommit', 'binarySHA256', 'webIndexSHA256', 'archiveSHA256')}
    result['currentArchiveVerified'] = archive
    if archive:
        assert sha(value['archive']) == value['archiveSHA256']
        expected = {}
        with tarfile.open(value['archive'], 'r:gz') as source:
            for entry in source:
                name = PurePosixPath(entry.name)
                assert not name.is_absolute() and '..' not in name.parts
                if entry.isdir():
                    continue
                assert entry.isfile() and str(name) not in expected
                digest = hashlib.sha256()
                with source.extractfile(entry) as stream:
                    for block in iter(lambda: stream.read(1024 * 1024), b''):
                        digest.update(block)
                expected[str(name)] = digest.hexdigest()
        actual = {}
        for entry in package.rglob('*'):
            assert not entry.is_symlink()
            if entry.is_dir():
                continue
            assert entry.is_file()
            actual[entry.relative_to(package).as_posix()] = sha(entry)
        assert actual == expected, 'Candidate files differ from verified archive'
        result['verifiedFiles'] = len(actual)
        identity = hashlib.sha256(b'luci-portable-package-v1\n')
        for name in ['luci', *sorted(name for name in actual if name.startswith('web/'))]:
            identity.update(name.encode() + b'\0' + bytes.fromhex(actual[name]))
        result['packageIdentity'] = identity.hexdigest()
        assert (package / 'package-identity').read_text() == result['packageIdentity'] + '\n'
    return result


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('old-package', 'old-provenance', 'package', 'provenance', 'report'):
        parser.add_argument('--' + name, required=True, type=Path)
    args = parser.parse_args()
    assert sys.platform == 'linux', 'Process ownership uses Linux /proc'
    package, old = args.package.resolve(), args.old_package.resolve()
    report = args.report.resolve()
    report.parent.mkdir(parents=True, exist_ok=False, mode=0o700)
    receipt = {'passed': False, 'checks': [], 'cleanup': [], 'driverSHA256': sha(__file__),
               'scope': 'Actual packaged Linux launchers/service, explicit upgrade after refusal; API-created account and context fixture, no browser or native Windows claim.',
               'dockerUsed': False, 'providerCalls': 0, 'cloudCalls': 0}
    receipt['old'] = verify_package(old, args.old_provenance)
    receipt['candidate'] = verify_package(package, args.provenance, archive=True)
    assert receipt['old']['binarySHA256'] != receipt['candidate']['binarySHA256']
    shutil.copyfile(__file__, report.parent / 'driver.py')
    shutil.copyfile(args.provenance, report.parent / 'package-provenance.json')
    shutil.copyfile(args.old_provenance, report.parent / 'old-package-provenance.json')
    fixture = Path(tempfile.mkdtemp(prefix='luci-portable-upgrade-'))
    state, home, projects = (fixture / name for name in ('state', 'home', 'projects'))
    for entry in (state, home, projects):
        entry.mkdir(mode=0o700)
    port, preview_port = free_port(), free_port()
    assert port not in (8730, 1420) and preview_port not in (8730, 1420)
    environment = {key: value for key, value in os.environ.items() if key in ('PATH', 'LANG', 'LC_ALL', 'TZ')}
    environment.update(HOME=str(home), XDG_CONFIG_HOME=str(home / '.config'), XDG_DATA_HOME=str(home / '.local/share'),
                       XDG_STATE_HOME=str(home / '.local/state'), GIT_CONFIG_GLOBAL=str(home / 'gitconfig'), GIT_CONFIG_NOSYSTEM='1',
                       LUCI_DOCKER_HOST='unix://' + str(fixture / 'absent-docker.sock'), LUCI_PROJECTS_DIR=str(projects),
                       LUCI_BROWSER_PREVIEW_ADDR=f'127.0.0.1:{preview_port}', LUCI_BROWSER_PREVIEW_PORT=str(preview_port))
    token, active, stage = '', None, 'old-start'
    launchers = []
    password = secrets.token_urlsafe(36)
    private = [password]

    def request(method, path, body=None, expected=200, proof=None):
        connection = http.client.HTTPConnection('127.0.0.1', port, timeout=15)
        headers = {'Content-Type': 'application/json'}
        if token:
            headers['Authorization'] = 'Bearer ' + token
        if proof:
            headers['X-Luci-Local-Access-Token'] = proof
        connection.request(method, path, None if body is None else json.dumps(body), headers)
        response = connection.getresponse()
        result = response.read()
        connection.close()
        assert response.status == expected, f'{method} {path}: HTTP {response.status}, expected {expected}'
        return result

    def api(method, path, body=None, expected=200):
        result = json.loads(request(method, path, body, expected))
        if 'token' in result:
            private.append(result['token'])
        return result

    def processes():
        result = []
        for path in Path('/proc').iterdir():
            if not path.name.isdigit():
                continue
            try:
                values = (path / 'environ').read_bytes().split(b'\0')
                scoped = any(value == ('LUCI_STATE_DIR=' + str(state)).encode() or value.startswith(('LUCI_STATE_DIR=' + str(state) + '/control-plane/').encode()) for value in values)
                if not scoped:
                    continue
                fields = (path / 'stat').read_text().rsplit(')', 1)[1].split()
                if fields[0] == 'Z':
                    continue
                executable = os.readlink(path / 'exe')
                assert executable in (str(old / 'luci'), str(package / 'luci')), 'Unknown writer in fixture state'
                result.append({'pid': int(path.name), 'start': fields[19], 'executable': executable})
            except (FileNotFoundError, ProcessLookupError, PermissionError):
                continue
        return sorted(result, key=lambda row: row['pid'])

    def launch(directory):
        output = tempfile.TemporaryFile()
        child = subprocess.Popen(['bash', str(directory / 'start-luci.sh'), '--port', str(port), '--state-dir', str(state), '--no-browser'], env=environment, stdout=output, stderr=subprocess.STDOUT)
        record = {'child': child, 'output': output, 'package': directory}
        launchers.append(record)
        return record

    def output(record):
        record['output'].seek(0)
        return record['output'].read().decode(errors='replace')

    def ready(record):
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            assert record['child'].poll() is None, 'Owned launcher exited before readiness'
            if f'Luci is running at http://127.0.0.1:{port}' in output(record):
                proof = (state / 'control-plane/owner-access-token').read_text().strip()
                private.append(proof)
                return proof
            time.sleep(.1)
        raise AssertionError('Launcher readiness timed out')

    def stop(record):
        owned = processes()
        assert owned and all(row['executable'] == str(record['package'] / 'luci') for row in owned)
        if record['child'].poll() is None:
            record['child'].send_signal(signal.SIGINT)
            record['child'].wait(timeout=20)
        assert record['child'].returncode in (0, 130)
        assert not processes(), 'Owned service or workers survived shutdown'
        assert not (state / 'control-plane/owner-access-token').exists()
        try:
            request('GET', '/v1/control-plane/status')
        except (OSError, TimeoutError):
            pass
        else:
            raise AssertionError('Owned listener survived shutdown')
        receipt['cleanup'].append({'package': record['package'].name, 'ownedServiceCount': len(owned), 'remaining': 0, 'launcherExit': record['child'].returncode})
        record['output'].close()

    def inventory():
        # Logs may gain a shutdown message; all private data and project bytes
        # remain witnesses. No secret bytes are copied to the public receipt.
        rows = {}
        for root in (state, projects):
            for path in root.rglob('*'):
                assert not path.is_symlink()
                if path.is_file() and path.name not in ('service.err.log', 'service.out.log'):
                    rows[str(path.relative_to(fixture))] = sha(path)
        return rows

    def check(name, **details):
        receipt['checks'].append({'name': name, **details})
        print('PASS ' + name, flush=True)

    try:
        active = launch(old)
        ready(active)
        bootstrap = (state / 'control-plane/bootstrap-token').read_text().strip()
        private.append(bootstrap)
        owner = api('POST', '/v1/auth/bootstrap', {'bootstrapToken': bootstrap, 'username': 'upgrade-owner', 'displayName': 'Upgrade owner', 'password': password}, 201)
        token = owner['token']
        project = api('POST', '/v1/product/projects', {'name': 'retained-upgrade-work'}, 201)
        assert Path(project['path']).resolve() == projects / 'retained-upgrade-work'
        route = '/v1/product/projects/' + project['id']
        brief = api('POST', route + '/brief', {'revision': 0, 'purpose': 'Keep agreed work when opening an updated package.', 'audience': 'Self-hosted operators', 'constraints': 'No cloud or inference', 'criteria': ['Account and context survive the explicit upgrade']})
        assert hashlib.sha256(request('GET', '/')).hexdigest() == receipt['old']['webIndexSHA256']
        check('Actual legacy package serves its own web and saves a real owner and project context', projectId=project['id'], briefRevision=brief['revision'])
        stage = 'refuse-running-legacy'
        before, owned = inventory(), processes()
        rejected = launch(package)
        rejected['child'].wait(timeout=20)
        message = output(rejected)
        rejected['output'].close()
        assert rejected['child'].returncode != 0 and 'different or older Luci package' in message
        assert 'Luci is running at' not in message and all(value not in message for value in private)
        assert inventory() == before and processes() == owned
        assert hashlib.sha256(request('GET', '/')).hexdigest() == receipt['old']['webIndexSHA256']
        check('New launcher refuses the real legacy service without changing its processes or private/project bytes', exitCode=rejected['child'].returncode, retainedFiles=len(before), message=message.strip())
        stage = 'explicit-old-stop'
        stop(active)
        active = None
        check('Explicitly closing the owned old launcher stops its service and removes its private proof')
        stage = 'new-start'
        active = launch(package)
        proof = ready(active)
        assert request('GET', '/v1/auth/local-package', proof=proof).decode() == receipt['candidate']['packageIdentity'] + '\n'
        assert hashlib.sha256(request('GET', '/')).hexdigest() == receipt['candidate']['webIndexSHA256']
        assert api('GET', '/v1/auth/me')['user']['id'] == owner['user']['id'], 'Retained session no longer authenticates'
        token = api('POST', '/v1/auth/login', {'username': 'upgrade-owner', 'password': password})['token']
        assert api('GET', route + '/brief') == brief
        check('New package starts only after explicit old shutdown, serves its exact identity/web and retains account, session and saved context')
        stage = 'same-package-reuse'
        before, owned = inventory(), processes()
        reused = launch(package)
        reused['child'].wait(timeout=20)
        message = output(reused)
        reused['output'].close()
        assert reused['child'].returncode == 0 and 'already running' in message
        assert all(value not in message for value in private)
        assert inventory() == before and processes() == owned
        check('An exact matching package reuses the active service without mutation or capability disclosure')
        stop(active)
        active = None
        receipt['passed'] = True
    except Exception as error:
        receipt['failure'] = {'stage': stage, 'type': type(error).__name__, 'message': str(error)}
    finally:
        def cleanup_error(label, error):
            receipt.setdefault('cleanupErrors', []).append(label + ': ' + str(error))

        def cleanup_processes():
            try:
                return processes()
            except Exception as error:
                # Unknown ownership stops cleanup, but must not erase the
                # failure receipt or authorize deletion of private state.
                cleanup_error('Inspect owned fixture processes', error)
                return None

        try:
            if active is not None and active['child'].poll() is None:
                stop(active)
        except Exception as error:
            cleanup_error('Normal fixture shutdown', error)
        # Also retain ownership of refused/reused launcher processes if their
        # response times out before the normal wait completes.
        for record in launchers:
            try:
                if record['child'].poll() is None:
                    cleanup_error('Launcher cleanup', 'Owned fixture launcher needed fallback termination')
                    record['child'].terminate()
                    try:
                        record['child'].wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        record['child'].kill()
                        record['child'].wait(timeout=5)
            except ProcessLookupError:
                pass
            except Exception as error:
                cleanup_error('Stop tracked launcher', error)
            try:
                record['output'].close()
            except Exception as error:
                cleanup_error('Close fixture output', error)
        remaining = cleanup_processes()
        if remaining:
            cleanup_error('Service cleanup', 'Owned fixture process needed fallback shutdown')
            for row in remaining:
                try:
                    current = cleanup_processes()
                    if current is not None and row in current:
                        os.kill(row['pid'], signal.SIGTERM)
                except ProcessLookupError:
                    pass  # The same owned process completed normal shutdown.
                except Exception as error:
                    cleanup_error('Stop proven fixture process', error)
            deadline = time.monotonic() + 10
            while cleanup_processes() and time.monotonic() < deadline:
                time.sleep(.1)
            if cleanup_processes():
                cleanup_error('Service cleanup', 'Owned fixture process remains')
        if receipt.get('cleanupErrors'):
            receipt['passed'] = False
        if receipt['passed']:
            try:
                shutil.rmtree(fixture)
                receipt['privateFixtureRemoved'] = True
            except Exception as error:
                cleanup_error('Remove stopped private fixture', error)
                receipt['passed'] = False
        if not receipt['passed']:
            receipt['retainedPrivateFixture'] = str(fixture)
        sanitized = json.dumps(receipt, indent=2)
        for value in private:
            sanitized = sanitized.replace(value, '[redacted]')
        report.write_text(sanitized + '\n')
    print(json.dumps({'passed': receipt['passed'], 'checks': len(receipt['checks']), 'report': str(report)}))
    return 0 if receipt['passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
