// Generates an explicit scripted native tool command. Import has no side effects.
import assert from 'node:assert/strict';
export function mixedTaskCommand({ task, operation, projectId, implementation, expectedOutput, authorityRoot, contextNames, durationSeconds=150 }) {
 assert(/^(build-[abcd]|amend-a)$/.test(task));assert(Number.isInteger(durationSeconds)&&durationSeconds>=0&&durationSeconds<=150);
 const config={task,operation,projectId,implementation,expectedOutput,authorityRoot,contextNames,durationSeconds};
 const script=String.raw`const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),crypto=require('node:crypto'),os=require('node:os'),assert=require('node:assert/strict');
const config=JSON.parse(Buffer.from('${Buffer.from(JSON.stringify(config)).toString('base64')}','base64'));
const hash=bytes=>crypto.createHash('sha256').update(bytes).digest('hex');
const events=[],commands=[];let active;
function stopGroup(child,signal='SIGKILL'){if(!child?.pid||child.groupSettled)return;try{process.kill(-child.pid,signal);}catch(e){if(e.code!=='ESRCH')throw e;}}
const stamp=()=>new Date().toISOString();
function event(type,details={}){const value={type,task:config.task,operation:config.operation,at:stamp(),...details};events.push(value);console.log('LUCI_MIXED_EVENT='+Buffer.from(JSON.stringify(value)).toString('base64'));}
function bytes(root){let total=0;for(const name of fs.readdirSync(root)){const file=path.join(root,name);let stat;try{stat=fs.lstatSync(file);}catch(e){if(e.code==='ENOENT')continue;throw e;}assert(!stat.isSymbolicLink());if(stat.isDirectory()){try{total+=bytes(file);}catch(e){if(e.code!=='ENOENT')throw e;}}else if(stat.isFile())total+=stat.size;else throw Error('Unexpected build cache entry');}return total;}
(async()=>{
 assert.equal(process.cwd(),'/home/dev/work');const files=Object.fromEntries(config.contextNames.concat('manifest.json').map(name=>{const b=fs.readFileSync(path.join(config.authorityRoot,name));return [name,{sha256:hash(b),bytes:b.length}];}));
 const brief=fs.readFileSync(path.join(config.authorityRoot,'LUCI.md'),'utf8');const selected=JSON.parse(brief.slice(brief.indexOf('## Assigned task contract')).match(/\x60\x60\x60json\n([\s\S]*?)\n\x60\x60\x60/)[1]);assert.equal(selected.selectedTasks.length,1);assert.equal(selected.selectedTasks[0].id,config.task);assert.deepEqual([...selected.selectedTasks[0].writePaths].sort(),['engine.go',config.task+'-result.json'].sort());assert.equal(JSON.parse(fs.readFileSync(path.join(config.authorityRoot,'.luci/plan.json'))).projectId,config.projectId);
 let readOnly=false;try{fs.writeFileSync(path.join(config.authorityRoot,'.write-refused'),'no',{flag:'wx'});}catch(e){assert.equal(e.code,'EROFS');readOnly=true;}assert(readOnly);
 const git=(...args)=>cp.execFileSync('git',args,{encoding:'utf8',timeout:15000}).trim();assert.equal(git('status','--porcelain'),'');const base=git('rev-parse','HEAD');
 const scratch=fs.mkdtempSync(path.join(os.tmpdir(),'luci-mixed-'));for(const dir of ['cache','tmp','corpus'])fs.mkdirSync(path.join(scratch,dir));
 const env={...process.env,GOWORK:'off',GOTOOLCHAIN:'local',GOPROXY:'off',GOSUMDB:'off',GOMAXPROCS:'1',GOFLAGS:'-p=1 -trimpath',GOCACHE:path.join(scratch,'cache'),GOTMPDIR:path.join(scratch,'tmp')};
 let cachePeak=0,corpusPeak=0,succeeded=false;let monitorFailure;
 const overall=setTimeout(()=>{monitorFailure=Error('Actual task command budget exceeded');stopGroup(active);},config.durationSeconds?540000:90000);
 const monitor=setInterval(()=>{try{cachePeak=Math.max(cachePeak,bytes(scratch));corpusPeak=Math.max(corpusPeak,bytes(path.join(scratch,'corpus')));if(corpusPeak>32*1024**2)throw Error('Owned fuzz corpus exceeds32MiB');if(cachePeak>512*1024**2)throw Error('Owned build scratch exceeds512MiB');}catch(e){monitorFailure=e;stopGroup(active);}},1000);
 async function command(label,executable,args,expectedCode=0,timeoutMs=300000){
  if(monitorFailure)throw monitorFailure;const began=process.hrtime.bigint(),start=stamp();let output='',totalBytes=0;event('command-start',{label,executable,args});
  const child=cp.spawn(executable,args,{env,detached:true,stdio:['ignore','pipe','pipe']});active=child;child.once('close',()=>{child.groupSettled=true;});
  const timer=setTimeout(()=>{monitorFailure=Error('Real command deadline exceeded: '+label);stopGroup(child);},timeoutMs);
  const collect=chunk=>{totalBytes+=chunk.length;if(totalBytes>1024**2){monitorFailure=Error('Actual command output exceeded1MiB');stopGroup(child);return;}output+=chunk.toString();if(label==='fuzz')process.stdout.write(chunk);};child.stdout.on('data',collect);child.stderr.on('data',collect);
  const code=await new Promise((done,reject)=>{child.once('error',reject);child.once('close',(code,signal)=>done({code,signal}));}).finally(()=>clearTimeout(timer));active=null;fs.writeFileSync(path.join(scratch,label+'.log'),output);if(monitorFailure)throw monitorFailure;
  const elapsedMs=Number(process.hrtime.bigint()-began)/1e6;const row={label,executable,args,startedAt:start,finishedAt:stamp(),elapsedMs,...code,outputBytes:totalBytes,outputSHA256:hash(output)};commands.push(row);event('command-end',row);
  assert.equal(code.signal,null);assert.equal(code.code,expectedCode,'Unexpected command exit: '+label+' '+output.slice(-3000));return {output,row};
 }
 try{
  const version=await command('toolchain','go',['version']);
  const red=await command('expected-red','go',['test','-run','^TestApprovedRequirement$','-count=1','./...'],1);assert(red.output.includes('APPROVED_REQUIREMENT_UNIMPLEMENTED'));assert(red.output.includes('--- FAIL: TestApprovedRequirement'));assert(!/build failed|undefined:|syntax error|cannot find|no required module/i.test(red.output));
  fs.writeFileSync('engine.go',config.implementation);await command('build','go',['build','-trimpath','-buildvcs=false','-o',path.join(scratch,'app'),'./cmd/app']);
  const fixed=await command('fixed-tests','go',['test','-json','-count=1','./...']);const testRows=fixed.output.split('\n').filter(Boolean).map(line=>JSON.parse(line));assert(!testRows.some(row=>row.Action==='fail'));fixed.row.testsPassed=testRows.filter(row=>row.Action==='pass'&&row.Test).map(row=>row.Test);assert(fixed.row.testsPassed.includes('TestApprovedRequirement'));assert(fixed.row.testsPassed.length>=3);const result=await command('artifact-behavior',path.join(scratch,'app'),[]);assert.equal(result.output,config.expectedOutput);
  if(config.durationSeconds){
   const fuzz=await command('fuzz','go',['test','-run','^$','-fuzz','^FuzzApprovedInvariant$','-fuzztime='+config.durationSeconds+'s','-parallel=1','-test.fuzzcachedir='+path.join(scratch,'corpus')],0,(config.durationSeconds+180)*1000);
   assert(fuzz.row.elapsedMs>=config.durationSeconds*1000);assert(/PASS/.test(fuzz.output));const progress=[...fuzz.output.matchAll(/fuzz: elapsed: (\d+)s, execs: (\d+)/g)].map(value=>({seconds:Number(value[1]),executions:Number(value[2])}));assert(progress.length);fuzz.row.activeFuzzSeconds=Math.max(...progress.map(value=>value.seconds));fuzz.row.fuzzExecutions=Math.max(...progress.map(value=>value.executions));assert(fuzz.row.activeFuzzSeconds>=config.durationSeconds);assert(fuzz.row.fuzzExecutions>0);
  }
  // Bounded diagnostic stress is separate from useful build/test duration.
  for(let line=0;line<64;line++)console.log('MIXED_DIAGNOSTIC '+config.task+' '+String(line).padStart(4,'0')+' '+'.'.repeat(224));
  const resultFile=config.task+'-result.json',artifact={task:config.task,operation:config.operation,projectId:config.projectId,implementationSHA256:hash(config.implementation),expectedOutputSHA256:hash(config.expectedOutput),binarySHA256:hash(fs.readFileSync(path.join(scratch,'app'))),commands,cachePeakBytes:cachePeak,corpusPeakBytes:corpusPeak,toolchain:version.output.trim()};
  fs.writeFileSync(resultFile,JSON.stringify(artifact,null,2)+'\n',{flag:'wx'});git('add','--','engine.go',resultFile);git('-c','user.name=Luci mixed fixture','-c','user.email=mixed@example.invalid','commit','-m','Controlled mixed workload '+config.task);assert.equal(git('status','--porcelain'),'');
  const observed={task:config.task,operation:config.operation,uid:process.getuid(),cwd:process.cwd(),files,readOnly,base,commit:git('rev-parse','HEAD'),changed:git('diff','--name-only',base,'HEAD').split('\n'),outputSHA256:hash(fs.readFileSync(resultFile)),artifact,events};
  console.log('LUCI_MIXED_OBSERVATION='+Buffer.from(JSON.stringify(observed)).toString('base64'));succeeded=true;
 }finally{clearTimeout(overall);clearInterval(monitor);if(active){stopGroup(active);await new Promise(done=>{if(active.exitCode!==null||active.signalCode!==null)return done();const timer=setTimeout(()=>{console.error('Owned command group did not close within5seconds');done();},5000);active.once('close',()=>{clearTimeout(timer);done();});});}if(!succeeded){const retained={task:config.task,commands,events,files:{}};let budget=40*1024**2;function capture(root,prefix){if(!fs.existsSync(root))return;for(const name of fs.readdirSync(root)){const file=path.join(root,name),stat=fs.lstatSync(file);if(stat.isSymbolicLink())continue;if(stat.isDirectory())capture(file,prefix+name+'/');else if(stat.isFile()){if(stat.size>budget)throw Error('Failure diagnostic bound exceeded');budget-=stat.size;retained.files[prefix+name]=fs.readFileSync(file).toString('base64');}}}capture(path.join(scratch,'corpus'),'corpus/');capture(path.join(process.cwd(),'testdata/fuzz'),'testdata/fuzz/');for(const name of fs.readdirSync(scratch).filter(n=>n.endsWith('.log'))){const b=fs.readFileSync(path.join(scratch,name));assert(b.length<=budget);budget-=b.length;retained.files[name]=b.toString('base64');}fs.writeFileSync('.luci-mixed-failure-'+config.task+'.json',JSON.stringify(retained)+'\n',{flag:'wx'});}if(succeeded)fs.rmSync(scratch,{recursive:true});else console.error('LUCI_MIXED_FAILURE_SCRATCH='+scratch);}
})().catch(e=>{console.error(e);process.exitCode=1;});`;
 return {cmd:'node -e "eval(Buffer.from(\''+Buffer.from(script).toString('base64')+'\',\'base64\').toString())"',workdir:'/home/dev/work',login:false,yield_time_ms:1000,max_output_tokens:16000};
}
