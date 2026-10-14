# Existing local Ollama model with a bounded analysis context

Luci's OpenCode analysis uses the saved Ollama model through its `/v1` endpoint. Ollama's default serving context can be smaller than the model's trained context. For an existing `qwen2.5-coder:7b` installation, make a local alias with an 8,192-token serving context; this does not download or overwrite the original model.

In PowerShell:

```powershell
$luciModelFile = Join-Path $env:TEMP 'luci-qwen8k.Modelfile'
@('FROM qwen2.5-coder:7b', 'PARAMETER num_ctx 8192') | Set-Content -Path $luciModelFile -Encoding ascii
ollama create luci-qwen2.5-coder-8k -f $luciModelFile
```

Set the existing OpenCode connection's model to `luci-qwen2.5-coder-8k`; keep its Ollama `/v1` endpoint and existing private connection settings. This alias is only a saved model choice. Luci still checks the loaded model's actual context and token count for each analysis turn. The 8k setting can increase host memory use; Luci's Docker admission and whole-host reserve do not change. A different supported model may need a larger bounded context for a larger project, and Luci will retain a specific failure if the actual window is insufficient or cannot be verified. No response or proposal is accepted from a partial or unverifiable turn.
