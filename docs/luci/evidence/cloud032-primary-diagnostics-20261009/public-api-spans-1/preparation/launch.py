#!/usr/bin/env python3
"""Root-selected exclusive diagnostic window only. No execution without --execute."""
from pathlib import Path
import argparse,subprocess,os,json,hashlib,datetime,time,re,tempfile,stat

ROOT=Path(__file__).resolve().parent;REPO=Path('/workspace/luci');PRODUCT='2da1a821b8456c20a858fb86ef87649ea917d9de'
BASE=Path('/workspace/.luci-tools/cloud032-idle-2da1a821/invocation.json')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()

def validate_socket_templates(parent):
 """Mirror both exact frozen proxy guards; length is UTF-8 bytes, excluding NUL."""
 parent=Path(parent)
 assert parent.is_absolute() and '\0' not in str(parent),'Absolute NUL-free fixture parent required'
 # Node mkdtemp appends exactly six characters. Both call sites and the frozen
 # proxy's own 108-byte guard remain bound by the candidate source manifest.
 fixture=parent/'luci-sustained-XXXXXX'
 templates={'public':str(fixture/'docker-proxy.sock'),'private':str(fixture/'.luci-fence-proxy-XXXXXX'/'docker.sock')}
 lengths={name:len(path.encode('utf-8')) for name,path in templates.items()}
 assert all(length<108 for length in lengths.values()),'Unix socket template must fit below108 UTF-8 bytes'
 return {'templates':templates,'utf8Bytes':lengths,'exclusiveLimitBytes':108,'fixtureSuffixCharacters':6,'proxySuffixCharacters':6}

def allocate_short_fixture_parent():
 # Fresh mkdtemp directly under workspace avoids traversing or reusing any old
 # .s fixtures. Evidence remains in the long, private run directory.
 validate_socket_templates('/workspace/.lp-XXXXXXXX')
 parent=Path(tempfile.mkdtemp(prefix='.lp-',dir='/workspace'))
 info=parent.lstat()
 assert stat.S_ISDIR(info.st_mode) and not stat.S_ISLNK(info.st_mode)
 assert info.st_uid==os.geteuid() and stat.S_IMODE(info.st_mode)==0o700
 assert parent.resolve()==parent
 bound=validate_socket_templates(parent)
 return parent,{'path':str(parent),'device':info.st_dev,'inode':info.st_ino,'uid':info.st_uid,'modeOctal':oct(stat.S_IMODE(info.st_mode)),'freshMkdtemp':True,'createdAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'socketPathBounds':bound,'cleanupPolicy':'Parent retained for explicit owned-fixture closure review; never reuse as a new fixture'}

def main():
 p=argparse.ArgumentParser();p.add_argument('--helper-source',required=True);p.add_argument('--run-name',required=True);p.add_argument('--execute',action='store_true');args=p.parse_args()
 assert re.fullmatch('[a-f0-9]{40}',args.helper_source)
 assert re.fullmatch('[a-z0-9][a-z0-9-]{0,60}',args.run_name)
 if not args.execute:raise SystemExit('Prepared only. Root must select the exclusive window and pass --execute; this launcher has not run.')
 # These are launch-time checks, not permission to change any checkout.
 assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True,timeout=15).strip()==args.helper_source
 assert not subprocess.check_output(['git','status','--porcelain'],cwd=REPO,text=True,timeout=15).strip()
 candidate=ROOT/'candidate';binding=json.loads((candidate/'external-source-binding.json').read_text())
 assert binding['executionDirectory']==str(candidate)
 for name,digest in binding['localSources'].items():assert sha(candidate/name)==digest,('candidate changed',name)
 for name,digest in binding['repositoryDependencies'].items():assert sha(REPO/name)==digest,('runtime tooling changed',name)
 for name,digest in binding['frozenRuntimeDependencies'].items():assert sha(candidate/Path(name).name)==digest,('frozen runtime changed',name)
 # Exclusive fixture contract of the accepted original driver, not a cleanup.
 assert not subprocess.check_output(['docker','ps','-aq'],text=True,timeout=15).strip()
 assert not subprocess.check_output(['docker','volume','ls','-q'],text=True,timeout=15).strip()
 old=json.loads(BASE.read_text());cmd=list(old['command']);cmd[1]=str(candidate/'sustained.mjs')
 def setarg(name,value):assert cmd.count(name)==1;cmd[cmd.index(name)+1]=str(value)
 run=ROOT/'runs'/args.run_name;assert not run.exists(),'Prior attempts must remain immutable'
 setarg('--expected-source',PRODUCT);setarg('--expected-helper-source',args.helper_source);setarg('--trace-after','no')
 setarg('--report',run/'result/report.json')
 provenance=Path(cmd[cmd.index('--provenance')+1]);v=json.loads(provenance.read_text());assert v['sourceCommit']==PRODUCT and v['sourceDirty'] is False
 run.mkdir(parents=True,mode=0o700);run.chmod(0o700)
 fixture_parent,fixture_identity=allocate_short_fixture_parent();setarg('--fixture-root',fixture_parent)
 (run/'fixture-parent-identity.json').write_text(json.dumps(fixture_identity,indent=2)+'\n');(run/'fixture-parent-identity.json').chmod(0o600)
 frozen={'launcher':sha(Path(__file__)),'candidateBinding':sha(candidate/'external-source-binding.json'),'sourceInvocation':sha(BASE),'provenance':sha(provenance)}
 invocation={'version':1,'preparedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'CDP-only instrumented PRIMARY400 diagnostic, no acceptance or causal improvement claim','productSource':PRODUCT,'cleanHelperCheckout':args.helper_source,'frozenRuntimeOriginalSource':PRODUCT,'command':cmd,'fixtureParentIdentity':fixture_identity,'environmentOverrideNames':['LUCI_ACCEPTANCE_REPO'],'bindings':frozen,'wrapperDeadlineSeconds':450,'originalDriverDeadlinesUnchanged':True}
 (run/'invocation.json').write_text(json.dumps(invocation,indent=2)+'\n');(run/'invocation.json').chmod(0o600)
 t=time.monotonic();start=datetime.datetime.now(datetime.timezone.utc).isoformat()
 env=dict(os.environ,LUCI_ACCEPTANCE_REPO=str(REPO))
 for name in ['DEBUG','DEBUG_FILE','PWDEBUG','PWDEBUGIMPL']:env.pop(name,None)
 with (run/'driver.stdout.log').open('x') as out,(run/'driver.stderr.log').open('x') as err:
  os.chmod(out.name,0o600);os.chmod(err.name,0o600)
  child=subprocess.Popen(cmd,cwd=REPO,env=env,stdout=out,stderr=err)
  st=Path('/proc',str(child.pid),'stat').read_text().rsplit(') ',1)[1].split();identity={'pid':child.pid,'startTicks':int(st[19])}
  (run/'driver-identity.json').write_text(json.dumps(identity)+'\n');(run/'driver-identity.json').chmod(0o600)
  try:code=child.wait(timeout=450);timeout=False
  except subprocess.TimeoutExpired:code=None;timeout=True
 result={'startedAt':start,'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'elapsedSeconds':time.monotonic()-t,'exitCode':code,'externalMonitorTimeout':timeout,'driverIdentity':identity,'closureRequiresOriginalReceiptAndIndependentOwnedIdentityCheck':True,'automaticSignalsFromWrapper':False,'bindings':frozen}
 (run/'wrapper-result.json').write_text(json.dumps(result,indent=2)+'\n');(run/'wrapper-result.json').chmod(0o600)
 print(json.dumps(result));raise SystemExit(124 if timeout else code)

if __name__=='__main__':main()
