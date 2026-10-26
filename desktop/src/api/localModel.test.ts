import { afterEach, expect, it, vi } from "vitest";
import { createProductClient } from "./product";
import { selectTeamScope } from "./teamScope";

afterEach(() => selectTeamScope(""));

it("keeps model ownership routes on the host while named connections stay workspace-scoped", async () => {
  selectTeamScope("team-abcdefghijklmnopqrstuv");
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async () => new Response("{}"));
  const client = createProductClient("http://127.0.0.1:4455", fetcher);
  await client.localModel();
  await client.saveLocalModel({ expectedRevision: 3, enabled: true, startWithLuci: false, executablePath: "C:\\Ollama\\ollama.exe", modelStore: "E:\\Models", port: 11435, model: "coder:latest" });
  await client.startLocalModel(3);
  await client.stopLocalModel(3, "18446744073709551614");
  await client.agentConnections();
  expect(fetcher.mock.calls.map(([url]) => String(url))).toEqual([
    "http://127.0.0.1:4455/v1/setup/local-model",
    "http://127.0.0.1:4455/v1/setup/local-model",
    "http://127.0.0.1:4455/v1/setup/local-model/start",
    "http://127.0.0.1:4455/v1/setup/local-model/stop",
    "http://127.0.0.1:4455/v1/teams/team-abcdefghijklmnopqrstuv/workspace/v1/setup/agent-connections",
  ]);
  expect(JSON.parse(String(fetcher.mock.calls[3][1]?.body))).toEqual({ expectedRevision: 3, expectedGeneration: "18446744073709551614" });
});

it("rejects a host-model response after its requesting workspace changes", async () => {
  let finish!: (value: Response) => void;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const client = createProductClient("http://127.0.0.1:4455", fetcher);
  const pending = client.startLocalModel(3);
  const rejected = expect(pending).rejects.toMatchObject({ status: 409 });
  selectTeamScope("team-abcdefghijklmnopqrstuv");
  finish(new Response("{}"));
  await rejected;
  expect(fetcher).toHaveBeenCalledTimes(1);
});
