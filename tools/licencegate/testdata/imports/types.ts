import type { A } from "type-only";
import { type B, c } from "mixed";
export type { D } from "type-reexport";
import eq = require("import-equals");
void c;
void eq;
export type { A, B };
