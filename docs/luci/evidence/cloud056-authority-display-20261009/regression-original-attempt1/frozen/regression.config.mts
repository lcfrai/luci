import { defineConfig } from '/workspace/luci/desktop/node_modules/vitest/dist/config.js';
import react from '/workspace/luci/desktop/node_modules/@vitejs/plugin-react/dist/index.js';
import fs from 'node:fs';
const root='/workspace/luci/desktop';
const stage='/workspace/.luci-tools/session-authority-review-edd0';
const selected=['src/api/client.ts','src/api/teamScope.ts','src/components/teams/TeamWorkspaceProvider.tsx'];
const overlay=process.env.CLOUD056_SOURCE_OVERLAY==='1';
export default defineConfig({root,plugins:[{
  name:'cloud056-selected-source-overlay',enforce:'pre',
  load(id){const pathname=id.split('?')[0];if(overlay&&selected.some(path=>pathname===`${root}/${path}`))return fs.readFileSync(`${stage}/after/desktop/${pathname.slice(root.length+1)}`,'utf8');},
},react()],server:{fs:{allow:['/workspace/luci',stage]}},test:{environment:'jsdom',include:[`${stage}/authority-regression.test.tsx`],setupFiles:[`${root}/src/test/setup.ts`],globals:false,restoreMocks:true,clearMocks:true,maxWorkers:1}});
