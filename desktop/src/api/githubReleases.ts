import { productApi } from './product';

export interface GitHubReleaseAccount { id:number; login:string }
export interface GitHubReleaseIntent {
 projectId:string;repository:string;repositoryId:number;tag:string;commit:string;gitReviewDigest:string;contextDigest:string;
 artifactId:string;artifactRevision:number;artifactDigest:string;artifactSourceDigest:string;title:string;body:string;account:GitHubReleaseAccount;
}
export interface GitHubReleaseRemote {
 id:number;tag_name:string;target_commitish:string;name:string;body:string;draft:boolean;prerelease:boolean;
 html_url:string;created_at:string;published_at:string|null;author:GitHubReleaseAccount;
}
export interface GitHubPublication {
 id:string;projectId:string;revision:number;intent:GitHubReleaseIntent;intentDigest:string;
 state:'reviewed'|'draft-unknown'|'draft'|'draft-reviewed'|'publish-unknown'|'published'|'conflict';
 createdAt:string;updatedAt:string;expiresAt:string;draftAttempted:boolean;publishAttempted:boolean;
 remote:GitHubReleaseRemote|null;unconfirmedRemote?:GitHubReleaseRemote;publishReviewDigest?:string;publishExpiresAt?:string;
 message:string;cliAuthScope:'workspace-cli-not-personal-identity';
}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const text=(v:unknown,max:number):v is string=>typeof v==='string'&&new TextEncoder().encode(v).length<=max&&!/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/.test(v);
const hash=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const integer=(v:unknown,max=Number.MAX_SAFE_INTEGER):v is number=>Number.isSafeInteger(v)&&(v as number)>0&&(v as number)<=max;
const date=(v:unknown):v is string=>text(v,64)&&Number.isFinite(Date.parse(v));
const id=(v:unknown):v is string=>text(v,80)&&/^gh-release-[A-Za-z0-9_-]+$/.test(v);
const account=(v:unknown):v is GitHubReleaseAccount=>object(v)&&integer(v.id)&&text(v.login,105)&&/^[A-Za-z0-9][A-Za-z0-9-]{0,99}(\[bot\])?$/.test(v.login);
function remote(v:unknown,intent:GitHubReleaseIntent):v is GitHubReleaseRemote{
 if(!object(v)||!integer(v.id)||v.tag_name!==intent.tag||!text(v.target_commitish,128)||!text(v.name,200)||!text(v.body,32*1024)||typeof v.draft!=='boolean'||typeof v.prerelease!=='boolean'||!date(v.created_at)||v.published_at!==null&&!date(v.published_at)||!account(v.author)||!text(v.html_url,2048))return false;
 try{const u=new URL(v.html_url),parts=u.pathname.split('/').filter(Boolean);return u.protocol==='https:'&&u.host==='github.com'&&!u.username&&!u.password&&!u.search&&!u.hash&&parts.length===5&&`${parts[0]}/${parts[1]}`.toLowerCase()===intent.repository&&parts[2]==='releases'&&(parts[3]==='tag'&&/^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$/.test(parts[4])||parts[3]==='edit'&&parts[4]===String(v.id));}catch{return false;}
}
export function validGitHubPublication(value:unknown,projectId:string):value is GitHubPublication{
 if(!object(value)||!id(value.id)||value.projectId!==projectId||!integer(value.revision,1e9)||!object(value.intent)||!hash(value.intentDigest)||typeof value.state!=='string'||!['reviewed','draft-unknown','draft','draft-reviewed','publish-unknown','published','conflict'].includes(value.state)||!date(value.createdAt)||!date(value.updatedAt)||!date(value.expiresAt)||Date.parse(value.expiresAt)-Date.parse(value.createdAt)!==600000||typeof value.draftAttempted!=='boolean'||typeof value.publishAttempted!=='boolean'||!text(value.message,2000)||value.cliAuthScope!=='workspace-cli-not-personal-identity')return false;
 const i=value.intent;
 if(i.projectId!==projectId||!text(i.repository,2048)||!/^[a-z0-9][a-z0-9-]*\/[a-z0-9._-]+$/.test(i.repository)||!integer(i.repositoryId)||!text(i.tag,64)||!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(i.tag)||!text(i.commit,40)||!/^[a-f0-9]{40}$/.test(i.commit)||!hash(i.gitReviewDigest)||!hash(i.contextDigest)||!text(i.artifactId,128)||!i.artifactId||!integer(i.artifactRevision,1e9)||!hash(i.artifactDigest)||!hash(i.artifactSourceDigest)||!text(i.title,200)||!i.title.trim()||!text(i.body,32*1024)||!i.body.trim()||!account(i.account))return false;
 const intent=i as unknown as GitHubReleaseIntent;
 if(value.remote!==null&&(!remote(value.remote,intent)||value.remote.author.id!==intent.account.id)||value.unconfirmedRemote!==undefined&&!remote(value.unconfirmedRemote,intent))return false;
 if(value.publishAttempted&&(!value.draftAttempted||value.remote===null)||value.state==='reviewed'&&(value.draftAttempted||value.publishAttempted||value.remote!==null))return false;
 if((value.state==='draft'||value.state==='draft-reviewed')&&(!value.draftAttempted||!object(value.remote)||value.remote.draft!==true))return false;
 if(value.state==='published'&&(!value.publishAttempted||!object(value.remote)||value.remote.draft!==false||value.remote.published_at===null))return false;
 return value.publishReviewDigest===undefined||hash(value.publishReviewDigest)&&date(value.publishExpiresAt);
}
const root=(projectId:string)=>`/v1/product/projects/${encodeURIComponent(projectId)}/distribute/github-releases`;
const bad=()=>new Error('The original publication response could not be verified. Read its retained record; do not repeat a GitHub effect.');
export const githubReleasesApi={
 list:async(projectId:string,signal?:AbortSignal):Promise<{projectId:string;publications:GitHubPublication[];earlierAvailable:boolean}>=>{
  const value:unknown=await productApi.request(root(projectId),undefined,'GET',30000,signal);
  if(!object(value)||value.projectId!==projectId||!Array.isArray(value.publications)||value.publications.length>64||!value.publications.every(row=>validGitHubPublication(row,projectId))||new Set(value.publications.map(row=>row.id)).size!==value.publications.length||typeof value.earlierAvailable!=='boolean')throw bad();return value as unknown as {projectId:string;publications:GitHubPublication[];earlierAvailable:boolean};
 },
 get:async(projectId:string,publicationId:string,signal?:AbortSignal):Promise<GitHubPublication>=>{
  if(!id(publicationId))throw bad();const value:unknown=await productApi.request(`${root(projectId)}/${encodeURIComponent(publicationId)}`,undefined,'GET',30000,signal);if(!validGitHubPublication(value,projectId)||value.id!==publicationId)throw bad();return value;
 },
 prepare:async(projectId:string,input:{artifactId:string;artifactRevision:number;artifactDigest:string;tag:string},signal?:AbortSignal):Promise<GitHubPublication>=>{
  const value:unknown=await productApi.request(`${root(projectId)}/review`,input,'POST',95000,signal);
  if(!validGitHubPublication(value,projectId)||value.intent.artifactId!==input.artifactId||value.intent.artifactRevision!==input.artifactRevision||value.intent.artifactDigest!==input.artifactDigest||value.intent.tag!==input.tag||value.state!=='reviewed'||value.draftAttempted||value.publishAttempted)throw bad();return value;
 },
 act:async(projectId:string,row:GitHubPublication,action:'draft'|'review'|'publish'|'observe',acknowledged:boolean,signal?:AbortSignal):Promise<GitHubPublication>=>{
  const value:unknown=await productApi.request(`${root(projectId)}/${encodeURIComponent(row.id)}/${action}`,{expectedRevision:row.revision,intentDigest:row.intentDigest,acknowledgeContent:acknowledged},'POST',95000,signal);
  if(!validGitHubPublication(value,projectId)||value.id!==row.id||value.intentDigest!==row.intentDigest||JSON.stringify(value.intent)!==JSON.stringify(row.intent)||value.revision<row.revision||row.draftAttempted&&!value.draftAttempted||row.publishAttempted&&!value.publishAttempted||row.remote&&value.remote?.id!==row.remote.id)throw bad();return value;
 },
};
