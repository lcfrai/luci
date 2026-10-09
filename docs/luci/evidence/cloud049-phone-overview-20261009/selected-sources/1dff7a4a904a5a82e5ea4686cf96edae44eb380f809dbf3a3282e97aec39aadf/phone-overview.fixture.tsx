// Actual styled AppShell with controlled read-only context; no backend or workers.
import '../../../desktop/src/styles/app.css';
import React from 'react';
import { createRoot } from 'react-dom/client';
import { AppShell } from '../../../desktop/src/components/AppShell';
import { resetCockpit, useCockpit } from '../../../desktop/src/store/store';
import { aProject, aMachine } from '../../../desktop/src/test/fixtures';
import { context, view } from '../../../desktop/src/components/orchestrator/fixtures';
import { projectOverviewApi } from '../../../desktop/src/api/projectOverview';
import { swarmSchedulerApi } from '../../../desktop/src/api/swarmScheduler';
import { definitionApi } from '../../../desktop/src/api/definition';
import { orchestratorApi } from '../../../desktop/src/api/orchestrator';
import { productApi } from '../../../desktop/src/api/product';
import { designPlanApi } from '../../../desktop/src/api/designPlan';

const ids = ['alpha', 'beta', 'gamma', 'delta'];
const hold = () => new Promise<never>(() => {});
projectOverviewApi.get = hold;
swarmSchedulerApi.get = hold;
swarmSchedulerApi.configure = async () => { throw Error('Scheduling mutations are forbidden in this fixture.'); };
// The Develop route remains mounted while its actual context reader is held.
// Navigation is entirely the real AppShell/Rail/TopBar, not a replacement view.
productApi.brief = hold; productApi.design = hold; productApi.git = hold;
designPlanApi.get = hold;
definitionApi.get = async id => ({ ...context(id).specification, purpose: `Unique saved purpose for ${id}.` });
orchestratorApi.get = async id => view({ projectId: id, context: { ...context(id), specification: { ...context(id).specification, purpose: `Unique saved purpose for ${id}.` } } });
productApi.agentConnections = async () => ({ revision: 1, activeConnectionId: '', connections: [] });
resetCockpit();
useCockpit.setState({ projects: ids.map(id => aProject({ id, name: id, path: '/fixture/' + id })), projectsLoaded: true, machines: [aMachine()] });
useCockpit.getState().selectProject('alpha', 'develop');
Object.assign(window, { __phoneOverviewFixture: {
  selected: () => useCockpit.getState().selection.projectId,
  lens: () => useCockpit.getState().lens,
} });
document.documentElement.dataset.theme = 'light';
createRoot(document.getElementById('root')!).render(<AppShell />);
