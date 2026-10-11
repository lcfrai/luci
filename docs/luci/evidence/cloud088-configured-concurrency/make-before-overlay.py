"""Write a Go overlay for the retained original concurrency validators."""
import json
from pathlib import Path
import sys

capsule = Path(__file__).resolve().parent
repo = capsule.parents[3]
paths = [
    "control-plane/internal/product/project_swarm.go",
    "control-plane/internal/product/project_swarm_store.go",
    "control-plane/internal/product/project_swarm_scheduler_http.go",
    "control-plane/internal/product/project_swarm_scheduler_store.go",
    "control-plane/internal/api/execution_leases.go",
]
if len(sys.argv) != 2:
    raise SystemExit("Usage: python make-before-overlay.py <output-overlay.json>")
output = Path(sys.argv[1]).resolve()
output.write_text(json.dumps({"Replace": {
    str(repo / path): str(capsule / "before" / (path + ".txt"))
    for path in paths
}}, indent=2) + "\n", encoding="utf-8")
print(output)
