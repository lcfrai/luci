import { productApi } from './product';
import { distributeArtifactsApi, type ArtifactView, type DistributeArtifact } from './distributeArtifacts';
import { designRasterPinValid, designSceneErrors, type DesignSceneFrame } from './designScene';
import type { DesignPlan } from './designPlan';

export interface LaunchCreativeBrief {
  product:string; headline:string; body:string; cta:string; alt:string; format:'wide'|'square';
  background:string; foreground:string; accent:string; frameId:string; assetId:string;
}
export interface LaunchCreative {
  version:1; renderer:'luci-launch-svg-v1'; brief:LaunchCreativeBrief; frame?:DesignSceneFrame; frameDigest?:string; assets:DesignPlan['assets'];
}
export type CreativeArtifact = DistributeArtifact & {creative:LaunchCreative};
export interface CreativePrepared {draft:CreativeArtifact;prepareDigest:string;svg:string;svgDigest:string}
export type CreativePending = {mode:'create';key:string;brief:LaunchCreativeBrief;prepareDigest:string}|{mode:'revise';key:string;brief:LaunchCreativeBrief;row:CreativeArtifact};
export interface CreativeEditorDraft {projectId:string;brief:LaunchCreativeBrief;saved:CreativeArtifact|null;pending:CreativePending|null}
const hash=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const text=(v:unknown,max:number,required=false):v is string=>typeof v==='string'&&new TextEncoder().encode(v).length<=max&&(!required||!!v.trim())&&!/[\x00-\x08\x0b\x0c\x0e-\x1f]/.test(v);
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const color=(v:unknown):v is string=>typeof v==='string'&&/^#[a-fA-F0-9]{6}$/.test(v);
const id=(v:unknown):v is string=>typeof v==='string'&&/^[a-zA-Z0-9_-]{1,80}$/.test(v);
export function creativeBriefValid(v:unknown):v is LaunchCreativeBrief {
  return object(v)&&Object.keys(v).every(k=>['product','headline','body','cta','alt','format','background','foreground','accent','frameId','assetId'].includes(k))&&text(v.product,120,true)&&text(v.headline,240,true)&&text(v.body,800)&&text(v.cta,80)&&text(v.alt,1000,true)&&['wide','square'].includes(v.format as string)&&color(v.background)&&color(v.foreground)&&color(v.accent)&&(v.frameId===''||id(v.frameId))&&(v.assetId===''||id(v.assetId))&&!(v.frameId&&v.assetId);
}
export function creativeEditorDraftValid(value:unknown,projectId:string):value is CreativeEditorDraft {
  if(!object(value)||value.projectId!==projectId||!object(value.brief))return false;
  const b=value.brief;
  if(!creativeBriefValid({...b,product:'Draft',headline:'Draft',alt:'Draft'})||!text(b.product,120)||!text(b.headline,240)||!text(b.alt,1000)||value.saved!==null&&!creativeArtifactValid(value.saved,projectId))return false;
  if(value.pending===null)return true;
  const p=value.pending;
  if(!object(p)||typeof p.key!=='string'||!/^creative-[a-f0-9-]{36}$/.test(p.key))return false;
  const pendingBrief=p.brief;
  if(!creativeBriefValid(pendingBrief)||Object.keys(pendingBrief).some(k=>pendingBrief[k as keyof LaunchCreativeBrief]!==b[k]))return false;
  return p.mode==='create'?value.saved===null&&hash(p.prepareDigest)&&Object.keys(p).every(k=>['mode','key','brief','prepareDigest'].includes(k)):p.mode==='revise'&&creativeArtifactValid(p.row,projectId)&&object(value.saved)&&p.row.digest===value.saved.digest&&Object.keys(p).every(k=>['mode','key','brief','row'].includes(k));
}
export function creativeArtifactValid(row:unknown,projectId:string):row is CreativeArtifact {
  if(!object(row)||row.projectId!==projectId||row.kind!=='launch-creative'||!id(row.id)||!Number.isSafeInteger(row.revision)||(row.revision as number)<1||!hash(row.digest)||!hash(row.sourceDigest)||!text(row.title,200,true)||!object(row.source)||!hash(row.source.designDigest)||!hash(row.source.gtmDigest)||!Number.isSafeInteger(row.source.designRevision)||!Number.isSafeInteger(row.source.gtmRevision)||(row.source.gtmRevision as number)<1||!['draft','reviewed'].includes(row.state as string)||!Array.isArray(row.sections)||row.sections.length>40||!row.sections.every(s=>object(s)&&id(s.id)&&text(s.heading,200,true)&&text(s.body,8192)&&Array.isArray(s.bullets)&&s.bullets.length<=100&&s.bullets.every(v=>text(v,8192))&&Array.isArray(s.sourceRefs)&&s.sourceRefs.length<=100&&s.sourceRefs.every(v=>text(v,8192)))||!Array.isArray(row.gaps)||row.gaps.length>100||!row.gaps.every(v=>text(v,8192))||!Array.isArray(row.assumptions)||row.assumptions.length>100||!row.assumptions.every(v=>text(v,8192)))return false;
  const c=row.creative;
  if(!object(c)||c.version!==1||c.renderer!=='luci-launch-svg-v1'||!creativeBriefValid(c.brief)||!Array.isArray(c.assets)||c.assets.length>4||!c.assets.every(a=>object(a)&&id(a.id)&&text(a.path,1024,true)&&text(a.licence,4000)&&designRasterPinValid(a.content)))return false;
  if(c.brief.frameId){
    const captured=c.frame;
    if(!object(captured)||captured.id!==c.brief.frameId||!hash(c.frameDigest)||!Array.isArray(captured.nodes))return false;
    // Validate only a copy; local interactions/library links are irrelevant to
    // the exported still, while the original frame and digest stay untouched.
    const frame={...captured,nodes:captured.nodes.map(n=>object(n)?{...n,componentId:undefined,behavior:{kind:'none',targetFrameId:''}}:n)};
    if(designSceneErrors({version:1,frames:[frame]}).length)return false;
  }else if(c.frame!==undefined||c.frameDigest!==undefined)return false;
  return !c.brief.assetId||c.assets.length===1&&c.assets[0].id===c.brief.assetId;
}
const root=(projectId:string)=>`/v1/product/projects/${encodeURIComponent(projectId)}/distribute/artifacts`;
export async function creativeHash(value:string):Promise<string>{return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(value)))).map(x=>x.toString(16).padStart(2,'0')).join('');}

