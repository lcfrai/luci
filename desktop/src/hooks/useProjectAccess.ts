import { useTeamProjectPermissions } from "../components/teams/TeamWorkspaceProvider";

/** UI guidance mirrors the gateway's independent edit and execution grants. */
export function useProjectAccess(projectId: string) {
  const access = useTeamProjectPermissions(projectId);
  const ready = !access.loading && !access.error;
  const canRead = access.permissions.read;
  const canEdit = ready && canRead && access.permissions.edit;
  const canExecute = canEdit && access.permissions.execute;
  const canAssist = canExecute && access.permissions.secrets;
  const message = access.loading ? "Checking project permissions…" : access.error ? `Project access unavailable: ${access.error}`
    : !canRead ? "This project is not available with your current team permissions."
      : !canEdit ? "Read-only access. Ask a team owner for editing access to change this project's context."
        : !canExecute ? "You can edit shared context. Execution access is required to run agents and checks." : "";
  return { ...access, canRead, canEdit, canExecute, canAssist, message };
}
