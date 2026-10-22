import { useCallback, useRef } from 'react';
import { ProjectRequired } from '../journey/shared';
import { DefinitionWorkspace } from '../definition/DefinitionWorkspace';
import { DiscoverDocuments } from '../discover/DiscoverDocuments';
import './discover-workspace.css';

export const BriefEditor = DefinitionWorkspace;
export function DiscoverLens() {
  return <ProjectRequired>{(id) => <DiscoverProject key={id} projectId={id} />}</ProjectRequired>;
}

function DiscoverProject({ projectId }: { projectId: string }) {
  const definition = useRef<HTMLDetailsElement>(null);
  const summary = useRef<HTMLElement>(null);
  const revealDefinition = useCallback(() => {
    const disclosure = definition.current;
    if (!disclosure) return;
    disclosure.open = true;
    disclosure.scrollIntoView?.({ block: 'start' });
    summary.current?.focus({ preventScroll: true });
  }, []);
  return <section className="discover-workspace" aria-label="Discover workspace" tabIndex={0}>
    <DiscoverDocuments projectId={projectId} />
    <details ref={definition}>
      <summary ref={summary}>Edit the structured project definition</summary>
      <DefinitionWorkspace projectId={projectId} onContextEdit={revealDefinition} />
    </details>
  </section>;
}
