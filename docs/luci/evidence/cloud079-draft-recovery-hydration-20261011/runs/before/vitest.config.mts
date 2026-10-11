import base from "/workspace/luci/desktop/vitest.config.ts";
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
export default {
  ...base,
  cacheDir: "/workspace/.luci-tools/draft-recovery-triage-20261011/runs/before/vite-cache",
  plugins: [{
    name: "recorded-draft-recovery-test-source",
    enforce: "pre",
    load(id) {
      if (id.split("?")[0] !== "/workspace/luci/desktop/src/components/common/ProjectDraftRecovery.test.tsx") return;
      const code = readFileSync("/workspace/.luci-tools/draft-recovery-triage-20261011/before/desktop/src/components/common/ProjectDraftRecovery.test.tsx", "utf8");
      writeFileSync("/workspace/.luci-tools/draft-recovery-triage-20261011/runs/before/loaded-test-source.json", JSON.stringify({ target: "/workspace/luci/desktop/src/components/common/ProjectDraftRecovery.test.tsx", source: "/workspace/.luci-tools/draft-recovery-triage-20261011/before/desktop/src/components/common/ProjectDraftRecovery.test.tsx",
        bytes: Buffer.byteLength(code), sha256: createHash("sha256").update(code).digest("hex") }, null, 2) + "\n");
      return { code, map: null };
    },
  }, ...(base.plugins ?? [])],
};
