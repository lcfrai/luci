import { useCallback, useEffect, useState } from "react";
import { dataApi, type DataColumn, type DataPolicy, type DataTable, type DataType } from "../../api/data";
import { errorText } from "../../api/product";

function useRead<T>(read: () => Promise<T>) {
  const [value, setValue] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    let active = true;
    setValue(null); setError(""); setNotice("");
    read().then(result => { if (active) setValue(result); }).catch(cause => { if (active) setError(errorText(cause)); });
    return () => { active = false; };
  }, [read]);
  async function run(label: string, task: () => Promise<unknown>) {
    if (pending) return false;
    setPending(label); setError(""); setNotice("");
    try { await task(); setValue(await read()); setNotice(`${label} completed.`); return true; }
    catch (cause) { setError(errorText(cause)); return false; }
    finally { setPending(""); }
  }
  return { value, error, pending, notice, run };
}
function Result({ state }: { state: { error: string; pending: string; notice: string } }) {
  return <>{state.error && <p role="alert" className="journey-error">{state.error}</p>}{(state.pending || state.notice) && <p role="status">{state.pending ? `${state.pending}…` : state.notice}</p>}</>;
}
const date = (value: string) => value ? new Date(value).toLocaleString() : "Not confirmed";

export function DataUsers({ projectId }: { projectId: string }) {
  const read = useCallback(() => dataApi.users(projectId), [projectId]);
  const state = useRead(read);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  return <section className="data-section" aria-label="Application users"><div className="data-heading"><div><h4>Application users</h4><p>Accounts for your app. These are separate from your Luci workspace account.</p></div><button type="button" className="luci-button" disabled={!!state.pending} onClick={() => void state.run("Refresh users", async () => {})}>Refresh users</button></div><Result state={state} />
    <details className="data-disclosure"><summary>Create a confirmed user</summary><form className="data-form" onSubmit={event => { event.preventDefault(); const submitted = password; setPassword(""); void state.run("Create user", () => dataApi.createUser(projectId, email, submitted)).then(saved => { if (saved) setEmail(""); }); }}><p>Create a confirmed account directly for testing or administration. This action does not send a verification email. Self-service confirmation and password recovery use the email connection below.</p><label>User email<input type="email" required autoComplete="off" value={email} disabled={!!state.pending} onChange={event => setEmail(event.target.value)} /></label><label>Initial password<input type="password" required minLength={12} maxLength={256} autoComplete="new-password" value={password} disabled={!!state.pending} onChange={event => setPassword(event.target.value)} /></label><button className="luci-button luci-primary" disabled={!!state.pending}>Create confirmed user</button></form></details>
    {!state.value && !state.error && <p role="status">Loading application users…</p>}
    {state.value && (state.value.users.length ? <div className="data-table-scroll"><table className="data-table"><caption>Showing up to {state.value.limit} users</caption><thead><tr><th>Email</th><th>Created</th><th>Confirmed</th></tr></thead><tbody>{state.value.users.map(user => <tr key={user.id}><th scope="row">{user.email}</th><td>{date(user.created_at)}</td><td>{date(user.email_confirmed_at || "")}</td></tr>)}</tbody></table></div> : <p>No application users yet.</p>)}
  </section>;
}

