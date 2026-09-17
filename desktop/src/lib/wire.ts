// Mirror of sandboxd/internal/wire. Keep the constants in step with the Go
// definition — they are the contract between the two.
export const Kind = {
  Stdout: 0x00,
  Stdin: 0x01,
  Resize: 0x02,
  Exit: 0x03,
} as const;

export type KindValue = (typeof Kind)[keyof typeof Kind];

export function encode(kind: KindValue, payload: Uint8Array): Uint8Array<ArrayBuffer> {
  const out = new Uint8Array(payload.length + 1);
  out[0] = kind;
  out.set(payload, 1);
  return out;
}

export function decode(frame: Uint8Array): { kind: number; payload: Uint8Array } {
  if (frame.length === 0) throw new Error("wire: empty frame");
  return { kind: frame[0], payload: frame.subarray(1) };
}

export function encodeResize(cols: number, rows: number): Uint8Array<ArrayBuffer> {
  const body = new Uint8Array(4);
  new DataView(body.buffer).setUint16(0, cols);
  new DataView(body.buffer).setUint16(2, rows);
  return encode(Kind.Resize, body);
}
