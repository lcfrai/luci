import type { ReactNode } from "react";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import "./journey.css";
export function ProjectRequired({
  children,
}: {
  children(projectId: string): ReactNode;
}) {
  const id = useCockpit((state) => state.selection.projectId);
  const setup = useSetup();
  if (!id)
    return (
      <div className="journey-empty">
        <span className="journey-eyebrow">YOUR NEXT IDEA STARTS HERE</span>
        <h1>Give it a place to grow.</h1>
        <p>
          Create a project or choose one from the rail. Its brief, design, code
          and releases stay together.
        </p>
        <button
          className="luci-button luci-primary"
          onClick={() => setup?.openProject()}
        >
          Create a project
        </button>
      </div>
    );
  return children(id);
}
export function JourneyHeader({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children?: ReactNode;
}) {
  return (
    <header className="journey-header">
      <div>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      <div className="journey-actions">{children}</div>
    </header>
  );
}
