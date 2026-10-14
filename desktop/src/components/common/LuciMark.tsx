import fourfold from "../../assets/brand/handoff-2026-10-07/derivatives/luci-fourfold-currentcolor.svg";
import "./luci-mark.css";

/** Decorative approved mark; keep the visible Luci name or an accessible parent label. */
export function LuciMark({ size = 32, className = "" }: { size?: number; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={`luci-mark ${className}`.trim()}
      style={{ width: size, height: size, maskImage: `url("${fourfold}")`, WebkitMaskImage: `url("${fourfold}")` }}
    />
  );
}
