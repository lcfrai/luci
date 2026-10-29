import { expect, it } from 'vitest';
import { validPortfolio, type Portfolio } from './portfolio';

export const portfolioFixture = (): Portfolio => ({
  version: 1, observedAt: '2026-10-29T00:00:00Z', status: 'ready', totalProjects: 1, omittedProjects: 0, omittedInterventions: 0,
  projects: [{ id: 'p-one', name: 'Notes', status: 'ready', coordinator: { status: 'none', unresolved: 0 } }],
  interventions: [{ id: 'a'.repeat(64), projectId: 'p-one', kind: 'question', title: 'Answer a project question', reason: 'A decision is unresolved.', blocks: 'The affected decision remains open.',
    nextAction: { id: 'answer-question', label: 'Review questions', lens: 'discover' }, evidence: [{ source: 'questions', id: 'p-one' }] }],
});

it('validates partial evidence without inventing a clear state', () => {
  expect(validPortfolio(portfolioFixture())).toBe(true);
  const partial = portfolioFixture(); partial.status = 'partial'; partial.projects[0].status = 'partial';
  partial.projects[0].coordinator = { status: 'unavailable', unresolved: 0 };
  expect(validPortfolio(partial)).toBe(true);
  partial.status = 'ready'; expect(validPortfolio(partial)).toBe(false);
});

it('refuses orphan or duplicate project and intervention identities', () => {
  const orphan = portfolioFixture(); orphan.interventions[0].projectId = 'p-hidden';
  expect(validPortfolio(orphan)).toBe(false);
  const duplicate = portfolioFixture(); duplicate.interventions.push(duplicate.interventions[0]);
  expect(validPortfolio(duplicate)).toBe(false);
  const duplicateProject = portfolioFixture(); duplicateProject.projects.push(duplicateProject.projects[0]); duplicateProject.totalProjects++;
  expect(validPortfolio(duplicateProject)).toBe(false);
});

it('refuses missing evidence, false complete totals and unsupported destinations', () => {
  const missing = portfolioFixture(); missing.interventions[0].evidence = [];
  expect(validPortfolio(missing)).toBe(false);
  const falseComplete = portfolioFixture(); falseComplete.omittedProjects = 1; falseComplete.totalProjects = 2;
  expect(validPortfolio(falseComplete)).toBe(false);
  const destination = portfolioFixture(); destination.interventions[0].nextAction.lens = 'admin' as 'discover';
  expect(validPortfolio(destination)).toBe(false);
});
