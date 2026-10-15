import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { SpecificationEvidenceLinks } from './SpecificationEvidenceLinks';
import type { DiscoverySourceLink } from '../../api/discoveryDocuments';

const link:DiscoverySourceLink={documentId:`doc-${'a'.repeat(64)}`,originalDigest:'b'.repeat(64),extractionRevision:1,page:2,segmentId:'p2-line1',textDigest:'c'.repeat(64),targetKind:'requirement',targetId:'requirement-removed'};
afterEach(()=>cleanup());
it('keeps a removed target visible until an explicit edit removes only its draft reference',()=>{
  const onChange=vi.fn();
  render(<SpecificationEvidenceLinks specification={{requirements:[],glossary:[],decisions:[],evidence:[link]}} disabled={false} onChange={onChange}/>);
  expect(screen.getByRole('status').textContent).toContain('removed context record');
  expect(screen.getByText(link.originalDigest)).toBeTruthy();expect(onChange).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button',{name:'Remove source reference 1 from this draft'}));
  expect(onChange).toHaveBeenCalledWith([]);expect(link.targetId).toBe('requirement-removed');
});
it('preserves legacy definitions and prevents reference edits for readers',()=>{
  const onChange=vi.fn();const view=render(<SpecificationEvidenceLinks specification={{requirements:[],glossary:[],decisions:[]}} disabled onChange={onChange}/>);
  expect(screen.getByText(/no private document references/)).toBeTruthy();
  view.rerender(<SpecificationEvidenceLinks specification={{requirements:[],glossary:[],decisions:[],evidence:[link]}} disabled onChange={onChange}/>);
  fireEvent.click(screen.getByRole('button',{name:'Remove source reference 1 from this draft'}));expect(onChange).not.toHaveBeenCalled();
});
it('repairs only the draft target while preserving the exact original and extraction identity',()=>{
  const onChange=vi.fn();render(<SpecificationEvidenceLinks specification={{requirements:[],glossary:[],decisions:[],evidence:[link]}} disabled={false} onChange={onChange}/>);
  fireEvent.change(screen.getByRole('combobox',{name:'Source reference 1 target'}),{target:{value:'purpose:'}});
  expect(onChange).toHaveBeenCalledWith([{...link,targetKind:'purpose',targetId:''}]);
});
it('exposes malformed retained references without silently dropping them or attempting an effect',()=>{
  const onChange=vi.fn();render(<SpecificationEvidenceLinks specification={{requirements:[],glossary:[],decisions:[],evidence:{} as DiscoverySourceLink[]}} disabled={false} onChange={onChange}/>);
  expect(screen.getByRole('alert').textContent).toContain('Preserve this draft');expect(onChange).not.toHaveBeenCalled();
});
