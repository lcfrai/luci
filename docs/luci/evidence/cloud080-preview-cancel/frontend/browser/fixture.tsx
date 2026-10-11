import '/workspace/luci/desktop/src/styles/app.css';
import '/workspace/luci/desktop/src/components/journey/journey.css';
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { ProjectDeploy } from '/workspace/luci/desktop/src/components/lens/DeployLens';
import { Button } from '/workspace/luci/desktop/src/components/fluid/components/button';
function Fixture() {
  const [project, setProject] = useState('alpha');
  return <main style={{maxWidth:1100,margin:'0 auto',padding:16}}><p>Controlled preview cancellation fixture; no execution backend</p><nav aria-label="Fixture project"><Button variant="secondary" aria-pressed={project==='alpha'} onClick={()=>setProject('alpha')}>Project Alpha</Button><Button variant="secondary" aria-pressed={project==='beta'} onClick={()=>setProject('beta')}>Project Beta</Button></nav><h1>{project==='alpha'?'Alpha project':'Beta project'}</h1><ProjectDeploy projectId={project} configurationManaged /></main>;
}
const host=window as typeof window & { __previewRoot?: ReturnType<typeof createRoot> };
host.__previewRoot ??= createRoot(document.getElementById('root')!);
host.__previewRoot.render(<Fixture/>);
