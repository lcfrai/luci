"""Prepared wrapper; execution requires a separately reviewed immutable config.
No service, Docker command or scenario is started by preparation.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

os.umask(0o077)
root=Path(__file__).resolve().parent
config=Path(sys.argv[1]).resolve(strict=True)
output=Path(sys.argv[2])
assert output.is_absolute() and output.parent.resolve()==root
assert not output.exists()
helper=root/'cancel-preview.mjs'
binding=json.loads((root/'prepared-manifest.json').read_text())
digest=lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
for relative,expected in binding['files'].items():
    assert digest(root/relative)==expected, 'Prepared helper or fixture changed'
assert config.parent==root and config.name.startswith('run-config-') and config.name.endswith('.json')
settings=json.loads(config.read_text())
assert settings['acknowledgeFixtureMutation'] is True
assert settings['acknowledgeWarmBaseOnly'] is True
assert settings['acknowledgeUnknownLeaseRetention'] is True
output.mkdir(mode=0o700)
command=['node',str(helper),str(config)]
started=time.monotonic()
invocation={'command':command,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'helperSHA256':digest(helper),'configSHA256':digest(config),'preparedManifestSHA256':digest(root/'prepared-manifest.json'),'scope':'One installed HTTP live preview cancellation; wrapper success is not Docker settlement or capacity release'}
(output/'invocation.json').write_text(json.dumps(invocation,indent=2)+'\n')
timed_out=False;forced=False
with (output/'stdout.log').open('x') as stdout,(output/'stderr.private.log').open('x') as stderr:
    child=subprocess.Popen(command,cwd=root,stdout=stdout,stderr=stderr)
    try:code=child.wait(timeout=495)
    except subprocess.TimeoutExpired:
        timed_out=True;child.send_signal(signal.SIGTERM)
        try:code=child.wait(timeout=15)
        except subprocess.TimeoutExpired:
            forced=True;child.kill()
            try:code=child.wait(timeout=10)
            except subprocess.TimeoutExpired:code=None
receipt={'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'elapsedSeconds':time.monotonic()-started,'exitCode':code,'helperPID':child.pid,'externalTimeout':timed_out,'forcedHelperKill':forced,'helperSHA256Unchanged':digest(helper)==invocation['helperSHA256'],'configSHA256Unchanged':digest(config)==invocation['configSHA256'],'allPreparedFilesUnchanged':all(digest(root/relative)==expected for relative,expected in binding['files'].items()),'independentClosureStillRequired':True,'unknownLeaseMustRemainConservative':True}
(output/'result.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt))
raise SystemExit(code if code is not None and code>=0 and receipt['allPreparedFilesUnchanged'] else 1)