const types: DataType[] = ["text", "uuid", "boolean", "integer", "bigint", "numeric", "timestamptz", "jsonb"];
export function DataTables({ projectId }: { projectId: string }) {
  const read = useCallback(() => dataApi.tables(projectId), [projectId]);
  const state = useRead(read);
  const [name, setName] = useState("");
  const [columns, setColumns] = useState<DataColumn[]>([{ name: "title", type: "text", nullable: false }]);
  const [selected, setSelected] = useState("");
  const selectedTable = state.value?.tables.find(table => table.name === selected);
  function column(index: number, update: Partial<DataColumn>) { setColumns(value => value.map((item, current) => current === index ? { ...item, ...update } : item)); }
  return <section className="data-section" aria-label="Application tables"><div className="data-heading"><div><h4>Tables</h4><p>Create your data model and manage records. New tables start with row-level security enabled.</p></div><button type="button" className="luci-button" disabled={!!state.pending} onClick={() => void state.run("Refresh tables", async () => {})}>Refresh tables</button></div><Result state={state} />
    <details className="data-disclosure"><summary>Create a table</summary><form className="data-form" onSubmit={event => { event.preventDefault(); void state.run("Create table", () => dataApi.createTable(projectId, name, columns)).then(saved => { if (saved) { setSelected(name); setName(""); } }); }}><label>Table name<input required pattern="[a-z][a-z0-9_]{0,62}" placeholder="tasks" value={name} disabled={!!state.pending} onChange={event => setName(event.target.value)} /></label><p>A UUID <code>id</code> column is added automatically. Use lower-case names with underscores.</p><div className="data-columns">{columns.map((item, index) => <div className="data-column" key={index}><label>Column {index + 1} name<input required pattern="[a-z][a-z0-9_]{0,62}" value={item.name} disabled={!!state.pending} onChange={event => column(index, { name: event.target.value })} /></label><label>Column {index + 1} type<select value={item.type} disabled={!!state.pending} onChange={event => column(index, { type: event.target.value })}>{types.map(type => <option key={type}>{type}</option>)}</select></label><label className="data-check"><input type="checkbox" checked={item.nullable} disabled={!!state.pending} onChange={event => column(index, { nullable: event.target.checked })} />Optional</label><button type="button" className="luci-button" aria-label={`Remove column ${index + 1}`} disabled={!!state.pending || columns.length === 1} onClick={() => setColumns(value => value.filter((_, current) => current !== index))}>Remove</button></div>)}</div><div className="journey-actions"><button type="button" className="luci-button" disabled={!!state.pending || columns.length >= 32} onClick={() => setColumns(value => [...value, { name: "", type: "text", nullable: false }])}>Add column</button><button className="luci-button luci-primary" disabled={!!state.pending}>Create table with RLS</button></div></form></details>
    {!state.value && !state.error && <p role="status">Loading project tables…</p>}
    {state.value && !state.value.tables.length && <p>No public application tables yet.</p>}
    {!!state.value?.tables.length && <label className="data-select">Selected table<select value={selected} onChange={event => setSelected(event.target.value)}><option value="">Choose a table</option>{state.value.tables.map(table => <option key={table.name} value={table.name}>{table.name} · {table.rlsEnabled ? "RLS enabled" : "RLS disabled"}</option>)}</select></label>}
    {selectedTable && <TableData key={selectedTable.name} projectId={projectId} table={selectedTable} />}
  </section>;
}

