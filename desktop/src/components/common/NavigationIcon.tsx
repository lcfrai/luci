import type { JSX } from "react";

export type NavigationIconName =
  | "projects"
  | "close"
  | "add"
  | "back"
  | "discover"
  | "design"
  | "develop"
  | "deploy"
  | "chevron-down"
  | "chevron-right"
  | "check";

const paths: Record<NavigationIconName, string> = {
  "chevron-down": "m4 6 4 4 4-4",
  "chevron-right": "m6 4 4 4-4 4",
  check: "m3 8 3 3 7-7",
  projects: "M2.5 3.5h11v9h-11zM6 3.5v9",
  close: "m4 4 8 8M12 4l-8 8",
  add: "M8 3v10M3 8h10",
  back: "m7 3-5 5 5 5M2 8h12",
  discover: "M4 2.5h8v11H4zM6.5 5.5h3M6.5 8h3M6.5 10.5h2",
  design: "M2.5 3h11v10h-11zM2.5 6h11M6 6v7",
  develop: "m5.5 4-4 4 4 4m5-8 4 4-4 4M9 2.5l-2 11",
  deploy: "M3 10v3h10v-3M8 11V2m-4 4 4-4 4 4",
};

/** The navigation vocabulary shares one small, quiet stroke system. */
export function NavigationIcon({
  name,
}: {
  name: NavigationIconName;
}): JSX.Element {
  return (
    <svg
      className="navigation-icon"
      viewBox="0 0 16 16"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d={paths[name]} />
    </svg>
  );
}
