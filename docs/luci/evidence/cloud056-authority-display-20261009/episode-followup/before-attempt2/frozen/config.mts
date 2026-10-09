import { defineConfig } from '/workspace/luci/desktop/node_modules/vitest/dist/config.js';
import react from '/workspace/luci/desktop/node_modules/@vitejs/plugin-react/dist/index.js';
import fs from 'node:fs';
const root='/workspace/luci/desktop';
const stage='/workspace/.luci-tools/session-authority-review-edd0/episode-followup';
const overlay=process.env.CLOUD056_EPISODE_FIXED==='1';
export default defineConfig({root,plugins:[{name:'cloud056-denied-episode-overlay',enforce:'pre',load(id){if(id.split('?')[0]===`${root}/src/components/teams/TeamWorkspaceProvider.tsx`)return fs.readFileSync(`${stage}/${overlay?'after':'before'}.tsx`,'utf8');}},react()],server:{fs:{allow:['/workspace/luci','/workspace/.luci-tools/session-authority-review-edd0']}},test:{environment:'jsdom',include:[`${stage}/episode.test.tsx`],setupFiles:[`${root}/src/test/setup.ts`],globals:false,restoreMocks:true,clearMocks:true,maxWorkers:1}});