function TableData({ projectId, table }: { projectId: string; table: DataTable }) {
  const read = useCallback(async () => { const [records, rls] = await Promise.all([dataApi.rows(projectId, table.name), dataApi.rls(projectId, table.name)]); return { ...records, rls }; }, [projectId, table.name]);
  const state = useRead(read);
  const [values, setValues] = useState<Record<string, string>>({});
  const [policy, setPolicy] = useState<DataPolicy>({ name: "", operation: "select", role: "authenticated", using: "owner", ownerColumn: "" });
  const [rowError, setRowError] = useState("");
  function insert() {
    setRowError("");
    try {
      const row: Record<string, unknown> = {};
      for (const item of table.columns.filter(item => item.name !== "id")) {
        const value = values[item.name];
        if ((value === undefined || value === "") && item.nullable) continue;
        if (item.type === "boolean") row[item.name] = value === "true";
        else if (["integer", "bigint", "numeric"].includes(item.type)) { if (!value?.trim() || !Number.isFinite(Number(value))) throw new Error(`Enter a number for ${item.name}.`); row[item.name] = value.trim(); }
        else if (item.type === "jsonb") { try { row[item.name] = JSON.parse(value || ""); } catch { throw new Error(`Enter valid JSON for ${item.name}.`); } }
        else row[item.name] = value || "";
      }
      void state.run("Insert record", () => dataApi.insertRow(projectId, table.name, row)).then(saved => { if (saved) setValues({}); });
    } catch (cause) { setRowError(errorText(cause)); }
  }
  return <div className="data-section"><h4>{table.name}</h4><Result state={state} />
    <details className="data-disclosure"><summary>Add a record</summary><form className="data-form" onSubmit={event => { event.preventDefault(); insert(); }}>{table.columns.filter(item => item.name !== "id").map(item => <label key={item.name}>{item.name}<small>{item.type}{item.nullable ? " · optional" : " · required"}</small>{item.type === "boolean" ? <select value={values[item.name] || "false"} disabled={!!state.pending} onChange={event => setValues(value => ({ ...value, [item.name]: event.target.value }))}><option value="false">False</option><option value="true">True</option></select> : item.type === "jsonb" ? <textarea required={!item.nullable} rows={3} value={values[item.name] || ""} disabled={!!state.pending} onChange={event => setValues(value => ({ ...value, [item.name]: event.target.value }))} /> : <input required={!item.nullable} value={values[item.name] || ""} disabled={!!state.pending} onChange={event => setValues(value => ({ ...value, [item.name]: event.target.value }))} />}</label>)}{rowError && <p className="journey-error" role="alert">{rowError}</p>}<button className="luci-button" disabled={!!state.pending}>Insert record</button></form></details>
    {state.value && <><div className="data-table-scroll"><table className="data-table"><caption>First {state.value.limit} records · project administrator view</caption><thead><tr>{table.columns.map(item => <th key={item.name}>{item.name}</th>)}</tr></thead><tbody>{state.value.rows.map((row, index) => <tr key={String(row.id ?? index)}>{table.columns.map(item => <td key={item.name}>{row[item.name] === null ? "NULL" : typeof row[item.name] === "object" ? JSON.stringify(row[item.name]) : String(row[item.name] ?? "")}</td>)}</tr>)}</tbody></table>{!state.value.rows.length && <p>No records in this table.</p>}</div>
      <div className="data-heading"><div><h4>Row-level security</h4><p>{state.value.rls.enabled ? "Enabled. Browser users can access only what your policies allow." : "Disabled. Enable RLS before exposing this table to your browser client."}</p></div>{!state.value.rls.enabled && <button type="button" className="luci-button" disabled={!!state.pending} onClick={() => void state.run("Enable row-level security", () => dataApi.enableRls(projectId, table.name))}>Enable RLS</button>}</div>
      <ul className="data-records">{state.value.rls.policies.map(rule => <li key={rule.name}><div><strong>{rule.name}</strong><small>{rule.operation} · {rule.roles.join(", ")}</small><code>{rule.using || rule.check}</code></div></li>)}</ul>{!state.value.rls.policies.length && <p>No policies. Browser clients cannot access rows while RLS is enabled.</p>}
      <details className="data-disclosure"><summary>Add an access policy</summary><form className="data-form" onSubmit={event => { event.preventDefault(); void state.run("Create access policy", () => dataApi.policy(projectId, table.name, policy)).then(saved => { if (saved) setPolicy(value => ({ ...value, name: "" })); }); }}><label>Policy name<input required pattern="[a-z][a-z0-9_]{0,62}" value={policy.name} disabled={!!state.pending} onChange={event => setPolicy(value => ({ ...value, name: event.target.value }))} /></label><label>Allowed operation<select value={policy.operation} disabled={!!state.pending} onChange={event => setPolicy(value => ({ ...value, operation: event.target.value as DataPolicy["operation"] }))}>{["select", "insert", "update", "delete", "all"].map(operation => <option key={operation}>{operation}</option>)}</select></label><label>Who can access rows<select value={policy.using} disabled={!!state.pending} onChange={event => setPolicy(value => ({ ...value, using: event.target.value as DataPolicy["using"] }))}><option value="owner">Signed-in user owns the row</option><option value="authenticated">Any signed-in user</option></select></label>{policy.using === "owner" && <label>UUID column containing the user ID<select required value={policy.ownerColumn} disabled={!!state.pending} onChange={event => setPolicy(value => ({ ...value, ownerColumn: event.target.value }))}><option value="">Choose an owner column</option>{table.columns.filter(item => item.type === "uuid" && item.name !== "id").map(item => <option key={item.name}>{item.name}</option>)}</select></label>}<p>{policy.using === "owner" ? "Access is checked against auth.uid(). Add a UUID owner column to your data model." : "This grants the selected operation on every row to every authenticated app user."}</p><button className="luci-button" disabled={!!state.pending}>Create access policy</button></form></details>
    </>}
  </div>;
}

