// PREPARED, NOT YET RUN: this file was written during the DAG10 exclusive window.
// Controlled source-binding tests only. At a later authorized test window they
// create one tiny temporary Git fixture and copy it per case. No helper source
// fixture is imported or executed; no Docker, browser, service or model runs.
import test, {before, after} from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {cpSync, copyFileSync, linkSync, mkdirSync, mkdtempSync, readFileSync,
  realpathSync, renameSync, rmSync, symlinkSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname, join} from 'node:path';
import {helperSourcePaths, verifyHelperSourceBinding} from './provenance.mjs';

// Keep this independent of the implementation export: adding or dropping an
// executed source must require a conscious change to both contract and tests.
const expectedPaths = [
  'check-create-recovery/installed-recovery.mjs',
  'check-create-recovery/recovery-scenario.mjs',
  'check-create-recovery/fixture-guards.mjs',
  'check-create-recovery/check-create-guard.mjs',
  'check-create-recovery/provenance.mjs',
  'check-create-recovery/run-with-observer.py',
  'launch-recovery/acceptance-lifecycle.mjs',
  'launch-recovery/acceptance-startup.mjs',
  'launch-recovery/durable-acceptance-journal.mjs',
  'launch-recovery/fenced-docker-proxy.mjs',
  'four-project-queue/sample-linux-owned.py',
];
const pins = Object.freeze({product: '1'.repeat(40), helper: '2'.repeat(40), image: 'sha256:' + '3'.repeat(64)});
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const gitEnvironment = {
  PATH: process.env.PATH,
  LANG: 'C', LC_ALL: 'C', GIT_CONFIG_NOSYSTEM: '1',
  GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_SYSTEM: '/dev/null', GIT_TERMINAL_PROMPT: '0',
  GIT_AUTHOR_NAME: 'Controlled binding fixture', GIT_AUTHOR_EMAIL: 'binding-fixture@example.invalid',
  GIT_COMMITTER_NAME: 'Controlled binding fixture', GIT_COMMITTER_EMAIL: 'binding-fixture@example.invalid',
};
function git(repo, args) {
  return execFileSync('git', ['-c', 'core.hooksPath=/dev/null', ...args], {
    cwd: repo, env: gitEnvironment, timeout: 5000, maxBuffer: 2 * 1024 ** 2, encoding: 'utf8',
  }).trim();
}
let base;
before(() => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), 'controlled-helper-binding-base-')));
  base = {root, repo: join(root, 'repo')};
  mkdirSync(base.repo);
  base.acceptanceRoot = join(base.repo, 'scripts/acceptance');
  const rows = expectedPaths.map(path => {
    const file = join(base.acceptanceRoot, path);
    mkdirSync(dirname(file), {recursive: true});
    const bytes = Buffer.from('Controlled, deliberately nonexecutable source fixture for ' + path + '\n');
    writeFileSync(file, bytes);
    return {path, repositoryPath: 'scripts/acceptance/' + path, bytes: bytes.length, sha256: sha(bytes)};
  });
  git(base.repo, ['init', '--quiet']);
  git(base.repo, ['add', '--', 'scripts/acceptance']);
  git(base.repo, ['-c', 'commit.gpgsign=false', 'commit', '--quiet', '-m', 'Controlled helper bytes']);
  base.helperCommit = git(base.repo, ['rev-parse', 'HEAD']);
  base.binding = {version: 1, sourceRoot: 'scripts/acceptance', helperSourceCommit: base.helperCommit,
    pins: structuredClone(pins), status: 'committed-source-binding-not-runtime-evidence',
    executedFiles: rows.map(row => ({...row, origin: {commit: base.helperCommit, path: row.repositoryPath}}))};
  base.bindingPath = join(base.root, 'source-binding.json');
  writeFileSync(base.bindingPath, JSON.stringify(base.binding));
});
after(() => {if (base) rmSync(base.root, {recursive: true, force: true});});

