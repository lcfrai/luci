import { act,cleanup,renderHook } from '@testing-library/react';
import { afterEach,beforeEach,expect,it,vi } from 'vitest';
import { useProjectDraft } from './useProjectDraft';
import { creativeEditorDraftValid,type CreativeEditorDraft } from '../api/launchCreative';

const scope='["test-service","actor-one","team-one","project-one"]';
const valid=(v:unknown):v is CreativeEditorDraft=>creativeEditorDraftValid(v,'project-one');
const brief={product:'Product',headline:'Launch',body:'',cta:'',alt:'Launch artwork',format:'wide' as const,background:'#ffffff',foreground:'#18181b',accent:'#244f44',frameId:'',assetId:''};
const pending:CreativeEditorDraft={projectId:'project-one',brief,saved:null,pending:{mode:'create',key:'creative-11111111-1111-4111-8111-111111111111',brief,prepareDigest:'a'.repeat(64)}};
beforeEach(()=>localStorage.clear());
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.useRealTimers();});

it('synchronously retains the exact original identity and restores it after the ordinary draft expiry',()=>{
  vi.useFakeTimers();vi.setSystemTime(new Date('2026-11-01T00:00:00Z'));
  const mounted=renderHook(()=>useProjectDraft({scope,kind:'launch-creative',base:'new',value:null,dirty:false,valid}));
  let retained=false;act(()=>{retained=mounted.result.current.retain(pending,'new')});
  expect(retained).toBe(true);
  const raw=localStorage.getItem('luci.project-draft.v1:'+JSON.stringify([scope,'launch-creative']));
  expect(JSON.parse(raw!).value).toEqual(pending);
  mounted.unmount();vi.setSystemTime(new Date('2027-03-01T00:00:00Z'));
  const reopened=renderHook(()=>useProjectDraft({scope,kind:'launch-creative',base:'new',value:null,dirty:false,valid}));
  expect(reopened.result.current.pending?.value).toEqual(pending);
});

it('refuses request retention when storage cannot confirm the write',()=>{
  const mounted=renderHook(()=>useProjectDraft({scope,kind:'launch-creative',base:'new',value:null,dirty:false,valid}));
  vi.spyOn(Storage.prototype,'setItem').mockImplementation(()=>{});
  let retained=true;act(()=>{retained=mounted.result.current.retain(pending,'new')});
  expect(retained).toBe(false);
  expect(mounted.result.current.error).toMatch(/recovery could not be confirmed/i);
});
