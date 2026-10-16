import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import type { ProjectRun } from '../../api/projectRuns';
import type { ProjectSwarmQueue } from '../../api/swarmQueue';
import { Button } from '../fluid/components/button';
import { Input } from '../ui/input';
import { dependencyFleetCounts, dependencyTasks, matchesDependencyFilter, type DependencyFilter, type DependencyTask } from './swarmDependencyModel';
import './swarm-dependency-navigator.css';

export const DEPENDENCY_PAGE_SIZE = 40;
const RELATION_PAGE_SIZE = 20;
const filters: {id: DependencyFilter; label: string; description: string}[] = [
  {id: 'all', label: 'All tasks', description: 'Every saved task in this run.'},
  {id: 'active', label: 'Working', description: 'Current attempt recorded running and ready, without a start or recovery blocker. Not a live worker observation.'},
  {id: 'queued', label: 'Queued', description: 'Matching queue entries recorded queued. Omitted entries are not counted.'},
  {id: 'waiting', label: 'Waiting', description: 'Not integrated and waiting on dependencies, retained unanswered questions, or a paused or blocked queue entry.'},
  {id: 'review', label: 'Review', description: 'Current attempt explicitly records review-required and is not integrated. Inspect its task controls; independent review assignment and outcome are not available in these counts.'},
  {id: 'attention', label: 'Needs attention', description: 'Saved input, review, recovery, queue or unconfirmed-state signals need inspection.'},
  {id: 'integrated', label: 'Integrated', description: 'Task integration is saved. This does not prove combined checks or verification.'},
  {id: 'unknown', label: 'Unconfirmed', description: 'Attempt, dependency, handoff, integration or queue facts are missing or unrecognized, or a legacy success lacks an explicit review disposition.'},
];
interface Props { run: ProjectRun; queue?: ProjectSwarmQueue; onOpenTask(id: string): void }

