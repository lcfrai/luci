import { productApi } from './product';

export interface OutcomeCostInput {runId:string;operationId:string;category:'model-api'|'infrastructure';provider:string;currency:string;amountMicros:string;billedOn:string;sourceFileName:string;sourceSha256:string;sourceRow:number;providerReference:string}
export interface OutcomeCostReceipt extends OutcomeCostInput {id:string;revision:number;runInputDigest:string;operationRequestDigest:string;childId:string;harness:string;model:string;recordedAt:string;updatedAt:string;attribution:'operator-reconciled'}
export interface OutcomeCosts {projectId:string;revision:number;rows:OutcomeCostReceipt[];totals:{currency:string;amountMicros:string;receiptRows:number}[];omittedRows:number;attribution:'operator-reconciled-retained-window';providerVerified:false;inputTokens:null;outputTokens:null}
export interface OutcomeCostSave {expectedRevision:number;id:string;remove:boolean;input:OutcomeCostInput|null;acknowledgeActualBill:boolean}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const text=(v:unknown,max:number):v is string=>typeof v==='string'&&new TextEncoder().encode(v).length<=max&&!/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/.test(v);
const count=(v:unknown,max=1e9):v is number=>Number.isSafeInteger(v)&&(v as number)>=0&&(v as number)<=max;
const hash=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const date=(v:unknown):v is string=>text(v,64)&&Number.isFinite(Date.parse(v));
const costId=(v:unknown):v is string=>text(v,85)&&/^cost-[a-zA-Z0-9-]{1,80}$/.test(v);
const micros=(v:unknown):v is string=>typeof v==='string'&&/^(0|[1-9][0-9]{0,15})$/.test(v)&&BigInt(v)<=1000000000000000n;
export function parseActualCost(value:string):string|null {
 const match=/^(0|[1-9][0-9]{0,9})(?:\.([0-9]{1,6}))?$/.exec(value.trim());if(!match)return null;
 const amount=BigInt(match[1])*1000000n+BigInt((match[2]??'').padEnd(6,'0'));return amount<=1000000000000000n?amount.toString():null;
}
export function formatActualCost(currency:string,value:string):string {
 const amount=BigInt(value),whole=amount/1000000n,fraction=(amount%1000000n).toString().padStart(6,'0').replace(/0+$/,'');return `${currency} ${whole.toString()}${fraction?'.'+fraction:''}`;
}
export function validOutcomeCosts(v:unknown,projectId:string):v is OutcomeCosts {
 if(!object(v)||v.projectId!==projectId||!count(v.revision)||!Array.isArray(v.rows)||v.rows.length>256||!Array.isArray(v.totals)||v.totals.length>256||!count(v.omittedRows)||v.attribution!=='operator-reconciled-retained-window'||v.providerVerified!==false||v.inputTokens!==null||v.outputTokens!==null)return false;
 const ids=new Set<string>(),sources=new Set<string>(),totals=new Map<string,{amount:bigint;count:number}>();
 for(const row of v.rows){
  if(!object(row)||!costId(row.id)||ids.has(row.id)||!count(row.revision)||row.revision<1||!text(row.runId,128)||!/^run-[A-Za-z0-9-]+$/.test(row.runId)||!text(row.operationId,128)||!/^op-[A-Za-z0-9-]+$/.test(row.operationId)||row.category!=='model-api'&&row.category!=='infrastructure'||!text(row.provider,64)||!row.provider.trim()||typeof row.currency!=='string'||!/^[A-Z]{3}$/.test(row.currency)||!micros(row.amountMicros)||typeof row.billedOn!=='string'||!/^\d{4}-\d{2}-\d{2}$/.test(row.billedOn)||!date(row.billedOn)||new Date(row.billedOn).toISOString().slice(0,10)!==row.billedOn||!text(row.sourceFileName,128)||!row.sourceFileName||/[\\/\r\n]/.test(row.sourceFileName)||!hash(row.sourceSha256)||!count(row.sourceRow,1e6)||row.sourceRow<1||!text(row.providerReference,128)||!hash(row.runInputDigest)||!hash(row.operationRequestDigest)||!text(row.childId,128)||!row.childId||!text(row.harness,128)||!text(row.model,256)||!date(row.recordedAt)||!date(row.updatedAt)||row.attribution!=='operator-reconciled')return false;
  const source=`${row.sourceSha256}/${row.sourceRow}/${row.category}`;if(sources.has(source))return false;sources.add(source);ids.add(row.id);
  const total=totals.get(row.currency)??{amount:0n,count:0};total.amount+=BigInt(row.amountMicros);total.count++;totals.set(row.currency,total);
 }
 const currencies=new Set<string>();
 for(const total of v.totals){if(!object(total)||typeof total.currency!=='string'||currencies.has(total.currency)||typeof total.amountMicros!=='string'||!/^(0|[1-9][0-9]{0,18})$/.test(total.amountMicros)||!count(total.receiptRows,256))return false;const expected=totals.get(total.currency);if(!expected||expected.amount.toString()!==total.amountMicros||expected.count!==total.receiptRows)return false;currencies.add(total.currency);}
 return currencies.size===totals.size;
}
const root=(id:string)=>`/v1/product/projects/${encodeURIComponent(id)}/runs/outcome-costs`;
const bad=()=>new Error('These retained bill reconciliations could not be confirmed. Read the saved revision before another change.');
export const outcomeCostsApi={
 get:async(projectId:string,signal?:AbortSignal):Promise<OutcomeCosts>=>{const v:unknown=await productApi.request(root(projectId),undefined,'GET',30_000,signal);if(!validOutcomeCosts(v,projectId))throw bad();return v;},
 save:async(projectId:string,input:OutcomeCostSave,signal?:AbortSignal):Promise<OutcomeCosts>=>{
  const v:unknown=await productApi.request(root(projectId),input,'POST',30_000,signal);if(!validOutcomeCosts(v,projectId)||v.revision!==input.expectedRevision+1)throw bad();
  if(input.remove){if(v.rows.some(row=>row.id===input.id))throw bad();}else{const submitted=input.input;if(!submitted||!v.rows.some(row=>(!input.id||row.id===input.id)&&Object.entries(submitted).every(([key,value])=>row[key as keyof OutcomeCostInput]===value)))throw bad();}return v;
 },
};
