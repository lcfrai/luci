import { expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createProductClient } from "../../api/product";
import { LocalTools } from "./LocalTools";

it("waits for the saved identity before allowing edits so initial loading cannot overwrite typing", async () => {
  let resolveIdentity!: (response: Response) => void;
  let resolveRuntime!: (response: Response) => void;
  const identity = new Promise<Response>((resolve) => {
    resolveIdentity = resolve;
  });
  const runtime = new Promise<Response>((resolve) => {
    resolveRuntime = resolve;
  });
  const fetcher = vi
    .fn<typeof fetch>()
    .mockImplementation(async (url, init) => {
      if (String(url).endsWith("/git/identity")) {
        return init?.method === "POST"
          ? new Response(init.body as string)
          : identity;
      }
      if (String(url).endsWith("/runtime/install")) return runtime;
      return new Response(JSON.stringify({ providers: [] }));
    });
  render(
    <LocalTools client={createProductClient("http://localhost", fetcher)} />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await screen.findByText("Loading Git identity and runtime…");
  const name = screen.getByRole("textbox", {
    name: "Commit author name",
  }) as HTMLInputElement;
  const email = screen.getByRole("textbox", {
    name: "Commit author email",
  }) as HTMLInputElement;
  const save = screen.getByRole("button", {
    name: "Save Git identity",
  }) as HTMLButtonElement;
  expect(name.disabled).toBe(true);
  expect(email.disabled).toBe(true);
  expect(save.disabled).toBe(true);
  await userEvent.type(name, "Would be lost");
  await userEvent.click(save);
  expect(name.value).toBe("");
  expect(fetcher.mock.calls.some((call) => call[1]?.method === "POST")).toBe(
    false,
  );
  await act(async () =>
    resolveIdentity(
      new Response(
        JSON.stringify({ name: "Saved name", email: "saved@example.invalid" }),
      ),
    ),
  );
  expect(name.disabled).toBe(true);
  await act(async () =>
    resolveRuntime(
      new Response(
        JSON.stringify({ id: "", status: "idle", message: "Not installed" }),
      ),
    ),
  );
  await waitFor(() => expect(name.disabled).toBe(false));
  expect(name.value).toBe("Saved name");
  expect(email.value).toBe("saved@example.invalid");
  expect(screen.queryByText("Loading Git identity and runtime…")).toBeNull();
  await userEvent.clear(name);
  await userEvent.type(name, "My chosen name");
  await userEvent.clear(email);
  await userEvent.type(email, "chosen@example.invalid");
  await userEvent.click(save);
  await screen.findByText(/Git identity saved for this machine/);
  const posts = fetcher.mock.calls.filter((call) => call[1]?.method === "POST");
  expect(posts).toHaveLength(1);
  expect(JSON.parse(posts[0][1]?.body as string)).toEqual({
    name: "My chosen name",
    email: "chosen@example.invalid",
  });
});

it("keeps identity controls disabled after a load failure and offers a working retry", async () => {
  let failIdentity = true;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    if (String(url).endsWith("/git/identity")) {
      if (failIdentity) {
        failIdentity = false;
        return new Response(
          JSON.stringify({
            error: "unavailable",
            message: "Git identity unavailable",
          }),
          { status: 503 },
        );
      }
      return new Response(
        JSON.stringify({
          name: "Recovered",
          email: "recovered@example.invalid",
        }),
      );
    }
    return new Response(
      JSON.stringify(
        String(url).endsWith("/runtime/install")
          ? { id: "", status: "idle", message: "Not installed" }
          : { providers: [] },
      ),
    );
  });
  render(
    <LocalTools client={createProductClient("http://localhost", fetcher)} />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await screen.findByText(/Git identity unavailable/);
  const name = screen.getByRole("textbox", {
    name: "Commit author name",
  }) as HTMLInputElement;
  expect(name.disabled).toBe(true);
  await userEvent.click(
    screen.getByRole("button", {
      name: "Retry loading Git identity and runtime",
    }),
  );
  await screen.findByDisplayValue("Recovered");
  expect(name.disabled).toBe(false);
  expect(screen.queryByText(/Git identity unavailable/)).toBeNull();
});

