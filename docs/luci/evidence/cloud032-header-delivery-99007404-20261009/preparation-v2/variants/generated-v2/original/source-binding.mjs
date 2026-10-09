// Pure source binding: callers read Git with their own bounded command budget.
import assert from 'node:assert/strict';

/** Read only the two source options; retain the driver's existing CLI semantics. */
export function parseSourceBindingOptions(args) {
  assert(Array.isArray(args), 'Source options must be an argument array');
  const option = name => {
    const index = args.indexOf(name);
    if (index < 0) return undefined;
    const value = args[index + 1];
    assert(typeof value === 'string' && value && !value.startsWith('--'), 'Missing value for ' + name);
    return value;
  };
  return {
    expectedProduct: option('--expected-source'),
    expectedHelper: option('--expected-helper-source'),
  };
}

function exactCommit(value, description) {
  // The length check also excludes a trailing newline, which JavaScript's $
  // regular-expression anchor would otherwise allow before the final newline.
  assert(typeof value === 'string' && value.length === 40 && /^[a-f0-9]{40}$/.test(value),
    description + ' must be an exact lowercase 40-hex commit');
}

/** Validate already-read values without inspecting a checkout or changing provenance. */
export function validateSourceBinding({provenance, expectedProduct, expectedHelper, helperCommit, helperDirty}) {
  assert(provenance && typeof provenance === 'object' && !Array.isArray(provenance),
    'Matching clean package provenance is required');
  assert.equal(provenance.sourceDirty, false, 'Packaged product source must be clean');
  exactCommit(provenance.sourceCommit, 'Packaged product source');
  exactCommit(expectedProduct, '--expected-source');
  assert.equal(provenance.sourceCommit, expectedProduct,
    'Packaged product source must match --expected-source');

  const expectedSource = expectedHelper === undefined ? provenance.sourceCommit : expectedHelper;
  exactCommit(expectedSource, '--expected-helper-source');
  exactCommit(helperCommit, 'Acceptance helper checkout HEAD');
  assert.equal(helperCommit, expectedSource,
    'Acceptance helper checkout must match its separately declared source');
  assert.equal(helperDirty, false, 'Acceptance helper source must be clean');

  return {
    commit: helperCommit,
    dirty: helperDirty,
    expectedSource,
    productSource: provenance.sourceCommit,
    separateHelperRevision: helperCommit !== provenance.sourceCommit,
  };
}

/** Inputs are canonical directories resolved by the caller before any workload. */
export function validateHelperLocation(actualDirectory, checkoutDirectory) {
  assert(typeof actualDirectory === 'string' && actualDirectory.length > 0, 'Executed helper directory is required');
  assert.equal(actualDirectory, checkoutDirectory, 'Executed helper must belong to the declared clean checkout');
}
