import assert from 'node:assert/strict';
import test from 'node:test';
import {parseSourceBindingOptions, validateSourceBinding, validateHelperLocation} from './source-binding.mjs';

const product = 'a'.repeat(40), helper = 'b'.repeat(40), other = 'c'.repeat(40);
const valid = overrides => ({
  provenance: {sourceCommit: product, sourceDirty: false},
  expectedProduct: product,
  helperCommit: product,
  helperDirty: false,
  ...overrides,
});

test('default source binding requires the clean helper HEAD to equal the product revision', () => {
  const options = parseSourceBindingOptions(['--expected-source', product]);
  assert.deepEqual(options, {expectedProduct: product, expectedHelper: undefined});
  assert.deepEqual(validateSourceBinding(valid(options)), {
    commit: product, dirty: false, expectedSource: product,
    productSource: product, separateHelperRevision: false,
  });
  assert.throws(() => validateSourceBinding(valid({...options, helperCommit: helper})), /separately declared source/);
});

test('an explicit separate helper revision preserves the packaged product binding', () => {
  const provenance = Object.freeze({sourceCommit: product, sourceDirty: false});
  const options = parseSourceBindingOptions(['--expected-source', product, '--expected-helper-source', helper]);
  assert.deepEqual(validateSourceBinding(valid({...options, provenance, helperCommit: helper})), {
    commit: helper, dirty: false, expectedSource: helper,
    productSource: product, separateHelperRevision: true,
  });
  assert.equal(provenance.sourceCommit, product);
});

test('an explicit helper revision equal to the product is recorded as the same revision', () => {
  assert.equal(validateSourceBinding(valid({expectedHelper: product})).separateHelperRevision, false);
});

test('unknown arguments are ignored without mutating the original command line', () => {
  const args = Object.freeze(['--unknown-option', 'value', '--expected-source', product, '--another-flag']);
  assert.deepEqual(parseSourceBindingOptions(args), {expectedProduct: product, expectedHelper: undefined});
  assert.deepEqual(args, ['--unknown-option', 'value', '--expected-source', product, '--another-flag']);
});

test('duplicate source options preserve the existing first occurrence behavior', () => {
  assert.deepEqual(parseSourceBindingOptions([
    '--expected-source', product, '--expected-source', other,
    '--expected-helper-source', helper, '--expected-helper-source', other,
  ]), {expectedProduct: product, expectedHelper: helper});
});

test('an absent product option fails validation instead of inferring trust from provenance', () => {
  const options = parseSourceBindingOptions(['--unrelated', 'value']);
  assert.throws(() => validateSourceBinding(valid(options)), /--expected-source must be an exact/);
});

test('each recognized option rejects a missing value or the following flag', () => {
  for (const name of ['--expected-source', '--expected-helper-source']) {
    for (const args of [[name], [name, ''], [name, '--other'], [name, '--expected-source', product]]) {
      assert.throws(() => parseSourceBindingOptions(args), /Missing value for/);
    }
  }
});

test('equal-sign option spellings retain the existing unknown-option behavior', () => {
  assert.deepEqual(parseSourceBindingOptions([
    '--expected-source', product, '--expected-helper-source=' + helper,
  ]), {expectedProduct: product, expectedHelper: undefined});
});

test('missing or malformed product provenance cannot establish a source binding', () => {
  for (const provenance of [undefined, null, [], {}, {sourceCommit: product}, {sourceDirty: false}]) {
    assert.throws(() => validateSourceBinding(valid({provenance})));
  }
});

test('product provenance and expected product revisions require exact lowercase full hashes', () => {
  const malformed = [undefined, null, '', 'HEAD', 'a'.repeat(39), 'a'.repeat(41),
    'A'.repeat(40), 'g'.repeat(40), product + '\n', ' ' + product, 123];
  for (const value of malformed) {
    assert.throws(() => validateSourceBinding(valid({
      provenance: {sourceCommit: value, sourceDirty: false},
    })), /Packaged product source must be an exact/);
    assert.throws(() => validateSourceBinding(valid({expectedProduct: value})), /--expected-source must be an exact/);
  }
});

test('explicit helper revisions and helper HEAD require exact lowercase full hashes', () => {
  const malformed = [null, '', 'HEAD', 'b'.repeat(39), 'b'.repeat(41),
    'B'.repeat(40), 'g'.repeat(40), helper + '\n', ' ' + helper, 123];
  for (const value of malformed) {
    assert.throws(() => validateSourceBinding(valid({expectedHelper: value})), /--expected-helper-source must be an exact/);
    assert.throws(() => validateSourceBinding(valid({helperCommit: value})), /helper checkout HEAD must be an exact/);
  }
  assert.throws(() => validateSourceBinding(valid({helperCommit: undefined})), /helper checkout HEAD must be an exact/);
});

test('dirty or unknown product state is rejected even with a separately declared clean helper', () => {
  for (const sourceDirty of [true, undefined, null, 0, '', 'false']) {
    assert.throws(() => validateSourceBinding(valid({
      provenance: {sourceCommit: product, sourceDirty}, expectedHelper: helper, helperCommit: helper,
    })), /Packaged product source must be clean/);
  }
});

test('dirty or unknown helper state is rejected for same and separate revisions', () => {
  for (const helperDirty of [true, undefined, null, 0, '', 'false']) {
    for (const expectedHelper of [product, helper]) {
      assert.throws(() => validateSourceBinding(valid({
        expectedHelper, helperCommit: expectedHelper, helperDirty,
      })), /Acceptance helper source must be clean/);
    }
  }
});

test('declaring the correct helper cannot override a product provenance mismatch', () => {
  assert.throws(() => validateSourceBinding(valid({
    expectedProduct: other, expectedHelper: helper, helperCommit: helper,
  })), /Packaged product source must match --expected-source/);
});

test('a clean but undeclared helper HEAD fails exact revision binding', () => {
  assert.throws(() => validateSourceBinding(valid({
    expectedHelper: helper, helperCommit: other,
  })), /Acceptance helper checkout must match its separately declared source/);
});

test('executed helper directory must belong to the declared clean checkout', () => {
  validateHelperLocation('/checkout/scripts/acceptance/sustained-four-project', '/checkout/scripts/acceptance/sustained-four-project');
  assert.throws(() => validateHelperLocation('/external/staged/scripts/acceptance/sustained-four-project', '/checkout/scripts/acceptance/sustained-four-project'), /declared clean checkout/);
  assert.throws(() => validateHelperLocation('', ''), /directory is required/);
});
