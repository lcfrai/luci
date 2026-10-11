#!/usr/bin/env python3
from pathlib import Path
import datetime,hashlib,json,os,signal,subprocess,sys,time
B=Path(__file__).resolve().parent;label=sys.argv[1]
commands={
 'windows-production':['/workspace/.luci-tools/rust-1.99.0/bin/cargo','check','--lib','--offline','--locked','--target','x86_64-pc-windows-msvc','--manifest-path',str(B/'compile/Cargo.toml')],
 'windows-tests':['/workspace/.luci-tools/rust-1.99.0/bin/cargo','check','--tests','--offline','--locked','--target','x86_64-pc-windows-msvc','--manifest-path',str(B/'compile/Cargo.toml')],
 'quote-compile':['/workspace/.luci-tools/rust-1.99.0/bin/rustc','--edition=2021','--test',str(B/'compile/quoting-tests.rs'),'-o',str(B/'compile/quoting-tests')],
 'quote-test':[str(B/'compile/quoting-tests'),'--test-threads=1'],
}
assert label in commands
out=B/'validation'/label;out.mkdir(parents=True,exist_ok=False)
sha=lambda raw:hashlib.sha256(raw).hexdigest()
paths=list((B/'fixed/desktop/src-tauri').glob('Cargo.*'))+[B/'fixed/desktop/src-tauri/src/lib.rs',B/'fixed/desktop/src-tauri/src/windows_sidecar.rs',B/'fixed/desktop/src-tauri/src/windows_command_line.rs',B/'compile/Cargo.toml',B/'compile/Cargo.lock',B/'compile/lib.rs',B/'compile/quoting-tests.rs']
if label=='windows-tests':paths.append(B/'fixed/desktop/src-tauri/src/windows_sidecar_tests.rs')
def snapshot():return {str(p.relative_to(B)):{'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())} for p in paths}
inputs=snapshot();(out/'inputs-before.json').write_text(json.dumps(inputs,indent=2)+'\n')
env={k:os.environ[k] for k in ['PATH','HOME','LANG','LC_ALL','TZ','TMPDIR','TEMP','TMP'] if k in os.environ};env.update(CARGO_HOME='/workspace/.luci-tools/rust-cargo',RUSTC='/workspace/.luci-tools/rust-1.99.0/bin/rustc',CARGO_TARGET_DIR=str(B/'target'),CARGO_BUILD_JOBS='1')
if label.startswith('windows-'):env['RUSTFLAGS']='--sysroot='+str(B/'windows-sysroot')
row={'label':label,'command':commands[label],'cwd':str(B),'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'timeoutSeconds':180,'scope':'Focused module/controlled quoting only; no Tauri app, browser, Docker or Windows runtime execution.','explicitEnvironment':{k:env[k] for k in ['CARGO_HOME','RUSTC','CARGO_TARGET_DIR','CARGO_BUILD_JOBS']},'rustc':{'path':env['RUSTC'],'sha256':sha(Path(env['RUSTC']).read_bytes())}}
(out/'invocation.json').write_text(json.dumps(row,indent=2)+'\n');start=time.monotonic()
with (out/'stdout.log').open('xb') as so,(out/'stderr.log').open('xb') as se:
 p=subprocess.Popen(commands[label],cwd=B,env=env,stdout=so,stderr=se,start_new_session=True)
 try:p.wait(timeout=180)
 except subprocess.TimeoutExpired:
  row['timedOut']=True;os.killpg(p.pid,signal.SIGTERM)
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait(timeout=10)
after=snapshot();(out/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n');row.update(exitCode=p.returncode,elapsedSeconds=time.monotonic()-start,completedAt=datetime.datetime.now(datetime.timezone.utc).isoformat(),inputsStable=inputs==after)
row['logs']={name:{'bytes':(out/name).stat().st_size,'sha256':sha((out/name).read_bytes())} for name in ['stdout.log','stderr.log']};(out/'result.json').write_text(json.dumps(row,indent=2)+'\n');print(json.dumps(row,indent=2));print((out/'stdout.log').read_text()[-8000:]);print((out/'stderr.log').read_text()[-8000:]);sys.exit(p.returncode if inputs==after else 2)
