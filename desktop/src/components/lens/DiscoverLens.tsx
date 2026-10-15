import { ProjectRequired } from '../journey/shared';
import { DefinitionWorkspace } from '../definition/DefinitionWorkspace';
import { DiscoverDocuments } from '../discover/DiscoverDocuments';

export const BriefEditor = DefinitionWorkspace;
export function DiscoverLens() {
  return <ProjectRequired>{(id) => <div key={id}><DiscoverDocuments projectId={id}/><details><summary>Edit the structured project definition</summary><DefinitionWorkspace projectId={id}/></details></div>}</ProjectRequired>;
}
