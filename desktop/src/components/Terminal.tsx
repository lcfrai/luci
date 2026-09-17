import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Kind, decode, encode, encodeResize } from "../lib/wire";

export function Terminal({ url }: { url: string }) {
  const host = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!host.current) return;

    const term = new XTerm({
      fontFamily: '"Cascadia Mono", ui-monospace, Menlo, Consolas, monospace',
      fontSize: 12,
      theme: { background: "#0a0908", foreground: "#d6d0c8" },
      cursorBlink: true,
      // Let the agent CLI own the screen; we never reinterpret its output.
      convertEol: false,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host.current);
    fit.fit();

    const ws = new WebSocket(url);
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      ws.send(encodeResize(term.cols, term.rows));
      term.focus();
    };
    ws.onmessage = (ev) => {
      const { kind, payload } = decode(new Uint8Array(ev.data as ArrayBuffer));
      if (kind === Kind.Stdout) term.write(payload);
      else if (kind === Kind.Exit) term.write("\r\n\x1b[2m[session ended]\x1b[0m\r\n");
    };
    ws.onclose = () => term.write("\r\n\x1b[2m[disconnected]\x1b[0m\r\n");

    const onData = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(encode(Kind.Stdin, new TextEncoder().encode(data)));
      }
    });

    const onResize = () => {
      fit.fit();
      if (ws.readyState === WebSocket.OPEN) ws.send(encodeResize(term.cols, term.rows));
    };
    window.addEventListener("resize", onResize);

    return () => {
      window.removeEventListener("resize", onResize);
      onData.dispose();
      ws.close();
      term.dispose();
    };
  }, [url]);

  return <div ref={host} style={{ width: "100%", height: "100%" }} />;
}
