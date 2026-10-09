import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {navigationContextWitness} from './navigation-verifier.mjs';
export const originalSource=readFileSync(new URL('./original-callback.js',import.meta.url),'utf8');
export function fixture(which='fixed',{connected=true,value='saved-context',expected='saved-context',markThrows=false}={}){
 let tick=0;const raf=[],events=[];const state={connected,value};
 const el={get isConnected(){events.push('connected');return state.connected},get value(){events.push('value');return state.value}};
 const context=vm.createContext({performance:{timeOrigin:1234567,now:()=>tick},requestAnimationFrame:cb=>{events.push('raf');raf.push(cb)},window:{__luciAcceptanceLatency:{markExpectedDOM:kind=>{events.push('mark:'+kind);if(markThrows)throw Error('marker failed')}}}});
 const fn=vm.runInContext(which==='original'?'('+originalSource+')':'('+navigationContextWitness.toString()+')',context);
 const promise=fn(el,expected);
 return {promise,events,state,queued:()=>raf.length,step(time){tick=time;const cb=raf.shift();if(!cb)throw Error('No requested frame');cb()}};
}
