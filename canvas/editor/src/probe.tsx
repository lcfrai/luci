// Deleted by task 23, which replaces it with real exports.
import { useState, type ReactNode } from "react";
export function PackageProbe(props: { children?: ReactNode }): ReactNode {
  const [n] = useState(1); // a hook: fails with "Invalid hook call" under two Reacts
  return <span data-testid="probe">{n}{props.children}</span>;
}
