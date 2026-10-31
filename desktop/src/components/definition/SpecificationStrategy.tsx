import { useState } from 'react';
import type { Requirement } from '../../api/definition';
import { emptySpecificationStrategy, strategyProblems, type SpecificationStrategy as Strategy, type StrategyAssumption } from '../../api/specificationStrategy';
import { Button } from '../fluid/components/button';

const id = (kind: string) => `${kind}-${crypto.randomUUID().slice(0, 12)}`;
function Links({label, selected, choices, onChange}: {label: string; selected: string[]; choices: {id: string; label: string}[]; onChange(value: string[]): void}) {
  return <fieldset className="definition-links"><legend>{label}</legend>
    {!choices.length && !selected.length && <p className="journey-muted">Add a {label === 'Sources' ? 'source below' : 'requirement in the specification'} to link it here.</p>}
    {choices.map(choice => <label key={choice.id}><input type="checkbox" checked={selected.includes(choice.id)} onChange={event => onChange(event.target.checked ? [...selected, choice.id] : selected.filter(value => value !== choice.id))}/>{choice.label || choice.id}</label>)}
    {selected.filter(value => !choices.some(choice => choice.id === value)).map(value => <label key={value}><input type="checkbox" checked onChange={() => onChange(selected.filter(item => item !== value))}/>{value} · unavailable. Uncheck to remove this link.</label>)}
  </fieldset>;
}

