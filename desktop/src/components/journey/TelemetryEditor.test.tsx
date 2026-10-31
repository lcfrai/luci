import { useState } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { TelemetryCollectorEditor, TelemetryClientEditor, newCollectorService, telemetryIntegrationDiscussion } from './TelemetryEditor';
import type { BlueprintTelemetry, CollectorConfiguration } from '../../api/deploymentBlueprint';

vi.mock('./KubernetesReferencePicker',()=>({KubernetesReferencePicker:()=>null}));
afterEach(cleanup);
it('edits an existing destination and TLS references without fabricating values or acquiring anything',async()=>{
  let current:CollectorConfiguration;const initial=newCollectorService('collector').collector!;
  function Harness(){const [value,setValue]=useState(initial);current=value;return <TelemetryCollectorEditor projectId="p-one" targetId="target-one" namespace="apps" active value={value} onChange={setValue}/>;}
  const user=userEvent.setup();render(<Harness/>);
  await user.type(screen.getByLabelText('Existing OTLP backend HTTPS endpoint'),'https://owned.example/otlp');
  await user.type(screen.getByLabelText('Collector receiver Secret name'),'receiver-tls');
  await user.type(screen.getByLabelText('Collector receiver CA key'),'client-ca');
  await user.type(screen.getByLabelText('Collector receiver certificate key'),'server-cert');
  await user.type(screen.getByLabelText('Collector receiver private-key key'),'server-key');
  await user.click(screen.getByRole('button',{name:'Add business outcome'}));
  await user.type(screen.getByLabelText('Event 2 ID'),'purchase');await user.type(screen.getByLabelText('Event 2 meaning'),'A completed purchase occurrence');
  expect(current!.receiverTLS).toEqual({resourceName:'receiver-tls',caKey:'client-ca',certKey:'server-cert',keyKey:'server-key'});
  expect(current!.destination.endpoint).toBe('https://owned.example/otlp');expect(current!.events[1].id).toBe('purchase');
  expect(screen.queryByRole('button',{name:/Apply|Install|Start/})).toBeNull();expect(JSON.stringify(current!)).not.toContain('BEGIN PRIVATE KEY');
});
it('keeps instrumentation explicit and preserves exact references until the person removes the binding',async()=>{
  const changed=vi.fn();let current:BlueprintTelemetry|undefined;
  function Harness(){const [value,setValue]=useState<BlueprintTelemetry>();current=value;return <TelemetryClientEditor serviceId="web" collectorId="collector" value={value} onChange={next=>{changed(next);setValue(next);}}/>;}
  const user=userEvent.setup();render(<Harness/>);expect(changed).not.toHaveBeenCalled();
  await user.click(screen.getByRole('checkbox',{name:'Instrument web with the planned Collector'}));
  expect(current!.resourceId).toBe('collector');expect(current!.endpoint).toBe('');expect(current!.clientTLS!.resourceName).toBe('');
  await user.type(screen.getByLabelText('web client Secret name'),'existing-client-tls');
  await user.click(screen.getByText('Exact SDK dependency versions from this app'));
  await user.type(screen.getByLabelText('@opentelemetry/api'),'1.9.0');
  expect(current!.clientTLS!.resourceName).toBe('existing-client-tls');expect(current!.instrumentation!.dependencies['@opentelemetry/api']).toBe('1.9.0');
  await user.click(screen.getByRole('checkbox',{name:'Instrument web with the planned Collector'}));expect(current).toBeUndefined();
});
it('drafts exact source-bound integration files without sending or silently truncating code',()=>{
  const app={...newCollectorService('web'),kind:'app' as const,collector:undefined,telemetry:{resourceId:'collector',endpoint:'',instrumentation:{runtime:'nodejs' as const,dependencies:{}}}};
  const value:Parameters<typeof telemetryIntegrationDiscussion>[0]={digest:'a'.repeat(64),blueprint:{version:1,projectId:'p-one',source:{commit:'b'.repeat(40),configurationRevision:1},infrastructure:{id:'cluster',kind:'kubernetes',reference:'target-one',namespace:'apps'},services:[app]},supportingArtifacts:['bootstrap.mjs','events.mjs','dependencies.json','instrumentation.md'].map(suffix=>({name:`web.${suffix}`,mediaType:'text/javascript' as const,sha256:'c'.repeat(64),content:`Reviewed source for ${suffix}`}))};
  const draft=telemetryIntegrationDiscussion(value,'web');expect(draft).toContain('Reviewed source for bootstrap.mjs');expect(draft).toContain('"sha256"');expect(draft).toContain('Do not install packages');expect(draft).toContain(value.blueprint.source.commit);
  value.supportingArtifacts![0].content='x'.repeat(20000);const bounded=telemetryIntegrationDiscussion(value,'web');expect(bounded).toContain('Ask me to attach the four downloaded files');expect(bounded).not.toContain('x'.repeat(100));expect(new TextEncoder().encode(bounded).length).toBeLessThan(16000);
});
