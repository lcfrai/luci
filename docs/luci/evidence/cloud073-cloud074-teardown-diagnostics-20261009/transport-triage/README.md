# Inert transport-disposal characterization

Eight controlled cases passed using the unchanged exported `installTransportDiagnostics`, `installTerminalMonitor`, and lifecycle `boundedAttempt` from the accepted CLOUD-072 cold-attempt2 sources. The test imports those files directly. It starts no browser, Docker operation, product service, compiler, network request, or private-log read.

The ordinary never-settling detach promise reaches the inner 1,500 ms timeout before the outer 2,000 ms timeout. The outer timer is registered before the monitor operation begins; `client.detach()` is invoked before the inner timer is registered. Synthetic 700 ms clock advancement during page-listener removal or eager detach invocation makes the outer deadline precede the inner deadline. This is a controlled ordering example, not a measured delay or explanation of the actual attempt.

Successful detach, asynchronous rejection, synchronous invocation failure, full monitor listener removal, both deadlines, and late acknowledgement/rejection are covered. Timeout races do not cancel the underlying detach; late settlement never clears the already-observed failure. All timers are manual, and no budgets were increased.

The actual retained receipt records `transport-observer-dispose-timeout` but no disposal-entry, detach-invocation, inner-timer-registration, or detach-settlement timestamps and no post-disposal protocol snapshot. Its separate first `workflow-closing` callback does not retain the original request method. These results support neither an actual-cause attribution nor a GET claim.

`characterize.mjs` is the executed source; `report.json` contains the complete controlled traces, and `run.stdout` / `run.stderr` preserve its process output. `source-bindings.json` records before/after source hashes without rewriting. `execution-receipt.json` additionally checks those identities against the accepted run receipt and binds the output. `analysis.json` states the observability gap and limits. `manifest.json` and `SHA256SUMS` bind this directory's artifacts.
