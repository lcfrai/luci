import { expect, it } from 'vitest';
import { designWorkScope, observedDesignWorkPhase, ownDesignWorkStatus, readDesignWorkStatus } from './designWorkStatus';
it('keys actual observations by actor/workspace/team/project and exact job; cleanup cannot erase a newer owner', () => {
  const key = designWorkScope('workspace', 'alice', 'team-a', 'project')!, a = ownDesignWorkStatus(key);
  a.publish({jobId: 'job-a', phase: 'working', updatedAt: 'now', observedAt: Date.now()}); expect(readDesignWorkStatus(key)?.jobId).toBe('job-a');
  expect(readDesignWorkStatus(designWorkScope('workspace', 'bob', 'team-a', 'project'))).toBeNull();
  expect(readDesignWorkStatus(designWorkScope('workspace', 'alice', 'team-b', 'project'))).toBeNull();
  const b = ownDesignWorkStatus(key); a.close(); b.publish({jobId: 'job-b', phase: 'waiting', updatedAt: 'later', observedAt: Date.now()}); expect(readDesignWorkStatus(key)?.jobId).toBe('job-b'); b.close(); expect(readDesignWorkStatus(key)).toBeNull();
});
it('keeps waiting/unknown/response-ready separate from actual working', () => {
  expect(observedDesignWorkPhase('reserved')).toBe('waiting'); expect(observedDesignWorkPhase('queued')).toBe('waiting'); expect(observedDesignWorkPhase('unknown')).toBe('unknown');
  expect(observedDesignWorkPhase('future')).toBe('unknown'); expect(observedDesignWorkPhase('running')).toBe('working'); expect(observedDesignWorkPhase('succeeded')).toBe('response-ready');
  expect(designWorkScope(null, 'alice', '', 'project')).toBeNull();
});
