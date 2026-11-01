import { beforeEach, expect, it, vi, afterEach } from 'vitest';
import { readHTTPSRecovery, writeHTTPSRecovery, validHTTPSRecovery, type HTTPSRecovery } from './publicHTTPSRecovery';
const value=():HTTPSRecovery=>({version:1,projectId:'p-one',teamId:'team-one',pending:[{kind:'action',reviewKey:'review_'+'a'.repeat(32),id:'https-'+'b'.repeat(32),digest:'c'.repeat(64),expectedRevision:2,requestKey:'original_'+'d'.repeat(32),action:'apply'}]});
beforeEach(()=>localStorage.clear());afterEach(()=>vi.restoreAllMocks());
it('retains unresolved originals without expiry and refuses another scope or credential-shaped data',()=>{
 const record=value(),raw=writeHTTPSRecovery('one',null,record);
 expect(readHTTPSRecovery('one','p-one','team-one')).toEqual({raw,value:record});
 expect(validHTTPSRecovery(record,'p-one','team-two')).toBe(false);
 expect(validHTTPSRecovery({...record,dnsCredential:{clientSecret:'private'}},'p-one','team-one')).toBe(false);
 expect(validHTTPSRecovery({...record,pending:[...record.pending,...record.pending]},'p-one','team-one')).toBe(false);
});
it('keeps a prior uncertain effect alongside observation and never overwrites another window',()=>{
 const record=value(),raw=writeHTTPSRecovery('one',null,record);
 const original=record.pending[0];if(original.kind!=='action')throw new Error('Expected the original action fixture.');
 const combined={...record,pending:[...record.pending,{...original,requestKey:'observe_'+'e'.repeat(32),action:'observe' as const}]};
 const newer=writeHTTPSRecovery('one',raw,combined);
 expect(readHTTPSRecovery('one','p-one','team-one').value?.pending).toHaveLength(2);
 expect(()=>writeHTTPSRecovery('one',raw,null)).toThrow('Another window');
 expect(localStorage.getItem('one')).toBe(newer);
});
it('refuses an unconfirmed browser write before a caller can send an action',()=>{
 vi.spyOn(Storage.prototype,'setItem').mockImplementation(()=>undefined);
 expect(()=>writeHTTPSRecovery('one',null,value())).toThrow('could not be confirmed');
 expect(localStorage.getItem('one')).toBeNull();
});
