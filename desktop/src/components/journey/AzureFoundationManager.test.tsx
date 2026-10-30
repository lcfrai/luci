import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { CloudManager } from '../setup/CloudManager';
import { createProductClient, type CloudPlan } from '../../api/product';
afterEach(cleanup);

it('shows only the selected foundation job and observes it through the existing retained-job route', async () => {
 const job = { id: 'cj-one', planId: 'cp-one', provider: 'azure', name: 'selected-foundation', region: 'australiaeast', domain: 'example.com', status: 'unknown', message: 'Retained operation needs observation', publicIp: '192.0.2.1' };
 const fetcher = vi.fn<typeof fetch>().mockImplementation(async url => new Response(JSON.stringify(String(url).endsWith('/refresh') ? { ...job, status: 'provisioning', message: 'Owned VM observed' } : { jobs: [job, { ...job, id: 'other', planId: 'cp-other', name: 'unrelated-foundation' }] }), { status: 200 }));
 const plan = { id: 'cp-one', provider: 'azure' } as CloudPlan;
 render(<CloudManager client={createProductClient('http://localhost', fetcher)} plan={plan} mode="managed" foundationOnly onlyPlanId="cp-one"/>);
 await screen.findByText('selected-foundation');
 expect(screen.queryByText('unrelated-foundation')).toBeNull();
 expect(screen.queryByRole('button', { name: 'Connect this server' })).toBeNull();
 expect(screen.queryByText('Private server connection')).toBeNull();
 expect(fetcher.mock.calls.some(row => /targets|tunnels|\/create/.test(String(row[0])))).toBe(false);
 await userEvent.click(screen.getByRole('button', { name: 'Check progress' }));
 await screen.findByText('Owned VM observed');
 expect(fetcher.mock.calls.some(row => String(row[0]).endsWith('/v1/setup/cloud/jobs/cj-one/refresh'))).toBe(true);
 expect(fetcher.mock.calls.some(row => /\/create$/.test(String(row[0])))).toBe(false);
});
