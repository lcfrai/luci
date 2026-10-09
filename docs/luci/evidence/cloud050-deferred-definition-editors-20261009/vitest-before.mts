import config from '/workspace/luci/desktop/vitest.config.ts';
import {readFileSync} from 'node:fs';
export default {...config,plugins:[{name:'cloud050-original-workspace',enforce:'pre',load(id){if(id==='/workspace/luci/desktop/src/components/definition/DefinitionWorkspace.tsx')return readFileSync('/workspace/.luci-tools/cloud050-context-scale/before/desktop/src/components/definition/DefinitionWorkspace.tsx','utf8');}},...(config.plugins||[])]};
