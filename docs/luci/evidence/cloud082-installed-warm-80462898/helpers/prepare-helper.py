from pathlib import Path
root=Path(__file__).resolve().parent
p=(root/'preview-smoke-original.mjs').read_text()
def replace(old,new):
 global p
 assert old in p, old[:120]
 p=p.replace(old,new,1)
replace("'evidenceParent','go','provenanceFile'", "'evidenceParent','provenanceFile'")
replace("assert.ok(cfg.mode==='scratch'||cfg.mode==='warm','mode: scratch or warm');\nif(cfg.mode==='warm') assert.match(cfg.baseImageID,/^sha256:[a-f0-9]{64}$/);", "assert.equal(cfg.mode,'warm-multistage');\nassert.match(cfg.baseImageID,/^sha256:[a-f0-9]{64}$/);\nassert.equal(cfg.acknowledgeWarmBaseOnly,true,'No image pull/tag is authorized by this fixture');")
replace("scope:'hand-written fixture preview transport only; no live model, project check pass, installer or full product acceptance'", "scope:'handwritten Node warm-base multistage RUN test/build and preview; no live model, cold pull, project check pass, installer or full product acceptance'")
replace("let requestNumber=0,logsBytes=0,logsTruncated=false,processWatch,processScanError;", "let requestNumber=0,logsBytes=0,logsTruncated=false,processWatch,processScanError;\nlet buildWatch,buildWatchReads=0,buildWatchError; const buildSnapshots=[]; let buildSnapshotHash;\nconst fixtureNames=['calculator.mjs','calculator.test.mjs','server.mjs','build.mjs'];\nconst fixtureDir=join(helperDir,'fixture');")
replace("async function api(method,path,body,", """function sampleBuildLedger() {
 try {
  assert.ok(++buildWatchReads<=4200,'Build lease observation exceeds declared bound');
  if(!existsSync(ledgerFile))return;
  const stat=lstatSync(ledgerFile);assert.ok(stat.isFile()&&!stat.isSymbolicLink()&&stat.size<=2<<20);const raw=readFileSync(ledgerFile);assert.ok(raw.length<=2<<20);const value=JSON.parse(raw),own=(value.previews??[]).filter(x=>x.identity?.projectId===project?.id);
  const digest=createHash('sha256').update(JSON.stringify(own)).digest('hex');
  if(digest!==buildSnapshotHash){assert.ok(buildSnapshots.length<64,'Build lease transition receipt bound exceeded');buildSnapshotHash=digest;buildSnapshots.push({at:new Date().toISOString(),ownPreviews:own,rawSHA256:createHash('sha256').update(raw).digest('hex')});write('build-lease-observations.json',{scope:'Read-only 50ms samples, not exhaustive transition capture',reads:buildWatchReads,snapshots:buildSnapshots,error:null});}
 }catch(error){buildWatchError=error.message;if(buildWatch)clearInterval(buildWatch);}
}
function assertBuildProof(proof,manifest) {
 assert.equal(proof.schema,1);assert.equal(proof.nonce,manifest.nonce);assert.deepEqual(proof.sources,manifest.sources);
 assert.equal(proof.platform,'linux');assert.match(proof.node,/^v22\\./);assert.equal(proof.uid,0);
 assert.equal(proof.testProcessExit,0);assert.match(proof.tap,/\\n# tests 10\\n/);assert.match(proof.tap,/\\n# pass 10\\n/);assert.match(proof.tap,/\\n# fail 0\\n/);assert.match(proof.tap,/\\n# skipped 0\\n/);assert.match(proof.tap,/\\n# cancelled 0\\n/);assert.doesNotMatch(proof.tap,/^not ok /m);
 assert.equal(createHash('sha256').update(proof.tap).digest('hex'),proof.tapSHA256);
 const text=p=>proof.cgroup[p]?.status==='read'?proof.cgroup[p].text.trim():null;
 const memory=text('/sys/fs/cgroup/memory.max')??text('/sys/fs/cgroup/memory/memory.limit_in_bytes');
 const cpu=text('/sys/fs/cgroup/cpu.max');
 const [quota,period]=cpu?cpu.split(/\\s+/):[text('/sys/fs/cgroup/cpu/cpu.cfs_quota_us'),text('/sys/fs/cgroup/cpu/cpu.cfs_period_us')];
 assert.equal(memory,'1073741824','Actual RUN memory cap unavailable or differs from admitted 1 GiB');
 assert.equal(quota,'100000','Actual RUN CPU quota unavailable or differs from one CPU');assert.equal(period,'100000');
 const swap=text('/sys/fs/cgroup/memory.swap.max');
 if(swap!==null)assert.equal(swap,'0','Actual RUN swap must be disabled');
 else assert.equal(text('/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes'),'1073741824','Actual RUN memory+swap cap unavailable');
 return {memoryBytes:Number(memory),cpuQuotaMicros:Number(quota),cpuPeriodMicros:Number(period),swapEvidence:swap!==null?'cgroup-v2-zero-swap':'cgroup-v1-memory-plus-swap-equals-memory',scope:'Actual RUN child namespace cgroup files, not host capacity or whole-process resource sampling'};
}
async function api(method,path,body,""")
replace("'control-plane/internal/backend/dockerbe/preview_context.go',", "'control-plane/internal/backend/dockerbe/preview_context.go','control-plane/internal/backend/dockerbe/preview_bases.go',")
replace("result.helperSHA256=hash(fileURLToPath(import.meta.url));result.fixtureSourceSHA256=hash(join(helperDir,'fixture.go'));", "result.helperSHA256=hash(fileURLToPath(import.meta.url));const fixtureSources=Object.fromEntries(fixtureNames.map(name=>[name,hash(join(fixtureDir,name))]));result.fixtureSources=fixtureSources;result.fixtureDockerfileTemplateSHA256=hash(join(fixtureDir,'Dockerfile.template'));write('fixture-sources.json',fixtureSources);")
start=p.index(" if(cfg.mode==='warm') {const [base]")
end=p.index(" const apiPort=await availablePort();",start)
p=p[:start]+""" const [base]=dockerJSON(['image','inspect',cfg.baseImageID]);assert.equal(base.Id,cfg.baseImageID);assert.equal(base.Os,'linux');assert.equal(base.Architecture,arch);assert.ok(base.Size>0);assert.equal((base.Config.OnBuild??[]).length,0);assert.equal(Object.keys(base.Config.Volumes??{}).length,0);write('base-image.json',base);result.baseImageID=base.Id;result.minimumBuildDiskBytes=3*2**30+4*base.Size;
 const manifest={nonce:randomUUID(),sources:fixtureSources};write('expected-build-manifest.json',manifest);
 const fixtureBytes=fixtureNames.reduce((total,name)=>total+statSync(join(fixtureDir,name)).size,0);
"""+p[end:]
replace("+4*statSync(app).size", "+4*fixtureBytes")
replace("LUCI_SANDBOX_IMAGE:cfg.mode==='warm'?cfg.baseImageID:'luci/preview-fixture-no-worker:absent'", "LUCI_SANDBOX_IMAGE:'luci/preview-fixture-no-worker:absent'")
start=p.index(" copyFileSync(source,join(project.path,'fixture.go'));")
end=p.index(" const gitBefore=await api",start)
p=p[:start]+""" for(const name of fixtureNames)copyFileSync(join(fixtureDir,name),join(project.path,name));
 writeFileSync(join(project.path,'fixture-manifest.json'),JSON.stringify(manifest)+'\\n',{mode:0o600});
 const dockerfile=readFileSync(join(fixtureDir,'Dockerfile.template'),'utf8').replaceAll('__BASE_IMAGE_ID__',cfg.baseImageID);
 assert.equal((dockerfile.match(new RegExp(cfg.baseImageID,'g'))??[]).length,2);writeFileSync(join(project.path,'Dockerfile'),dockerfile,{mode:0o600});
 assert.equal(existsSync(join(project.path,'dist')),false);assert.equal(existsSync(join(project.path,'test-output.tap')),false);write('reviewed-Dockerfile',dockerfile);
"""+p[end:]
replace("message:'Reviewed preview transport fixture'", "message:'Reviewed warm Node calculator and multistage test/build fixture'")
replace("attemptedRelease=true;\n const created=await api", "attemptedRelease=true;sampleBuildLedger();buildWatch=setInterval(sampleBuildLedger,50);buildWatch.unref();\n const created=await api")
replace("if(created.id){release=created;result.releaseId=created.id;}", "if(buildWatch)clearInterval(buildWatch);sampleBuildLedger();write('build-lease-observations.json',{scope:'Read-only 50ms samples, not exhaustive transition capture',reads:buildWatchReads,snapshots:buildSnapshots,error:buildWatchError??null});\n if(created.id){release=created;result.releaseId=created.id;}")
replace(" const url=new URL(r.runtime.url);", """ assert.equal(buildWatchError,undefined,'Build lease observation failed');
 const buildRows=buildSnapshots.flatMap(x=>x.ownPreviews).filter(x=>x.kind==='preview-build');
 assert.ok(buildRows.some(x=>x.id===r.runtime.buildAllocationId&&x.state!=='finished'),'No charged build reservation was actually observed');
 assert.deepEqual([...new Set(buildRows.map(x=>x.id))],[r.runtime.buildAllocationId]);
 for(const row of buildRows){assert.equal(row.identity.commit,commit);assert.equal(row.identity.sourceDigest,r.runtime.sourceDigest);assert.equal(row.identity.projectId,project.id);assert.equal(row.identity.releaseId,release.id);assert.equal(row.resources.MemoryMiB??row.resources.memoryMiB,1024);assert.equal(row.resources.CPUMillis??row.resources.cpuMillis,1000);}
 // Finished rows remain as lineage, but at most one active allocation may be
 // charged during build-to-serve transition in any observed snapshot.
 for(const snap of buildSnapshots)assert.ok(snap.ownPreviews.filter(x=>x.state!=='finished').length<=1,'Observed overlapping charged preview allocations');
 const url=new URL(r.runtime.url);""")
