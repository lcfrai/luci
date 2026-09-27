// Types for defect6.mjs, so desktop/src/e2eDefect6.test.ts typechecks under `strict` without
// `allowJs` (tsconfig.json includes src/ only; this file is reached through the import).

export declare const DEFECT6_UNCHECKED: string;
export declare const TILE_TOLERANCE_PX: number;

export interface ResizedTile {
  id: string;
  width: number;
  height: number;
}

export declare function tileKeptSize(
  resized: ResizedTile | undefined,
  measure: (id: string) => Promise<{ width: number; height: number }>,
  already: string,
): Promise<ResizedTile>;
