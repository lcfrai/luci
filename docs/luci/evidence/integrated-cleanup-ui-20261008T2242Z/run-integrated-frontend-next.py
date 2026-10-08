from pathlib import Path
import datetime, hashlib, json, os, subprocess, time
r=Path("/workspace/luci"); out=r/"artifacts/quality-swarm/integrated-cleanup-ui-20261008T2242Z"
out.mkdir(parents=True,exist_ok=False)
def manifest():
 paths=[]
 for root in [r/"desktop/src",r/"desktop/e2e",r/"desktop/scripts"]:
  paths.extend(p for p in root.rglob("*") if p.is_file())
 paths.extend(p for p in (r/"desktop").iterdir() if p.is_file() and p.suffix in [".ts",".json",".js",".mjs",".html"])
 return {str(p.relative_to(r)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(set(paths))}
before=manifest();(out/"source-before.json").write_text(json.dumps(before,indent=2)+"\n")
results=[]
for name,cmd in [("full-frontend",["npm","test","--","--maxWorkers=1","--no-file-parallelism","--reporter=json","--outputFile="+str(out/"vitest.json")]),("typecheck",["npm","run","typecheck"])]:
 start=datetime.datetime.now(datetime.timezone.utc).isoformat();t=time.monotonic()
 with (out/(name+".stdout.log")).open("w") as stdout,(out/(name+".stderr.log")).open("w") as stderr:
  try:
   result=subprocess.run(cmd,cwd=r/"desktop",stdout=stdout,stderr=stderr,timeout=900)
   code=result.returncode;error=None
  except subprocess.TimeoutExpired as e: code=None;error=str(e)
 row={"name":name,"command":cmd,"startedAt":start,"elapsedSeconds":time.monotonic()-t,"exitCode":code,"error":error};results.append(row)
 (out/"commands.json").write_text(json.dumps(results,indent=2)+"\n"); print(json.dumps(row),flush=True)
after=manifest();(out/"source-after.json").write_text(json.dumps(after,indent=2)+"\n")
report={"baseCommit":subprocess.check_output(["git","rev-parse","HEAD"],cwd=r,text=True).strip(),"inputs":len(before),"unchanged":before==after,"commands":results}
if (out/"vitest.json").exists():
 raw=json.loads((out/"vitest.json").read_text()); report["vitest"]={k:raw.get(k) for k in ["numTotalTests","numPassedTests","numFailedTests","numPendingTests","numTotalTestSuites","success"]}
 report["failedTests"]=[{"file":x["name"],"title":a.get("fullName"),"failures":a.get("failureMessages")} for x in raw["testResults"] for a in x.get("assertionResults",[]) if a.get("status")=="failed"]
(out/"result.json").write_text(json.dumps(report,indent=2)+"\n");print(json.dumps(report),flush=True)
