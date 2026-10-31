export interface ProjectSkillMetadata {
 license?:string;sourceUrl?:string;sourceRevision?:string;declaredTools?:string[];
}
const bytes=(value:string)=>new TextEncoder().encode(value).length;
const text=(value:unknown,max:number):value is string=>typeof value==='string'&&value===value.trim()&&bytes(value)<=max&&!/[\p{Cc}\p{Cf}]/u.test(value);
export function validProjectSkillMetadata(value:unknown):value is ProjectSkillMetadata|undefined{
 if(value===undefined)return true;
 if(!value||typeof value!=='object'||Array.isArray(value))return false;
 const m=value as Record<string,unknown>;
 if(Object.keys(m).some(key=>!['license','sourceUrl','sourceRevision','declaredTools'].includes(key))||m.license!==undefined&&!text(m.license,128)||m.sourceRevision!==undefined&&!text(m.sourceRevision,200)||m.sourceUrl!==undefined&&!text(m.sourceUrl,2048))return false;
 if(m.sourceUrl){try{const u=new URL(m.sourceUrl as string);if(u.protocol!=='https:'||!u.hostname||u.username||u.password||/[\s\\]/.test(m.sourceUrl as string))return false;}catch{return false;}}
 return m.declaredTools===undefined||Array.isArray(m.declaredTools)&&m.declaredTools.length<=16&&new Set(m.declaredTools).size===m.declaredTools.length&&m.declaredTools.every(tool=>typeof tool==='string'&&/^[A-Za-z0-9][A-Za-z0-9._:/-]{0,79}$/.test(tool));
}
export function normalizeProjectSkillMetadata(value:ProjectSkillMetadata|undefined):ProjectSkillMetadata|undefined{
 if(!value)return undefined;
 const license=value.license?.trim()||undefined,sourceUrl=value.sourceUrl?.trim()||undefined,sourceRevision=value.sourceRevision?.trim()||undefined;
 const declaredTools=value.declaredTools?.map(tool=>tool.trim()).filter(Boolean);
 if(!license&&!sourceUrl&&!sourceRevision&&!declaredTools?.length)return undefined;
 return {...(license?{license}:{}),...(sourceUrl?{sourceUrl}:{}),...(sourceRevision?{sourceRevision}:{}),...(declaredTools?.length?{declaredTools}:{})};
}