export function DataStorage({ projectId }: { projectId: string }) {
  const read = useCallback(() => dataApi.buckets(projectId), [projectId]);
  const state = useRead(read);
  const [name, setName] = useState("");
  const [isPublic, setPublic] = useState(false);
  const [selected, setSelected] = useState("");
  return <section className="data-section" aria-label="Application storage"><div className="data-heading"><div><h4>File storage</h4><p>Private buckets by default. Uploaded files stay in this project’s retained storage volume.</p></div><button type="button" className="luci-button" disabled={!!state.pending} onClick={() => void state.run("Refresh buckets", async () => {})}>Refresh buckets</button></div><Result state={state} />
    <details className="data-disclosure"><summary>Create a bucket</summary><form className="data-form" onSubmit={event => { event.preventDefault(); void state.run("Create bucket", () => dataApi.createBucket(projectId, name, isPublic)).then(saved => { if (saved) { setSelected(name); setName(""); setPublic(false); } }); }}><label>Bucket name<input required pattern="[a-z0-9][a-z0-9._-]{0,62}" value={name} disabled={!!state.pending} onChange={event => setName(event.target.value)} /></label><label className="data-check"><input type="checkbox" checked={isPublic} disabled={!!state.pending} onChange={event => setPublic(event.target.checked)} />Make objects publicly readable by anyone with their URL.</label><button className="luci-button" disabled={!!state.pending}>Create {isPublic ? "public" : "private"} bucket</button></form></details>
    {!state.value && !state.error && <p role="status">Loading storage buckets…</p>}
    {state.value && !state.value.length && <p>No buckets yet.</p>}
    {!!state.value?.length && <label className="data-select">Selected bucket<select value={selected} onChange={event => setSelected(event.target.value)}><option value="">Choose a bucket</option>{state.value.map(item => <option key={item.id} value={item.id}>{item.name} · {item.public ? "Public" : "Private"}</option>)}</select></label>}
    {selected && state.value?.some(item => item.id === selected) && <BucketObjects key={selected} projectId={projectId} name={selected} />}
  </section>;
}
function BucketObjects({ projectId, name }: { projectId: string; name: string }) {
  const [prefix, setPrefix] = useState("");
  const read = useCallback(() => dataApi.objects(projectId, name, prefix), [projectId, name, prefix]);
  const state = useRead(read);
  const [file, setFile] = useState<File | null>(null);
  const [key, setKey] = useState("");
  const [inputVersion, setInputVersion] = useState(0);
  return <div className="data-section"><div className="data-heading"><h4>{name}{prefix ? ` / ${prefix}` : ""}</h4>{prefix && <button type="button" className="luci-button" disabled={!!state.pending} onClick={() => setPrefix(value => value.split("/").slice(0, -1).join("/"))}>Parent folder</button>}</div><Result state={state} />
    <form className="data-form" onSubmit={event => { event.preventDefault(); if (file) void state.run("Upload file", () => dataApi.upload(projectId, name, key, file)).then(saved => { if (saved) { setFile(null); setKey(""); setInputVersion(value => value + 1); } }); }}><label>File to upload<input key={inputVersion} type="file" required disabled={!!state.pending} onChange={event => { const next = event.target.files?.[0] || null; setFile(next); setKey(next ? [prefix, next.name].filter(Boolean).join("/") : ""); }} /></label><label>Object path<input required value={key} disabled={!!state.pending} onChange={event => setKey(event.target.value)} /><small>Relative path, such as reports/summary.pdf. Maximum file size: 8 MiB.</small></label><button className="luci-button" disabled={!!state.pending || !file || file.size > 8 * 1024 * 1024}>Upload file</button>{file && file.size > 8 * 1024 * 1024 && <p role="alert" className="journey-error">Choose a file no larger than 8 MiB.</p>}</form>
    {state.value && <><p>Showing up to {state.value.limit} entries in this folder.</p><ul className="data-records">{state.value.objects.map(item => <li key={item.name}><div><strong>{item.name}</strong><small>{item.id ? `${Math.ceil((item.metadata?.size || 0) / 1024)} KiB` : "Folder"}</small></div>{item.id ? <button type="button" className="luci-button" disabled={!!state.pending} onClick={() => void state.run("Download file", () => dataApi.downloadObject(projectId, name, [prefix, item.name].filter(Boolean).join("/")))}>Download {item.name}</button> : <button type="button" className="luci-button" disabled={!!state.pending} onClick={() => setPrefix([prefix, item.name].filter(Boolean).join("/"))}>Open {item.name}</button>}</li>)}</ul>{!state.value.objects.length && <p>This folder is empty.</p>}</>}
  </div>;
}

