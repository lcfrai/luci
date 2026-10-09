import { ProjectRequired } from '../journey/shared';
import { DefinitionWorkspace } from '../definition/DefinitionWorkspace';

export const BriefEditor = DefinitionWorkspace;
export function DiscoverLens() {
  return <ProjectRequired>{(id) => <DefinitionWorkspace key={id} projectId={id} />}</ProjectRequired>;
}
