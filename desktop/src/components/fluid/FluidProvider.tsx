import type { ReactNode } from "react";
import { MotionConfig } from "framer-motion";
import { useAppearance } from "../../preferences/appearance";
import { SizeProvider } from "./lib/size-context";

export function FluidProvider({ children }: { children: ReactNode }) {
  const { density } = useAppearance();
  return <MotionConfig reducedMotion="user"><SizeProvider size={density === "compact" ? "compact" : "default"}>{children}</SizeProvider></MotionConfig>;
}
