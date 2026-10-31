import { useEffect, useRef, useState } from 'react';
import type { DesignPlan } from '../../api/designPlan';
import { captureComponent, componentLibraryErrors, componentTokenProperties, instantiateComponent, parseComponentPackage, prepareComponentImport, resolveComponentStyle, type DesignComponentTemplate } from '../../api/designComponents';
import type { DesignSceneFrame, DesignSceneNode, EditableDesignScene } from '../../api/designScene';
import { Button } from '../fluid/components/button';
import './design-components.css';

type ImportPreview = ReturnType<typeof prepareComponentImport> & { basis: string };
interface Props {
  plan: DesignPlan; frame?: DesignSceneFrame; node?: DesignSceneNode; canModify: boolean; canRead: boolean;
  onDocumentChange(value: Partial<Pick<DesignPlan, 'componentLibrary' | 'tokens' | 'scene'>>): void;
  onInsert(node: DesignSceneNode): void;
  onStyle(value: Pick<DesignSceneNode, 'style' | 'componentId'>): void;
  onScene(scene: EditableDesignScene): void;
}
export function DesignComponentLibrary({plan, frame, node, canModify, canRead, onDocumentChange, onInsert, onStyle, onScene}: Props) {
  const [selected, setSelected] = useState(''), [notice, setNotice] = useState(''), [pending, setPending] = useState<ImportPreview | null>(null);
  const generation = useRef(0), input = useRef<HTMLInputElement>(null);
  const live = useRef({canModify, canRead, plan}); live.current = {canModify, canRead, plan};
  const readable = plan.componentLibrary === undefined || plan.componentLibrary?.version === 1 && Array.isArray(plan.componentLibrary.items) && plan.componentLibrary.items.every(value => value && typeof value === 'object' && typeof value.id === 'string' && typeof value.name === 'string' && Array.isArray(value.bindings) && value.bindings.every(binding => binding && typeof binding.property === 'string' && typeof binding.token === 'string'));
  const library = readable && plan.componentLibrary ? plan.componentLibrary : {version: 1 as const, items: []};
  const item = library.items.find(value => value.id === selected) ?? library.items[0];
  const basis = JSON.stringify({componentLibrary: plan.componentLibrary, tokens: plan.tokens});
  const errors = componentLibraryErrors(plan.componentLibrary, plan.tokens);
  const linked = plan.scene?.frames.flatMap(value => value.nodes).filter(value => value.componentId === item?.id) ?? [];
  const unlocked = linked.filter(value => !value.locked);
  useEffect(() => () => { generation.current++; }, []);
  useEffect(() => { generation.current++; setPending(null); setNotice(''); }, [plan.projectId, canRead, canModify]);
  function safely(action: () => void) { try { action(); } catch (cause) { setNotice(cause instanceof Error ? cause.message : 'The component change could not be applied.'); } }
  function update(value: Partial<DesignComponentTemplate>) {
    if (!live.current.canModify || !item) return;
    onDocumentChange({componentLibrary: {...library, items: library.items.map(current => current.id === item.id ? {...current, ...value} : current)}});
  }
  async function importFile(file: File | undefined) {
    if (!file || !live.current.canModify) return;
    const ticket = ++generation.current;
    setPending(null); setNotice('');
    try {
      if (file.size > 128 * 1024) throw new Error('Choose a component package smaller than 128 KiB.');
      const raw = await file.text();
      if (ticket !== generation.current || !live.current.canModify) return;
      const current = live.current.plan;
      setPending({...prepareComponentImport(current, parseComponentPackage(raw)), basis: JSON.stringify({componentLibrary: current.componentLibrary, tokens: current.tokens})});
    } catch (cause) { if (ticket === generation.current && live.current.canModify) setNotice(cause instanceof Error ? cause.message : 'The file could not be read.'); }
  }
  function exportFile() {
    if (!live.current.canRead || errors.length) return;
    safely(() => {
      const raw = JSON.stringify({format: 'luci-design-components', version: 1, components: library, tokens: plan.tokens}, null, 2);
      parseComponentPackage(raw);
      const url = URL.createObjectURL(new Blob([raw], {type: 'application/json'}));
      const anchor = document.createElement('a'); anchor.href = url; anchor.download = 'luci-design-components.json';
      try { anchor.click(); setNotice('Component package download requested. It includes your component text and design tokens.'); }
      finally { window.setTimeout(() => URL.revokeObjectURL(url), 1000); }
    });
  }
  if (!canRead) return null;
  if (!readable) return <p role="alert">This component library uses an unsupported structure. Preserve its saved history before editing it.</p>;
  return <details className="design-component-library">
    <summary>Reusable components · {library.items.length}</summary>
    <p>Save a selected control once, then reuse it across your screens. The library travels with the design handoff that agents read.</p>
    <div className="design-component-actions">
      <Button type="button" variant="secondary" disabled={!canModify || !node || node.kind === 'image' || library.items.length >= 64} onClick={() => safely(() => {
        if (!node || !live.current.canModify) return;
        const captured = captureComponent(node, library), next = {...library, items: [...library.items, captured]};
        const invalid = componentLibraryErrors(next, plan.tokens); if (invalid.length) throw new Error(invalid[0]);
        onDocumentChange({componentLibrary: next}); setSelected(captured.id); setNotice('Component added to this draft. Save the design to retain it.');
      })}>Save selected as component</Button>
      <label className="luci-field">Component<select value={item?.id ?? ''} onChange={event => setSelected(event.target.value)}>
        {!library.items.length && <option value="">Select a shape or control on the canvas first</option>}{library.items.map(value => <option key={value.id} value={value.id}>{value.name || 'Unnamed component'} · {value.kind}</option>)}
      </select></label>
      <Button type="button" variant="secondary" disabled={!canModify || !item || !frame || !!errors.length} onClick={() => safely(() => { if (item && frame && live.current.canModify) onInsert(instantiateComponent(item, frame, plan.tokens)); })}>Insert in current frame</Button>
    </div>
    {item && <fieldset disabled={!canModify} className="design-component-fields"><legend>{item.name || 'Component'} settings</legend>
      <label className="luci-field">Name<input value={item.name} maxLength={100} onChange={event => update({name: event.target.value})} /></label>
      <label className="luci-field">When to use it<textarea rows={2} value={item.description} maxLength={1500} onChange={event => update({description: event.target.value})} placeholder="For the primary action on a form. Describe focus, disabled and loading states for the builder." /></label>
      <label className="luci-field">Default text<input value={item.text} maxLength={2000} onChange={event => update({text: event.target.value})} /></label>
      <details><summary>Connect design tokens</summary><p>Use colours such as #123456, and whole pixel sizes such as 16. Edit token values in the implementation handoff.</p>
        <div className="design-component-bindings">{componentTokenProperties.map(property => <label className="luci-field" key={property}>{({fill:'Background', stroke:'Border colour', color:'Text colour', fontSize:'Text size', radius:'Corner radius', padding:'Padding', strokeWidth:'Border width'})[property]}
          <select value={item.bindings.find(binding => binding.property === property)?.token ?? ''} onChange={event => update({bindings: [...item.bindings.filter(binding => binding.property !== property), ...(event.target.value ? [{property, token: event.target.value}] : [])]})}>
            <option value="">Keep component value</option>{item.bindings.some(binding => binding.property === property && !plan.tokens.some(token => token.name === binding.token)) && <option value={item.bindings.find(binding => binding.property === property)?.token}>Missing token</option>}{plan.tokens.map(token => <option key={token.name} value={token.name}>{token.name} · {token.value}</option>)}
          </select></label>)}</div>
      </details>
      <div className="design-component-actions">
        <Button type="button" variant="secondary" disabled={!node || node.kind !== item.kind} onClick={() => { if (node && live.current.canModify) { update({style: {...node.style}, width: node.width, height: node.height}); setNotice('Template defaults updated from the selected control. Existing copies change only when you choose Update.'); } }}>Use selected as template defaults</Button>
        <Button type="button" variant="secondary" disabled={!node || node.locked || node.kind !== item.kind || !!errors.length} onClick={() => safely(() => { if (live.current.canModify) onStyle({style: resolveComponentStyle(item, plan.tokens), componentId: item.id}); })}>Apply style to selected</Button>
        <Button type="button" variant="secondary" disabled={!unlocked.length || !!errors.length} onClick={() => safely(() => {
          if (!live.current.canModify || !plan.scene) return;
          const style = resolveComponentStyle(item, plan.tokens);
          onScene({...plan.scene, frames: plan.scene.frames.map(current => ({...current, nodes: current.nodes.map(value => value.componentId === item.id && !value.locked ? {...value, style: {...style}} : value)}))});
          setNotice(`Updated styles on ${unlocked.length} unlocked copies. Text, position, size and interactions stay as you designed them.`);
        })}>Update {unlocked.length} unlocked copies</Button>
        <Button type="button" variant="tertiary" onClick={() => {
          if (!live.current.canModify) return;
          const scene = plan.scene && {...plan.scene, frames: plan.scene.frames.map(current => ({...current, nodes: current.nodes.map(value => { if (value.componentId !== item.id) return value; const {componentId: _link, ...copy} = value; return copy; })}))};
          onDocumentChange({componentLibrary: {...library, items: library.items.filter(value => value.id !== item.id)}, ...(scene ? {scene} : {})});
          setNotice('Template removed. Its existing copies keep their appearance and are now independent.');
        }}>Remove template; keep its copies</Button>
      </div>
      <p className="journey-muted">{linked.length} linked copies. Changes to tokens or templates take effect on existing copies only when you choose Update. Locked copies are preserved.</p>
    </fieldset>}
    <details><summary>Reuse between projects</summary><p>Export component text, styles and tokens as a file. Import reviews an additive draft; matching token names with different values are renamed. It contains no images, executable code or project conversations.</p>
      <div className="design-component-actions"><Button type="button" variant="secondary" disabled={!library.items.length || !!errors.length} onClick={exportFile}>Export component package</Button>
        <Button type="button" variant="secondary" disabled={!canModify} onClick={() => input.current?.click()}>Choose component package</Button>
        <input ref={input} type="file" accept=".json,application/json" hidden onChange={event => { const file = event.target.files?.[0]; event.target.value = ''; void importFile(file); }} />
      </div>
      {pending && <section className="design-component-import" aria-label="Review component import"><h4>Review import</h4><p>Add {pending.addedComponents} components and {pending.addedTokens} tokens. Your existing styles stay unchanged.</p>
        <ul>{pending.componentLibrary.items.slice(library.items.length).map(value => <li key={value.id}>{value.name} · {value.kind} · {value.width} × {value.height}<p>{value.description}</p></li>)}</ul>
        {!!pending.renamed.length && <><p>Token names that will change:</p><ul>{pending.renamed.map(name => <li key={name}>{name}</li>)}</ul></>}
        {pending.basis !== basis && <p role="status">The design changed after this review. Choose the file again to review the current merge.</p>}
        <Button type="button" disabled={!canModify || pending.basis !== basis} onClick={() => {
          if (!live.current.canModify || pending.basis !== JSON.stringify({componentLibrary: live.current.plan.componentLibrary, tokens: live.current.plan.tokens})) return;
          onDocumentChange({componentLibrary: pending.componentLibrary, tokens: pending.tokens}); setPending(null); setNotice('Components added to this draft. Save and review the design through the usual handoff.');
        }}>Add to design draft</Button><Button type="button" variant="tertiary" onClick={() => setPending(null)}>Cancel import</Button>
      </section>}
    </details>
    {!!errors.length && <p role="alert">{errors[0]}</p>}{notice && <p role="status">{notice}</p>}
  </details>;
}
