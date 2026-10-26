import { useCallback, useEffect, useRef, useState } from 'react';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { ProjectRequired } from '../journey/shared';
import { DefinitionWorkspace } from '../definition/DefinitionWorkspace';
import { DiscoverDocuments } from '../discover/DiscoverDocuments';
import './discover-workspace.css';

export const BriefEditor = DefinitionWorkspace;
export function DiscoverLens() {
  return <ProjectRequired>{(id) => <DiscoverProject key={id} projectId={id} />}</ProjectRequired>;
}

function DiscoverProject({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  return <DiscoverWorkspace key={`${scope}:${projectId}`} projectId={projectId} />;
}

function DiscoverWorkspace({ projectId }: { projectId: string }) {
  const [newDefinition, setNewDefinition] = useState<boolean | null>(null);
  const [noSources, setNoSources] = useState<boolean | null>(null);
  const [layout, setLayout] = useState<'idea' | 'sources' | null>(null);
  const definition = useRef<HTMLDetailsElement>(null);
  const sources = useRef<HTMLDetailsElement>(null);
  const summary = useRef<HTMLElement>(null);
  const chosen = useRef(false);
  const introduced = useRef(false);
  const ideaFirst = layout === 'idea';
  useEffect(() => {
    if (layout !== null || newDefinition === null || noSources === null) return;
    // Choose the initial hierarchy once. Saving, refreshing or attaching a source
    // must not move an editor underneath the person already using it.
    setLayout(!chosen.current && newDefinition && noSources ? 'idea' : 'sources');
  }, [layout, newDefinition, noSources]);
  useEffect(() => {
    if (!ideaFirst || introduced.current) return;
    introduced.current = true;
    if (chosen.current) return;
    if (definition.current) definition.current.open = true;
    if (sources.current) sources.current.open = false;
  }, [ideaFirst]);
  const revealDefinition = useCallback(() => {
    const disclosure = definition.current;
    if (!disclosure) return;
    chosen.current = true;
    disclosure.open = true;
    disclosure.scrollIntoView?.({ block: 'start' });
    summary.current?.focus({ preventScroll: true });
  }, []);
  const definitionSection = <details key="definition" ref={definition} className="discover-definition" onFocusCapture={() => { chosen.current = true; }} onClickCapture={() => { chosen.current = true; }}>
    <summary ref={summary}>Edit the structured project definition</summary>
    <DefinitionWorkspace projectId={projectId} onContextEdit={revealDefinition} onInitialDefinition={setNewDefinition} ideaFirst={ideaFirst} />
  </details>;
  const sourcesSection = <details key="sources" ref={sources} className="discover-sources" open onFocusCapture={() => { chosen.current = true; }} onClickCapture={() => { chosen.current = true; }}>
    <summary>Sources <span className="journey-muted">Documents and supporting evidence</span></summary>
    <DiscoverDocuments projectId={projectId} onEmptySources={setNoSources} />
  </details>;
  return <section className={`discover-workspace${ideaFirst ? ' discover-workspace--idea-first' : ''}`} aria-label="Discover workspace" tabIndex={0}>
    {ideaFirst ? [definitionSection, sourcesSection] : [sourcesSection, definitionSection]}
  </section>;
}
