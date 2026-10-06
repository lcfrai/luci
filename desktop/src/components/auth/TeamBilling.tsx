import { useEffect, useRef, useState, type MouseEvent } from 'react';
import { errorText } from '../../api/product';
import { teamBillingApi, type TeamBillingState } from '../../api/teamBilling';

export function TeamBilling({ teamId }: { teamId: string }) {
  const [state, setState] = useState<TeamBillingState>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [ack, setAck] = useState(false);
  const [receipt, setReceipt] = useState('');
  const [destination, setDestination] = useState<{ url: string; label: string }>();
  const generation = useRef(0), pending = useRef(false);
  useEffect(() => {
    const current = ++generation.current; setState(undefined); setReceipt(''); pending.current = false; setBusy(false); setAck(false); setError(''); setDestination(undefined);
    void teamBillingApi.read(teamId).then(value => { if (generation.current === current) setState(value); }).catch(e => { if (generation.current === current) setError(errorText(e)); });
    return () => { generation.current++; };
  }, [teamId]);
  const run = async (operation: () => Promise<void>) => {
    if (pending.current) return;
    const current = generation.current; pending.current = true; setBusy(true); setError(''); setDestination(undefined);
    try { await operation(); } catch (e) { if (generation.current === current) setError(errorText(e)); }
    finally { if (generation.current === current) { pending.current = false; setBusy(false); } }
  };
  const replace = async (operation: () => Promise<TeamBillingState>) => {
    const current = generation.current; const next = await operation();
    if (generation.current === current) { setState(next); setAck(false); }
  };
  const external = async (kind: 'checkout' | 'portal') => {
    const current = generation.current;
    const result = kind === 'portal' ? await teamBillingApi.portal(teamId) : await teamBillingApi.checkout(teamId, state!.quote!.id, state!.revision!);
    if (generation.current !== current) return;
    if (result.url) {
      const url = new URL(result.url);
      const host = kind === 'portal' ? 'billing.stripe.com' : 'checkout.stripe.com';
      if (url.protocol !== 'https:' || url.host !== host || url.username || url.password) throw new Error('The billing service returned an invalid Stripe destination.');
      setDestination({ url: result.url, label: kind === 'portal' ? 'Open Stripe subscription management' : 'Continue to Stripe checkout' });
    }
    await replace(() => teamBillingApi.read(teamId));
  };
  const openNativeLink = (event: MouseEvent<HTMLAnchorElement>, url: string) => {
    if (!("__TAURI_INTERNALS__" in window)) return;
    event.preventDefault(); const current = generation.current;
    void import('@tauri-apps/plugin-opener').then(opener => opener.openUrl(url)).catch(e => { if (generation.current === current) setError(errorText(e)); });
  };
  const pendingCheckout = state?.checkoutState === 'reserved' || state?.checkoutState === 'open';
  const canSubscribe = state?.status === 'unsubscribed' || state?.status === 'canceled';
  return <section aria-label="Team subscription">
    <h3>Team subscription</h3>
    {!state && !error && <p role="status">Loading subscription status…</p>}
    {error && <p className="journey-error" role="alert">{error}</p>}
    {state?.enabled && <>
      <p role="status">Subscription: {state.status?.replace(/_/g, ' ')}. {state.executionAllowed ? 'Team execution is available.' : 'New team execution is unavailable. Context and permitted cleanup remain available.'}</p>
      <p>{state.separateCosts}</p>
      {state.periodEnd ? <p>Last verified billing period ends {new Date(state.periodEnd * 1000).toLocaleString()}.{state.cancelAtPeriodEnd ? ' Cancellation is scheduled for that date.' : ''}</p> : null}
      <p>Returning from Stripe does not activate access. Luci verifies the subscription with Stripe. Canceling retains your team context and data.</p>
      {state.owner ? <>
        {canSubscribe && !pendingCheckout && <button className="luci-button" disabled={busy} onClick={() => void run(() => replace(() => teamBillingApi.quote(teamId)))}>Review current subscription price</button>}
        {pendingCheckout && <p>A checkout reservation already exists. Resume it using the same reviewed price; do not start another checkout. {state.checkoutRecoverable === false ? 'Its safe retry window has ended. Ask the operator to reconcile the Stripe outcome.' : ''}</p>}
        {state.quote && canSubscribe && (!pendingCheckout || state.checkoutRecoverable) && <div>
          <p><strong>{new Intl.NumberFormat(undefined, { style: 'currency', currency: state.quote.price.currency }).format(state.quote.price.unitAmount / 100)} per team per {state.quote.price.interval}</strong>. Tax: {state.quote.price.taxBehavior === 'inclusive' ? 'included' : state.quote.price.taxBehavior === 'exclusive' ? 'exclusive; review the final total in Stripe' : 'determined at Stripe checkout'}.</p>
          <p><a href={state.quote.termsUrl} onClick={e => openNativeLink(e, state.quote!.termsUrl)} target="_blank" rel="noopener noreferrer">Terms of service</a> · <a href={state.quote.privacyUrl} onClick={e => openNativeLink(e, state.quote!.privacyUrl)} target="_blank" rel="noopener noreferrer">Privacy notice</a></p>
          <label><input type="checkbox" checked={ack} disabled={busy} onChange={e => setAck(e.target.checked)} />I accept the terms and this recurring team price, with model usage and infrastructure billed separately.</label>
          <button className="luci-button" disabled={busy || !ack} onClick={() => void run(() => external('checkout'))}>{pendingCheckout ? 'Resume reserved checkout' : 'Prepare Stripe checkout'}</button>
        </div>}
        {pendingCheckout && <details><summary>Recover a checkout receipt</summary>
          <p>If the checkout response was lost, ask the service operator for the Stripe Checkout session ID matching this team reservation. Luci verifies the receipt before changing subscription access.</p>
          <label>Stripe Checkout session ID<input value={receipt} disabled={busy} onChange={e => setReceipt(e.target.value)} placeholder="cs_…" /></label>
          <button className="luci-button" disabled={busy || !receipt.startsWith('cs_')} onClick={() => void run(() => replace(() => teamBillingApi.reconcile(teamId, receipt.trim(), state.revision!)))}>Verify checkout receipt</button>
        </details>}
        {state.canManageSubscription && <button className="luci-button" disabled={busy} onClick={() => void run(() => external('portal'))}>Manage or cancel subscription</button>}
        <button className="luci-button" disabled={busy} onClick={() => void run(() => replace(() => teamBillingApi.refresh(teamId)))}>Verify subscription with Stripe</button>
      </> : <p>A team owner can review pricing, subscribe or cancel.</p>}
    </>}
    {destination && <p><a className="luci-button" href={destination.url} onClick={e => openNativeLink(e, destination.url)} rel="noreferrer">{destination.label}</a></p>}
    <button className="luci-button" disabled={busy} onClick={() => void run(() => replace(() => teamBillingApi.read(teamId)))}>Reload subscription status</button>
  </section>;
}