function fixture(t) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), 'controlled-helper-binding-case-')));
  t.after(() => rmSync(root, {recursive: true, force: true}));
  const repo = join(root, 'repo');
  cpSync(base.repo, repo, {recursive: true});
  const bindingPath = join(root, 'source-binding.json');
  copyFileSync(base.bindingPath, bindingPath);
  const options = {repo, acceptanceRoot: join(repo, 'scripts/acceptance'), bindingPath,
    helperCommit: base.helperCommit, pins: structuredClone(pins)};
  return {root, options, binding: structuredClone(base.binding),
    save() {writeFileSync(options.bindingPath, JSON.stringify(this.binding));},
    source(index = 0) {return join(options.acceptanceRoot, expectedPaths[index]);},
    verify() {return verifyHelperSourceBinding(options);}};
}
function updateRow(f, index = 0) {
  const bytes = readFileSync(f.source(index));
  Object.assign(f.binding.executedFiles[index], {bytes: bytes.length, sha256: sha(bytes)});
  f.save();
}

test('binding contract names exactly eleven frozen executed helper sources', () => {
  assert.equal(expectedPaths.length, 11);
  assert.deepEqual([...helperSourcePaths].sort(), [...expectedPaths].sort());
  assert.equal(Object.isFrozen(helperSourcePaths), true);
});

test('binding accepts exact ordinary local bytes from the explicit committed helper and returns the manifest', t => {
  const f = fixture(t), result = f.verify();
  assert.deepEqual(result, f.binding);
  assert.equal(result.helperSourceCommit, f.options.helperCommit);
  assert.equal(result.executedFiles.length, 11);
  assert.notEqual(result.helperSourceCommit, result.pins.helper, 'Helper source commit is separate from historical helper provenance');
});

test('binding permits a complete manifest in a different row order', t => {
  const f = fixture(t); f.binding.executedFiles.reverse(); f.save();
  assert.deepEqual(f.verify(), f.binding);
});

test('binding verifies the selected commit rather than silently substituting current HEAD', t => {
  const f = fixture(t);
  writeFileSync(join(f.options.repo, 'unrelated.txt'), 'A later unrelated fixture commit\n');
  git(f.options.repo, ['add', '--', 'unrelated.txt']);
  git(f.options.repo, ['-c', 'commit.gpgsign=false', 'commit', '--quiet', '-m', 'Unrelated later fixture commit']);
  assert.notEqual(git(f.options.repo, ['rev-parse', 'HEAD']), f.options.helperCommit);
  assert.deepEqual(f.verify(), f.binding);
});

