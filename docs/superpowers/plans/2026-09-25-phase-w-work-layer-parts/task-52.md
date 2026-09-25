### Task 52: attachments by blob digest; subscribe and unsubscribe

Stream **WD3** (the issue view), about 2.5 hours (an estimate, not a measurement). Depends on task
51 (the `activity` area it shares). Uses SW.1 (task 09's `POST …/attachments {digest}`,
`task-09.md:49-53`, `:122-126`; task 13's server-side subscribe triggers), SW.15 (`IssueAttachment`,
`Subscription`, `SubscriptionCause`, `attachBlob`, `attachments`, `subscribe` in
`desktop/src/api/work.ts`, `task-37.md:98`, `:160-165`, `:332-345`), SW.16 (`useWork`, `mutate`'s
`subscribe` op, `task-56.md:168`), task 46's `IssueHead` (`task-46.md:574-632`) and SW.8
(`expectNoPrediction`). The Hub's blob routes are phase H's (`hub-contract.md:590-591`, §7.6,
`:844-870`).

A file on an issue is a claim about bytes. The desktop can make that claim honest cheaply: hash the
bytes itself, name the blob by that hash, and re-hash whatever comes back before showing it. It can
also make it dishonest cheaply: trust a server-reported hash, cache a copy on disk "for speed", or
let an SVG run. This part does the first and refuses the three others, each with a test.

**Decisions made here, so nobody re-derives them:**

1. **The desktop hashes first and names the blob by its own hash.** `sha256Hex(bytes)` runs
   `crypto.subtle.digest("SHA-256", …)` over the file's bytes; the `PUT` path's `{sha}` is that
   hex (64 lowercase, `hub-contract.md:559`) and the attachment's digest is `"sha256:" + hex`. The
   Hub hashes the body as it streams and answers 422 `digest_mismatch` if they differ
   (`hub-contract.md:853-854`). The response's `digest` is compared with ours and a difference is a
   refusal shown in mono, never adopted.
2. **The Content-Type sent is the Hub's allowlist or `application/octet-stream`.** The Hub accepts
   ten types (`hub-contract.md:850`); PNG, JPEG, WebP and SVG are among them, **GIF and PDF are
   not**. A file whose browser type is on the list is sent with it; every other file is sent as
   `application/octet-stream`, so a GIF or a PDF uploads rather than failing with 400.
3. **Preview is decided by the bytes, not by any type string.** `sniffImage(bytes)` reads the magic
   numbers of PNG (`89 50 4E 47 0D 0A 1A 0A`), JPEG (`FF D8 FF`), GIF (`GIF87a`, `GIF89a`) and WebP
   (`RIFF` … `WEBP`); only those four preview, through an `<img>` whose object URL's `Blob` carries
   the **sniffed** type. Every other file, SVG included (it is text and matches no magic number),
   is a file row. No attachment enters the DOM as markup, so none runs script.
4. **A download is re-hashed before it is opened.** `openAttachment` `GET`s the bytes, hashes
   them, and compares with the attachment's digest. A mismatch shows *the bytes do not hash to
   sha256:…; not opened* and makes no object URL.
5. **Bytes live in memory; the person's *Save as* is the one write.** Object URLs are revoked on
   unmount. Nothing is written to `localStorage`, IndexedDB, the Cache API or a file by this code
   (`ADR-H1:467-474`: neither the renderer nor the control plane persists Hub data to disk).
   *Save as* is an `<a download>` on the in-memory object URL, so the file is written by
   WebView2's own download flow and only on the person's click; that flow in the Tauri 2 build is
   **UNVERIFIED** (the desktop has no `plugin-fs` or `plugin-dialog`, `desktop/package.json:16-17`,
   and this part adds neither). A saved copy is the person's export, like copied text, and an
   erase does not reach it; the row says *saved copies are yours to delete*.
6. **The Hub's limits are the Hub's numbers.** `blob_too_large` 413 and `quota_exceeded` 409 each
   carry `limitBytes` (`hub-contract.md:260-261`); the refusal line shows that value verbatim in
   mono with its binary size beside it (*16777216 bytes · 16 MiB*). The desktop keeps **no copy** of
   16 MiB. It keeps one ceiling of its own, `READ_CEILING_BYTES` = 64 MiB, because hashing reads the
   whole file into memory; a larger file is refused before it is read, and the line says it is the
   desktop's ceiling, not the Hub's.
