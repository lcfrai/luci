from pathlib import Path
import importlib.util,sys,shutil
p=Path('/workspace/.luci-tools/check-recovery-installed-preparation/validate-integration.py')
spec=importlib.util.spec_from_file_location('controlled',p);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
out=Path('/workspace/.luci-tools/cloud034-reservation-summary-review/helper');out.mkdir(exist_ok=False)
shutil.copy2(p,out/'runner-source.py')
prefix='scripts/acceptance/check-create-recovery/'
passed=m.run(out,'node',['node','--test','--test-concurrency=1','--test-timeout=30000','--test-reporter=tap',*[prefix+s for s in ['check-guard.test.mjs','provenance-binding.test.mjs','scenario-flow.test.mjs']]],timeout=45)
sys.exit(0 if passed else 1)
