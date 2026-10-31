import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { ResearchEvidence } from './ResearchEvidence';
import { researchApi } from '../../api/research';
const environment=vi.hoisted(()=>({secrets:true,drafts:vi.fn()}));
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=> 'alice/workspace/project'}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>({canRead:true,canEdit:true,permissions:{secrets:environment.secrets}})}));
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:(...args:unknown[])=>environment.drafts(...args)}));
afterEach(()=>{cleanup();vi.restoreAllMocks();environment.secrets=true;environment.drafts.mockClear();});
it('adds only reviewed exact collected quotes to the ordinary unsaved definition',async()=>{
  const capture={id:'research-original',requestedUrl:'https://public.example/page',url:'https://public.example/page',title:'A source',fetchedAt:'2026-10-31T00:00:00Z',bodySha256:'a'.repeat(64),textSha256:'b'.repeat(64),text:'A reported customer problem.',omittedTextBytes:100};
  const collect=vi.spyOn(researchApi,'collect').mockResolvedValue(capture),change=vi.fn(),user=userEvent.setup();
  render(<ResearchEvidence projectId="project" disabled={false} onChange={change}/>);
  await user.click(screen.getByText('Research public sources'));
  await user.type(screen.getByLabelText('Or choose a public source URL'),capture.url);
  await user.click(screen.getByRole('button',{name:'Collect public page'}));
  await screen.findByText(/Partial extracted text/);expect(change).not.toHaveBeenCalled();expect(collect).toHaveBeenCalledTimes(1);
  await user.type(screen.getByLabelText('Select a verbatim quote by pasting it from the source text'),'Invented claim');
  expect(environment.drafts.mock.calls[environment.drafts.mock.calls.length-1][1]).toBe(true);
  expect(screen.getByRole('button',{name:'Select exact quote'}).matches(':disabled')).toBe(true);
  await user.clear(screen.getByLabelText('Select a verbatim quote by pasting it from the source text'));
  await user.type(screen.getByLabelText('Select a verbatim quote by pasting it from the source text'),'reported customer problem');
  await user.click(screen.getByRole('button',{name:'Select exact quote'}));await user.click(screen.getByRole('button',{name:'Add selected quotes to definition draft'}));
  await waitFor(()=>expect(change).toHaveBeenCalledTimes(1));expect(change.mock.calls[0][0].evidence[0].research.quotes).toEqual(['reported customer problem']);expect(change.mock.calls[0][0].evidence[0].research.omittedTextBytes).toBe(100);
});
it('guards an unsaved search key without retaining credentials and supports explicit discard',async()=>{
 vi.spyOn(researchApi,'connection').mockResolvedValue({provider:'brave',revision:0,configured:false});const user=userEvent.setup();render(<ResearchEvidence projectId="project" disabled={false} onChange={vi.fn()}/>);
 await user.click(screen.getByText('Research public sources'));await user.click(screen.getByText('Search connection'));await user.click(screen.getByRole('button',{name:'Read search connection metadata'}));
 await user.type(await screen.findByLabelText('New Brave Search API key'),'unsaved-generated-test-key');
 const registration=environment.drafts.mock.calls[environment.drafts.mock.calls.length-1];expect(registration[1]).toBe(true);expect(JSON.stringify(registration)).not.toContain('unsaved-generated-test-key');
 const recovery=registration[4] as {retained:boolean;discard():void};expect(recovery.retained).toBe(false);await act(async()=>recovery.discard());expect((screen.getByLabelText('New Brave Search API key') as HTMLInputElement).value).toBe('');
});
it('withholds late credentialed metadata on Secrets revocation while public URL collection remains available',async()=>{
 let resolve!:(value:{provider:'brave';revision:number;configured:boolean})=>void;
 vi.spyOn(researchApi,'connection').mockImplementationOnce(()=>new Promise(done=>{resolve=done;}));const user=userEvent.setup(),mounted=render(<ResearchEvidence projectId="project" disabled={false} onChange={vi.fn()}/>);
 await user.click(screen.getByText('Research public sources'));await user.click(screen.getByText('Search connection'));await user.click(screen.getByRole('button',{name:'Read search connection metadata'}));environment.secrets=false;mounted.rerender(<ResearchEvidence projectId="project" disabled={false} onChange={vi.fn()}/>);
 await act(async()=>resolve({provider:'brave',revision:99,configured:true}));expect(screen.queryByLabelText('New Brave Search API key')).toBeNull();expect(screen.getByRole('button',{name:'Find public source candidates'}).matches(':disabled')).toBe(true);
 await user.type(screen.getByLabelText('Or choose a public source URL'),'https://public.example/page');expect(screen.getByRole('button',{name:'Collect public page'}).matches(':disabled')).toBe(false);
});
it('opens a review draft without sending a model request or saving a definition',async()=>{
  const receive=vi.fn(),change=vi.fn(),user=userEvent.setup();window.addEventListener('luci:orchestrator-open',receive);
  try{render(<ResearchEvidence projectId="project" disabled={false} onChange={change}/>);await user.click(screen.getByText('Research public sources'));await user.click(screen.getByRole('button',{name:'Ask Luci to review saved evidence and roadmap'}));expect(receive).toHaveBeenCalledTimes(1);expect((receive.mock.calls[0][0] as CustomEvent).detail.message).toContain('Propose one compatible specification amendment for review');expect(change).not.toHaveBeenCalled();}finally{window.removeEventListener('luci:orchestrator-open',receive);}
});
