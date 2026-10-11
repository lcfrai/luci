from pathlib import Path
import hashlib,json,difflib,shutil,subprocess
B=Path(__file__).resolve().parent;R=Path('/workspace/luci');S='9536e417069254681133b6eb5afed1e82b0dd56e'
observedHead=subprocess.check_output(['git','-C',str(R),'rev-parse','HEAD'],text=True).strip()
out=B/'overlay';out.mkdir(mode=0o700,exist_ok=False)
paths=['desktop/src/components/rail/Rail.tsx','desktop/src/components/AppShell.tsx','desktop/src/store/slices/uiSlice.ts','desktop/src/preferences/projectNavigation.ts','desktop/src/store/store.ts']
changes=[];patch=[]
def sha(x):return hashlib.sha256(x).hexdigest()
def replace(s,a,b):
 assert s.count(a)==1,(a[:100],s.count(a));return s.replace(a,b)
for name in paths:
 raw=(R/name).read_bytes();assert subprocess.check_output(['git','-C',str(R),'show',S+':'+name])==raw
 s=raw.decode()
 if name.endswith('Rail.tsx'):
  s='import { homeHandlerBegin, homeHandlerEnd } from "../../diagnostics/homePhaseTiming";\n'+s
  s=replace(s,'              useCockpit.getState().selectProject(null);\n              onClose?.();','              const timing = homeHandlerBegin();\n              try {\n                useCockpit.getState().selectProject(null);\n                onClose?.();\n              } finally { homeHandlerEnd(timing); }')
 elif name.endswith('AppShell.tsx'):
  s='import { homeRenderBegin, homeCommit } from "../diagnostics/homePhaseTiming";\n'+s
  s=replace(s,'import { useEffect, useRef, useState, type JSX }','import { useEffect, useLayoutEffect, useRef, useState, type JSX }')
  s=replace(s,'export function AppShell(): JSX.Element {','export function AppShell(): JSX.Element {\n  const homeRender = homeRenderBegin();')
  s=replace(s,'  // §6.8: the ONE window keydown listener','  useLayoutEffect(() => {\n    if (projectId === null && branchId === null) homeCommit(homeRender);\n  }, [projectId, branchId, homeRender]);\n\n  // §6.8: the ONE window keydown listener')
 elif name.endswith('uiSlice.ts'):
  s='import { homePhase } from "../../diagnostics/homePhaseTiming";\n'+s
  s=replace(s,'const navigationState = (navigation: PendingNavigation, state: CockpitState): Partial<UiSlice> => {','const navigationState = (navigation: PendingNavigation, state: CockpitState): Partial<UiSlice> => homePhase(3, () => {')
  s=replace(s,'  return { selection, lens, projectPositions: bounded, projectNavigationError: !saved };\n};','  return { selection, lens, projectPositions: bounded, projectNavigationError: !saved };\n});')
 elif name.endswith('projectNavigation.ts'):
  s='import { homePhase, homeCount } from "../diagnostics/homePhaseTiming";\n'+s
  a='export function writeProjectNavigation(scope: string, navigation: ProjectNavigation): boolean {\n  try {'
  s=replace(s,a,a.replace('\n  try {','\n  return homePhase(4, () => {\n  try {'))
  s=replace(s,'    const projects = Object.fromEntries(Object.entries(navigation.projects)\n      .sort((a, b) => b[1].updatedAt - a[1].updatedAt).slice(0, MAX_NAVIGATION_PROJECTS));\n    const raw = JSON.stringify({ version: 1, ...navigation, projects, updatedAt: Date.now() });','    const raw = homePhase(5, () => {\n      const projects = Object.fromEntries(Object.entries(navigation.projects)\n        .sort((a, b) => b[1].updatedAt - a[1].updatedAt).slice(0, MAX_NAVIGATION_PROJECTS));\n      return JSON.stringify({ version: 1, ...navigation, projects, updatedAt: Date.now() });\n    });')
  s=replace(s,'    // Bound only our small preferences; never evict project drafts or application data.','    homePhase(6, () => {\n    homeCount(10, localStorage.length);\n    // Bound only our small preferences; never evict project drafts or application data.')
  s=replace(s,'    localStorage.setItem(key, raw);','    homeCount(11, others.length);\n    });\n    homePhase(7, () => localStorage.setItem(key, raw));')
  assert s.endswith('  } catch { return false; }\n}\n');s=s[:-2]+'  });\n}\n'
 elif name.endswith('store.ts'):
  s='import { homePhase } from "../diagnostics/homePhaseTiming";\n'+s
  s=replace(s,'export const createCockpitStore = () =>','function homeTimingStorage() {\n  const storage = createJSONStorage(() => localStorage);\n  return storage && { ...storage, setItem: (name: string, value: Parameters<typeof storage.setItem>[1]) => homePhase(8, () => storage.setItem(name, value)) };\n}\n\nexport const createCockpitStore = () =>')
  s=replace(s,'        storage: createJSONStorage(() => localStorage),\n        partialize,','        storage: homeTimingStorage(),\n        partialize: state => homePhase(9, () => partialize(state)),')
 target=out/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_text(s)
 changes.append({'path':name,'originalSHA256':sha(raw),'overlaySHA256':sha(s.encode()),'bytes':len(s.encode())})
 patch.extend(difflib.unified_diff(raw.decode().splitlines(True),s.splitlines(True),fromfile='a/'+name,tofile='b/'+name))
name='desktop/src/diagnostics/homePhaseTiming.ts';target=out/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(B/'homePhaseTiming.ts',target)
changes.append({'path':name,'originalSHA256':None,'overlaySHA256':sha(target.read_bytes()),'bytes':target.stat().st_size})
patch.extend(difflib.unified_diff([],target.read_text().splitlines(True),fromfile='/dev/null',tofile='b/'+name))
(B/'overlay.patch').write_text(''.join(patch));(B/'overlay-binding.json').write_text(json.dumps({'sourceCommit':S,'observedPreparationHead':observedHead,'baseProduct':'577f7a0fdb039b7f875901a6a567d62fdfee88c8','diagnosticOnly':True,'optimization':False,'files':changes},indent=2)+'\n')
print(json.dumps({'overlayFiles':len(changes),'buildExecuted':False,'source':S}))
