// A one-shot, identity-scoped UI draft transfer, not a search/model request.
// Caller publishes only after ordinary guarded navigation has committed.
export const RESEARCH_QUESTION = 'luci:research-question';
let pending:{scope:string;projectId:string;query:string;token:symbol}|null=null;
export function requestResearchQuestion(scope:string,projectId:string,query:string){
  if(!scope||!projectId||!query.trim()||new TextEncoder().encode(query).length>300)return ()=>{};
  const token=Symbol();pending={scope,projectId,query,token};window.dispatchEvent(new Event(RESEARCH_QUESTION));
  return ()=>{if(pending?.token===token)pending=null;};
}
export function takeResearchQuestion(scope:string,projectId:string){if(pending?.scope!==scope||pending.projectId!==projectId)return null;const query=pending.query;pending=null;return query;}