export function SpecificationStrategy({strategy, requirements, onChange}: {strategy?: Strategy; requirements: Requirement[]; onChange(value: Strategy): void}) {
  const [mounted, setMounted] = useState(false);
  const value = strategy ?? emptySpecificationStrategy();
  const patch = (changes: Partial<Strategy>) => onChange({...value, ...changes});
  const errors = strategyProblems(strategy, requirements.map(row => row.id));
  const sources = value.evidence.map(row => ({id: row.id, label: row.source}));
  const outcomes = requirements.map(row => ({id: row.id, label: `${row.id} · ${row.title}`}));
  return <details className="definition-group" onToggle={event => { if (event.currentTarget.open) setMounted(true); }}>
    <summary><span>Product strategy</span><span className="journey-muted">Customer, assumptions and success</span></summary>
    {mounted && <div className="definition-group-body">
      <p>Explain why this product should exist, what you still need to learn and how you will judge success. Luci carries this strategy into design, agent tasks and your go-to-market plan.</p>
      {errors.map(error => <p key={error} role="alert" className="journey-warning">{error}</p>)}
      <label className="luci-field">First customer and their job<textarea rows={2} value={value.customer} onChange={event => patch({customer: event.target.value})} placeholder="Who needs this most, and what are they trying to accomplish?" maxLength={12000}/></label>
      <label className="luci-field">Problem and current workaround<textarea rows={2} value={value.problem} onChange={event => patch({problem: event.target.value})} placeholder="What is difficult today? What do they use instead?" maxLength={12000}/></label>
      <label className="luci-field">Why choose this product?<textarea rows={2} value={value.valueProposition} onChange={event => patch({valueProposition: event.target.value})} placeholder="The useful difference you intend to deliver" maxLength={12000}/></label>
      <h3>Alternatives</h3>
      <p className="journey-muted">Include competing products, manual work and doing nothing. Uncited comparisons remain hypotheses.</p>
      {value.alternatives.map((row, index) => <fieldset className="definition-record" key={row.id}><legend>Alternative {index + 1}</legend>
        <label className="luci-field">Alternative name<input value={row.name} maxLength={500} onChange={event => patch({alternatives: value.alternatives.map(item => item.id === row.id ? {...item, name: event.target.value} : item)})}/></label>
        <label className="luci-field">Difference and tradeoff<textarea rows={2} value={row.difference} maxLength={12000} onChange={event => patch({alternatives: value.alternatives.map(item => item.id === row.id ? {...item, difference: event.target.value} : item)})}/></label>
        <Links label="Sources" selected={row.evidenceIds} choices={sources} onChange={evidenceIds => patch({alternatives: value.alternatives.map(item => item.id === row.id ? {...item, evidenceIds} : item)})}/>
        <Button type="button" variant="secondary" onClick={() => patch({alternatives: value.alternatives.filter(item => item.id !== row.id)})}>Remove alternative {index + 1}</Button>
      </fieldset>)}
      <Button type="button" variant="secondary" disabled={value.alternatives.length >= 40} onClick={() => patch({alternatives: [...value.alternatives, {id: id('alternative'), name: '', difference: '', evidenceIds: []}]})}>Add alternative</Button>
      <h3>Assumptions to resolve</h3>
      {value.assumptions.map((row, index) => {
        const update = (changes: Partial<StrategyAssumption>) => patch({assumptions: value.assumptions.map(item => item.id === row.id ? {...item, ...changes} : item)});
        return <fieldset className="definition-record" key={row.id}><legend>Assumption {index + 1}</legend>
          <label className="luci-field">Claim<textarea rows={2} value={row.claim} maxLength={12000} onChange={event => update({claim: event.target.value})}/></label>
          <label className="luci-field">Finding<select value={row.status} onChange={event => update({status: event.target.value as StrategyAssumption['status']})}><option value="untested">Untested</option><option value="supported">Supported by linked observations</option><option value="contradicted">Contradicted by linked observations</option></select></label>
          <label className="luci-field">How to learn<textarea rows={2} value={row.experiment} maxLength={12000} onChange={event => update({experiment: event.target.value})} placeholder="Interview, experiment or decision that resolves this uncertainty"/></label>
          <Links label="Sources" selected={row.evidenceIds} choices={sources} onChange={evidenceIds => update({evidenceIds})}/>
          <Links label="Affected requirements" selected={row.requirementIds} choices={outcomes} onChange={requirementIds => update({requirementIds})}/>
          <Button type="button" variant="secondary" onClick={() => patch({assumptions: value.assumptions.filter(item => item.id !== row.id)})}>Remove assumption {index + 1}</Button>
        </fieldset>;
      })}
      <Button type="button" variant="secondary" disabled={value.assumptions.length >= 60} onClick={() => patch({assumptions: [...value.assumptions, {id: id('assumption'), claim: '', status: 'untested', experiment: '', evidenceIds: [], requirementIds: []}]})}>Add assumption</Button>
      <h3>Sources and observations</h3>
      <p className="journey-muted">Record what the source actually says. Saving a citation does not fetch it or verify its claim.</p>
      {value.evidence.map((row, index) => <fieldset className="definition-record" key={row.id}><legend>Source {index + 1}</legend>
        {([{key: 'source', label: 'Source or interview', max: 500}, {key: 'url', label: 'Source URL (optional)', max: 2048}] as const).map(field => <label className="luci-field" key={field.key}>{field.label}<input value={row[field.key]} readOnly={field.key === 'url' && !!row.research} maxLength={field.max} onChange={event => patch({evidence: value.evidence.map(item => item.id === row.id ? {...item, [field.key]: event.target.value} : item)})}/></label>)}
        <label className="luci-field">Observed on (optional)<input type="date" readOnly={!!row.research} value={row.observedAt.slice(0, 10)} onChange={event => patch({evidence: value.evidence.map(item => item.id === row.id ? {...item, observedAt: event.target.value ? `${event.target.value}T00:00:00Z` : ''} : item)})}/></label>
        {row.research && <details><summary>Collected source attribution and exact quotes</summary><p>Fetched {row.research.fetchedAt}; raw body SHA256 {row.research.bodySha256}; retained extracted text SHA256 {row.research.textSha256}. {row.research.omittedTextBytes > 0 ? `Partial capture: ` + row.research.omittedTextBytes + ` further text bytes omitted.` : 'Within the bounded extracted text.'}</p>{row.research.quotes.map(quote => <blockquote key={quote} style={{overflowWrap:'anywhere'}}>{quote}</blockquote>)}<p>Fetched URL and timestamp are fixed attribution. Your observation below is editable; it does not change what the source said.</p></details>}
        <label className="luci-field">Observation<textarea rows={3} value={row.observation} maxLength={12000} onChange={event => patch({evidence: value.evidence.map(item => item.id === row.id ? {...item, observation: event.target.value} : item)})}/></label>
        <Button type="button" variant="secondary" onClick={() => patch({evidence: value.evidence.filter(item => item.id !== row.id)})}>Remove source {index + 1}</Button>
      </fieldset>)}
      <Button type="button" variant="secondary" disabled={value.evidence.length >= 80} onClick={() => patch({evidence: [...value.evidence, {id: id('source'), source: '', url: '', observation: '', observedAt: ''}]})}>Add source</Button>
      <h3>Success measures</h3>
      <p className="journey-muted">Describe user outcomes, not agent activity. Leave an unknown baseline empty rather than treating it as zero.</p>
      {value.metrics.map((row, index) => <fieldset className="definition-record" key={row.id}><legend>Success measure {index + 1}</legend>
        {([{key: 'name', label: 'Measure and unit', max: 500}, {key: 'baseline', label: 'Current baseline (if known)', max: 2000}, {key: 'target', label: 'Target outcome', max: 2000}, {key: 'window', label: 'Time window', max: 2000}, {key: 'measurement', label: 'How to measure it', max: 12000}] as const).map(field => <label className="luci-field" key={field.key}>{field.label}<input value={row[field.key]} maxLength={field.max} onChange={event => patch({metrics: value.metrics.map(item => item.id === row.id ? {...item, [field.key]: event.target.value} : item)})}/></label>)}
        <Links label="Affected requirements" selected={row.requirementIds} choices={outcomes} onChange={requirementIds => patch({metrics: value.metrics.map(item => item.id === row.id ? {...item, requirementIds} : item)})}/>
        <Button type="button" variant="secondary" onClick={() => patch({metrics: value.metrics.filter(item => item.id !== row.id)})}>Remove success measure {index + 1}</Button>
      </fieldset>)}
      <Button type="button" variant="secondary" disabled={value.metrics.length >= 40} onClick={() => patch({metrics: [...value.metrics, {id: id('metric'), name: '', baseline: '', target: '', window: '', measurement: '', requirementIds: []}]})}>Add success measure</Button>
    </div>}
  </details>;
}
