import ast,subprocess
from pathlib import Path
for name in ['actual/sustained.mjs','actual/header-delivery.mjs','header-delivery.test.mjs']:
 subprocess.run(['node','--check',name],check=True,timeout=10);print('PASS Node syntax '+name)
for name in ['prepare-v2.py','launch.py','validation/verify-preflight.py','validation/record.py']:
 ast.parse(Path(name).read_text(),filename=name);print('PASS Python AST '+name)