for (const [name, mutate] of [
  ['missing source row', f => {f.binding.executedFiles.pop();}],
  ['duplicate source replacing a required row', f => {f.binding.executedFiles[10] = structuredClone(f.binding.executedFiles[0]);}],
  ['extra duplicate source row', f => {f.binding.executedFiles.push(structuredClone(f.binding.executedFiles[0]));}],
  ['foreign source replacing a required row', f => {f.binding.executedFiles[0].path = 'foreign/unreviewed.mjs';}],
  ['extra foreign source row', f => {f.binding.executedFiles.push({...f.binding.executedFiles[0], path: 'foreign/unreviewed.mjs'});}],
  ['relative traversal source path', f => {f.binding.executedFiles[0].path = '../outside.mjs';}],
  ['absolute source path', f => {f.binding.executedFiles[0].path = '/tmp/outside.mjs';}],
  ['noncanonical equivalent source path', f => {f.binding.executedFiles[0].path = 'check-create-recovery/../check-create-recovery/installed-recovery.mjs';}],
  ['null source row', f => {f.binding.executedFiles[0] = null;}],
  ['string source row', f => {f.binding.executedFiles[0] = expectedPaths[0];}],
  ['missing executedFiles', f => {delete f.binding.executedFiles;}],
  ['object instead of executedFiles array', f => {f.binding.executedFiles = {0: f.binding.executedFiles[0]};}],
  ['wrong repository path', f => {f.binding.executedFiles[0].repositoryPath = 'other/' + expectedPaths[0];}],
  ['missing repository path', f => {delete f.binding.executedFiles[0].repositoryPath;}],
  ['incorrect byte count', f => {f.binding.executedFiles[0].bytes++;}],
  ['string byte count', f => {f.binding.executedFiles[0].bytes = String(f.binding.executedFiles[0].bytes);}],
  ['negative byte count', f => {f.binding.executedFiles[0].bytes = -1;}],
  ['missing digest', f => {delete f.binding.executedFiles[0].sha256;}],
  ['malformed digest', f => {f.binding.executedFiles[0].sha256 = 'invalid';}],
  ['uppercase digest', f => {f.binding.executedFiles[0].sha256 = 'A'.repeat(64);}],
  ['wrong ordinary digest', f => {f.binding.executedFiles[0].sha256 = '0'.repeat(64);}],
  ['wrong source root', f => {f.binding.sourceRoot = 'scripts/other';}],
  ['unsupported version', f => {f.binding.version = 2;}],
  ['string version', f => {f.binding.version = '1';}],
  ['manifest helper different from CLI helper', f => {f.binding.helperSourceCommit = '0'.repeat(40);}],
  ['wrong product pin', f => {f.binding.pins.product = '4'.repeat(40);}],
  ['wrong historical helper pin', f => {f.binding.pins.helper = '4'.repeat(40);}],
  ['wrong image pin', f => {f.binding.pins.image = 'sha256:' + '4'.repeat(64);}],
  ['extra pin', f => {f.binding.pins.unreviewed = true;}],
  ['missing pins', f => {delete f.binding.pins;}],
]) test('binding rejects ' + name, t => {
  const f = fixture(t); mutate(f); f.save(); assert.throws(() => f.verify());
});

for (const value of ['HEAD', 'a'.repeat(39), 'A'.repeat(40), 'a'.repeat(41), '', undefined]) {
  test('binding refuses a nonexact CLI helper commit ' + String(value), t => {
    const f = fixture(t); f.options.helperCommit = value; assert.throws(() => f.verify());
  });
}

test('binding refuses an unavailable exact helper commit even when the manifest agrees with the CLI', t => {
  const f = fixture(t);
  f.options.helperCommit = '0'.repeat(40); f.binding.helperSourceCommit = f.options.helperCommit; f.save();
  assert.throws(() => f.verify());
});

test('binding rejects tampered local helper bytes at unchanged byte length', t => {
  const f = fixture(t), file = f.source(), bytes = readFileSync(file);
  bytes[0] ^= 1; writeFileSync(file, bytes);
  assert.throws(() => f.verify(), /Executed helper differs from frozen binding/);
});

test('binding rejects local bytes and manifest hashes rewritten together because selected committed bytes differ', t => {
  const f = fixture(t); writeFileSync(f.source(), 'Rewritten local source and manifest\n'); updateRow(f);
  assert.throws(() => f.verify(), /Executed helper differs from selected committed source/);
});

test('binding rejects a selected newer commit with changed source even when local bytes and manifest retain the old source', t => {
  const f = fixture(t), original = readFileSync(f.source());
  writeFileSync(f.source(), 'Different helper source in selected commit\n');
  git(f.options.repo, ['add', '--', 'scripts/acceptance/' + expectedPaths[0]]);
  git(f.options.repo, ['-c', 'commit.gpgsign=false', 'commit', '--quiet', '-m', 'Different committed helper source']);
  f.options.helperCommit = git(f.options.repo, ['rev-parse', 'HEAD']);
  f.binding.helperSourceCommit = f.options.helperCommit;
  writeFileSync(f.source(), original); f.save();
  assert.throws(() => f.verify(), /Executed helper differs from selected committed source/);
});

test('binding rejects a missing executed source file', t => {
  const f = fixture(t); rmSync(f.source()); assert.throws(() => f.verify());
});

test('binding rejects a directory in place of an executed source file', t => {
  const f = fixture(t); rmSync(f.source()); mkdirSync(f.source()); assert.throws(() => f.verify());
});