// Image-mode SVG never receives arbitrary markup, URLs, CSS or active content.
export function creativeSVGBlob(svg:string):Blob {
  if(!svg||new TextEncoder().encode(svg).length>12*1024*1024||/<!DOCTYPE|<!ENTITY|<\?/i.test(svg))throw new Error('The composition exceeded the supported self-contained SVG format.');
  const doc=new DOMParser().parseFromString(svg,'image/svg+xml');
  const tags=['svg','title','desc','rect','g','text','image'];
  const attrs=['xmlns','width','height','viewBox','role','fill','x','y','rx','stroke','stroke-width','opacity','transform','font-family','font-size','font-weight','text-anchor','xml:space','preserveAspectRatio','href'];
  if(doc.documentElement.localName!=='svg'||doc.querySelector('parsererror'))throw new Error('The composition could not be read as a complete SVG.');
  for(const el of [doc.documentElement,...Array.from(doc.documentElement.querySelectorAll('*'))]){
    if(el.namespaceURI!=='http://www.w3.org/2000/svg'||!tags.includes(el.localName))throw new Error('This SVG includes unsupported content.');
    for(const a of Array.from(el.attributes)){
      const numeric=/^-?(?:\d+\.?\d*|\.\d+)$/.test(a.value)&&Number.isFinite(Number(a.value))&&Number(a.value)>=0&&Number(a.value)<=8192;
      let allowed=attrs.includes(a.name);
      if(['x','y','width','height','rx','stroke-width','font-size','font-weight'].includes(a.name))allowed&&=numeric;
      else if(a.name==='fill'||a.name==='stroke')allowed&&=/^(#[a-fA-F0-9]{6}|transparent)$/.test(a.value);
      else if(a.name==='href')allowed&&=el.localName==='image'&&/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+=*$/.test(a.value);
      else if(a.name==='opacity')allowed&&=numeric&&Number(a.value)<=1;
      else if(a.name==='font-family')allowed&&=['sans-serif','serif','monospace'].includes(a.value);
      else if(a.name==='text-anchor')allowed&&=['start','middle','end'].includes(a.value);
      else if(a.name==='xml:space')allowed&&=a.value==='preserve';
      else if(a.name==='preserveAspectRatio')allowed&&=a.value==='xMidYMid meet';
      else if(a.name==='xmlns')allowed&&=a.value==='http://www.w3.org/2000/svg';
      else if(a.name==='role')allowed&&=a.value==='img';
      else if(a.name==='viewBox')allowed&&=/^0 0 (1200 675|1080 1080)$/.test(a.value);
      else if(a.name==='transform')allowed&&=/^translate\(\d+(?:\.\d+)? \d+(?:\.\d+)?\) scale\(\d+(?:\.\d+)?\)$/.test(a.value);
      if(!allowed)throw new Error('This SVG is not a self-contained composition.');
    }
  }
  const w=Number(doc.documentElement.getAttribute('width')),h=Number(doc.documentElement.getAttribute('height'));
  if(!(w===1200&&h===675||w===1080&&h===1080))throw new Error('The composition dimensions did not match a supported launch format.');
  return new Blob([svg],{type:'image/svg+xml'});
}
function view(v:ArtifactView,projectId:string,id?:string):ArtifactView & {artifact:CreativeArtifact}{
  if(!creativeArtifactValid(v.artifact,projectId)||id&&v.artifact.id!==id||!['current','outdated','unavailable','readback'].includes(v.freshness))throw new Error('The saved creative response could not be confirmed. Keep your draft and refresh the gallery.');
  return v as ArtifactView & {artifact:CreativeArtifact};
}
export const launchCreativeApi={
  async sendPending(projectId:string,p:CreativePending,signal:AbortSignal){
    return p.mode==='create'?view(await productApi.request<ArtifactView>(root(projectId),{kind:'launch-creative',creative:p.brief,prepareDigest:p.prepareDigest,idempotencyKey:p.key},'POST',30_000,signal),projectId):launchCreativeApi.revise(projectId,p.row,p.brief,p.key,signal);
  },
  async prepare(projectId:string,brief:LaunchCreativeBrief,signal:AbortSignal):Promise<CreativePrepared>{
    if(!creativeBriefValid(brief))throw new Error('Complete the bounded creative brief before generating.');
    const v=await productApi.request<CreativePrepared>(`${root(projectId)}/prepare`,{kind:'launch-creative',creative:brief},'POST',30_000,signal);
    // Prepared rows have no assigned ID/revision until the normal artifact save.
    if(v.draft?.kind!=='launch-creative'||v.draft.projectId!==projectId||!v.draft.creative||Object.keys(brief).some(k=>v.draft.creative.brief[k as keyof LaunchCreativeBrief]!==brief[k as keyof LaunchCreativeBrief])||!hash(v.prepareDigest)||!hash(v.svgDigest)||await creativeHash(v.svg)!==v.svgDigest)throw new Error('The generated creative did not match the requested brief.');
    creativeSVGBlob(v.svg);return v;
  },
  async save(projectId:string,p:CreativePrepared,key:string,signal:AbortSignal){return view(await productApi.request<ArtifactView>(root(projectId),{kind:'launch-creative',creative:p.draft.creative.brief,prepareDigest:p.prepareDigest,idempotencyKey:key},'POST',30_000,signal),projectId);},
  async get(projectId:string,id:string,signal:AbortSignal){return view(await distributeArtifactsApi.get(projectId,id,signal),projectId,id);},
  async revise(projectId:string,row:CreativeArtifact,brief:LaunchCreativeBrief,key:string,signal:AbortSignal){return view(await productApi.request<ArtifactView>(`${root(projectId)}/${encodeURIComponent(row.id)}/revise`,{expectedRevision:row.revision,expectedDigest:row.digest,expectedSourceDigest:row.sourceDigest,idempotencyKey:key,title:row.title,sections:row.sections,gaps:row.gaps,assumptions:row.assumptions,creative:brief},'POST',30_000,signal),projectId,row.id);},
  async svg(projectId:string,row:CreativeArtifact,signal:AbortSignal):Promise<string>{
    const blob=await productApi.requestBlob(`${root(projectId)}/${encodeURIComponent(row.id)}/exports/svg?digest=${row.digest}`,{},30_000,signal);
    if(blob.size<1||blob.size>12*1024*1024||blob.type!=='image/svg+xml')throw new Error('The saved artwork export is incomplete. Keep this record and restore its pinned source assets, or compose a new creative.');
    const svg=await blob.text();signal.throwIfAborted();creativeSVGBlob(svg);return svg;
  },
};

export async function downloadLaunchCreative(svg:string,format:'svg'|'png',filename:string,current:()=>boolean):Promise<void>{
  const source=creativeSVGBlob(svg);let blob=source;
  if(format==='png'){
    const url=URL.createObjectURL(source),image=new Image();let timer:ReturnType<typeof setTimeout>|undefined;
    const canvas=document.createElement('canvas');
    try{
      await new Promise<void>((resolve,reject)=>{timer=setTimeout(()=>reject(new Error('PNG export timed out. Download the SVG or try again.')),15_000);image.onload=()=>{if(timer)clearTimeout(timer);resolve()};image.onerror=()=>{if(timer)clearTimeout(timer);reject(new Error('PNG export could not render this artwork. The SVG is still available.'))};image.src=url;});
      if(!current())return;
      canvas.width=image.naturalWidth;canvas.height=image.naturalHeight;
      if(canvas.width*canvas.height>1200*1080)throw new Error('PNG export dimensions exceeded the launch format.');
      const context=canvas.getContext('2d');if(!context)throw new Error('PNG export is unavailable here. Download SVG instead.');
      context.drawImage(image,0,0);
      blob=await new Promise<Blob>((resolve,reject)=>{timer=setTimeout(()=>reject(new Error('PNG encoding timed out. Download SVG instead.')),15_000);canvas.toBlob(value=>{if(timer)clearTimeout(timer);value?resolve(value):reject(new Error('PNG export failed. Download SVG instead.'))},'image/png')});
    }finally{if(timer)clearTimeout(timer);image.onload=null;image.onerror=null;image.src='';URL.revokeObjectURL(url);canvas.width=0;canvas.height=0;}
  }
  if(!current())return;
  const url=URL.createObjectURL(blob);try{const link=document.createElement('a');link.href=url;link.download=`${filename}.${format}`;link.click();}finally{window.setTimeout(()=>URL.revokeObjectURL(url),1000);}
}
