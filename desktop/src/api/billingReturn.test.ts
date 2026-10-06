import { afterEach, expect, it } from 'vitest';
import { consumeBillingReturn, readBillingReturn } from './billingReturn';

const team = 'team-' + 'a'.repeat(22);
afterEach(() => window.history.replaceState(null, '', '/'));
it.each(['return', 'cancel', 'portal'])('retains only the bounded %s navigation hint, never a claimed payment or redirect', kind => {
  expect(readBillingReturn(`https://app.example/?billing=${kind}&billingTeam=${team}&session_id=untrusted&status=active&returnTo=https://other.invalid`)).toEqual({ valid: true, kind, teamId: team });
});
it('keeps a legacy return unscoped until authenticated managed-team identity is available', () => {
  expect(readBillingReturn('https://app.example/?billing=return')).toEqual({ valid: true, kind: 'return', teamId: null });
  expect(readBillingReturn('https://app.example/?unrelated=value')).toBeNull();
});
it.each([
  '?billing=active', '?billing=', '?billing=return&billing=cancel', '?billingTeam=' + team,
  '?billing=return&billingTeam=wrong', '?billing=return&billingTeam=', '?billing=return&billingTeam=' + team + '&billingTeam=' + team,
])('rejects incomplete, forged or ambiguous return fields: %s', query => {
  expect(readBillingReturn('https://app.example/' + query)).toEqual({ valid: false });
});
it('consumes return navigation without changing unrelated location or browser state', () => {
  window.history.replaceState({ previous: 'project' }, '', `/work?billing=return&billingTeam=${team}&view=recent#context`);
  consumeBillingReturn();
  expect(window.location.pathname + window.location.search + window.location.hash).toBe('/work?view=recent#context');
  expect(window.history.state).toEqual({ previous: 'project' });
  expect(readBillingReturn()).toBeNull();
});
