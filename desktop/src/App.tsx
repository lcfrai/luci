export default function App() {
  return (
    <div style={{ display: "flex", height: "100vh", alignItems: "center", justifyContent: "center" }}>
      {/* Task 24 replaces this component entirely: <AppShell/> mounts here, startPolling()
          runs alongside it, and app.css is imported from main.tsx. Nothing else in this file
          is meant to survive. The Phase 1 ws:// harness was retired in task 19, when
          <Terminal/> stopped taking a URL (contract §6.6). */}
      <p style={{ opacity: 0.6, fontSize: 13 }}>The cockpit mounts here in task 24.</p>
    </div>
  );
}
