// CLOUD-062 acceptance helper. Reads real DOM state; never changes application
// state through evaluate, forced clicks, synthetic events, or store hooks.
import assert from 'node:assert/strict';
import {setTimeout as delay} from 'node:timers/promises';

export function inspectRailNavigation(){
 const hosts=[...document.querySelectorAll('.team-app-host')],host=hosts.length===1?hosts[0]:null;
 const displayed=node=>!!node&&!node.closest('[hidden],[inert]')&&node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden';
 const shell=host?.querySelector('.shell'),nav=host?.querySelector('#project-navigation'),toggle=host?.querySelector('.top-projects'),workspace=host?.querySelector('#team-workspace');
 // The admitted host is display:contents; only its actual shell has a box.
 return {hostCount:hosts.length,hostReady:!!host&&!host.closest('[hidden],[inert]')&&displayed(shell),workspaceId:workspace?.value??null,
  expanded:toggle?.getAttribute('aria-expanded')??null,controls:toggle?.getAttribute('aria-controls')??null,
  navigationReady:displayed(nav)&&nav.getBoundingClientRect().width>0,viewport:innerWidth};
}

export async function clickRailControl({page,teamId,name,health,record=()=>{},timeoutMs=10000,pollMs=50,now=Date.now,pause=delay}){
 assert(/^[A-Za-z0-9_-]{1,80}$/.test(teamId),'Exact fixture team required');
 assert(['All projects','Team access','Settings'].includes(name),'Unexpected rail destination');
 assert(Number.isFinite(timeoutMs)&&timeoutMs>0&&timeoutMs<=15000);
 const end=now()+timeoutMs;let opened=false;
 const remaining=()=>{health();const value=end-now();assert(value>0,'Exact workspace rail navigation deadline');return Math.max(1,Math.ceil(value));};
 const read=async()=>{let timer;try{return await Promise.race([page.evaluate(inspectRailNavigation),new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('Rail DOM observation deadline')),remaining());})]);}finally{clearTimeout(timer);}};
 // :checked reflects the select's CURRENT value, including state changes after
 // a preceding read. The selector cannot resolve in another workspace or in a
 // retained hidden host. Reading expanded state works while compact main is inert.
 const scope=page.locator(`.team-app-host:not([hidden]):has(#team-workspace option:checked[value="${teamId}"])`);
 for(;;){
  remaining();const state=await read();remaining();
  const ready=state.hostCount===1&&state.hostReady&&state.workspaceId===teamId;
  if(!ready){await pause(Math.min(pollMs,remaining()));continue;}
  assert.equal(state.controls,'project-navigation','Projects must control the actual rail');
  assert(['true','false'].includes(state.expanded),'Projects expansion state must be explicit');
  if(state.expanded==='false'&&!opened){
   record({kind:'rail-navigation',action:'open',name,teamId,...state});remaining();
   // A concurrent opening makes this false-state selector stop matching rather
   // than blindly toggling the already-open drawer closed.
   await scope.locator('.top-projects[aria-controls="project-navigation"][aria-expanded="false"]').click({timeout:remaining()});opened=true;remaining();continue;
  }
  if(state.expanded==='true'&&state.navigationReady){
   record({kind:'rail-navigation',action:'select',name,teamId,opened,...state});remaining();
   await scope.locator('#project-navigation:not([inert])').getByRole('button',{name,exact:true}).click({timeout:remaining()});remaining();
   return {name,teamId,opened,viewport:state.viewport};
  }
  await pause(Math.min(pollMs,remaining()));
 }
}
