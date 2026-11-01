import { productApi, ProductApiError } from './product';

export const DEFAULT_HTTPS_IMAGE = 'ghcr.io/traefik/traefik@sha256:6d3e069ee484ab1da5adfdda555a75eb9b1545ef09096a0cc7c3043101986a20';
export type HTTPSAction = 'apply'|'observe'|'pause'|'resume'|'retire';
export interface HTTPSReviewInput {
  reviewKey:string; deploymentId:string; serviceId:string; hostname:string; path:string; pathType:'Exact'|'Prefix';
  azureDnsZoneId:string; traefikImage:string; storageClass:string; acmeEmail:string;
  dnsCredential:{tenantId:string;clientId:string;clientSecret:string}; monthlyBudgetUsd:number;
  acknowledgePublicCostsAndRenewal:boolean; acknowledgeAcmeTerms:boolean; acknowledgePrivateDnsCredential:boolean;
}
export interface HTTPSPlan {
  id:string;projectId:string;deploymentId:string;
  destination:{targetId:string;targetFingerprint:string;namespace:string;namespaceUid:string;serviceId:string;serviceName:string;serviceUid:string;servicePort:number};
  hostname:string;path:string;pathType:'Exact'|'Prefix';azureDnsZoneId:string;azureDnsZoneEtag:string;azureDnsRecordName:string;azureJobId:string;azureJobPlanDigest:string;azureClusterUid:string;
  traefikImage:string;profile:'traefik-native-acme-azuredns-file-v1';storageClass:string;storageClassUid:string;storageClassVersion:string;storageBindingMode:'Immediate'|'WaitForFirstConsumer';storageGiB:1;
  acmeEmail:string;dnsClientId:string;monthlyBudgetUsd:number;createdAt:string;expiresAt:string;digest:string;
}
export interface HTTPSResource {kind:'PersistentVolumeClaim'|'Secret'|'ConfigMap'|'Deployment'|'Service';name:string;attempted:boolean;uid?:string;state:'planned'|'unknown'|'observed'|'pending'|'ready'|'deleted'|'retained';deleteAttempted:boolean;absentObserved:boolean}
export interface HTTPSActionReceipt {requestKey:string;requestDigest:string;action:HTTPSAction;state:'attempted'|'settled'|'unknown';startedRevision:number;settledRevision:number}
export interface HTTPSStatus {
  id:string;projectId:string;revision:number;reviewKey:string;lastActionRequestKey:string;plan:HTTPSPlan;
  state:'reviewed'|'provisioning'|'pending-address'|'pending-dns'|'pending-certificate'|'ready'|'paused'|'retiring'|'retired'|'unknown'|'access-changed';resources:HTTPSResource[];actions:HTTPSActionReceipt[];
  address:string;dnsAttempted:boolean;dnsAcknowledged:boolean;dnsObserved:boolean;httpsVerified:boolean;certificateNotAfter?:string;certificateSha256?:string;httpStatus:number;applicationHealthVerified:false;url?:string;observedAt:string;lastAction:string;message:string;pvcDataRetained:boolean;
}
const obj=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const str=(v:unknown,max:number):v is string=>typeof v==='string'&&v.length<=max&&!/[\x00-\x1f\x7f]/.test(v);
const nonempty=(v:unknown,max=128):v is string=>str(v,max)&&v.length>0;
const digest=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const date=(v:unknown):v is string=>str(v,64)&&Number.isFinite(Date.parse(v));
const integer=(v:unknown,min:number,max:number):v is number=>Number.isSafeInteger(v)&&(v as number)>=min&&(v as number)<=max;
const keys=(v:Record<string,unknown>,allowed:string[])=>Object.keys(v).every(k=>allowed.includes(k));
const bools=(v:Record<string,unknown>,names:string[])=>names.every(k=>typeof v[k]==='boolean');
export const validHTTPSID=(v:unknown):v is string=>typeof v==='string'&&/^https-[a-f0-9]{32}$/.test(v);
export const validHTTPSKey=(v:unknown):v is string=>typeof v==='string'&&/^[a-zA-Z0-9_-]{16,128}$/.test(v);
export function validHTTPSStatus(v:unknown,projectId:string):v is HTTPSStatus {
  if(!obj(v)||!keys(v,['id','projectId','revision','reviewKey','lastActionRequestKey','plan','state','resources','actions','address','dnsAttempted','dnsAcknowledged','dnsObserved','httpsVerified','certificateNotAfter','certificateSha256','httpStatus','applicationHealthVerified','url','observedAt','lastAction','message','pvcDataRetained'])||!validHTTPSID(v.id)||v.projectId!==projectId||!integer(v.revision,1,1e9)||!validHTTPSKey(v.reviewKey)||!(v.lastActionRequestKey===''||validHTTPSKey(v.lastActionRequestKey))||!obj(v.plan))return false;
  const p=v.plan,d=p.destination;
  if(!keys(p,['id','projectId','deploymentId','destination','hostname','path','pathType','azureDnsZoneId','azureDnsZoneEtag','azureDnsRecordName','azureJobId','azureJobPlanDigest','azureClusterUid','traefikImage','profile','storageClass','storageClassUid','storageClassVersion','storageBindingMode','storageGiB','acmeEmail','dnsClientId','monthlyBudgetUsd','createdAt','expiresAt','digest'])||p.id!==v.id||p.projectId!==projectId||!str(p.deploymentId,41)||!/^workload-[a-f0-9]{32}$/.test(p.deploymentId)||!obj(d)||!keys(d,['targetId','targetFingerprint','namespace','namespaceUid','serviceId','serviceName','serviceUid','servicePort'])||!nonempty(d.targetId,40)||!digest(d.targetFingerprint)||!nonempty(d.namespace,63)||!nonempty(d.namespaceUid)||!nonempty(d.serviceId,63)||!nonempty(d.serviceName,63)||!nonempty(d.serviceUid)||!integer(d.servicePort,1,65535))return false;
  if(!str(p.hostname,253)||p.hostname!==p.hostname.toLowerCase()||!p.hostname.includes('.')||!/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/.test(p.hostname)||p.hostname.split('.').some(label=>!label||label.length>63||label.startsWith('-')||label.endsWith('-'))||!nonempty(p.path,1024)||!p.path.startsWith('/')||/[?#\\\s]/.test(p.path)||(p.pathType!=='Exact'&&p.pathType!=='Prefix')||!nonempty(p.azureDnsZoneId,2048)||!nonempty(p.azureDnsZoneEtag,256)||!nonempty(p.azureDnsRecordName,253)||!nonempty(p.azureJobId)||!digest(p.azureJobPlanDigest)||!nonempty(p.azureClusterUid)||!str(p.traefikImage,256)||!/^(ghcr\.io\/traefik\/traefik|docker\.io\/library\/traefik)@sha256:[a-f0-9]{64}$/.test(p.traefikImage)||p.profile!=='traefik-native-acme-azuredns-file-v1'||!nonempty(p.storageClass,253)||!nonempty(p.storageClassUid)||!nonempty(p.storageClassVersion)||(p.storageBindingMode!=='Immediate'&&p.storageBindingMode!=='WaitForFirstConsumer')||p.storageGiB!==1||!nonempty(p.acmeEmail,320)||!nonempty(p.dnsClientId)||typeof p.monthlyBudgetUsd!=='number'||!Number.isFinite(p.monthlyBudgetUsd)||p.monthlyBudgetUsd<=0||!date(p.createdAt)||!date(p.expiresAt)||Date.parse(p.expiresAt)-Date.parse(p.createdAt)!==600000||!digest(p.digest))return false;
  const kinds=['PersistentVolumeClaim','Secret','ConfigMap','Deployment','Service'],states=['planned','unknown','observed','pending','ready','deleted','retained'];
  if(!Array.isArray(v.resources)||v.resources.length!==5||!v.resources.every(r=>obj(r)&&keys(r,['kind','name','attempted','uid','state','deleteAttempted','absentObserved'])&&typeof r.kind==='string'&&kinds.includes(r.kind)&&nonempty(r.name,253)&&bools(r,['attempted','deleteAttempted','absentObserved'])&&(r.uid===undefined||str(r.uid,128))&&typeof r.state==='string'&&states.includes(r.state))||new Set(v.resources.map(r=>(r as HTTPSResource).kind)).size!==5)return false;
  if(!Array.isArray(v.actions)||v.actions.length>32||!v.actions.every(a=>obj(a)&&keys(a,['requestKey','requestDigest','action','state','startedRevision','settledRevision'])&&validHTTPSKey(a.requestKey)&&digest(a.requestDigest)&&typeof a.action==='string'&&['apply','observe','pause','resume','retire'].includes(a.action)&&typeof a.state==='string'&&['attempted','settled','unknown'].includes(a.state)&&integer(a.startedRevision,1,v.revision as number)&&integer(a.settledRevision,0,v.revision as number)&&(a.state==='settled'?a.settledRevision>a.startedRevision:a.settledRevision===0))||new Set(v.actions.map(a=>(a as HTTPSActionReceipt).requestKey)).size!==v.actions.length)return false;
  if(typeof v.state!=='string'||!['reviewed','provisioning','pending-address','pending-dns','pending-certificate','ready','paused','retiring','retired','unknown','access-changed'].includes(v.state)||!bools(v,['dnsAttempted','dnsAcknowledged','dnsObserved','httpsVerified','pvcDataRetained'])||!str(v.address,64)||!integer(v.httpStatus,0,599)||v.applicationHealthVerified!==false||!str(v.lastAction,32)||!str(v.message,2000)||!str(v.observedAt,64)||(v.observedAt!==''&&!date(v.observedAt))||(v.certificateNotAfter!==undefined&&!date(v.certificateNotAfter))||(v.certificateSha256!==undefined&&!digest(v.certificateSha256))||(v.url!==undefined&&!str(v.url,2048)))return false;
  // A public link is a server-observed endpoint, never an inferred DNS result.
  if(v.httpsVerified)return v.dnsObserved===true&&!!v.address&&date(v.observedAt)&&date(v.certificateNotAfter)&&digest(v.certificateSha256)&&v.url===`https://${p.hostname}${p.path}`;
  return v.url===undefined||v.url==='';
}
const base=(id:string)=>`/v1/product/projects/${encodeURIComponent(id)}/deploy/public-https`;
function checked(value:unknown,projectId:string,id?:string):HTTPSStatus {if(!validHTTPSStatus(value,projectId)||id!==undefined&&value.id!==id)throw new Error('The original HTTPS record could not be confirmed. Read it again before continuing.');return value;}
export const publicHTTPSApi={
  async list(projectId:string,signal?:AbortSignal,reviewKey?:string):Promise<HTTPSStatus[]> {
    if(reviewKey!==undefined&&!validHTTPSKey(reviewKey))throw new Error('The saved review identity is invalid.');
    const v:unknown=await productApi.request(base(projectId)+(reviewKey?`?reviewKey=${encodeURIComponent(reviewKey)}`:''),undefined,'GET',30000,signal);
    if(!obj(v)||!keys(v,['projectId','commissions','retainedRecentOnly'])||v.projectId!==projectId||v.retainedRecentOnly!==(reviewKey===undefined)||!Array.isArray(v.commissions)||v.commissions.length>(reviewKey?1:128)||!v.commissions.every(r=>validHTTPSStatus(r,projectId)&&(!reviewKey||r.reviewKey===reviewKey))||new Set(v.commissions.map(r=>(r as HTTPSStatus).id)).size!==v.commissions.length)throw new Error('Saved HTTPS records could not be confirmed.');
    return v.commissions as HTTPSStatus[];
  },
  async get(projectId:string,id:string,signal?:AbortSignal){if(!validHTTPSID(id))throw new Error('The original HTTPS identity is invalid.');return checked(await productApi.request(`${base(projectId)}/${encodeURIComponent(id)}`,undefined,'GET',30000,signal),projectId,id);},
  async review(projectId:string,input:HTTPSReviewInput,signal?:AbortSignal){const row=checked(await productApi.request(`${base(projectId)}/reviews`,input,'POST',60000,signal),projectId);if(row.reviewKey!==input.reviewKey||row.plan.deploymentId!==input.deploymentId||row.plan.destination.serviceId!==input.serviceId||row.plan.hostname!==input.hostname||row.plan.path!==input.path||row.plan.pathType!==input.pathType||row.plan.azureDnsZoneId.toLowerCase()!==input.azureDnsZoneId.toLowerCase()||row.plan.traefikImage!==input.traefikImage||row.plan.storageClass!==input.storageClass||row.plan.acmeEmail!==input.acmeEmail||row.plan.dnsClientId!==input.dnsCredential.clientId||row.plan.monthlyBudgetUsd!==input.monthlyBudgetUsd)throw new Error('The returned review differs from your request. Look up the original review before continuing.');return row;},
  async action(projectId:string,row:HTTPSStatus,action:HTTPSAction,requestKey:string,signal?:AbortSignal){if(!validHTTPSStatus(row,projectId))throw new Error('The saved HTTPS action is invalid.');return publicHTTPSApi.originalAction(projectId,row.id,row.plan.digest,row.reviewKey,row.revision,action,requestKey,signal);},
  async originalAction(projectId:string,id:string,planDigest:string,reviewKey:string,expectedRevision:number,action:HTTPSAction,requestKey:string,signal?:AbortSignal){
    if(!validHTTPSID(id)||!digest(planDigest)||!validHTTPSKey(reviewKey)||!validHTTPSKey(requestKey)||!integer(expectedRevision,1,1e9)||!['apply','observe','pause','resume','retire'].includes(action))throw new Error('The saved original action is invalid.');
    const next=checked(await productApi.request(`${base(projectId)}/${encodeURIComponent(id)}/action`,{expectedRevision,requestKey,action},'POST',90000,signal),projectId,id);
    if(next.plan.digest!==planDigest||next.reviewKey!==reviewKey||!httpsActionSettled(next,requestKey,action,expectedRevision))throw new Error('The original HTTPS action is not yet confirmed. Read its original record.');return next;
  }
};
export function httpsActionSettled(row:HTTPSStatus,key:string,action:HTTPSAction,expectedRevision:number):boolean {return row.actions.some(receipt=>receipt.requestKey===key&&receipt.action===action&&receipt.state==='settled'&&receipt.startedRevision===expectedRevision&&receipt.settledRevision>0&&receipt.settledRevision<=row.revision);}
// Only these named Review validation errors precede its metadata write. No
// resource action uses this classification, even if it returns the same code.
export function definitiveHTTPSReviewRefusal(cause:unknown):boolean {return cause instanceof ProductApiError&&cause.status===422&&['public_https_domain','public_https_storage','public_https_application','public_https_input'].includes(cause.code);}
