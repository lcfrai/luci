import { StrictMode } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { productApi } from '../../api/product';
import { swarmQueueFixture, swarmRunFixture } from '../../test/swarmFixtures';
import { DEPENDENCY_PAGE_SIZE, SwarmDependencyNavigator } from './SwarmDependencyNavigator';

it('selects a dependency by persisted ID across a filter, then jumps only on explicit request', () => {
  const run = swarmRunFixture(), open = vi.fn();
  run.swarm!.tasks[0].title = 'Same title'; run.swarm!.tasks[1].title = 'Same title';
  render(<SwarmDependencyNavigator run={run} onOpenTask={open} />);
  fireEvent.change(screen.getByLabelText('Show'), {target: {value: 'waiting'}});
  fireEvent.click(screen.getByRole('button', {name: 'Select task Build c (c)'}));
  fireEvent.click(screen.getByRole('button', {name: 'Same title (b) · not integrated'}));
  expect(screen.getByRole('button', {name: 'Select task Same title (b)'}).getAttribute('aria-pressed')).toBe('true');
  expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Select task Same title (b)'}));
  expect(open).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', {name: 'Go to task controls'})); expect(open).toHaveBeenCalledWith('b');
});
it('has no reads, writes or navigation on StrictMode mount or local selection/filtering', () => {
  const request = vi.spyOn(productApi, 'request'), open = vi.fn();
  render(<StrictMode><SwarmDependencyNavigator run={swarmRunFixture()} onOpenTask={open} /></StrictMode>);
  fireEvent.click(screen.getByRole('button', {name: 'Select task Build c (c)'}));
  fireEvent.change(screen.getByLabelText('Find by title or task ID'), {target: {value: 'b'}});
  fireEvent.change(screen.getByLabelText('Show'), {target: {value: 'attention'}});
  expect(request).not.toHaveBeenCalled(); expect(open).not.toHaveBeenCalled(); request.mockRestore();
});
it('supports native keyboard activation and arrow/Home/End focus without dispatch', async () => {
  const open = vi.fn(); render(<SwarmDependencyNavigator run={swarmRunFixture()} onOpenTask={open} />);
  const a = screen.getByRole('button', {name: 'Select task Build a (a)'}), b = screen.getByRole('button', {name: 'Select task Build b (b)'}), c = screen.getByRole('button', {name: 'Select task Build c (c)'});
  a.focus(); fireEvent.keyDown(a, {key: 'ArrowDown'}); expect(document.activeElement).toBe(b);
  fireEvent.keyDown(b, {key: 'End'}); expect(document.activeElement).toBe(c);
  fireEvent.keyDown(c, {key: 'Home'}); expect(document.activeElement).toBe(a);
  fireEvent.keyDown(a, {key: 'ArrowUp'}); expect(document.activeElement).toBe(a);
  c.focus(); await userEvent.keyboard('{Enter}'); expect(c.getAttribute('aria-pressed')).toBe('true');
  b.focus(); await userEvent.keyboard(' '); expect(b.getAttribute('aria-pressed')).toBe('true');
  expect(open).not.toHaveBeenCalled();
});
it('bounds task and relationship DOM independently and exposes page/filter omissions', () => {
  const run = swarmRunFixture(), template = run.swarm!.tasks[0];
  run.swarm!.tasks = Array.from({length: 201}, (_, index) => ({...template, id: `task-${index}`, title: `Task ${index}`, dependsOn: index === 200 ? Array.from({length: 200}, (_, i) => `task-${i}`) : []}));
  const open = vi.fn(); render(<SwarmDependencyNavigator run={run} onOpenTask={open} />);
  const list = screen.getByRole('list', {name: 'Matching saved tasks'});
  expect(within(list).getAllByRole('button')).toHaveLength(DEPENDENCY_PAGE_SIZE);
  expect(screen.getByText(/Other matching tasks are on additional pages/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', {name: 'Next task page'}));
  expect(within(list).getByRole('button', {name: 'Select task Task 40 (task-40)'})).toBeTruthy();
  expect(within(list).getAllByRole('button')).toHaveLength(DEPENDENCY_PAGE_SIZE);
  fireEvent.change(screen.getByLabelText('Find by title or task ID'), {target: {value: 'task-200'}});
  expect(screen.getByText(/The filter hides other saved tasks/)).toBeTruthy();
  const detail = screen.getByRole('region', {name: 'Selected task dependencies'});
  expect(within(detail).getAllByRole('button', {name: /not integrated/})).toHaveLength(20);
  fireEvent.click(screen.getByRole('button', {name: 'Next depends on page'}));
  expect(screen.getByRole('button', {name: 'Task 20 (task-20) · not integrated'})).toBeTruthy();
  fireEvent.click(screen.getByRole('button', {name: 'Task 39 (task-39) · not integrated'}));
  fireEvent.click(screen.getByRole('button', {name: 'Go to task controls'})); expect(open).toHaveBeenCalledWith('task-39');
});
it('shows unavailable dependencies and queue omissions without inventing a node or verified outcome', () => {
  const run = swarmRunFixture(); run.swarm!.tasks[0].dependsOn = ['missing'];
  render(<SwarmDependencyNavigator run={run} queue={{...swarmQueueFixture([]), hasMore: true}} onOpenTask={vi.fn()} />);
  expect(screen.getByText('Unavailable task (missing) — dependency unresolved')).toBeTruthy();
  expect(screen.getByText(/absence here does not prove a task is unqueued/)).toBeTruthy();
  expect(screen.queryByRole('button', {name: /Select task.*missing/})).toBeNull();
  fireEvent.change(screen.getByLabelText('Show'), {target: {value: 'unknown'}});
  expect(screen.getByRole('button', {name: 'Select task Build a (a)'})).toBeTruthy();
});
it('retains selection across a revision, resets across runs, and keeps detail after list in narrow-screen reading order', () => {
  const run = swarmRunFixture(), open = vi.fn();
  const view = render(<SwarmDependencyNavigator run={run} onOpenTask={open} />);
  fireEvent.click(screen.getByRole('button', {name: 'Select task Build b (b)'}));
  view.rerender(<SwarmDependencyNavigator run={{...run, revision: 2}} onOpenTask={open} />);
  expect(screen.getByRole('button', {name: 'Select task Build b (b)'}).getAttribute('aria-pressed')).toBe('true');
  const list = screen.getByRole('list', {name: 'Matching saved tasks'}), detail = screen.getByRole('region', {name: 'Selected task dependencies'});
  expect(list.compareDocumentPosition(detail) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  view.rerender(<SwarmDependencyNavigator run={{...run, id: 'another-run'}} onOpenTask={open} />);
  expect(screen.getByRole('button', {name: 'Select task Build a (a)'}).getAttribute('aria-pressed')).toBe('true');
});
