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
