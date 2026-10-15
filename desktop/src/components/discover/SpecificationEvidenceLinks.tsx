import type { ProjectSpecification } from '../../api/definition';
import { discoverySourceLinksValid, type DiscoverySourceLink } from '../../api/discoveryDocuments';
import { Button } from '../fluid/components/button';

/** Controlled field editor for the existing specification draft; never saves or approves. */
export function SpecificationEvidenceLinks({specification,disabled,onChange,onInspect}:{specification:Pick<ProjectSpecification,'evidence'|'requirements'|'glossary'|'decisions'>;disabled:boolean;onChange(links:DiscoverySourceLink[]):void;onInspect?(link:DiscoverySourceLink):void}) {
  if(specification.evidence!==undefined && !discoverySourceLinksValid(specification.evidence))return <p role="alert">The retained source references could not be read safely. Preserve this draft before reloading or explicitly discarding it. No references were removed automatically.</p>;
  const links=specification.evidence ?? [];
  const targetExists=(link:DiscoverySourceLink)=>link.targetKind==='requirement'?specification.requirements.some(item=>item.id===link.targetId):link.targetKind==='glossary'?specification.glossary.some(item=>item.id===link.targetId):link.targetKind==='decision'?specification.decisions.some(item=>item.id===link.targetId):['purpose','audience','constraint'].includes(link.targetKind) && link.targetId==='';
  return <section className="discover-specification-evidence" aria-label="Specification source evidence"><h3>Source evidence</h3>
    {!links.length && <p className="journey-muted">This definition has no private document references. Sources can be selected in Discover, discussed with Luci and linked through the normal draft workflow.</p>}
    <ul>{links.map((link,index)=><li key={`${link.documentId}:${link.extractionRevision}:${link.segmentId}:${link.targetKind}:${link.targetId}:${index}`}><p>{link.targetKind}{link.targetId?` · ${link.targetId}`:''} · extraction {link.extractionRevision}, page {link.page}, segment {link.segmentId}</p><p className="discover-source-identity">Document <code>{link.documentId}</code><br/>Original SHA-256 <code>{link.originalDigest}</code><br/>Text SHA-256 <code>{link.textDigest}</code></p>
      {!targetExists(link) && <p role="status">This source reference targets a removed context record. Explicitly remove or repair this link before saving the same draft; its original and history remain retained.</p>}
      <label>Source reference {index+1} target<select disabled={disabled} value={`${link.targetKind}:${link.targetId}`} onChange={event=>{const [kind,id]=event.target.value.split(':');onChange(links.map((old,at)=>at===index?{...old,targetKind:kind as DiscoverySourceLink['targetKind'],targetId:id}:old));}}>
        {!targetExists(link) && <option value={`${link.targetKind}:${link.targetId}`}>Unavailable {link.targetKind} · {link.targetId}</option>}
        <option value="purpose:">Purpose</option><option value="audience:">Audience</option><option value="constraint:">Constraints</option>
        {specification.requirements.map(item=><option key={`requirement:${item.id}`} value={`requirement:${item.id}`}>{item.id} · {item.title}</option>)}
        {specification.glossary.map(item=><option key={`glossary:${item.id}`} value={`glossary:${item.id}`}>Term · {item.term}</option>)}
        {specification.decisions.map(item=><option key={`decision:${item.id}`} value={`decision:${item.id}`}>Decision · {item.title}</option>)}
      </select></label>
      {onInspect && <Button variant="tertiary" onClick={()=>onInspect(link)}>Inspect exact linked source</Button>}
      <Button variant="tertiary" disabled={disabled} onClick={()=>onChange(links.filter((_,at)=>at!==index))}>Remove source reference {index+1} from this draft</Button>
    </li>)}</ul><p className="journey-muted">Document text and OCR are evidence claims. Removing a draft reference does not erase its original, older extraction or approved context snapshots.</p>
  </section>;
}
