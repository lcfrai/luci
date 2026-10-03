import { useState } from "react";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import "./projects.css";

export function ProjectsHome() {
  const projects = useCockpit((state) => state.projects);
  const loading = useCockpit((state) => state.loading.projects);
  const lastError = useCockpit((state) => state.lastError);
  const [query, setQuery] = useState("");
  const setup = useSetup();
  const filtered = projects.filter((project) =>
    project.name.toLowerCase().includes(query.toLowerCase()),
  );
  return (
    <section className="projects-home">
      <header className="projects-heading">
        <div>
          <h1>Your projects</h1>
          <p>
            {setup?.workspace?.mode === "cloud"
              ? `Working in ${setup.workspace.host}. Your account’s projects stay in this workspace.`
              : "Bring an idea to life, from the first brief to a working release."}
          </p>
        </div>
        <button
          className="luci-button luci-primary"
          onClick={() => setup?.openProject()}
        >
          New project
        </button>
      </header>
      <div className="projects-utilities">
        <div className="setup-actions">
          <button
            className="luci-button"
            onClick={() => setup?.openProject("import")}
          >
            Import repository
          </button>
          <button
            className="luci-button"
            onClick={() => setup?.openProject("restore")}
          >
            Restore backup
          </button>
        </div>
        <button className="luci-button" onClick={() => setup?.openOnboarding()}>
          Workspace setup
        </button>
      </div>
      {setup?.statusError && (
        <p className="journey-error" role="alert">
          {setup.statusError}{" "}
          <button className="luci-button" onClick={() => setup.retryStatus()}>
            Retry workspace connection
          </button>
        </p>
      )}
      {setup?.status && !setup.status.configured && (
        <p className="projects-setup-note">
          Your workspace setup is unfinished. You can return to it at any time;
          agents and previews need Docker and the agent runtime.
        </p>
      )}
      {lastError && (
        <p className="journey-error" role="alert">
          {lastError.message}{" "}
          <button
            className="luci-button"
            onClick={() => setup?.refreshProjects()}
          >
            Refresh projects
          </button>
        </p>
      )}
      {projects.length > 0 && (
        <label className="luci-field projects-filter">
          Find a project
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search by name"
          />
        </label>
      )}
      {loading && projects.length === 0 ? (
        <p role="status">Loading your projects…</p>
      ) : projects.length === 0 ? (
        <div className="projects-empty">
          <h2>What would you like to make?</h2>
          <p>
            Start a project, describe the outcome and sketch the experience.
            Connect a coding agent when you are ready to build.
          </p>
          <ol>
            <li>
              <strong>Define</strong>
              <span>Decide what the solution should do.</span>
            </li>
            <li>
              <strong>Design</strong>
              <span>Shape the screens and interactions.</span>
            </li>
            <li>
              <strong>Develop</strong>
              <span>Build, inspect and verify the code.</span>
            </li>
            <li>
              <strong>Deploy</strong>
              <span>Preview locally, then publish.</span>
            </li>
          </ol>
          <button
            className="luci-button luci-primary"
            onClick={() => setup?.openProject()}
          >
            Create your first project
          </button>
        </div>
      ) : (
        <ul className="projects-list">
          {filtered.map((project) => (
            <li key={project.id}>
              <button
                onClick={() => {
                  useCockpit.getState().selectProject(project.id);
                  useCockpit.getState().setLens("discover");
                }}
              >
                <strong>{project.name}</strong>
                <span>{project.path}</span>
                <span className="project-open">Open project →</span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {projects.length > 0 && filtered.length === 0 && (
        <p>No projects match “{query}”.</p>
      )}
    </section>
  );
}