export function DataRealtime({ projectId }: { projectId: string }) {
  const read = useCallback(async () => { const [tables, realtime] = await Promise.all([dataApi.tables(projectId), dataApi.realtime(projectId)]); return { ...tables, ...realtime, available: tables.tables }; }, [projectId]);
  const state = useRead(read);
  return <section className="data-section" aria-label="Realtime tables"><div className="data-heading"><div><h4>Live database changes</h4><p>Publish changes for selected tables. Your app subscribes with a signed-in user; row-level security still applies.</p></div><button type="button" className="luci-button" disabled={!!state.pending} onClick={() => void state.run("Refresh realtime", async () => {})}>Refresh realtime</button></div><Result state={state} />
    {state.value && <ul className="data-records">{state.value.available.map(table => <li key={table.name}><div><strong>{table.name}</strong><small>{table.rlsEnabled ? "RLS enabled" : "Enable RLS before exposing changes"} · {state.value!.tables.includes(table.name) ? "Published" : "Not published"}</small></div><button type="button" className="luci-button" disabled={!!state.pending || (!table.rlsEnabled && !state.value!.tables.includes(table.name))} onClick={() => void state.run("Update realtime publication", () => dataApi.updateRealtime(projectId, table.name, !state.value!.tables.includes(table.name)))}>{state.value!.tables.includes(table.name) ? "Disable" : "Enable"} realtime for {table.name}</button></li>)}</ul>}
    {state.value && !state.value.available.length && <p>Create an application table first.</p>}
  </section>;
}

export function DataFunctions({ projectId }: { projectId: string }) {
  const read = useCallback(() => dataApi.functions(projectId), [projectId]);
  const state = useRead(read);
  const [input, setInput] = useState('{"name":"Luci"}');
  const [output, setOutput] = useState<unknown>(null);
  const [issue, setIssue] = useState("");
  return <section className="data-section" aria-label="Data functions"><h4>Functions</h4><p>Check the bundled function against the real running service. Publishing custom functions from your project is not yet available.</p><Result state={state} />{state.value?.functions.map(item => <form key={item.name} className="data-form" onSubmit={event => { event.preventDefault(); setIssue(""); setOutput(null); try { const body: unknown = JSON.parse(input); if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("Enter a JSON object for the function request."); void state.run("Invoke function", async () => { const response = await dataApi.invoke(projectId, item.name, body as Record<string, unknown>); setOutput(response.result); }); } catch (cause) { setIssue(errorText(cause)); } }}><h4>{item.name}</h4><label>Function request<textarea rows={4} maxLength={65536} value={input} disabled={!!state.pending} onChange={event => setInput(event.target.value)} /></label>{issue && <p role="alert" className="journey-error">{issue}</p>}<button className="luci-button" disabled={!!state.pending}>Invoke {item.name}</button></form>)}{output !== null && <pre className="journey-code" aria-label="Function response">{JSON.stringify(output, null, 2)}</pre>}</section>;
}
