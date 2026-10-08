// Structural/golden checks only. Go compilation and fuzz execution remain
// explicitly pending the shared compiler window; these tests never spawn Go.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { projectWorkload,redFailureMarker } from './workloads.mjs';
const knownImports=new Set(['bytes','encoding/csv','encoding/hex','encoding/json','errors','fmt','html','io','net/http','net/http/httptest','reflect','sort','strconv','strings','sync','testing','fixture.local/workload']);
for(let index=0;index<4;index++)test('workload '+index+' has a bounded offline source contract',()=>{
 const value=projectWorkload(index);assert.equal(value.redFailureMarker,redFailureMarker);assert.equal(value.files['go.mod'],'module fixture.local/workload\n\ngo 1.26.0\n');assert(value.expectedOutput.endsWith('\n'));assert(value.label);
 assert.notEqual(value.files['engine.go'],value.implementation);assert(value.files['engine_test.go'].includes('func TestApprovedRequirement(t *testing.T)'));assert(value.files['engine_test.go'].includes(redFailureMarker));assert(value.files['engine_test.go'].includes('func FuzzApprovedInvariant(f *testing.F)'));assert.match(value.files['engine_test.go'],/len\(data\)>\d+/);
 for(const [name,source] of Object.entries({...value.files,'green-engine.go':value.implementation,...(value.amendedImplementation?{'amended-engine.go':value.amendedImplementation}:{}),...(value.amendedFiles??{})})){
  assert(!name.startsWith('/')&&!name.split('/').includes('..'));assert.equal(typeof source,'string');assert(Buffer.byteLength(source)<16384);
  if(!name.endsWith('.go'))continue;
  assert(source.startsWith('package '));assert(!/time\.Sleep|os\/exec|"unsafe"|exec\.Command|os\.WriteFile|net\.Dial/.test(source));
  for(const group of source.matchAll(/\bimport\s+(?:\(([^)]*)\)|("[^"]+"))/g))for(const item of (group[1]??group[2]).matchAll(/"([^"]+)"/g))assert(knownImports.has(item[1]),'Unexpected external import '+item[1]);
 }
});
test('goldens describe distinct verified output contracts',()=>{
 const a=projectWorkload(0),b=projectWorkload(1),c=projectWorkload(2),d=projectWorkload(3);
 const escape=value=>value.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&#34;').replaceAll("'",'&#39;');
 const body='<section><h1>'+escape('Notes & plans')+'</h1><ul>'+['<ready>',"O'Reilly"].map(value=>'<li>'+escape(value)+'</li>').join('')+'</ul>';
 assert.equal(a.expectedOutput,body+'</section>\n');assert.equal(a.amendedExpectedOutput,body+'<p>Total: 2</p></section>\n');assert.notEqual(a.amendedImplementation,a.implementation);assert.notEqual(a.amendedFiles['engine_test.go'],a.files['engine_test.go']);
 assert.deepEqual(JSON.parse(b.expectedOutput),{value:'approved'});assert.equal(c.expectedOutput,'blue,3\nred,2\n');assert.equal(d.expectedOutput,'build,docs\n');assert.equal(new Set([a,b,c,d].map(value=>value.files['witness.txt'])).size,4);
});
test('workloads are returned as independent mutable objects and invalid selection refuses',()=>{
 const first=projectWorkload(0);first.files['engine.go']='changed';first.amendedFiles['engine_test.go']='changed';assert.notEqual(projectWorkload(0).files['engine.go'],'changed');assert.notEqual(projectWorkload(0).amendedFiles['engine_test.go'],'changed');
 for(const invalid of [-1,4,0.5,'0',null])assert.throws(()=>projectWorkload(invalid),RangeError);
});
