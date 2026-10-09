# Final followup validation — root invocation

After applying `episode-followup/combined-followup-final.patch`, run:

```
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/session-authority-review-edd0/validate-final.py \
  --output /workspace/.luci-tools/session-authority-review-edd0/final-integrated-validation
```

This separate wrapper preserves the initial integrated wrapper/evidence unchanged. It verifies exact final provider/test bytes and unchanged client/team-scope correction, snapshots frontend sources/configuration, runs the final18-case regression plus existing TeamWorkspaceProvider tests in one worker, then TypeScript. No Go rerun is needed for the UI-only episode/fixture typing followup. The initial383/383 selected frontend cases, Go contract pass and TypeScript error are retained separately; final affected results overlap those counts. New TypeScript success, if obtained, applies to the final source captured by this wrapper.
