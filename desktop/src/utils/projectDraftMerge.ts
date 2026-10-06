export type ProjectDraftMergeChoice = "draft" | "saved";

export interface ProjectDraftMergeConflict {
  /** JSON-encoded path; record IDs remain stable when array positions change. */
  key: string;
  path: string[];
  baseline: unknown;
  draft: unknown;
  saved: unknown;
}

export interface ProjectDraftMergeResult<T> {
  value: T;
  conflicts: ProjectDraftMergeConflict[];
  unresolved: number;
}

const savedMetadata = new Set([
  "projectId", "version", "revision", "briefRevision", "specificationRevision",
  "specificationDigest", "sketchRevision", "status", "digest", "updatedAt",
]);

type Fields = Record<string, unknown>;
type Identified = Fields & { id: string };

function own(value: Fields | undefined, key: string): unknown {
  return value && Object.prototype.hasOwnProperty.call(value, key) ? value[key] : undefined;
}

function isObject(value: unknown): value is Fields {
  return value !== null && typeof value === "object" && !Array.isArray(value) &&
    (Object.getPrototypeOf(value) === Object.prototype || Object.getPrototypeOf(value) === null);
}

function clone(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(clone);
  if (isObject(value)) return Object.fromEntries(Object.entries(value).map(([key, child]) => [key, clone(child)]));
  return value;
}

function equal(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    return left.length === right.length && left.every((value, index) => equal(value, right[index]));
  }
  if (!isObject(left) || !isObject(right)) return false;
  const keys = Object.keys(left);
  return keys.length === Object.keys(right).length && keys.every(key =>
    Object.prototype.hasOwnProperty.call(right, key) && equal(left[key], right[key]));
}

function identified(values: unknown[]): values is Identified[] {
  const ids = new Set<string>();
  return values.every(value => {
    if (!isObject(value) || typeof value.id !== "string" || !value.id || ids.has(value.id)) return false;
    ids.add(value.id);
    return true;
  });
}

/** Insert other-side additions before their next shared anchor, or after the last one. */
function combineOrder(preferred: string[], other: string[]): string[] {
  const result = [...preferred];
  const present = new Set(result);
  let pending: string[] = [];
  for (const id of other) {
    if (present.has(id)) {
      if (pending.length) {
        result.splice(result.indexOf(id), 0, ...pending);
        pending.forEach(added => present.add(added));
        pending = [];
      }
    } else {
      pending.push(id);
    }
  }
  // Trailing additions retain their side's order without displacing preferred IDs.
  result.push(...pending);
  return result;
}

/**
 * Three-way recovery for JSON-like structured context, not drawing/canvas scenes.
 * Missing baseline means unknown history: every differing leaf needs a decision.
 * Root revision/approval metadata always comes from saved, never a recovered draft.
 * Callers must not restore while unresolved > 0, and must bind choices to all inputs.
 */
export function mergeProjectDraft<T>(
  baseline: T | undefined,
  draft: T,
  saved: T,
  choices: Record<string, ProjectDraftMergeChoice> = {},
): ProjectDraftMergeResult<T> {
  const conflicts: ProjectDraftMergeConflict[] = [];
  let unresolved = 0;
  const baselineKnown = baseline !== undefined;

  function conflict(path: string[], before: unknown, local: unknown, remote: unknown): unknown {
    const key = JSON.stringify(path);
    const choice = choices[key];
    conflicts.push({ key, path: [...path], baseline: clone(before), draft: clone(local), saved: clone(remote) });
    if (choice !== "draft" && choice !== "saved") unresolved++;
    return clone(choice === "draft" ? local : remote);
  }

  function mergeRecords(before: Identified[], local: Identified[], remote: Identified[], path: string[]): unknown[] {
    const baseById = new Map(before.map(record => [record.id, record]));
    const localById = new Map(local.map(record => [record.id, record]));
    const remoteById = new Map(remote.map(record => [record.id, record]));
    const values = new Map<string, unknown>();
    const ids = [...new Set([...baseById.keys(), ...localById.keys(), ...remoteById.keys()])].sort();
    for (const id of ids) {
      const value = merge(baseById.get(id), localById.get(id), remoteById.get(id), [...path, `id:${id}`]);
      if (value !== undefined) values.set(id, value);
    }

    const localOrder = local.map(record => record.id).filter(id => values.has(id));
    const remoteOrder = remote.map(record => record.id).filter(id => values.has(id));
    const common = new Set(localOrder.filter(id => remoteById.has(id)));
    const oldCommon = new Set([...common].filter(id => baseById.has(id)));
    const baseOrder = before.map(record => record.id).filter(id => oldCommon.has(id));
    const localOldOrder = localOrder.filter(id => oldCommon.has(id));
    const remoteOldOrder = remoteOrder.filter(id => oldCommon.has(id));
    const localReordered = !equal(baseOrder, localOldOrder);
    const remoteReordered = !equal(baseOrder, remoteOldOrder);
    const disagree = !equal(localOrder.filter(id => common.has(id)), remoteOrder.filter(id => common.has(id)));
    const sharedAddition = [...common].some(id => !baseById.has(id));
    const orderConflict = disagree && (!baselineKnown || sharedAddition || (localReordered && remoteReordered));

    let preferLocal = localReordered && !remoteReordered;
    if (orderConflict) {
      const orderPath = [...path, "$order"];
      conflict(orderPath, baselineKnown ? before.map(record => record.id) : undefined, localOrder, remoteOrder);
      preferLocal = choices[JSON.stringify(orderPath)] === "draft";
    }
    const order = preferLocal ? combineOrder(localOrder, remoteOrder) : combineOrder(remoteOrder, localOrder);
    return order.map(id => values.get(id));
  }

  function merge(before: unknown, local: unknown, remote: unknown, path: string[]): unknown {
    // Walk the root object even when one side is unchanged so metadata cannot be recovered.
    const rootObject = path.length === 0 && isObject(local) && isObject(remote);
    if (!rootObject) {
      if (equal(local, remote)) return clone(remote);
      if (baselineKnown && equal(local, before)) return clone(remote);
      if (baselineKnown && equal(remote, before)) return clone(local);
    }
    if (isObject(local) && isObject(remote) && (before === undefined || isObject(before))) {
      const keys = [...new Set([...Object.keys(before ?? {}), ...Object.keys(local), ...Object.keys(remote)])].sort();
      const entries: [string, unknown][] = [];
      for (const key of keys) {
        if (path.length === 0 && savedMetadata.has(key)) {
          if (Object.prototype.hasOwnProperty.call(remote, key)) entries.push([key, clone(remote[key])]);
          continue;
        }
        const value = merge(own(before, key), own(local, key), own(remote, key), [...path, key]);
        if (value !== undefined) entries.push([key, value]);
      }
      return Object.fromEntries(entries);
    }
    if (Array.isArray(local) && Array.isArray(remote) && (before === undefined || Array.isArray(before)) &&
      identified(local) && identified(remote) && (before === undefined || identified(before))) {
      return mergeRecords(before ?? [], local, remote, path);
    }
    return conflict(path, before, local, remote);
  }

  return { value: merge(baseline, draft, saved, []) as T, conflicts, unresolved };
}
