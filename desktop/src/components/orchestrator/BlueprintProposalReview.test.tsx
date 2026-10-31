import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { BlueprintProposalReview } from './BlueprintProposalReview';
import { orchestratorApi } from '../../api/orchestrator';
import { resetCockpit, useCockpit } from '../../store/store';
import { assistedBlueprintFixture } from '../../test/blueprintAssistanceFixtures';

beforeEach(() => { resetCockpit(); useCockpit.setState(state => ({ selection: { ...state.selection, projectId: 'p-1' } })); vi.restoreAllMocks(); });
afterEach(cleanup);
it('checks the original context then transfers an unsaved scoped draft without generic Apply', async () => {
 const f = assistedBlueprintFixture(), get = vi.spyOn(orchestratorApi, 'get').mockResolvedValue(f.view), apply = vi.spyOn(orchestratorApi, 'apply');
 const user = userEvent.setup(); render(<BlueprintProposalReview turn={f.turn} scope="scope/p-1" canEdit blocked={false}/>);
 expect(get).not.toHaveBeenCalled(); await user.click(screen.getByRole('button', { name: 'Review in Deploy' }));
 await waitFor(() => expect(useCockpit.getState().projectBlueprintDraft?.turnId).toBe(f.turn.id));
 expect(useCockpit.getState().lens).toBe('deploy'); expect(useCockpit.getState().projectBlueprintDraft?.scope).toBe('scope/p-1'); expect(apply).not.toHaveBeenCalled();
});
it('refuses a source-changed proposal and leaves navigation and existing drafts untouched', async () => {
 const f = assistedBlueprintFixture(); vi.spyOn(orchestratorApi, 'get').mockResolvedValue({ ...f.view, context: { ...f.context, deploymentBlueprint: { ...f.context.deploymentBlueprint!, sourceReviewDigest: 'd'.repeat(64) } } });
 const user = userEvent.setup(); render(<BlueprintProposalReview turn={f.turn} scope="scope/p-1" canEdit blocked={false}/>);
 await user.click(screen.getByRole('button', { name: 'Review in Deploy' })); await screen.findByRole('alert');
 expect(useCockpit.getState().projectBlueprintDraft).toBeNull(); expect(useCockpit.getState().lens).toBe('develop');
});