test('binding rejects a source symlink even when it resolves to the exact committed bytes', t => {
  const f = fixture(t), outside = join(f.root, 'linked-source-target');
  renameSync(f.source(), outside); symlinkSync(outside, f.source()); assert.throws(() => f.verify());
});

test('binding rejects a multiply linked ordinary source file', t => {
  const f = fixture(t); linkSync(f.source(), join(f.root, 'source-hard-link')); assert.throws(() => f.verify());
});

test('binding rejects a linked parent directory of an executed source', t => {
  const f = fixture(t), sourceDir = dirname(f.source()), outside = join(f.root, 'outside-source-directory');
  renameSync(sourceDir, outside); symlinkSync(outside, sourceDir, 'dir'); assert.throws(() => f.verify());
});

test('binding rejects a foreign acceptance root containing all the same ordinary bytes', t => {
  const f = fixture(t), outside = join(f.root, 'foreign-acceptance');
  cpSync(f.options.acceptanceRoot, outside, {recursive: true});
  f.options.acceptanceRoot = outside; assert.throws(() => f.verify());
});

test('binding rejects a symlinked acceptance root even if the caller supplies its canonical outside target', t => {
  // Regression for comparing two realpaths without requiring the repository
  // acceptance directory itself to be canonical and confined to the checkout.
  const f = fixture(t), expected = f.options.acceptanceRoot, outside = join(f.root, 'outside-acceptance');
  renameSync(expected, outside); symlinkSync(outside, expected, 'dir');
  f.options.acceptanceRoot = realpathSync(expected);
  assert.throws(() => f.verify());
});

test('binding rejects a symlinked acceptance root when passed by its linked path', t => {
  const f = fixture(t), expected = f.options.acceptanceRoot, outside = join(f.root, 'outside-acceptance');
  renameSync(expected, outside); symlinkSync(outside, expected, 'dir'); assert.throws(() => f.verify());
});

test('binding rejects a direct manifest symlink', t => {
  const f = fixture(t), outside = join(f.root, 'manifest-target.json');
  renameSync(f.options.bindingPath, outside); symlinkSync(outside, f.options.bindingPath); assert.throws(() => f.verify());
});

test('binding rejects an ordinary manifest reached through a symlinked parent directory', t => {
  // Regression for checking only the leaf with lstat while accepting a linked
  // ancestor of the binding path supplied by the installed CLI.
  const f = fixture(t), directory = join(f.root, 'manifest-directory'), alias = join(f.root, 'manifest-alias');
  mkdirSync(directory); renameSync(f.options.bindingPath, join(directory, 'source-binding.json'));
  symlinkSync(directory, alias, 'dir'); f.options.bindingPath = join(alias, 'source-binding.json');
  assert.throws(() => f.verify());
});

test('binding rejects a multiply linked ordinary manifest', t => {
  const f = fixture(t); linkSync(f.options.bindingPath, join(f.root, 'manifest-hard-link.json')); assert.throws(() => f.verify());
});

test('binding rejects a directory in place of the manifest', t => {
  const f = fixture(t); rmSync(f.options.bindingPath); mkdirSync(f.options.bindingPath); assert.throws(() => f.verify());
});

test('binding rejects a missing manifest', t => {
  const f = fixture(t); rmSync(f.options.bindingPath); assert.throws(() => f.verify());
});

test('binding rejects a malformed JSON manifest', t => {
  const f = fixture(t); writeFileSync(f.options.bindingPath, '{malformed'); assert.throws(() => f.verify(), SyntaxError);
});

test('binding rejects a null JSON manifest', t => {
  const f = fixture(t); writeFileSync(f.options.bindingPath, 'null'); assert.throws(() => f.verify());
});

test('binding rejects an oversized ordinary manifest before reading its otherwise valid content', t => {
  const f = fixture(t);
  f.binding.padding = 'x'.repeat(2 * 1024 ** 2); f.save(); assert.throws(() => f.verify());
});
