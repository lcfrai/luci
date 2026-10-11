import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
const stage = '/workspace/.luci-tools/cloud076-attach-readiness/frontend';
const desktop = '/workspace/luci/desktop';
const variant = process.env.C076_VARIANT;
if (!['before', 'fixed'].includes(variant)) throw Error('Explicit C076_VARIANT required');
const files = new Set(['src/api/attachRegistry.ts', 'src/store/types.ts', 'src/components/develop/SessionPaneFooter.tsx']);
const externalTests = stage + '/tests/desktop/';
export default defineConfig({
  root: desktop,
  cacheDir: stage + '/cache-' + variant,
  plugins: [{
    name: 'exact-c076-source-overlay', enforce: 'pre',
    async resolveId(source, importer) {
      if (importer?.startsWith(externalTests) && source.startsWith('.')) {
        const canonical = desktop + '/' + importer.slice(externalTests.length);
        return this.resolve(source, canonical, {skipSelf: true});
      }
    },
    load(id) {
      const path = id.split('?')[0];
      const rel = path.startsWith(desktop + '/') ? path.slice(desktop.length + 1) : null;
      if (rel && files.has(rel)) return readFileSync(stage + '/' + variant + '/desktop/' + rel, 'utf8');
    },
  }, react()],
  server: { fs: { allow: ['/workspace/luci', stage] } },
  test: {
    environment: 'jsdom', globals: false, restoreMocks: true, clearMocks: true,
    setupFiles: [desktop + '/src/test/setup.ts'],
    include: [externalTests + 'src/api/attachRegistry.readiness.test.ts',
      'src/api/attachRegistry*.test.ts', 'src/api/poller.generation.test.ts',
      'src/components/develop/SessionPane.restartAttach.test.tsx', 'src/components/develop/SessionPane.test.tsx',
      'src/components/Terminal.test.tsx', 'src/store/store.test.ts'],
    fileParallelism: false, maxWorkers: 1,
    css: { include: [/\.css\?raw$/] },
  },
});