7. **The name of a file is not on the wire.** `IssueAttachment` is `{issueId, digest, contentType,
   size, addedAt, addedBy}` (`task-37.md:163-165`) and the attach body is `{digest}`; there is no
   name. The uploader's session keeps `digest → name` in memory and shows it; everyone else, and
   the uploader after a restart, sees the content type, the size, the short digest and who added
   it. Named as a gap against task 03 (an additive `name`), not invented here.
8. **Attachments are fetched, not replicated, in W1.** The replica (tasks 55-58) holds heads,
   revisions, comments and subscriptions, not attachments; `Attachments` reads
   `api.work.attachments(issueId)` on mount and after the person's own attach. A teammate's file
   appears on the next open of the issue; recorded, not hidden.
9. **Subscribe shows the Hub's reason and has no single key.** The head's line reads the replica's
   `Subscription` (task 55's `subscription(issueId)`): *Subscribed · you commented*, from `cause`
   (`create`, `assign`, `mention`, `comment`, `manual`, `task-37.md:98`), with `data-cause` carrying
   the Hub's word verbatim. The desktop never infers a cause from what it saw locally. One button
   unsubscribes or subscribes, as one `subscribe` op, undoable through task 56's handle. Linear's
   `Shift+S` does nothing here (05 §4.3 rule 3); the palette verb is task 59's.

**Files** (the scope names `Attachments.tsx`, `blob.ts` and the two edits; `Subscribe.tsx` is split
out of `IssueHead.tsx` so task 46's file changes by two lines, a gap in the File map):
- `desktop/src/work/blob.ts` — `sha256Hex`, `sniffImage`, `wireContentType`, `uploadAttachment`,
  `fetchVerified` (new)
- `desktop/src/work/blob.test.ts` — the hash, sniff and refusal tests (new)
- `desktop/src/components/issue/Attachments.tsx` — the list, the drop target, preview, *Save as* (new)
- `desktop/src/components/issue/Attachments.test.tsx` — the surface tests (new)
- `desktop/src/components/issue/Subscribe.tsx` — the reason line and the one control (new)
- `desktop/src/components/issue/Subscribe.test.tsx` — (new)
- `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46's) — renders `<Subscribe/>` (modified)
- `desktop/src/components/issue/issueData.ts` ⚑ (task 46's) — `useSubscription` (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ — `Attachments` above `Comments` (modified)
- `desktop/src/components/issue/issue.css` ⚑ — the `.iv-at-*` and `.iv-sub` rules (modified)

#### Interfaces — complete: `desktop/src/work/blob.ts`, part 1: bytes

```ts
// desktop/src/work/blob.ts — content-addressed attachments (hub-contract §7.6). Bytes stay in
// memory. Nothing in this file touches storage, a cache or the file system (decision 5).

/** The Hub's upload allowlist, verbatim (hub-contract.md:850). Anything else goes as octet-stream. */
export const HUB_BLOB_TYPES: readonly string[] = [
  "text/html; charset=utf-8", "text/css; charset=utf-8", "text/plain; charset=utf-8",
  "application/json", "image/png", "image/jpeg", "image/webp", "image/svg+xml", "font/woff2",
  "application/octet-stream",
];
/** The desktop's own memory guard for hashing, NOT the Hub's limit (decision 6). */
export const READ_CEILING_BYTES = 64 * 1024 * 1024;

export type ImageKind = "image/png" | "image/jpeg" | "image/gif" | "image/webp";

/** 64 lowercase hex of SHA-256 over exactly these bytes. */
export async function sha256Hex(bytes: ArrayBuffer): Promise<string> {
  const d = new Uint8Array(await crypto.subtle.digest("SHA-256", bytes));
  let s = "";
  for (const b of d) s += b.toString(16).padStart(2, "0");
  return s;
}
export const digestOf = (hex: string): string => `sha256:${hex}`;
export const hexOf = (digest: string): string | null => /^sha256:([0-9a-f]{64})$/.exec(digest)?.[1] ?? null;

/** Magic numbers only (decision 3). SVG, HTML and every text format answer null. */
export function sniffImage(bytes: ArrayBuffer): ImageKind | null {
  const b = new Uint8Array(bytes, 0, Math.min(bytes.byteLength, 12));
  const at = (i: number, ...xs: number[]) => xs.every((x, k) => b[i + k] === x);
  if (at(0, 0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a)) return "image/png";
  if (at(0, 0xff, 0xd8, 0xff)) return "image/jpeg";
  if (at(0, 0x47, 0x49, 0x46, 0x38) && (at(4, 0x37, 0x61) || at(4, 0x39, 0x61))) return "image/gif";
  if (at(0, 0x52, 0x49, 0x46, 0x46) && at(8, 0x57, 0x45, 0x42, 0x50)) return "image/webp";
  return null;
}

/** The type the PUT carries: the file's own when the Hub lists it, else octet-stream (decision 2).
 *  A text type is listed only with its charset, so "text/plain" alone becomes octet-stream too. */
export function wireContentType(fileType: string): string {
  return HUB_BLOB_TYPES.includes(fileType) ? fileType : "application/octet-stream";
}

/** "16777216 bytes · 16 MiB": the Hub's number verbatim, then its binary size. */
export function bytesFact(n: number): string {
  const units = ["bytes", "KiB", "MiB", "GiB"];
  let v = n, u = 0;
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u += 1; }
  return u === 0 ? `${n} bytes` : `${n} bytes · ${Number.isInteger(v) ? v : v.toFixed(1)} ${units[u]}`;
}
```

#### Interfaces — complete: `desktop/src/work/blob.ts`, part 2: the transport, upload, verified fetch

`WorkSend` is JSON in and JSON out (`task-37.md:372-374`), and a blob is raw bytes, so `blob.ts`
takes its own two-method transport. The control-plane route it names is a **proposal**: task 16's
relay table carries `AttachBlob` (`task-16.md:139`) and no blob row, so `PUT`/`GET
/v1/projects/{projectId}/blobs/{sha}` is not relayed today (a gap against task 16, below). The
Hub's `details` values are strings (`client.ts:62`), so `limitBytes` is parsed, and a value that
does not parse is shown as the Hub wrote it.

```ts
// desktop/src/work/blob.ts, continued.
import type { ApiError } from "../api/client";

/** Raw bytes to and from the Hub's blob routes (hub-contract.md:846-866), through the control
 *  plane. The route below is this part's proposal until task 16 adds the row. */
export interface BlobTransport {
  put(projectId: string, hex: string, body: ArrayBuffer, contentType: string):
    Promise<{ digest: string; size: number; contentType: string }>;
  get(projectId: string, hex: string): Promise<ArrayBuffer>;
}
export const blobRoute = (projectId: string, hex: string): string =>
  `/v1/hub/projects/${encodeURIComponent(projectId)}/blobs/${hex}`;

export type BlobRefusal =
  | { kind: "too_large"; limitBytes: string; source: "hub" | "desktop" }
  | { kind: "quota"; limitBytes: string }
  | { kind: "digest_mismatch"; expected: string; got: string | null }
  | { kind: "not_found" }
  | { kind: "other"; code: string; status: number };

export class BlobError extends Error {
  constructor(readonly refusal: BlobRefusal) { super(`blob: ${refusal.kind}`); this.name = "BlobError"; }
}

/** An ApiError from the transport, sorted. Every other throw passes through untouched. */
export function blobRefusal(e: ApiError): BlobRefusal {
  const limit = e.details.limitBytes ?? "";
  if (e.code === "blob_too_large") return { kind: "too_large", limitBytes: limit, source: "hub" };
  if (e.code === "quota_exceeded") return { kind: "quota", limitBytes: limit };
  if (e.code === "digest_mismatch") return { kind: "digest_mismatch", expected: "", got: null };
  if (e.code === "blob_not_found") return { kind: "not_found" };
  return { kind: "other", code: e.code, status: e.status };
}

export interface Uploaded { digest: string; name: string; size: number; contentType: string }
export interface Verified { digest: string; bytes: ArrayBuffer; image: ImageKind | null }

export function makeBlobs(t: BlobTransport) {
  const mapped = async <T,>(p: Promise<T>): Promise<T> => {
    try { return await p; } catch (e) {
      if (e instanceof Error && e.name === "ApiError") throw new BlobError(blobRefusal(e as ApiError));
      throw e;
    }
  };
  return {
    /** Hash first, then PUT under that hash (decision 1). The desktop's ceiling is checked
     *  before a byte is read (decision 6). The name never leaves this function's caller. */
    async uploadAttachment(projectId: string, file: File): Promise<Uploaded> {
      if (file.size > READ_CEILING_BYTES) {
        throw new BlobError({ kind: "too_large", limitBytes: String(READ_CEILING_BYTES), source: "desktop" });
      }
      const bytes = await file.arrayBuffer();
      const hex = await sha256Hex(bytes);
      const ours = digestOf(hex);
      const r = await mapped(t.put(projectId, hex, bytes, wireContentType(file.type)));
      if (r.digest !== ours) throw new BlobError({ kind: "digest_mismatch", expected: ours, got: r.digest });
      return { digest: ours, name: file.name, size: r.size, contentType: r.contentType };
    },
    /** GET, re-hash, compare (decision 4). A mismatch throws and no bytes reach the caller. */
    async fetchVerified(projectId: string, digest: string): Promise<Verified> {
      const hex = hexOf(digest);
      if (hex === null) throw new BlobError({ kind: "digest_mismatch", expected: digest, got: null });
      const bytes = await mapped(t.get(projectId, hex));
      const got = digestOf(await sha256Hex(bytes));
      if (got !== digest) throw new BlobError({ kind: "digest_mismatch", expected: digest, got });
      return { digest, bytes, image: sniffImage(bytes) };
    },
  };
}
export type Blobs = ReturnType<typeof makeBlobs>;
```

`makeBlobs` has one production transport, `controlPlaneBlobs(send)`, over the client's raw
`fetch` path: `PUT blobRoute(…)` with the body as an `ArrayBuffer` and `Content-Type` set, and
`GET blobRoute(…)` read with `response.arrayBuffer()`. Whether `api/client.ts` exposes a raw-body
send today is **UNVERIFIED** (its `send` is JSON, `task-37.md:372-374`); if it does not, the
transport is ten lines of `fetch` against the same base URL and bearer, written beside `send`
and nowhere else. The relay row is the gap either way.

#### Interfaces — complete: `desktop/src/components/issue/Attachments.tsx`, part 1: the list's state

```tsx
// desktop/src/components/issue/Attachments.tsx — task 52. Fetched, not replicated (decision 8).
import { useCallback, useEffect, useRef, useState, type JSX } from "react";
import type { IssueAttachment } from "../../api/work";
import { api } from "../../api";
import { BlobError, bytesFact, type BlobRefusal, type Blobs, type Verified } from "../../work/blob";

/** digest → the file name the uploader picked, this session only (decision 7). Module scope,
 *  in memory: it dies with the window and is never written anywhere. */
const namesThisSession = new Map<string, string>();
export const nameFor = (digest: string): string | null => namesThisSession.get(digest) ?? null;
export function __clearNamesForTest(): void { namesThisSession.clear(); }

export type AttachLine =
  | { kind: "none" }
  | { kind: "uploading"; name: string }
  | { kind: "refused"; refusal: BlobRefusal; name: string };

/** One fact line per refusal, in the words of the Hub's own numbers (decision 6). */
export function refusalWords(r: BlobRefusal): string {
  switch (r.kind) {
    case "too_large": return r.source === "hub"
      ? `blob_too_large · the Hub's limit is ${factOrRaw(r.limitBytes)}`
      : `not read · the desktop reads at most ${factOrRaw(r.limitBytes)} to hash a file`;
    case "quota": return `quota_exceeded · this project's blob quota is ${factOrRaw(r.limitBytes)}`;
    case "digest_mismatch": return r.got === null
      ? `digest_mismatch · the Hub refused the bytes as sent`
      : `digest_mismatch · expected ${r.expected}, the Hub answered ${r.got}; not attached`;
    case "not_found": return "blob_not_found · the Hub holds no bytes under this digest";
    case "other": return `${r.code} · ${r.status}`;
  }
}
/** "16777216" → "16777216 bytes · 16 MiB"; a value that is not a whole number is shown as sent. */
const factOrRaw = (s: string): string => (/^\d+$/.test(s) ? bytesFact(Number(s)) : s === "" ? "not stated" : s);

export const shortDigest = (digest: string): string => digest.replace(/^sha256:/, "").slice(0, 12);

/** The list, read on mount and after the person's own attach. A teammate's file shows on the
 *  next open (decision 8). `blobs` is injected so the tests hold the transport. */
export function useAttachmentList(issueId: string): {
  items: readonly IssueAttachment[]; reload(): void; failed: string | null;
} {
  const [items, setItems] = useState<readonly IssueAttachment[]>([]);
  const [failed, setFailed] = useState<string | null>(null);
  const [n, setN] = useState(0);
  useEffect(() => {
    let live = true;
    api.work.attachments(issueId).then(
      (xs) => { if (live) { setItems(xs); setFailed(null); } },
      (e: { code?: string }) => { if (live) setFailed(e.code ?? "unavailable"); });
    return () => { live = false; };
  }, [issueId, n]);
  return { items, reload: useCallback(() => setN((k) => k + 1), []), failed };
}
```

A failed list read keeps the last list on screen and adds one fact line (*attachments not read ·
upstream_unavailable*); it never empties the list to look like "no files".

#### Interfaces — complete: `Attachments.tsx`, part 2: one row, preview, *Save as*

```tsx
/** One attachment. Nothing is fetched until the person opens it: a list of twenty files costs
 *  one list call, not twenty downloads. Opening fetches, verifies, then shows. */
function AttachmentRow(props: { a: IssueAttachment; projectId: string; blobs: Blobs }): JSX.Element {
  const { a, projectId, blobs } = props;
  const [shown, setShown] = useState<{ v: Verified; url: string } | { refused: BlobRefusal } | null>(null);
  const urlRef = useRef<string | null>(null);
  useEffect(() => () => { if (urlRef.current) URL.revokeObjectURL(urlRef.current); }, []);
  const name = nameFor(a.digest);
  async function open() {
    try {
      const v = await blobs.fetchVerified(projectId, a.digest);
      // The Blob's type is the SNIFFED one or octet-stream, never a.contentType (decision 3).
      const url = URL.createObjectURL(new Blob([v.bytes], { type: v.image ?? "application/octet-stream" }));
      if (urlRef.current) URL.revokeObjectURL(urlRef.current);
      urlRef.current = url;
      setShown({ v, url });
    } catch (e) {
      if (e instanceof BlobError) setShown({ refused: e.refusal }); else throw e;
    }
  }
  return (
    <li className="iv-at" data-digest={a.digest}>
      <span className="iv-at-name">{name ?? a.contentType}</span>
      <span className="iv-fact" data-fact="size">{bytesFact(a.size)}</span>
      <span className="iv-fact" data-fact="digest" title={a.digest}>{shortDigest(a.digest)}</span>
      <span className="iv-at-by">added by {a.addedBy.displayName}</span>
      {shown === null ? <button type="button" onClick={() => void open()}>Open</button> : null}
      {shown && "refused" in shown ? (
        <p className="iv-fact iv-at-refused" data-fact="refused" role="status">
          {shown.refused.kind === "digest_mismatch" && shown.refused.got
            ? `the bytes do not hash to ${a.digest}; not opened` : refusalWords(shown.refused)}
        </p>
      ) : null}
      {shown && "v" in shown ? (
        <>
          {shown.v.image ? <img className="iv-at-img" src={shown.url} alt={name ?? `attachment ${shortDigest(a.digest)}`} /> : null}
          <a className="iv-at-save" href={shown.url} download={name ?? shortDigest(a.digest)}>Save as</a>
          <span className="iv-at-note">saved copies are yours to delete</span>
        </>
      ) : null}
    </li>
  );
}
```

There is no `dangerouslySetInnerHTML`, no `<iframe>`, no `<object>` and no `<embed>` in this file,
and a structural test says so. An SVG opens as a file row with *Save as* and nothing else: its bytes
never reach an `<img>` either, because `sniffImage` answers `null` for it, so no SVG is rendered
even through the image path (an `<img>`-loaded SVG runs no script, but it can still load
external resources, which a verified-bytes rule has no reason to allow).

#### Interfaces — complete: `Attachments.tsx`, part 3: the list and the drop target

```tsx
export function Attachments(props: { issueId: string; projectId: string; blobs: Blobs }): JSX.Element {
  const { issueId, projectId, blobs } = props;
  const list = useAttachmentList(issueId);
  const [line, setLine] = useState<AttachLine>({ kind: "none" });
  async function attach(file: File) {
    setLine({ kind: "uploading", name: file.name });
    try {
      const up = await blobs.uploadAttachment(projectId, file);
      namesThisSession.set(up.digest, up.name);
      await api.work.attachBlob(issueId, up.digest);
      setLine({ kind: "none" });
      list.reload();
    } catch (e) {
      if (e instanceof BlobError) setLine({ kind: "refused", refusal: e.refusal, name: file.name });
      else setLine({ kind: "refused", name: file.name,
        refusal: { kind: "other", code: (e as { code?: string }).code ?? "unavailable", status: 0 } });
    }
  }
  const pick = useRef<HTMLInputElement>(null);
  return (
    <section className="iv-at-list" aria-label="Attachments"
      onDragOver={(e) => { if (e.dataTransfer.types.includes("Files")) e.preventDefault(); }}
      onDrop={(e) => { e.preventDefault(); const f = e.dataTransfer.files[0]; if (f) void attach(f); }}>
      <h3 className="iv-sub-h">Attachments</h3>
      {list.failed ? <p className="iv-fact" data-fact="list-failed">attachments not read · {list.failed}</p> : null}
      {list.items.length === 0 && !list.failed ? <p className="iv-at-empty">No files. Drop one here.</p> : null}
      <ul className="iv-at-rows">
        {list.items.map((a) => <AttachmentRow key={a.digest} a={a} projectId={projectId} blobs={blobs} />)}
      </ul>
      {line.kind === "uploading" ? <p className="iv-fact" data-fact="uploading">hashing and sending {line.name}</p> : null}
      {line.kind === "refused" ? (
        <p className="iv-fact iv-at-refused" data-fact="refused" role="status">{line.name} · {refusalWords(line.refusal)}</p>
      ) : null}
      <button type="button" onClick={() => pick.current?.click()}>Attach a file</button>
      <input ref={pick} type="file" hidden onChange={(e) => { const f = e.target.files?.[0]; if (f) void attach(f); e.target.value = ""; }} />
    </section>
  );
}
```

One file per drop in W1: a multi-file drop attaches the first and says nothing of the rest, which
the *does not claim* list records. The *uploading* line names the act, not a percentage: the
desktop does not know how far a `PUT` has gone and does not pretend to (`OD-27`).

#### Interfaces — complete: `desktop/src/components/issue/Subscribe.tsx` and `useSubscription`

```tsx
// desktop/src/components/issue/Subscribe.tsx — the Hub's reason, and one control (decision 9).
import type { JSX } from "react";
import type { Subscription, SubscriptionCause } from "../../api/work";
import { mutate } from "../../work/replica/ops";

/** The cause as words. The KEY is the Hub's field; the desktop adds words, never a cause. */
export const CAUSE_WORDS: Record<SubscriptionCause, string> = {
  create: "you created it", assign: "you were assigned", mention: "you were mentioned",
  comment: "you commented", manual: "your choice",
};

export function Subscribe(props: { issueId: string; sub: Subscription | null }): JSX.Element {
  const { issueId, sub } = props;
  const on = sub?.subscribed === true;
  const line = sub === null ? "Not subscribed"
    : `${on ? "Subscribed" : "Not subscribed"} · ${CAUSE_WORDS[sub.cause]}`;
  return (
    <p className="iv-sub" data-subscribed={on} data-cause={sub?.cause ?? ""}>
      <span className="iv-sub-line">{line}</span>
      <button type="button" className="iv-sub-toggle"
        onClick={() => mutate({ kind: "subscribe", issueId, subscribed: !on })}>
        {on ? "Unsubscribe" : "Subscribe"}
      </button>
    </p>
  );
}
```

```ts
// issueData.ts ⚑ (task 46's; still the only file here that reads the replica's shape):
import type { Subscription } from "../../api/work";
export function useSubscription(issueId: string): Subscription | null {
  return useWork((r) => r.subscription(issueId));   // task 55's read; null when the Hub holds no row
}
```

`sub === null` means the Hub holds no row for this person and issue: nobody subscribed them, so
*Not subscribed* with no cause. A row with `subscribed: false` is an unsubscribe the person made
(the only way a row goes false is the manual `PUT`, `task-13.md:57`), so it reads *Not subscribed
· your choice*, and a later mention does not undo it (`task-13.md:527`). The optimistic paint of
the op writes `{subscribed, cause: "manual"}` (task 55's `setSubscribed`), which is exactly what
the Hub will answer, because the manual `PUT` always sets cause `manual`. The chip is task 56's:
`mutate` returns the handle and the view's `UndoChip` shows *Unsubscribed · Undo*, and Undo
sends the inverse op. There is no key; *Subscribe* and *Unsubscribe* are palette verbs (task 59).

#### Interfaces — complete: the edits to `IssueHead.tsx`, `IssueView.tsx` and `issue.css`

```tsx
// IssueHead.tsx ⚑ — two lines. Props gain `sub: Subscription | null`, and after .iv-facts:
      <Subscribe issueId={head.id} sub={props.sub} />

// IssueView.tsx ⚑ — IssueViewBody reads the subscription through issueData and passes it on,
// and the activity slot's default renders Attachments above task 51's Comments:
const sub = useSubscription(issueId);
<IssueHead … sub={sub} />
{slots.activity ?? (
  <>
    <Attachments issueId={head.id} projectId={head.projectId} blobs={blobsClient} />
    <Comments issueId={head.id} me={me} people={people} head={headRevision} />
  </>
)}
// blobsClient is `makeBlobs(controlPlaneBlobs())`, built once at module scope in Attachments.tsx
// and exported; tests pass their own `blobs` to <Attachments/> directly.
```

```css
/* issue.css ⚑ — task 52. A file is a row of facts; a preview is the image, not a card. */
.iv-at-list { display: grid; gap: var(--s-2); padding: var(--s-3) var(--s-4); }
.iv-at-rows { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--s-1); }
.iv-at { display: flex; flex-wrap: wrap; align-items: baseline; gap: var(--s-2);
  padding: var(--s-1) var(--s-2); border-radius: var(--r-ctl); box-shadow: var(--e-hair); }
