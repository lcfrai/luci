import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { TelemetryReadings, telemetryDiscussion } from './TelemetryReadings';
import type { KubernetesTelemetry } from '../../api/kubernetesDeployment';
const row=():KubernetesTelemetry=>({state:'recorded',collectorId:'collector',serviceUid:'service-original',deploymentUid:'deployment-original',collectorPodUid:'collector-pod',applicationPodUids:['app-pod'],planDigest:'a'.repeat(64),sourceCommit:'b'.repeat(40),observedAt:new Date().toISOString(),continuity:'baseline',samples:[{serviceId:'web',instanceUid:'app-pod',eventId:'activation',outcome:'success',reportedCount:7}],omitted:false,evidenceDigest:'c'.repeat(64)});
afterEach(cleanup);
it('shows attributed reported counts and creates no model request until explicit discussion',async()=>{
  const discuss=vi.fn(),exported=vi.fn();render(<TelemetryReadings row={row()} currentReferences onDiscuss={discuss} onExport={exported} disabled={false}/>);
  expect(screen.getByRole('cell',{name:'7'})).not.toBeNull();expect(screen.getByText(/not people, conversion rates/)).not.toBeNull();expect(discuss).not.toHaveBeenCalled();
  const user=userEvent.setup();await user.click(screen.getByRole('button',{name:'Review these samples with Luci'}));expect(discuss).toHaveBeenCalledTimes(1);expect(exported).not.toHaveBeenCalled();
  const message=telemetryDiscussion('p-one',row());expect(message).toContain('"reportedCount":7');expect(message).toContain('"evidenceDigest"');expect(message).not.toContain('app-pod');expect(message).toContain('Do not claim verified sales');
});
it('keeps no-data, partial reset and stale references distinct from a zero or verified success',()=>{
  const empty={...row(),state:'no-data' as const,samples:[]};const mounted=render(<TelemetryReadings row={empty} currentReferences onDiscuss={()=>{}} onExport={()=>{}} disabled/>);
  expect(screen.getByText(/This is not a zero count/)).not.toBeNull();expect(screen.queryByRole('table')).toBeNull();
  mounted.rerender(<TelemetryReadings row={{...row(),state:'partial',omitted:true,continuity:'reset'}} currentReferences={false} onDiscuss={()=>{}} onExport={()=>{}} disabled/>);
  expect(screen.getByText(/Historical sample/)).not.toBeNull();expect(screen.getByText(/Counter baseline changed/)).not.toBeNull();expect(screen.getByText(/shown counts are partial/)).not.toBeNull();
});
