// A fixture for imports.test.cjs: five real imports, and three things a
// pattern would misread as imports.
import React from "react";
import { x } from "@s/n/sub";
export { y } from "e";
// import x from "c"
const quote = /"/g;

export function Thing() {
  const r = require("g");
  return <p>Can't "d" {String(quote)} {x} {String(r)}</p>;
}

export async function later() {
  return import("f");
}

void React;
