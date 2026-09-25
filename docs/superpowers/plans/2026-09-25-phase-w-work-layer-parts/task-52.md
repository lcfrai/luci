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