it("saves a personal agent key only on explicit submission and clears it after success", async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(
    async (url, init) =>
      new Response(
        JSON.stringify(
          String(url).endsWith("/git/identity")
            ? { name: "Eval", email: "eval@example.invalid" }
            : String(url).endsWith("/runtime/install")
              ? { id: "", status: "idle", message: "Not installed" }
              : {
                  providers: [
                    {
                      id: "anthropic",
                      name: "Claude Code",
                      configured:
                        init?.method === "POST" &&
                        !!JSON.parse(init.body as string).apiKey,
                    },
                  ],
                },
        ),
      ),
  );
  render(
    <LocalTools client={createProductClient("http://localhost", fetcher)} />,
  );
  await userEvent.click(
    screen.getByText("Git identity, agent connection and runtime"),
  );
  await screen.findByDisplayValue("Eval");
  const input = screen.getByLabelText(/Anthropic API key/) as HTMLInputElement;
  await userEvent.type(input, "sk-ant-test-eval-only");
  expect(
    fetcher.mock.calls.filter((c) => c[1]?.method === "POST"),
  ).toHaveLength(0);
  await userEvent.click(
    screen.getByRole("button", { name: "Save agent connection" }),
  );
  await screen.findByText("Personal API key saved on this machine.");
  expect(input.value).toBe("");
  const posts = fetcher.mock.calls.filter((c) => c[1]?.method === "POST");
  expect(posts).toHaveLength(1);
  expect(JSON.parse(posts[0][1]?.body as string)).toEqual({
    apiKey: "sk-ant-test-eval-only",
  });
  await userEvent.click(
    screen.getByRole("button", { name: "Remove saved key" }),
  );
  await waitFor(() =>
    expect(
      screen.queryByText("Personal API key saved on this machine."),
    ).toBeNull(),
  );
  expect(
    JSON.parse(
      fetcher.mock.calls[fetcher.mock.calls.length - 1][1]?.body as string,
    ),
  ).toEqual({ apiKey: "" });
});


it('keeps manual runtime preparation blocked until the actual readiness state permits it', async () => {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => new Response(JSON.stringify(
    String(url).endsWith('/git/identity') ? { name: 'Builder', email: 'builder@example.invalid' }
      : String(url).endsWith('/runtime/install') ? init?.method === 'POST' ? { status: 'installing', message: 'Real job accepted.' } : { status: 'idle' }
        : { providers: [] }
  )));
  const client = createProductClient('http://localhost', fetcher);
  const view = render(<LocalTools client={client} runtimeBlockedReason="Confirm Docker storage in Settings." />);
  await userEvent.click(screen.getByText('Git identity, agent connection and runtime'));
  await screen.findByDisplayValue('Builder');
  const install = await screen.findByRole('button', { name: 'Install agent runtime' });
  expect((install as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole('button', {name: 'Prepare document extraction'})).toHaveProperty('disabled', true);
  await userEvent.click(install);
  expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith('/runtime/install') && init?.method === 'POST')).toHaveLength(0);
  view.rerender(<LocalTools client={client} runtimeBlockedReason="" />);
  await userEvent.click(install);
  await screen.findByText('installing · Real job accepted.');
  expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith('/runtime/install') && init?.method === 'POST')).toHaveLength(1);
});

it('prepares the distinct document package only on explicit administrator action', async () => {
 const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => new Response(JSON.stringify(
  String(url).endsWith('/git/identity') ? {name: 'Builder', email: 'builder@example.invalid'} : String(url).endsWith('/runtime/install') ? init?.method === 'POST' ? {id: 'document-job', status: 'installing', message: 'Verifying the document package.'} : {status: 'idle'} : {providers: []}
 )));
 render(<LocalTools client={createProductClient('http://localhost', fetcher)} />);
 await userEvent.click(screen.getByText('Git identity, agent connection and runtime'));
 await screen.findByDisplayValue('Builder');
 expect(fetcher.mock.calls.filter(([url, init]) => String(url).endsWith('/runtime/install') && init?.method === 'POST')).toHaveLength(0);
 await userEvent.click(screen.getByRole('button', {name: 'Prepare document extraction'}));
 await screen.findByText('installing · Verifying the document package.');
 const writes = fetcher.mock.calls.filter(([url, init]) => String(url).endsWith('/runtime/install') && init?.method === 'POST');
 expect(writes).toHaveLength(1);expect(JSON.parse(writes[0][1]!.body as string)).toEqual({mode: 'document-packaged'});
 expect(screen.getByRole('button', {name: 'Prepare document extraction'})).toHaveProperty('disabled', true);
});

