import { productApi } from './product';
export interface BillingQuote {
  id: string; expiresAt: number; termsUrl: string; privacyUrl: string;
  price: { priceId: string; unitAmount: number; currency: string; interval: 'month' | 'year'; intervalCount: number; taxBehavior: string };
}
export interface TeamBillingState {
  enabled: boolean; owner: boolean; executionAllowed: boolean; separateCosts: string;
  revision?: number; status?: string; periodEnd?: number; cancelAtPeriodEnd?: boolean;
  hasCustomer?: boolean; canManageSubscription?: boolean; quote?: BillingQuote; checkoutState?: string; checkoutRecoverable?: boolean;
}
const path = (teamId: string) => `/v1/teams/${encodeURIComponent(teamId)}/billing`;
export const teamBillingApi = {
  read: (teamId: string) => productApi.request<TeamBillingState>(path(teamId)),
  quote: (teamId: string) => productApi.request<TeamBillingState>(`${path(teamId)}/quote`, {}),
  refresh: (teamId: string) => productApi.request<TeamBillingState>(`${path(teamId)}/refresh`, {}),
  checkout: (teamId: string, quoteId: string, expectedRevision: number) => productApi.request<{ url?: string }>(`${path(teamId)}/checkout`, { quoteId, expectedRevision, acknowledgePrice: true, acknowledgeTerms: true }),
  reconcile: (teamId: string, sessionId: string, expectedRevision: number) => productApi.request<TeamBillingState>(`${path(teamId)}/reconcile`, { sessionId, expectedRevision }),
  portal: (teamId: string) => productApi.request<{ url: string }>(`${path(teamId)}/portal`, {}),
};
