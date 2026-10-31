import { productApi } from './product';
import type { ResearchEvidence } from './specificationStrategy';

export interface ResearchCapture extends Omit<ResearchEvidence, 'captureId' | 'quotes'> { id: string; text: string }
export interface ResearchCandidate { title: string; url: string; snippet: string }
export interface ResearchSearch { provider: 'brave'; query: string; fetchedAt: string; results: ResearchCandidate[] }
export interface ResearchConnection {provider:'brave';revision:number;configured:boolean}
export type ObservationKind = 'research' | 'feedback' | 'metrics' | 'telemetry' | 'operations';
export const observationKinds: ObservationKind[] = ['research', 'feedback', 'metrics', 'telemetry', 'operations'];
export interface Observation { kind: ObservationKind; digest: string; revision: number; summary: string }
export interface ObservationRoute { kinds: ObservationKind[]; telemetryIds: string[]; remaining: number; last: Record<string, string> }
export interface ObservationView { routeRevision: number; autoDiscuss: boolean; route: ObservationRoute | null; sources: Observation[]; unavailable: ObservationKind[]; telemetryOptions: { id: string; label: string }[] }
const base = (project: string) => `/v1/product/projects/${encodeURIComponent(project)}`;
const text = (v: unknown, max: number): v is string => typeof v === 'string' && new TextEncoder().encode(v).length <= max;
const hash = (v: unknown): v is string => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const kind = (v: unknown): v is ObservationKind => typeof v === 'string' && observationKinds.includes(v as ObservationKind);
export function validResearchEvidence(v: unknown): v is ResearchEvidence {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return false;
  const e = v as ResearchEvidence;
  return Number.isSafeInteger(e.omittedTextBytes) && e.omittedTextBytes >= 0 && e.omittedTextBytes <= 512*1024 && text(e.captureId, 200) && e.captureId.startsWith('research-') && text(e.requestedUrl, 2048) && text(e.url, 2048) && text(e.title, 500) && !!e.title &&
    /^https?:\/\//.test(e.url) && /^https?:\/\//.test(e.requestedUrl) && text(e.fetchedAt, 40) && Number.isFinite(Date.parse(e.fetchedAt)) && hash(e.bodySha256) && hash(e.textSha256) &&
    Array.isArray(e.quotes) && e.quotes.length > 0 && e.quotes.length <= 8 && new Set(e.quotes).size === e.quotes.length && e.quotes.every(q => text(q, 1000) && !!q.trim()) &&
    e.quotes.reduce((n, q) => n + new TextEncoder().encode(q).length, 0) <= 6000 && Object.keys(e).every(k => ['captureId','requestedUrl','url','title','fetchedAt','bodySha256','textSha256','quotes','omittedTextBytes'].includes(k));
}
function checkedObservation(v: ObservationView): ObservationView {
  if (!v || !Number.isSafeInteger(v.routeRevision) || v.routeRevision < 0 || typeof v.autoDiscuss !== 'boolean' ||
      !Array.isArray(v.sources) || v.sources.length > 5 || new Set(v.sources.map(s => s.kind)).size !== v.sources.length || v.sources.some(s => !kind(s.kind) || !hash(s.digest) || !Number.isSafeInteger(s.revision) || s.revision < 1 || !text(s.summary,1600)) ||
      !Array.isArray(v.unavailable) || v.unavailable.some(k => !kind(k)) || !Array.isArray(v.telemetryOptions) || v.telemetryOptions.length > 20 || v.telemetryOptions.some(t => !text(t.id,200) || !text(t.label,500))) throw new Error('The saved observation response could not be verified.');
  const r = v.route;
  if (r && (!Array.isArray(r.kinds) || r.kinds.length > 5 || new Set(r.kinds).size !== r.kinds.length || r.kinds.some(k => !kind(k)) || !Array.isArray(r.telemetryIds) || r.telemetryIds.length > 8 || r.telemetryIds.some(id => !text(id,200)) || !Number.isSafeInteger(r.remaining) || r.remaining < 0 || r.remaining > 16 || !r.last || typeof r.last !== 'object' || Array.isArray(r.last) || Object.entries(r.last).some(([k,d]) => !r.kinds.includes(k as ObservationKind) || !hash(d)))) throw new Error('The saved observation delegation could not be verified.');
  return v;
}
function checkedCapture(v:ResearchCapture):ResearchCapture {
  if (!v || !text(v.text,24*1024) || !v.text || !validResearchEvidence({ captureId:v.id,requestedUrl:v.requestedUrl,url:v.url,title:v.title,fetchedAt:v.fetchedAt,bodySha256:v.bodySha256,textSha256:v.textSha256,quotes:[v.text.slice(0,100)],omittedTextBytes:v.omittedTextBytes })) throw new Error('The collected public source could not be verified.');
  return v;
}
export const researchApi = {
  async connection(project:string,signal?:AbortSignal){const v=await productApi.request<ResearchConnection>(`${base(project)}/research/connection`,undefined,'GET',15_000,signal);if(!v||v.provider!=='brave'||!Number.isSafeInteger(v.revision)||v.revision<0||typeof v.configured!=='boolean'||Object.keys(v).some(k=>!['provider','revision','configured'].includes(k)))throw new Error('The private search metadata could not be verified.');return v;},
  async saveConnection(project:string,revision:number,key:string,remove:boolean,signal?:AbortSignal){const v=await productApi.request<ResearchConnection>(`${base(project)}/research/connection`,{expectedRevision:revision,key,remove},'PUT',15_000,signal);if(!v||v.provider!=='brave'||!Number.isSafeInteger(v.revision)||v.revision<1||typeof v.configured!=='boolean'||Object.keys(v).some(k=>!['provider','revision','configured'].includes(k)))throw new Error('The private connection response could not be verified.');return v;},
  async collect(project: string, url: string, signal?: AbortSignal) {
    const v = await productApi.request<ResearchCapture>(`${base(project)}/research/collect`, { url, acknowledgePublicRead: true }, 'POST', 20_000, signal);
    return checkedCapture(v);
  },
  async investigate(project:string,query:string,signal?:AbortSignal){
    const v=await productApi.request<{search:ResearchSearch;captures:ResearchCapture[];unavailable:string[];omitted:number}>(`${base(project)}/research/investigate`,{query,acknowledgeSearchCost:true,acknowledgePublicReads:true},'POST',50_000,signal);
    if(!v||v.search?.provider!=='brave'||v.search.query!==query.trim()||!Array.isArray(v.captures)||v.captures.length>3||!Array.isArray(v.unavailable)||v.unavailable.length>3||v.unavailable.some(url=>!text(url,2048))||!Number.isSafeInteger(v.omitted)||v.omitted<0||v.omitted>2)throw new Error('The bounded research result could not be verified.');
    v.captures.forEach(checkedCapture);return v;
  },
  async search(project: string, query: string, signal?: AbortSignal) {
    const v = await productApi.request<ResearchSearch>(`${base(project)}/research/search`, {query, acknowledgeSearchCost:true}, 'POST',20_000,signal);
    if (!v || v.provider !== 'brave' || v.query !== query.trim() || !Number.isFinite(Date.parse(v.fetchedAt)) || !Array.isArray(v.results) || v.results.length > 5 || v.results.some(r => !text(r.title,500) || !text(r.url,2048) || !/^https?:\/\//.test(r.url) || !text(r.snippet,2000))) throw new Error('The search response could not be verified.');
    return v;
  },
  async observations(project: string, signal?: AbortSignal) { return checkedObservation(await productApi.request<ObservationView>(`${base(project)}/observations`,undefined,'GET',15_000,signal)); },
  async route(project: string, input: { expectedRouteRevision:number; kinds:ObservationKind[]; telemetryIds:string[]; maxTurns:number; acknowledgeModelCharges:boolean }, signal?: AbortSignal) { return checkedObservation(await productApi.request<ObservationView>(`${base(project)}/observations/route`,input,'PUT',15_000,signal)); },
  queue: (project:string, routeRevision:number, source:Observation, signal?:AbortSignal) => productApi.request(`${base(project)}/observations/queue`, {expectedRouteRevision:routeRevision,kind:source.kind,digest:source.digest,acknowledgeModelCharges:true},'POST',15_000,signal),
};
