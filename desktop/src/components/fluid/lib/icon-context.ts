// Adapted from registry/default/lib/icon-context.ts in Fluid Functionalism.
// MIT copyright Micka Touillaud; only the reusable icon contract is needed.
import type { ComponentType } from "react";

export interface IconComponentProps {
  size?: number;
  strokeWidth?: number;
  className?: string;
}
export type IconComponent = ComponentType<IconComponentProps>;
