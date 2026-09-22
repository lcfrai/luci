// Contract §4.2's non-collapse rule, in one place.
//
// Three surfaces discriminate the same three cases and each needs its OWN prose, because the
// tail of the sentence names the act that failed: "…so the session was not created", "…so the
// session was not restarted", "…so the sandbox could not be rebuilt". A shared
// `sentenceFor(error, verb)` would produce worse English at every one of them and would change
// strings task 18's tests pin. What is actually duplicated is WHICH CODE MEANS WHAT, so that is
// what lives here; the prose stays at the call sites.

import type { ApiError } from "./client";

/** What a failed request means to a human, independent of which act failed. */
export type ProblemKind =
  /** Our own process is not up. Never say Docker: that costs the user ten minutes. */
  | "control-plane-down"
  /** The control plane answered; Docker did not. */
  | "backend-down"
  /** Everything else, including both restart 409s and the sixteenth code,
   *  `non_loopback_publish`, whose sandbox is RUNNING and must never be called stopped.
   *  The server wrote a sentence for a human; render it verbatim. */
  | "server-said";

/** Two named codes and a catch-all. Written as `if` returns rather than a switch on purpose:
 *  the input is `ProblemCode`, which has EIGHTEEN members, and enumerating them here would be
 *  a list to maintain for no gain — the rule is "these two are special, everything else is the
 *  server's sentence", and that is what this shape says.
 *
 *  THE EXHAUSTIVENESS THAT MATTERS IS AT THE CALL SITES, not here. Each of the three switches
 *  over the RETURN type, `ProblemKind`, with a declared return type and no `default:` — so with
 *  `strict: true` (`tsconfig.json:18`) a fourth `ProblemKind` fails every one of them with
 *  TS2366, "Function lacks ending return statement". That is the whole reason this module is
 *  worth having over three copies of an `if` chain, and it is why the guard that protects it
 *  greps the three CALLERS rather than this file. An earlier draft of that guard pointed here,
 *  which would have proved nothing: this function needs its catch-all and they must not have
 *  one. `attachLabel` in `components/develop/SessionPaneFooter.tsx` is the same call-site
 *  shape for the same reason. */
export function classify(error: ApiError): ProblemKind {
  if (error.code === "network_unavailable") return "control-plane-down";
  if (error.code === "backend_unavailable") return "backend-down";
  return "server-said";
}
