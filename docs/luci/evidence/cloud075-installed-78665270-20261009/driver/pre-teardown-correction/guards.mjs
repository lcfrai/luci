import assert from 'node:assert/strict';
export const budgets={workflowMs:180000,cleanupMs:45000,httpMs:10000,gateMs:15000,bodyBytes:2*1024**2,maxRequests:2048,maxJournalBytes:4*1024**2,maxEventBytes:8192};
export const emptyGrants={execute:false,secrets:false,deploy:false,admin:false};
export function validateProduct(value){assert.match(value??'',/^[a-f0-9]{40}$/,'An immutable final product revision is required');return value;}
export function browserRequestAllowed({method,url,origin,teamId}) {
 const u=new URL(url);if(u.origin!==origin||u.username||u.password||u.hash)return false;
 if(['GET','HEAD','OPTIONS'].includes(method))return true;
 if(method==='POST'&&u.pathname==='/v1/auth/login'&&!u.search)return true;
 // These are normal read-only readiness probes despite their POST transport.
 // Runtime installation and every agent/session/check/task mutation remain refused.
 return method==='POST'&&!u.search&&['/v1/setup/check','/v1/setup/dependencies/check',...(teamId?[`/v1/teams/${teamId}/workspace/v1/setup/check`]:[])].includes(u.pathname);
}
export function operatorRequestAllowed({method,path,teamId,projectIds=[],memberId}){
 const u=new URL(path,'http://fixture.invalid');if(u.origin!=='http://fixture.invalid'||u.hash||u.search||u.pathname!==path)return false;
 if(method==='GET')return ['/v1/control-plane/status','/v1/auth/me','/v1/teams'].includes(path)||!!teamId&&[`/v1/teams/${teamId}`,`/v1/teams/${teamId}/workspace/v1/sessions`,`/v1/teams/${teamId}/workspace/v1/projects`,...projectIds.flatMap(id=>[`/v1/teams/${teamId}/workspace/v1/product/projects/${id}/specification`,`/v1/teams/${teamId}/projects/${id}/members`])].includes(path);
 if(method==='POST'&&['/v1/auth/bootstrap','/v1/auth/login','/v1/admin/users','/v1/teams'].includes(path))return true;
 if(!teamId)return false;
 if(method==='POST'&&[`/v1/teams/${teamId}/members`,`/v1/teams/${teamId}/workspace/v1/setup/git/identity`,`/v1/teams/${teamId}/workspace/v1/product/projects`].includes(path))return true;
 if(method==='DELETE'&&memberId&&path===`/v1/teams/${teamId}/members/${memberId}`)return true;
 if(method==='PUT'&&memberId&&projectIds.some(id=>path===`/v1/teams/${teamId}/projects/${id}/members/${memberId}`))return true;
 return method==='POST'&&projectIds.some(id=>path===`/v1/teams/${teamId}/workspace/v1/product/projects/${id}/specification`);
}
export function assertEmptySessions(value){assert(value&&Array.isArray(value.sessions));assert.equal(value.sessions.length,0,'No agent/shell/task/check execution is allowed');}
export function managedScopeMatches(env,scopes,runRoot){
 const values=new Map();for(const entry of env){const split=entry.indexOf('=');if(split<1)continue;const key=entry.slice(0,split);if(values.has(key))return false;values.set(key,entry.slice(split+1));}
 if(values.get('LUCI_MANAGED_WORKER')!=='1'||values.get('LUCI_WORKER')!=='1'||values.get('LUCI_EXECUTION_AUTHORITY_FILE')!==runRoot+'/state/control-plane/identity.json')return false;
 return scopes.some(scope=>values.get('LUCI_STATE_DIR')===scope.stateDir&&(scope.kind==='user'?values.get('LUCI_ACTOR_ID')===scope.id&&!values.has('LUCI_TEAM_ID'):values.get('LUCI_TEAM_ID')===scope.id&&!values.has('LUCI_ACTOR_ID')&&!values.has('LUCI_ACTOR_ROLE')));
}
