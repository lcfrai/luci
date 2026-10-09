import { createContext, useContext } from 'react';

// Only the admitted private workspace supplies this boundary. Account sign-in,
// invitations and workspace recovery outside it remain usable during denial.
export interface WorkspaceOverlayAdmission {
  available: boolean;
  retry?(): Promise<void>;
}
const WorkspaceOverlayContext = createContext<WorkspaceOverlayAdmission>({available: true});
export const WorkspaceOverlayScope = WorkspaceOverlayContext.Provider;
export const useWorkspaceOverlayAdmission = () => useContext(WorkspaceOverlayContext);
