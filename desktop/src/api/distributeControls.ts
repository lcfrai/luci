import { productApi } from './product';
import type { DiscoverySourceLink } from './discoveryDocuments';

export interface ControlRow { id:string; requirement:string; category:string; applicability:string; owner:string; evidenceExpected:string; severity?:string; reviewCadence?:string; sheet:string; row:number; unmapped:Record<string,string>;unmappedHeaders:Record<string,string> }
export interface ControlRevision { revision:number; digest:string; sourceSHA256:string; sheet:string; columns:Record<string,string>; controls:ControlRow[]; added:string[]; changed:string[]; removed:string[]; diffDigest:string; actorId:string; at:string }
export interface ControlLibraryView {state:'not-configured'|'configured';teamId:string;revision:number;library:ControlRevision|null}
export interface WorkbookPreview {sourceSHA256:string;workbook:{sheets:{name:string;headers:{column:string;name:string}[];rows:{number:number;cells:Record<string,string>}[]}[];issues:string[]};suggestedColumns:Record<string,string>}
export interface ControlDiff {expectedRevision:number;sourceSHA256:string;sheet:string;columns:Record<string,string>;added:string[];changed:string[];removed:string[];controlCount:number;diffDigest:string}
export type ControlStatus='supported'|'partial'|'missing'|'not-applicable'|'unreviewed'|'unknown';
export interface ControlResult {controlId:string;status:ControlStatus;evidenceRefs:DiscoverySourceLink[];note:string}
export interface GapProposal {id:string;controlId:string;assessmentRevision:number;target:'discover'|'design';summary:string;state:'pending-review'|'applied-draft';actorId:string;at:string;appliedRevision?:number;appliedDigest?:string}
export interface ControlAssessment {version:number;teamId:string;projectId:string;revision:number;libraryRevision:number;libraryDigest:string;contextDigest:string;results:ControlResult[];proposals:GapProposal[];actorId:string;at:string}
export interface AssessmentView {state:'not-configured'|'unreviewed'|'current'|'stale';projectId:string;contextDigest:string;library:ControlRevision|null;assessment:ControlAssessment}
export interface ImportInput {expectedRevision:number;idempotencyKey:string;sourceSHA256:string;sheet:string;columns:Record<string,string>;workbookBase64:string;expectedDiffDigest:string}

export async function workbookBytes(file:File):Promise<{sourceSHA256:string;workbookBase64:string}> {
  if(file.size<1||file.size>2*1024*1024||!file.name.toLowerCase().endsWith('.xlsx'))throw new Error('Choose an XLSX workbook under 2 MiB.');
  const bytes=new Uint8Array(await file.arrayBuffer());
  const digest=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))).map(byte=>byte.toString(16).padStart(2,'0')).join('');
  let binary='';for(let offset=0;offset<bytes.length;offset+=8192)binary+=String.fromCharCode(...bytes.subarray(offset,offset+8192));
  return {sourceSHA256:digest,workbookBase64:btoa(binary)};
}
const project=(id:string)=>`/v1/product/projects/${encodeURIComponent(id)}/control-assessment`;
function requiredList<T>(value:T[]|null|undefined):T[]{
  if(value==null)return [];
  if(!Array.isArray(value))throw new Error('The control library response is invalid. Refresh and try again.');
  return value;
}
function previewShape(value:WorkbookPreview):WorkbookPreview{
  return {...value,workbook:{...value.workbook,sheets:requiredList(value.workbook.sheets),issues:requiredList(value.workbook.issues)}};
}
function diffShape(value:ControlDiff):ControlDiff{
  return {...value,added:requiredList(value.added),changed:requiredList(value.changed),removed:requiredList(value.removed)};
}
export const distributeControlsApi={
  library:(signal?:AbortSignal)=>productApi.request<ControlLibraryView>('/v1/product/team-controls',undefined,'GET',30_000,signal),
  preview:async(file:File,signal?:AbortSignal)=>previewShape(await productApi.uploadJSON<WorkbookPreview>('/v1/product/team-controls/preview',file,signal)),
  compare:async(input:Omit<ImportInput,'idempotencyKey'|'expectedDiffDigest'>,signal?:AbortSignal)=>diffShape(await productApi.request<ControlDiff>('/v1/product/team-controls/compare',input,'POST',30_000,signal)),
  import:(input:ImportInput,signal?:AbortSignal)=>productApi.request<ControlRevision>('/v1/product/team-controls/import',input,'POST',30_000,signal),
  assessment:(projectId:string,signal?:AbortSignal)=>productApi.request<AssessmentView>(project(projectId),undefined,'GET',30_000,signal),
  downloadReport:async(projectId:string,current:()=>boolean)=>{
    const blob=await productApi.requestBlob(`${project(projectId)}/report.xlsx`);
    if(!current())return;
    if(blob.size<1||blob.size>16*1024*1024)throw new Error('The bounded assessment export is incomplete. Refresh the assessment and try again.');
    const url=URL.createObjectURL(blob);
    const link=document.createElement('a');link.href=url;link.download='luci-control-assessment.xlsx';link.click();
    window.setTimeout(()=>URL.revokeObjectURL(url),1000);
  },
  saveAssessment:(projectId:string,input:{expectedRevision:number;libraryRevision:number;libraryDigest:string;contextDigest:string;idempotencyKey:string;results:ControlResult[]},signal?:AbortSignal)=>productApi.request<ControlAssessment>(project(projectId),input,'POST',30_000,signal),
  proposeGap:(projectId:string,controlId:string,input:{assessmentRevision:number;target:'discover'|'design';summary:string},signal?:AbortSignal)=>productApi.request<GapProposal>(`${project(projectId)}/gaps/${encodeURIComponent(controlId)}/proposal`,input,'POST',30_000,signal),
  applyGap:(projectId:string,proposal:GapProposal,input:{expectedAssessmentRevision:number;expectedContextDigest:string;expectedTargetRevision:number},signal?:AbortSignal)=>productApi.request<GapProposal>(`${project(projectId)}/gaps/${encodeURIComponent(proposal.controlId)}/proposal/${encodeURIComponent(proposal.id)}/apply`,input,'POST',30_000,signal),
};