replace("record('actual image build, limited serving allocation and useful HTTP response verified'", """const proofResponse=await fetch(new URL('/build-proof',url),{redirect:'error',signal:AbortSignal.timeout(5_000)});assert.equal(proofResponse.status,200);const proofText=await proofResponse.text();assert.ok(Buffer.byteLength(proofText)<256*1024);const proof=JSON.parse(proofText);write('http-build-proof.json',proof);result.actualBuildCaps=assertBuildProof(proof,manifest);write('actual-build-caps.json',result.actualBuildCaps);
 assert.equal(proof.architecture,arch==='amd64'?'x64':'arm64');
 const afterBase=dockerJSON(['image','inspect',cfg.baseImageID])[0];assert.equal(afterBase.Id,base.Id);assert.deepEqual(afterBase.RepoTags,base.RepoTags);assert.deepEqual(afterBase.RepoDigests,base.RepoDigests);assert.equal(afterBase.Size,base.Size);write('base-image-after.json',afterBase);
 record('actual Node tests and multistage RUN build, observed build caps, limited serving allocation and useful HTTP response verified'""")
replace("finally {\n //", "finally {\n if(buildWatch)clearInterval(buildWatch);\n write('build-lease-observations.json',{scope:'Read-only 50ms samples, not exhaustive transition capture',reads:buildWatchReads,snapshots:buildSnapshots,error:buildWatchError??null});\n //")
(root/'preview-smoke.mjs').write_text(p)
