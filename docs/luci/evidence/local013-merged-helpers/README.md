# Merged acceptance helper verification

The actual integrated working tree based on `6002b2ee` passed **95 Node controlled tests**, **28 sustained-wrapper tests**, and **4 dependency-wrapper tests**. All 82 acceptance JavaScript/Python input hashes were unchanged through the run. Commands, timestamps, exact input manifests and exit codes are in [result.json](result.json); the Node output uses its default human-readable reporter despite the historical `.tap` suffix.

These are the affected startup/lifecycle/proxy, stale-review diagnostic and sustained source-binding tests after both patches were applied together. They overlap earlier staged batches and must not be summed with them. Wrappers use mocked child handles. No Docker, browser, Go compiler, installed application or model ran.

A subsequent read-only review found two more DAG cancellation edges: orphan response-waiter rejection after a click failure, and provider dispatch after a pending body read. The later follow-up under [startup evidence](../local013-late-startup/README.md) records those separately; this earlier passing batch does not establish their correction. The installed dependency and 30-minute sustained workflows remain open.
