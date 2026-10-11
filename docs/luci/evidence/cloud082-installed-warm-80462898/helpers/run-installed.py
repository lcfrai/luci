import datetime
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import time

os.umask(0o077)
root = Path(__file__).resolve().parent
config = root / 'run-config-001.json'
helper = root / 'preview-smoke.mjs'
assert hashlib.sha256(helper.read_bytes()).hexdigest() == '767db04a94975c16c773c4b4e8cf4bcbf43ac991550bfa9311cd01d5b27f659e'
output = root / 'wrapper-001'
output.mkdir(mode=0o700)
command = ['node', str(helper), str(config)]
invocation = {'command': command, 'startedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
              'helperSHA256': hashlib.sha256(helper.read_bytes()).hexdigest(),
              'configSHA256': hashlib.sha256(config.read_bytes()).hexdigest(),
              'scope': 'Bounded installed warm-base multistage fixture; wrapper exit alone is not cleanup or product acceptance'}
(output / 'invocation.json').write_text(json.dumps(invocation, indent=2) + '\n')
started = time.monotonic()
timed_out = False
forced = False
wait_error = None
with (output / 'stdout.log').open('x') as stdout, (output / 'stderr.log').open('x') as stderr:
    child = subprocess.Popen(command, cwd=root, stdout=stdout, stderr=stderr)
    try:
        code = child.wait(timeout=510)
    except subprocess.TimeoutExpired:
        timed_out = True
        child.send_signal(signal.SIGTERM)
        try:
            code = child.wait(timeout=35)
        except subprocess.TimeoutExpired:
            forced = True
            child.kill()
            try:
                code = child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                code = None
                wait_error = 'Exact helper did not settle after the bounded kill wait; cleanup remains unknown'
result = {'finishedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'elapsedSeconds': time.monotonic() - started, 'exitCode': code,
          'helperPID': child.pid, 'externalTimeout': timed_out, 'forcedHelperKill': forced,
          'waitError': wait_error,
          'helperSHA256Unchanged': hashlib.sha256(helper.read_bytes()).hexdigest() == invocation['helperSHA256'],
          'configSHA256Unchanged': hashlib.sha256(config.read_bytes()).hexdigest() == invocation['configSHA256'],
          'controllerAndDockerCleanupRequireIndependentReceipt': True}
(output / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result))
raise SystemExit(code if code is not None and code >= 0 else 1)
