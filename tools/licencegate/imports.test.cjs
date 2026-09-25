'use strict';
// node --test tools/licencegate/imports.test.cjs, from the repository root.
// It parses with the real typescript in desktop/node_modules (a real install:
// the gate reads the project's own tree, so a junction is refused there).
const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const { execFileSync } = require('child_process');

const helper = path.join(__dirname, 'imports.cjs');
const project = path.join(__dirname, '..', '..', 'desktop');
const fixture = (name) => path.join(__dirname, 'testdata', 'imports', name);

function run(...files) {
  return JSON.parse(execFileSync(process.execPath, [helper, project, ...files], { encoding: 'utf8' }));
}

test('a parser: exactly the five real imports, never the comment, JSX text or regex', () => {
  const got = run(fixture('fixture.tsx')).map((r) => r.specifier).sort();
  assert.deepStrictEqual(got, ['@s/n/sub', 'e', 'f', 'g', 'react']);
});

test('type-only statements are marked, a mixed import is not', () => {
  const got = Object.fromEntries(run(fixture('types.ts')).map((r) => [r.specifier, r.typeOnly]));
  assert.deepStrictEqual(got, {
    'type-only': true,
    mixed: false,
    'type-reexport': true,
    'import-equals': false,
  });
});

test('each record names its file', () => {
  const recs = run(fixture('fixture.tsx'), fixture('types.ts'));
  assert.ok(recs.every((r) => r.file === fixture('fixture.tsx') || r.file === fixture('types.ts')));
  assert.strictEqual(recs.length, 9);
});
