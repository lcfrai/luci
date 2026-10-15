import { useState } from "react";
import { ProjectOverviewBoard } from "./ProjectOverviewBoard";
import { useProjectHome } from "./ProjectHomeProvider";
import { Input } from "../ui/input";
import { Button } from "../fluid/components/button";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import "./projects.css";
import { useTeamWorkspace } from "../teams/TeamWorkspaceProvider";

export function ProjectsHome() {
  const projects = useCockpit((state) => state.projects);
  const loading = useCockpit((state) => state.loading.projects);
  const lastError = useCockpit((state) => state.lastError);
  const [query, setQuery] = useState("");
  const [showDetails, setShowDetails] = useState(false);
  const home = useProjectHome();
  const visibleIDs = home ? new Set(home.home?.projects.map(project => project.id) ?? []) : null;
  const activity = new Map(home?.home?.projects.map(project => [project.id, project]) ?? []);
  const setup = useSetup();
  const team = useTeamWorkspace();
  const canCreate = !team?.teamId || !!(team.selected?.permissions.edit && team.selected.permissions.admin);
  const filtered = projects.filter((project) =>
    (!visibleIDs || visibleIDs.has(project.id)) && project.name.toLowerCase().includes(query.toLowerCase()),
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
        <Button type="button"
          variant="primary"
          disabled={!canCreate}
          onClick={() => setup?.openProject()}
        >
          New project
        </Button>
      </header>
      <details className="projects-tools">
        <summary>Project tools</summary>
      <div className="projects-utilities">
        <div className="setup-actions">
          <Button type="button"
            variant="secondary"
            disabled={!canCreate}
          onClick={() => setup?.openProject("import")}
          >
            Import repository
          </Button>
          <Button type="button"
            variant="secondary"
            disabled={!canCreate}
          onClick={() => setup?.openProject("restore")}
          >
            Restore backup
          </Button>
        </div>
        <Button type="button" variant="secondary" onClick={() => setup?.openOnboarding()}>
          Workspace setup
        </Button>
      </div>
      </details>
      {!canCreate && <p className="projects-setup-note">Your team administrator can create or import projects and share them with you.</p>}
      {setup?.statusError && (
        <p className="journey-error" role="alert">
          {setup.statusError}{" "}
          <Button type="button" variant="secondary" onClick={() => setup.retryStatus()}>
            Retry workspace connection
          </Button>
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
          <Button type="button"
            variant="secondary" loading={loading}
            onClick={() => setup?.refreshProjects()}
          >
            Refresh projects
          </Button>
        </p>
      )}
      {projects.length > 0 && (
        <label className="luci-field projects-filter">
          Find a project
          <Input
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
          <h2>{canCreate ? "What would you like to make?" : "No projects shared yet"}</h2>
          <p>
            {canCreate ? "Start a project, describe the outcome and sketch the experience. Connect a coding agent when you are ready to build." : "Once your team administrator grants project access, your shared projects will appear here."}
          </p>
          <ol>
            <li>
              <strong>Discover</strong>
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
          <Button type="button"
            variant="secondary"
            disabled={!canCreate}
          onClick={() => setup?.openProject()}
          >
            Create your first project
          </Button>
        </div>
      ) : (
        <>
          {home?.error && <p className="journey-error" role="status">{home.error} <Button variant="secondary" onClick={home.refresh}>Refresh activity</Button></p>}
          {!home?.home && <p role="status">{home?.busy ? 'Loading project activity…' : 'Project activity is unavailable. Open a project to inspect its saved work.'}</p>}
          {(!home || home.home) && <ul className="home-activity-list">{filtered.map(project => {
            const row = activity.get(project.id);
            const pinned = !!home?.home?.pinnedProjectIds.includes(project.id);
            return <li key={project.id}>
              <div><strong>{project.name}</strong><div className="home-activity-status" aria-label={`Activity for ${project.name}`}>
                {row?.working && <span>Working</span>}{row?.needsInput && <span>Needs input</span>}
                {row?.queued && <span>Queued</span>}{row?.stops.length ? <span>Stopped · {row.stops[row.stops.length - 1].kind}</span> : null}
                {row?.unknown && <span>Some activity unconfirmed</span>}{!row && <span>Activity unavailable</span>}
                {row && !row.working && !row.needsInput && !row.queued && !row.unknown && !row.stops.length && <span>Ready to open</span>}
              </div></div>
              <div className="home-activity-actions">
                <Button variant="secondary" disabled={!row || !!home?.busy} aria-pressed={pinned} onClick={() => void home?.pin(project.id, !pinned)}>{pinned ? 'Unpin' : 'Pin'}</Button>
                <Button variant="primary" onClick={() => { if (home) home.open(project.id); else useCockpit.getState().selectProject(project.id); }}>Open project</Button>
              </div>
            </li>;
          })}</ul>}
          {(!home || home.home) && <Button type="button" variant="secondary" aria-expanded={showDetails} onClick={() => setShowDetails(value => !value)}>{showDetails ? 'Hide detailed overview' : 'Detailed overview and queue controls'}</Button>}
          {showDetails && (!home || home.home) && <ProjectOverviewBoard projects={filtered} />}
        </>
      )}
      {projects.length > 0 && filtered.length === 0 && query && (!home || home.home) && (
        <p>No projects match “{query}”.</p>
      )}
    </section>
  );
}
