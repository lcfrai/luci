// Exact stopControllerTree reuse from installed-recovery.mjs; source digest in reused-inputs.json.
import assert from 'node:assert/strict';
import {setTimeout as delay} from 'node:timers/promises';
export async function stopControllerTree({identity,readIdentity,listOwned,signal,wait=delay,now=Date.now,onForce=()=>{},graceMs=20000,forceMs=5000}){
  const matches=id=>{const current=readIdentity(id.pid);return !!current&&current.start===id.start&&current.status!=='Z';};
  const send=(id,pid,kind)=>{if(!matches(id))return;try{signal(pid,kind);}catch(error){if(error.code!=='ESRCH'||matches(id))throw error;}};
  if(identity)send(identity,identity.pid,'SIGTERM');
  // An environment scan may omit an unreadable process. The controller's
  // captured identity must independently reach absence, exit or replacement.
  const observedOwned=()=>{const rows=[...listOwned()];if(identity&&matches(identity)&&!rows.some(row=>row.pid===identity.pid&&row.start===identity.start))rows.push(identity);return rows;};
  const end=now()+graceMs;while(now()<end&&observedOwned().length)await wait(100);
  const left=observedOwned();
  if(left.length){onForce();for(const id of left)send(id,id.group===id.pid?-id.pid:id.pid,'SIGKILL');}
  const final=now()+forceMs;while(now()<final&&observedOwned().length)await wait(100);
  assert.equal(observedOwned().length,0,'Owned controller cleanup did not settle');
  assert.equal(left.length,0,'Owned controller required forced shutdown');return true;
}
