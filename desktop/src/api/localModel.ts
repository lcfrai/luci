export interface LocalModelProfile {
  contextTokens: number;
  kvCacheType: string;
  flashAttention: boolean;
  parallelism: number;
  jobMemoryMiB: number;
  processMemoryMiB: number;
  cpuLimitPercent: number;
}

export interface LocalModelConfiguration {
  enabled: boolean;
  startWithLuci: boolean;
  // The service projects paths only to the current host administrator.
  executablePath?: string;
  modelStore?: string;
  port: number;
  model: string;
  profile: LocalModelProfile;
}

export interface LocalModelHostCapacity {
  phase: string;
  reason: string;
  physicalAvailableBytes: number | null;
  commitAvailableBytes: number | null;
  cpuPercent: number | null;
  sampleAgeMillis: number | null;
}

export interface LocalModelSnapshot {
  available: boolean;
  canManage: boolean;
  revision: number;
  config: LocalModelConfiguration | null;
  status: {
    state: "disabled" | "stopped" | "starting" | "available" | "loaded" | "stopping" | "blocked" | "unknown";
    // uint64 values cross the browser boundary as decimal strings.
    generation: string;
    observedAt: string;
    endpoint?: string;
    model?: string;
    failure?: { code: string; exitCode?: number; hostCapacity?: LocalModelHostCapacity };
  };
}

export interface LocalModelSave {
  expectedRevision: number;
  enabled: boolean;
  startWithLuci: boolean;
  executablePath: string;
  modelStore: string;
  port: number;
  model: string;
}
