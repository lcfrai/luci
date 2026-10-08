/** Recorded review obligations are separate from activity and authorize no execution. */
export interface ProjectRunAttention {
  id: string;
  state: string;
  kind: "task-review" | "task-question" | "verification" | "release-review" | "blocked";
  contextStatus: "current" | "changed" | "unavailable";
}

export function validRunAttention(value: unknown, maximum: number): value is ProjectRunAttention[] {
  if (!Array.isArray(value) || value.length > maximum) return false;
  return value.every(run => run && typeof run.id === "string" && run.id.length > 0 && run.id.length <= 512 &&
    typeof run.state === "string" && run.state.length > 0 && run.state.length <= 128 && (run.state !== "complete" || run.kind === "task-question") &&
    (run.state !== "cancelled" || run.kind === "task-review" || run.kind === "task-question") &&
    ["task-review", "task-question", "verification", "release-review", "blocked"].includes(run.kind) &&
    ["current", "changed", "unavailable"].includes(run.contextStatus)) && new Set(value.map(run => run.id)).size === value.length;
}
