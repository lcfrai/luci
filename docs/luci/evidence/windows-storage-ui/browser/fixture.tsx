import '../../../desktop/src/styles/app.css';
import '../../../desktop/src/components/setup/setup.css';
import React from 'react';
import { createRoot } from 'react-dom/client';
import type { ExecutionStorage, ExecutionStorageClient } from '../../../desktop/src/api/executionStorage';
import { DockerStorageConfirmation } from '../../../desktop/src/components/setup/DockerStorageConfirmation';
declare global { interface Window { __storageFixture: { calls: { method: string; expectedRevision?: number; vhdPath?: string; confirmActiveAndPendingLocal?: boolean }[] } } }

// Explicit controlled observations. These are not native Windows measurements.
let state: ExecutionStorage = { platform: 'windows', scope: 'local-docker-desktop', required: true, state: 'invalidated', revision: 3, reasonCode: 'desktop_changed', message: 'Docker Desktop settings changed. Confirm the current active and pending local storage locations.', basis: 'operator-confirmed-local-layout; minimum-free-space-across-all-mounted-writable-local-volumes', desktopVersion: '4.44.0', hostAvailableBytes: 12 * 1024 ** 3, guestAvailableBytes: 220 * 1024 ** 3, mountedVolumeCount: 2, limitingVolume: 'D:\\', observedAt: new Date().toISOString() };
window.__storageFixture = { calls: [] };
const client: ExecutionStorageClient = {
  async get() { window.__storageFixture.calls.push({ method: 'GET' }); if (state.state === 'pending-observation') state = { ...state, state: 'confirmed', message: 'Current local layout confirmed and physical space measured.', hostAvailableBytes: 40 * 1024 ** 3 }; return { ...state }; },
  async confirm(snapshot, vhdPath) { window.__storageFixture.calls.push({ method: 'POST', expectedRevision: snapshot.revision, vhdPath, confirmActiveAndPendingLocal: true }); await new Promise(done => setTimeout(done, 200)); state = { ...state, revision: state.revision + 1, state: 'pending-observation', vhdPath, message: 'Waiting for a fresh background measurement.', confirmedAt: new Date().toISOString() }; return { ...state }; },
};
createRoot(document.getElementById('root')!).render(<main style={{ maxWidth: 850, margin: '0 auto', height: '100dvh', overflowY: 'auto', padding: 24 }}><h1>Workspace settings</h1><p>Controlled UI fixture · no Windows probe, Docker or agent execution.</p><DockerStorageConfirmation client={client} enabled /></main>);
