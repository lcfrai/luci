import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { CoordinatorPolicy } from './CoordinatorPolicy';
import { coordinatorPolicyApi } from '../../api/coordinatorPolicy';
import { orchestratorApi } from '../../api/orchestrator';
import { assistedBlueprintFixture } from '../../test/blueprintAssistanceFixtures';
afterEach(() => { cleanup(); vi.restoreAllMocks(); localStorage.clear(); });
it('keeps topology proposals outside automatic document saving and its target controls', async () => {
 const f = assistedBlueprintFixture(), advance = vi.fn();
 vi.spyOn(coordinatorPolicyApi, 'get').mockResolvedValue({ projectId: 'p-1', revision: 1, mode: 'auto-drafts', reviewMode: 'approved-amendments', allowedTargets: ['specification', 'design-plan', 'plan'], executionMode: 'manual', deploymentMode: 'manual', actor: { id: 'alice', displayName: 'Alice' }, updatedAt: '2026-10-31T00:00:00Z' });
 vi.spyOn(orchestratorApi, 'get').mockResolvedValue(f.view);
 render(<CoordinatorPolicy projectId="p-1" scope="scope/p-1" actorId="alice" view={f.view} canRead canEdit blocked="" onAdvance={advance} onObserved={vi.fn()} beginObservation={() => vi.fn()}/>);
 await waitFor(() => expect(coordinatorPolicyApi.get).toHaveBeenCalledTimes(1));
 expect(advance).not.toHaveBeenCalled(); expect(screen.queryByRole('checkbox', { name: /Service blueprint|Topology/ })).toBeNull();
});
