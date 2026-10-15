import { productApi } from './product';

export type ArtifactKind = 'risk' | 'go-to-market' | 'proposal' | 'pricing' | 'presentation';
export interface ArtifactSource {
  briefRevision:number;briefDigest:string;specificationRevision:number;specificationDigest:string;
  designRevision:number;designDigest:string;planRevision:number;planDigest:string;
  teamId?:string;libraryRevision?:number;libraryDigest?:string;assessmentRevision?:number;assessmentDigest?:string;
  checks:string;release:string;selectedEvidence?:{runId:string;runRevision:number;checkId:string;releaseId:string;commit:string;sourceDigest:string;checkDigest:string;releaseDigest:string;acceptedAt:string};
}
export interface ArtifactSection {id:string;heading:string;body:string;bullets:string[];sourceRefs:string[]}
export interface ArtifactReview {actor:{id:string;displayName:string};at:string;digest:string}
export interface DistributeArtifact {
  id:string;projectId:string;revision:number;kind:ArtifactKind;source:ArtifactSource;sourceDigest:string;
  title:string;sections:ArtifactSection[];gaps:string[];assumptions:string[];state:'draft'|'reviewed';digest:string;
  review?:ArtifactReview;at:string;
}
export interface ArtifactView {artifact:DistributeArtifact;freshness:'current'|'outdated'|'unavailable'|'readback';history?:DistributeArtifact[]}
export interface ArtifactSummary {id:string;kind:ArtifactKind;title:string;revision:number;state:'draft'|'reviewed';freshness:'current'|'outdated'|'requires-readback';digest:string}
export interface ArtifactList {projectId:string;artifacts:ArtifactSummary[]}
const root=(projectId:string)=>`/v1/product/projects/${encodeURIComponent(projectId)}/distribute/artifacts`;
const item=(projectId:string,id:string)=>`${root(projectId)}/${encodeURIComponent(id)}`;
export const distributeArtifactsApi={
  list:(projectId:string,signal?:AbortSignal)=>productApi.request<ArtifactList>(root(projectId),undefined,'GET',30_000,signal),
  prepare:(projectId:string,kind:ArtifactKind,runId='')=>productApi.request<{draft:DistributeArtifact;prepareDigest:string}>(`${root(projectId)}/prepare`,{kind,runId},'POST'),
  save:(projectId:string,kind:ArtifactKind,runId:string,prepareDigest:string,idempotencyKey:string)=>productApi.request<ArtifactView>(root(projectId),{kind,runId,prepareDigest,idempotencyKey},'POST'),
  get:(projectId:string,id:string,signal?:AbortSignal)=>productApi.request<ArtifactView>(item(projectId,id),undefined,'GET',30_000,signal),
  revise:(projectId:string,row:DistributeArtifact,title:string,sections:ArtifactSection[],gaps:string[],assumptions:string[],idempotencyKey:string)=>productApi.request<ArtifactView>(`${item(projectId,row.id)}/revise`,{expectedRevision:row.revision,expectedDigest:row.digest,expectedSourceDigest:row.sourceDigest,idempotencyKey,title,sections,gaps,assumptions},'POST'),
  review:(projectId:string,row:DistributeArtifact,idempotencyKey:string)=>productApi.request<ArtifactView>(`${item(projectId,row.id)}/review`,{expectedRevision:row.revision,expectedDigest:row.digest,idempotencyKey},'POST'),
  download:async(projectId:string,row:DistributeArtifact,format:'docx'|'pptx'|'xlsx',current:()=>boolean)=>{
    const blob=await productApi.requestBlob(`${item(projectId,row.id)}/exports/${format}?digest=${encodeURIComponent(row.digest)}`);
    if(!current())return;
    if(blob.size<1||blob.size>16*1024*1024)throw new Error('The bounded artifact export is incomplete. Refresh this artifact and try again.');
    const url=URL.createObjectURL(blob);
    const link=document.createElement('a');link.href=url;link.download=`luci-${row.kind}-${row.id}-r${row.revision}.${format}`;link.click();
    window.setTimeout(()=>URL.revokeObjectURL(url),1000);
  },
};
