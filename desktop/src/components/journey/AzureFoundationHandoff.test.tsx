import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AzureFoundationHandoff } from './AzureFoundationHandoff';
import { productApi, type CloudPlan } from '../../api/product';
import type { DeploymentBlueprintPlan } from '../../api/deploymentBlueprint';

const environment = vi.hoisted(() => ({ canEdit: true }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({ canRead: true, canEdit: environment.canEdit, loading: false }) }));
vi.mock('../../hooks/useUnsavedProjectDraft', () => ({ useUnsavedProjectDraft: () => undefined }));
vi.mock('../setup/CloudManager', () => ({ CloudManager: ({ plan, foundationOnly, onlyPlanId }: { plan: CloudPlan; foundationOnly: boolean; onlyPlanId: string }) => <div>Existing manager {plan.id} · {String(foundationOnly)} · {onlyPlanId}</div> }));
const plan = { digest: 'a'.repeat(64), blueprint: { projectId: 'p-one' } } as DeploymentBlueprintPlan;
const foundation = (): CloudPlan => ({ id: 'cp-one', provider: 'azure', resources: ['Existing Azure VM'], foundation: { version: 1, request: { blueprint: plan.blueprint, expectedBlueprintDigest: plan.digest }, actor: { userId: 'alice', projectId: 'p-one' }, contextDigest: 'b'.repeat(64), planDigest: 'c'.repeat(64), fit: { blueprintDigest: plan.digest, tier: 'small', fits: true, capacityVerified: false, milliCpu: 2000, memoryMiB: 4096, systemReserveMiB: 1024, decisions: [] } } } as unknown as CloudPlan);
beforeEach(() => { vi.restoreAllMocks(); environment.canEdit = true; });
afterEach(cleanup);
async function fill() {
 const user = userEvent.setup(); await user.click(screen.getByText('Use this blueprint to size an Azure VM foundation'));
 for (const [label, value] of [['Azure resource-group name', 'my-foundation'], ['Azure region', 'australiaeast'], ['Domain you control', 'example.com'], ['Monthly planning budget (USD)', '50'], ['Your SSH public key', 'ssh-ed25519 public-key-for-test']]) await user.type(screen.getByLabelText(label), value);
 return user;
}
it('reviews the exact project blueprint before showing the existing explicit-cost manager', async () => {
 const prepare = vi.spyOn(productApi, 'cloudPlan').mockResolvedValue(foundation()); const create = vi.spyOn(productApi, 'createCloud');
 render(<AzureFoundationHandoff plan={plan} active/>); const user = await fill();
 expect(prepare).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button', { name: 'Review Azure foundation' }));
 await screen.findByText('Existing manager cp-one · true · cp-one');
 expect(prepare).toHaveBeenCalledWith(expect.objectContaining({ provider: 'azure', size: 'small', foundation: { blueprint: plan.blueprint, expectedBlueprintDigest: plan.digest } }), expect.any(AbortSignal));
 expect(create).not.toHaveBeenCalled();
 await user.selectOptions(screen.getByLabelText('Foundation tier'), 'standard');
 expect(screen.queryByText(/Existing manager/)).toBeNull();
});
it('refuses a returned foundation for another project before offering any creation control', async () => {
 const response = foundation(); response.foundation!.actor.projectId = 'foreign';
 vi.spyOn(productApi, 'cloudPlan').mockResolvedValue(response);
 render(<AzureFoundationHandoff plan={plan} active/>); const user = await fill(); await user.click(screen.getByRole('button', { name: 'Review Azure foundation' }));
 expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'The returned foundation does not match this project and blueprint. Review it again.');
 expect(screen.queryByText(/Existing manager/)).toBeNull();
});
it('withdraws an in-flight foundation response when this project panel becomes inactive', async () => {
 let finish!: (value: CloudPlan) => void;
 vi.spyOn(productApi, 'cloudPlan').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
 const rendered = render(<AzureFoundationHandoff plan={plan} active/>); const user = await fill(); await user.click(screen.getByRole('button', { name: 'Review Azure foundation' }));
 rendered.rerender(<AzureFoundationHandoff plan={plan} active={false}/>); await act(async () => { finish(foundation()); });
 await waitFor(() => expect(screen.queryByText(/Existing manager/)).toBeNull());
});