/** Local navigation only. Parent owns every read, permission and task action. */
export function SwarmDependencyNavigator(props: Props) {
  // A different run must never inherit selection or a queue snapshot from another run.
  return <Navigator key={`${props.run.projectId}:${props.run.id}`} {...props} />;
}
function Navigator({run, queue, onOpenTask}: Props) {
  const searchId = useId(), filterId = useId(), overviewId = useId();
  const [query, setQuery] = useState(''), [filter, setFilter] = useState<DependencyFilter>('all');
  const [page, setPage] = useState(0), [selectedId, setSelectedId] = useState('');
  const searchInput = useRef<HTMLInputElement>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const requestedFocus = useRef<string | null>(null);
  const model = useMemo(() => dependencyTasks(run, queue), [run, queue]);
  const counts = useMemo(() => dependencyFleetCounts(model), [model]);
  const byId = useMemo(() => new Map(model.map(item => [item.task.id, item])), [model]);
  const normalized = query.trim().toLocaleLowerCase();
  const matches = model.filter(item => (!normalized || `${item.task.title} ${item.task.id}`.toLocaleLowerCase().includes(normalized)) &&
    matchesDependencyFilter(item, filter));
  const lastPage = Math.max(0, Math.ceil(matches.length / DEPENDENCY_PAGE_SIZE) - 1), currentPage = Math.min(page, lastPage);
  const visible = matches.slice(currentPage * DEPENDENCY_PAGE_SIZE, (currentPage + 1) * DEPENDENCY_PAGE_SIZE);
  const selected = byId.get(selectedId) ?? visible[0];
  useEffect(() => {
    const id = requestedFocus.current;
    if (!id) return;
    requestedFocus.current = null;
    buttons.current.get(id)?.focus();
  }, [selectedId, currentPage]);
  function selectRelated(id: string) {
    const index = model.findIndex(item => item.task.id === id);
    if (index < 0) return;
    requestedFocus.current = id;
    setQuery(''); setFilter('all'); setPage(Math.floor(index / DEPENDENCY_PAGE_SIZE)); setSelectedId(id);
  }
  function navigate(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const target = event.key === 'ArrowDown' ? Math.min(index + 1, visible.length - 1) : event.key === 'ArrowUp' ? Math.max(0, index - 1) :
      event.key === 'Home' ? 0 : event.key === 'End' ? visible.length - 1 : -1;
    if (target < 0) return;
    event.preventDefault(); buttons.current.get(visible[target].task.id)?.focus(); setSelectedId(visible[target].task.id);
  }
  return <section className="swarm-navigator" aria-label="Task dependencies and attention">
    <h4>Find a task or blocker</h4>
    <p className="swarm-navigator-note">Saved task and attempt records. Integrated changes still need combined checks; activity and agent reports do not prove verification.</p>
    {run.codingBlocker && <p className="journey-warning">Build attention: {run.codingBlocker.reason}</p>}
    {run.contextFreshness?.status !== 'current' && <p className="journey-warning">Reviewed context: {run.contextFreshness?.reason || 'Freshness unavailable. Inspect the build before new starts.'}</p>}
    <div className="swarm-fleet-overview" role="group" aria-label="Saved fleet overview" aria-describedby={overviewId}>
      {filters.map(item => <Button key={item.id} variant="tertiary" type="button" title={item.description}
        aria-label={`${item.label}: ${counts[item.id]} ${counts[item.id] === 1 ? 'task' : 'tasks'}`}
        aria-pressed={filter === item.id} onClick={() => { setQuery(''); setFilter(item.id); setPage(0); setSelectedId(''); }}>
        <span>{item.label}</span><strong>{counts[item.id]}</strong>
      </Button>)}
    </div>
    <p id={overviewId} className="swarm-navigator-note">Counts cover all {model.length} saved tasks, before search and pagination. Categories overlap; saved activity is not a live observation. Queue counts include only matching entries in this snapshot.</p>
    <p className="swarm-navigator-note">{filters.find(item => item.id === filter)?.description}</p>
    <div className="swarm-navigator-filters">
      <label htmlFor={searchId}>Find by title or task ID<Input ref={searchInput} id={searchId} value={query} onChange={event => { setQuery(event.target.value); setPage(0); setSelectedId(''); }} /></label>
      <label htmlFor={filterId}>Show<select id={filterId} value={filter} onChange={event => { setFilter(event.target.value as DependencyFilter); setPage(0); setSelectedId(''); }}>
        {filters.map(item => <option key={item.id} value={item.id}>{item.label}</option>)}
      </select></label>
    </div>
    <p className="swarm-navigator-note" role="status">{matches.length ? `Showing ${currentPage * DEPENDENCY_PAGE_SIZE + 1}–${currentPage * DEPENDENCY_PAGE_SIZE + visible.length} of ${matches.length} matching tasks` : 'No matching tasks'} · {model.length} saved tasks.
      {matches.length > visible.length ? ' Other matching tasks are on additional pages.' : ''}
      {matches.length < model.length ? ' The filter hides other saved tasks.' : ''}</p>
    <div className="swarm-navigator-layout">
      <div>
        {!matches.length && <div className="swarm-navigator-empty">
          <p>{model.length ? 'No saved tasks match this search and filter. Clear them to see the full task list.' : 'No saved tasks in this run.'}</p>
          {model.length > 0 && <Button variant="secondary" onClick={() => {
            setQuery(''); setFilter('all'); setPage(0); setSelectedId(''); searchInput.current?.focus();
          }}>Clear search and filters</Button>}
        </div>}
        <ul className="swarm-navigator-list" aria-label="Matching saved tasks">
          {visible.map((item, index) => <li key={item.task.id}><button type="button" ref={element => { if (element) buttons.current.set(item.task.id, element); else buttons.current.delete(item.task.id); }}
            aria-label={`Select task ${item.task.title} (${item.task.id})`} aria-pressed={selected?.task.id === item.task.id} onClick={() => setSelectedId(item.task.id)} onKeyDown={event => navigate(event, index)}>
            <strong>{item.task.title}</strong><code>{item.task.id}</code>
            <span>{item.activity}{item.integrated ? ' · Integrated' : ''}</span>
            {item.attention.length > 0 && <span className="swarm-navigator-attention">{item.attention.join(' · ')}</span>}
            {!item.integrated && item.waitingOn.length > 0 && <span>Waiting on {item.waitingOn.length} dependency/dependencies</span>}
          </button></li>)}
        </ul>
        {matches.length > 0 && <div className="swarm-navigator-pages" aria-label="Task pages">
          <Button variant="tertiary" disabled={currentPage === 0} onClick={() => { setPage(currentPage - 1); setSelectedId(''); }}>Previous task page</Button>
          <span>Page {currentPage + 1} of {lastPage + 1}</span>
          <Button variant="tertiary" disabled={currentPage >= lastPage} onClick={() => { setPage(currentPage + 1); setSelectedId(''); }}>Next task page</Button>
        </div>}
      </div>
      {selected && <section className="swarm-navigator-detail" aria-label="Selected task dependencies">
        <h5>{selected.task.title}</h5><code>{selected.task.id}</code>
        <p>{selected.activity}. {selected.integrated ? 'Integrated; inspect combined build checks separately.' : selected.task.integration ? 'Integration is pending or unconfirmed.' : 'Integration is not recorded.'}</p>
        {selected.attention.length > 0 && <ul>{selected.attention.map(value => <li key={value}>{value}</li>)}</ul>}
        <Button variant="secondary" onClick={() => onOpenTask(selected.task.id)}>Go to task controls</Button>
        <Relations key={`${selected.task.id}:dependencies`} label="Depends on" ids={selected.task.dependsOn} byId={byId} onSelect={selectRelated} />
        <Relations key={`${selected.task.id}:dependents`} label="Needed by" ids={selected.dependents} byId={byId} onSelect={selectRelated} />
      </section>}
    </div>
    <p className="swarm-navigator-note">Use ↑ / ↓ or Home / End within a task page. Select a dependency to find its task; Go to task controls opens the existing scope and focuses its heading.</p>
    <p className="swarm-navigator-note">{queue ? `Queue snapshot: ${queue.observedAt}. ${queue.hasMore ? 'Additional queue entries are omitted; absence here does not prove a task is unqueued.' : 'Queue activity is separate from attempt evidence.'}` : 'Queue observation is not yet available here.'}</p>
  </section>;
}
function Relations({label, ids, byId, onSelect}: {label: string; ids: string[]; byId: Map<string, DependencyTask>; onSelect(id: string): void}) {
  const [page, setPage] = useState(0);
  const unique = [...new Set(ids)], lastPage = Math.max(0, Math.ceil(unique.length / RELATION_PAGE_SIZE) - 1), currentPage = Math.min(page, lastPage);
  return <div className="swarm-navigator-relations"><h6>{label} ({unique.length})</h6>
    {!unique.length ? <p>None recorded.</p> : <ul>{unique.slice(currentPage * RELATION_PAGE_SIZE, (currentPage + 1) * RELATION_PAGE_SIZE).map(id => {
      const item = byId.get(id);
      return <li key={id}>{item ? <Button variant="tertiary" onClick={() => onSelect(id)}>{item.task.title} ({id}) · {item.integrated ? 'integrated' : 'not integrated'}</Button> : <span>Unavailable task ({id}) — dependency unresolved</span>}</li>;
    })}</ul>}
    {unique.length > RELATION_PAGE_SIZE && <div className="swarm-navigator-pages"><Button variant="tertiary" disabled={!currentPage} onClick={() => setPage(currentPage - 1)}>Previous {label.toLowerCase()} page</Button><span>Page {currentPage + 1} of {lastPage + 1}; other relationships omitted from this page</span><Button variant="tertiary" disabled={currentPage >= lastPage} onClick={() => setPage(currentPage + 1)}>Next {label.toLowerCase()} page</Button></div>}
  </div>;
}
