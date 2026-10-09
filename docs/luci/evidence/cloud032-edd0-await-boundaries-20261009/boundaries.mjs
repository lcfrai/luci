// Offline projection only. No browser/driver actions or private context fields.
export const SWITCH_STAGES=Object.freeze(['action-start','all-projects-click-end','overview-heading-ready','open-project-click-start','open-project-click-end','action-end','verify-start','idea-field-wait-end','idea-context-evaluate-end','verify-end','frames-start','frames-end']);
const OTHER_STAGES=Object.freeze(['action-start','action-end','verify-start','verify-end','frames-start','frames-end']);
export function projectBoundaryRows(interactions) {
 if(!Array.isArray(interactions)||interactions.length>6000)throw Error('Interaction bound exceeded');
 return interactions.map((row,index)=>{
  const expected=row.kind==='project-switch'?SWITCH_STAGES:OTHER_STAGES,raw=row.diagnostics?.nodeStages,issues=[];
  if(!['project-switch','input','draft-reset'].includes(row.kind))issues.push('unknown-kind');
  if(!Array.isArray(raw)||raw.length>12)throw Error('Node stage bound invalid');
  const validNumber=n=>typeof n==='number'&&Number.isFinite(n)&&n>=0;
  const stages=raw.map((v,i)=>{if(v.stage!==expected[i])issues.push('stage-order-or-label');if(!validNumber(v.offsetMs)||(i&&v.offsetMs<raw[i-1].offsetMs))issues.push('stage-clock-invalid');return {stage:expected.includes(v.stage)?v.stage:'unknown',offsetMs:validNumber(v.offsetMs)?v.offsetMs:null};});
  if(row.diagnostics?.nodeClockReads!==raw.length)issues.push('clock-count-mismatch');
  if(row.success===true&&raw.length!==expected.length)issues.push('incomplete-success');
  if(!validNumber(row.elapsedMs)||raw.some(v=>validNumber(v.offsetMs)&&v.offsetMs>row.elapsedMs))issues.push('full-span-invalid');
  const by=Object.fromEntries(stages.map(v=>[v.stage,v.offsetMs])),segments={};
  if(!issues.length)for(const [name,a,b] of [['openCommand','open-project-click-start','open-project-click-end'],['fieldWait','verify-start','idea-field-wait-end'],['contextEvaluate','idea-field-wait-end','idea-context-evaluate-end'],['inputValueAndAssert','idea-context-evaluate-end','verify-end']])if(validNumber(by[a])&&validNumber(by[b]))segments[name]=by[b]-by[a];
  return {index,kind:['project-switch','input','draft-reset'].includes(row.kind)?row.kind:'unknown',success:row.success===true,elapsedMs:validNumber(row.elapsedMs)?row.elapsedMs:null,nodeClockReads:validNumber(row.diagnostics?.nodeClockReads)?row.diagnostics.nodeClockReads:null,nodeStages:stages,segments,issues:[...new Set(issues)]};
 });
}
