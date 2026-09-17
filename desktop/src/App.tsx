import { useState } from "react";
import { Terminal } from "./components/Terminal";

const DEFAULT_URL = "ws://localhost:8080/attach";

export default function App() {
  const [url, setUrl] = useState(DEFAULT_URL);
  const [connected, setConnected] = useState(false);

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh", background: "#0a0908", color: "#f2ede7" }}>
      <div style={{ display: "flex", gap: 8, padding: 8, borderBottom: "1px solid rgba(255,248,240,.075)" }}>
        <input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          style={{ flex: 1, background: "#1a1715", color: "#f2ede7", border: 0, padding: "6px 9px", borderRadius: 7 }}
        />
        <button onClick={() => setConnected((c) => !c)} style={{ padding: "6px 12px", borderRadius: 7 }}>
          {connected ? "Disconnect" : "Connect"}
        </button>
      </div>
      <div style={{ flex: 1, minHeight: 0, padding: 8 }}>
        {connected ? <Terminal url={url} /> : <div style={{ opacity: 0.5, fontSize: 13 }}>Not connected.</div>}
      </div>
    </div>
  );
}
