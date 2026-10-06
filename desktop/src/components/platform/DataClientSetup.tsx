import { useState } from 'react';
import { prepareBrowserAccess } from '../../api/browserAccess';
import { selectedTeamId } from '../../api/teamScope';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';
import { useProjectAccess } from '../../hooks/useProjectAccess';

export function browserDataClientSource(address: string, publishableKey: string) {
  return `import { createClient } from '@supabase/supabase-js';

// Public client settings only. Database policies still enforce application access.
const url = ${JSON.stringify(address)};
const publishableKey = ${JSON.stringify(publishableKey)};
const dataOrigin = new URL(url).origin;

// Luci team previews use a separate, project-authorized HttpOnly cookie.
// Include it only for the configured data origin, never for unrelated fetches.
const browserFetch: typeof fetch = (input, init) => {
  const address = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
  const target = new URL(address, window.location.href);
  return fetch(input, { ...init,
    ...(target.origin === dataOrigin ? { credentials: 'include' as const } : {}),
  });
};

export const supabase = createClient(url, publishableKey, {
  global: { fetch: browserFetch },
});
`;
}

export function DataClientSetup({ projectId, address, publishableKey, disabled }: { projectId: string; address?: string; publishableKey?: string; disabled: boolean }) {
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const access = useProjectAccess(projectId);
  if (!address || !publishableKey) return null;
  const team = selectedTeamId();
  const source = browserDataClientSource(address, publishableKey);
  async function openDataBrowser() {
    if (disabled || busy || !access.canExecute || !access.permissions.deploy || !access.permissions.secrets) return;
    const popup = team ? window.open('about:blank', '_blank') : null;
    if (popup) popup.opener = null;
    setBusy(true); setError('');
    try {
      const opened = await prepareBrowserAccess(projectId, new URL('/auth/v1/health', address).href);
      if (popup) popup.location.replace(opened);
      else window.open(opened, '_blank', 'noopener,noreferrer');
    } catch (cause) { popup?.close(); setError(errorText(cause)); }
    finally { setBusy(false); }
  }
  return <section aria-label="Browser client setup">
    <h5>Browser client setup</h5>
    <p>Add <code>@supabase/supabase-js</code> to your application and use this client for authentication, tables, storage, functions and live updates.</p>
    {!!team && <p>Open the application from Luci to connect its authorized data browser automatically. Team previews require editing and execution access; data also requires deployment and secret access. Signing out or losing project access closes these connections. Existing clients need the credentialed fetch option below.</p>}
    <label className="data-readonly">Client module<textarea readOnly rows={14} value={source} /></label>
    <Button type="button" variant="secondary" disabled={disabled || busy || !access.canExecute || !access.permissions.deploy || !access.permissions.secrets} onClick={() => void openDataBrowser()}>{busy ? 'Opening data browser…' : 'Open data browser health'}</Button>
    {error && <p className="journey-error" role="alert">{error}</p>}
  </section>;
}
