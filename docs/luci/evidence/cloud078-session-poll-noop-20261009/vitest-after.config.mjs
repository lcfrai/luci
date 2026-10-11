import { readFileSync, appendFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
const repo = "/workspace/luci/desktop";
const base = "/workspace/.luci-tools/cloud078-session-poll-noop";
const arm = "after";
const newTest = base + '/after/desktop/src/store/sessionPollNoop.test.ts';
const production = repo + '/src/store/slices/dataSlice.ts';
const pollTest = repo + '/src/api/poller.test.ts';
const selected = new Map([
 [production, base + '/' + arm + '/desktop/src/store/slices/dataSlice.ts'],
 [pollTest, base + '/after/desktop/src/api/poller.test.ts'],
]);
export default {
 root: repo,
 cacheDir: base + '/generated-cache-' + arm,
 plugins: [{
  name: 'cloud078-selected-read-only-overlay', enforce: 'pre',
  async resolveId(source, importer) {
   if (importer?.split('?')[0] === newTest)
    return this.resolve(source, repo + '/src/store/sessionPollNoop.test.ts', {skipSelf: true});
  },
  load(id) {
   const clean = id.split('?')[0], replacement = selected.get(clean);
   if (!replacement) return;
   const bytes = readFileSync(replacement);
   appendFileSync(base + '/loaded-source-' + arm + '.jsonl', JSON.stringify({id:clean, replacement, sha256:createHash('sha256').update(bytes).digest('hex')}) + '\n');
   return bytes.toString('utf8');
  },
 }],
 server: {fs: {allow: [repo, base]}},
 test: {
  environment: 'jsdom',
  include: [pollTest, newTest, repo + '/src/api/poller.generation.test.ts'],
  setupFiles: [repo + '/src/test/setup.ts'],
  globals: false, restoreMocks: true, clearMocks: true,
  maxWorkers: 1, fileParallelism: false,
  reporters: ['default', 'json'],
  outputFile: {json: base + '/validation-' + arm + '.json'},
 },
};