it('does not offer document preparation without current backing management permission', async () => {
 const fetcher=vi.fn<typeof fetch>().mockImplementation(async()=>new Response(JSON.stringify({name: '', email: ''})));
 render(<LocalTools client={createProductClient('http://localhost', fetcher)} manageBacking={false} />);
 await userEvent.click(screen.getByText('Git identity, agent connection and runtime'));
 expect(screen.queryByRole('button', {name: 'Prepare document extraction'})).toBeNull();
 expect(screen.queryByRole('button', {name: 'Assemble document image artifact'})).toBeNull();
 expect(fetcher.mock.calls.filter(([,init])=>init?.method==='POST')).toHaveLength(0);
});

it('requires a positively settled binary proof and explicit action for maintainer assembly', async () => {
 const fetcher=vi.fn<typeof fetch>().mockImplementation(async(url,init)=>new Response(JSON.stringify(
  String(url).endsWith('/git/identity')?{name:'Maintainer',email:'maintainer@example.invalid'}:String(url).endsWith('/runtime/install')?init?.method==='POST'?{id:'assembly-job',status:'installing',message:'Staging exact verified inputs.'}:{id:'binary-job',status:'facts-recorded',inputProof:{identity:{purpose:'binary-bindings'},phase:'facts-recorded',cleanupConfirmed:true,resourcesHeld:false,publicationUnknown:false}}:{providers:[]}
 )));
 render(<LocalTools client={createProductClient('http://localhost',fetcher)} />);
 await userEvent.click(screen.getByText('Git identity, agent connection and runtime'));
 await screen.findByDisplayValue('Maintainer');
 expect(fetcher.mock.calls.filter(([,init])=>init?.method==='POST')).toHaveLength(0);
 await userEvent.click(screen.getByText('Maintainer document image assembly'));
 await userEvent.click(screen.getByRole('button',{name:'Assemble document image artifact'}));
 await screen.findByText('installing · Staging exact verified inputs.');
 const writes=fetcher.mock.calls.filter(([,init])=>init?.method==='POST');
 expect(writes).toHaveLength(1);expect(JSON.parse(writes[0][1]!.body as string)).toEqual({mode:'document-assemble'});
 expect(screen.getByText(/does not prepare extraction/)).toBeTruthy();
});

it.each(['signature-proof','held','unknown'])('does not offer assembly authority for a %s result',async(kind)=>{
 const proof={identity:{purpose:kind==='signature-proof'?'signature-bindings':'binary-bindings'},phase:'facts-recorded',cleanupConfirmed:kind!=='held',resourcesHeld:kind==='held',publicationUnknown:kind==='unknown'};
 const fetcher=vi.fn<typeof fetch>().mockImplementation(async(url)=>new Response(JSON.stringify(String(url).endsWith('/git/identity')?{name:'Maintainer',email:'maintainer@example.invalid'}:String(url).endsWith('/runtime/install')?{id:'proof-job',status:'facts-recorded',inputProof:proof}:{providers:[]})));
 render(<LocalTools client={createProductClient('http://localhost',fetcher)} />);
 await userEvent.click(screen.getByText('Git identity, agent connection and runtime'));await screen.findByDisplayValue('Maintainer');
 await userEvent.click(screen.getByText('Maintainer document image assembly'));
 const action=screen.getByRole('button',{name:'Assemble document image artifact'});expect(action).toHaveProperty('disabled',true);await userEvent.click(action);
 expect(fetcher.mock.calls.filter(([,init])=>init?.method==='POST')).toHaveLength(0);
});
