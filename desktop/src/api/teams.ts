import { productApi } from './product';
import type { ControlUser } from '../components/auth/AuthProvider';
export interface TeamGrants { execute: boolean; secrets: boolean; deploy: boolean; admin: boolean }
export interface TeamPermissions extends TeamGrants { read: boolean; edit: boolean }
export interface Team { id: string; name: string; revision: number; createdAt: string; updatedAt: string }
export interface TeamMember { userId: string; role: 'owner' | 'editor' | 'viewer'; grants: TeamGrants; revision: number }
export interface TeamSummary { team: Team; membership: TeamMember; permissions: TeamPermissions }
export interface TeamDetail extends TeamSummary { members: (TeamMember & { user: ControlUser })[]; executionBoundary: string }
export interface ProjectGrant { userId: string; role: 'editor' | 'viewer'; grants: TeamGrants; revision: number }
export interface ProjectAccess { team: Team; projectId: string; permissions: TeamPermissions; grants: ProjectGrant[] }
export const emptyGrants: TeamGrants = { execute: false, secrets: false, deploy: false, admin: false };
export const noPermissions: TeamPermissions = { read: false, edit: false, ...emptyGrants };
export const personalPermissions: TeamPermissions = { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true };
const teamPath = (id: string) => `/v1/teams/${encodeURIComponent(id)}`;
export const teamsApi = {
  list: () => productApi.request<{teams: TeamSummary[]; managedTeamId?: string}>('/v1/teams'),
  create: (name: string) => productApi.request<TeamDetail>('/v1/teams', { name }),
  detail: (id: string) => productApi.request<TeamDetail>(teamPath(id)),
  rename: (id: string, expectedRevision: number, name: string) => productApi.request<TeamDetail>(teamPath(id), {expectedRevision, name}, 'PATCH'),
  addMember: (id: string, expectedRevision: number, username: string, role: TeamMember['role'], grants: TeamGrants) => productApi.request<TeamDetail>(`${teamPath(id)}/members`, {expectedRevision, username, role, grants}),
  saveMember: (id: string, expectedRevision: number, member: Pick<TeamMember,'userId'|'role'|'grants'>) => productApi.request<TeamDetail>(`${teamPath(id)}/members/${encodeURIComponent(member.userId)}`, {expectedRevision, role: member.role, grants: member.grants}, 'PUT'),
  removeMember: (id: string, expectedRevision: number, userId: string) => productApi.request(`${teamPath(id)}/members/${encodeURIComponent(userId)}`, {expectedRevision}, 'DELETE'),
  projectAccess: (id: string, projectId: string, signal?: AbortSignal) => productApi.request<ProjectAccess>(`${teamPath(id)}/projects/${encodeURIComponent(projectId)}/members`, undefined, "GET", 120_000, signal),
  saveProjectGrant: (id: string, projectId: string, expectedRevision: number, grant: Pick<ProjectGrant,'userId'|'role'|'grants'>) => productApi.request<ProjectAccess>(`${teamPath(id)}/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(grant.userId)}`, {expectedRevision,role:grant.role,grants:grant.grants}, 'PUT'),
  removeProjectGrant: (id: string, projectId: string, expectedRevision: number, userId: string) => productApi.request(`${teamPath(id)}/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}`, {expectedRevision}, 'DELETE'),
};
