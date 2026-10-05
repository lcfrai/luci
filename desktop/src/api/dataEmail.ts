import { productApi } from "./product";

export interface DataEmailConfiguration {
  host: string; port: number; username: string; fromAddress: string;
  senderName: string; applicationOrigin: string; hasPassword: boolean;
}
export interface DataEmailReceipt {
  status: string; revision: number; checkedAt?: string; acceptedAt?: string;
}
export interface DataEmailStatus {
  projectId: string; status: string; message: string; revision: number;
  appliedRevision: number; configuration: DataEmailConfiguration;
  applicationOrigin: string; originSource: "public-domain" | "local";
  connection: DataEmailReceipt; test: DataEmailReceipt;
  operation?: { id: string; action: string; status: string; stage: string; message: string; createdAt: string; updatedAt: string };
}
export interface DataEmailRequest {
  action: "save" | "check" | "apply" | "test" | "disable";
  expectedRevision: number;
  configuration?: Omit<DataEmailConfiguration, "hasPassword"> & { password?: string };
  recipient?: string; acknowledgeSend?: boolean;
}
const path = (projectId: string) => `/v1/platform/projects/${encodeURIComponent(projectId)}/data/email`;
export const dataEmailApi = {
  status: (projectId: string) => productApi.request<DataEmailStatus>(path(projectId)),
  action: (projectId: string, input: DataEmailRequest) => productApi.request<DataEmailStatus>(path(projectId), input),
};