.iv-at-name, .iv-at-by, .iv-at-empty, .iv-at-note { font-family: var(--sans); color: var(--ink-2); }
.iv-at-name { color: var(--ink); }
.iv-at [data-fact], .iv-at-refused { font-family: var(--mono); font-variant-numeric: tabular-nums; }
.iv-at-refused { color: var(--fail); }
.iv-at-img { display: block; max-inline-size: 100%; max-block-size: 320px; border-radius: var(--r-card);
  box-shadow: var(--e-card); }
.iv-sub { display: flex; align-items: baseline; gap: var(--s-2); margin: 0;
  font-family: var(--sans); color: var(--ink-2); }
```

No `transition`, no `@keyframes`: an attachment or a subscribe change is caused by the person's
own click, and the row's appearance in place already shows the cause. The radius and elevation
names are the ones in `desktop/src/styles/tokens.css:110-122` today (`--r-ctl`, `--r-card`,
`--e-hair`, `--e-card`); `--s-*` are task 26's (`task-26.md:90`).

#### Tests to write first — `desktop/src/work/blob.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../api/client";
import { BlobError, bytesFact, makeBlobs, READ_CEILING_BYTES, sha256Hex, sniffImage,
  wireContentType, type BlobTransport } from "./blob";

const enc = (s: string) => new TextEncoder().encode(s).buffer as ArrayBuffer;
const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0]).buffer;
const SVG = enc(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`);
// SHA-256 of "abc", from FIPS 180-2's worked example.
const ABC = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad";

function transport(over: Partial<BlobTransport> = {}) {
  const put = vi.fn<BlobTransport["put"]>(async (_p, hex, body, type) =>
    ({ digest: `sha256:${hex}`, size: body.byteLength, contentType: type }));
  const get = vi.fn<BlobTransport["get"]>(async () => enc("abc"));
  return { put, get, ...over };
}
const file = (bytes: ArrayBuffer, name: string, type: string) => new File([bytes], name, { type });

describe("blob", () => {
  it("hashes the bytes itself and PUTs under that hash", async () => {
    expect(await sha256Hex(enc("abc"))).toBe(ABC);
    const t = transport({ put: vi.fn(async () => ({ digest: "sha256:" + "f".repeat(64), size: 3, contentType: "x" })) });
    const b = makeBlobs(t);
    await expect(b.uploadAttachment("pj-1", file(enc("abc"), "a.txt", ""))).rejects.toMatchObject(
      { refusal: { kind: "digest_mismatch", expected: `sha256:${ABC}`, got: "sha256:" + "f".repeat(64) } });
    expect(t.put.mock.calls[0][1]).toBe(ABC);          // the path's {sha} is OUR hash
  });
  it("names the upload by the local hash and keeps the name off the wire", async () => {
    const t = transport();
    const up = await makeBlobs(t).uploadAttachment("pj-1", file(enc("abc"), "notes.txt", "text/plain"));
    expect(up).toEqual({ digest: `sha256:${ABC}`, name: "notes.txt", size: 3, contentType: "application/octet-stream" });
    expect(JSON.stringify(t.put.mock.calls[0])).not.toContain("notes.txt");
  });
  it("sends a GIF and a PDF as octet-stream, a PNG as image/png", () => {
    expect(wireContentType("image/gif")).toBe("application/octet-stream");
    expect(wireContentType("application/pdf")).toBe("application/octet-stream");
    expect(wireContentType("image/png")).toBe("image/png");
  });
  it("sniffs by magic number, so an SVG named .png is not an image", () => {
    expect(sniffImage(PNG)).toBe("image/png");
    expect(sniffImage(SVG)).toBeNull();
    expect(sniffImage(enc("GIF89a....."))).toBe("image/gif");
    expect(sniffImage(enc("RIFF\0\0\0\0WEBPVP8 "))).toBe("image/webp");
  });
  it("carries the Hub's limitBytes verbatim on a 413", async () => {
    const e = new ApiError("blob_too_large" as never, "too large", 413, { limitBytes: "16777216" });
    const b = makeBlobs(transport({ put: vi.fn(async () => { throw e; }) }));
    await expect(b.uploadAttachment("pj-1", file(enc("abc"), "a", ""))).rejects.toMatchObject(
      { refusal: { kind: "too_large", limitBytes: "16777216", source: "hub" } });
    expect(bytesFact(16777216)).toBe("16777216 bytes · 16 MiB");
  });
  it("refuses past the desktop ceiling before reading a byte", async () => {
    const big = { size: READ_CEILING_BYTES + 1, name: "big", type: "", arrayBuffer: vi.fn() } as unknown as File;
    const t = transport();
    await expect(makeBlobs(t).uploadAttachment("pj-1", big)).rejects.toBeInstanceOf(BlobError);
    expect(big.arrayBuffer).not.toHaveBeenCalled();
    expect(t.put).not.toHaveBeenCalled();
  });
  it("does not hand over downloaded bytes that do not hash to the digest", async () => {
    const b = makeBlobs(transport({ get: vi.fn(async () => enc("abd")) }));
    await expect(b.fetchVerified("pj-1", `sha256:${ABC}`)).rejects.toMatchObject({ refusal: { kind: "digest_mismatch" } });
    await expect(makeBlobs(transport()).fetchVerified("pj-1", `sha256:${ABC}`)).resolves.toMatchObject({ image: null });
  });
});
```

| Test | Mutant it kills |
|---|---|
| `hashes the bytes itself and PUTs under that hash` | trusting a server-reported hash; a path named by anything but the local hash |
| `names the upload by the local hash …` | the file name sent to the Hub (decision 7) |
| `sends a GIF and a PDF as octet-stream …` | a 400 on every GIF from sending an unlisted type |
| `sniffs by magic number …` | preview decided by the file's type string or extension |
| `carries the Hub's limitBytes verbatim` | a generic *upload failed*; a desktop copy of 16 MiB |
| `refuses past the desktop ceiling …` | reading a 2 GB file into memory to hash it |
| `does not hand over downloaded bytes …` | trusting the bytes because the route said so |

`ApiError`'s fourth argument is `details` (`client.ts:62`); if its constructor takes them in
another order, the test follows the client (**UNVERIFIED** against the file's current signature).
