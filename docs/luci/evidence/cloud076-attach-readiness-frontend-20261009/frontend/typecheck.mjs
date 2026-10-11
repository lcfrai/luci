// TypeScript's compiler host reads the exact staged replacements at canonical
// repository paths. No checkout mutation, generated source or output emission.
import ts from 'typescript';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { createHash } from 'node:crypto';
const stage = '/workspace/.luci-tools/cloud076-attach-readiness/frontend';
const root = '/workspace/luci/desktop';
const out = process.argv[2]; if (!out?.startsWith(stage + '/')) throw Error('Fresh external result path required');
mkdirSync(out);
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const paths = ['src/api/attachRegistry.ts','src/store/types.ts','src/components/develop/SessionPaneFooter.tsx'];
const replacements = new Map(paths.map(rel => [root + '/' + rel, stage + '/fixed/desktop/' + rel]));
const test = root + '/src/api/attachRegistry.readiness.test.ts';
replacements.set(test, stage + '/tests/desktop/src/api/attachRegistry.readiness.test.ts');
const inputs = new Map();
const readFile = path => {
  const source = replacements.get(path) ?? path;
  try { const bytes = readFileSync(source); inputs.set(source, digest(bytes)); return bytes.toString('utf8'); }
  catch (error) { if (error.code === 'ENOENT' || error.code === 'ENOTDIR') return undefined; throw error; }
};
const config = ts.readConfigFile(root + '/tsconfig.json', readFile);
if (config.error) throw Error(ts.flattenDiagnosticMessageText(config.error.messageText, '\n'));
const parsed = ts.parseJsonConfigFileContent(config.config, {...ts.sys, readFile}, root);
const host = ts.createCompilerHost(parsed.options);
host.readFile = readFile;
const exists = host.fileExists;
host.fileExists = path => replacements.has(path) || exists(path);
const startedAt = new Date().toISOString();
const program = ts.createProgram({rootNames:[...parsed.fileNames,test],options:{...parsed.options,noEmit:true},projectReferences:parsed.projectReferences,host});
const diagnostics = [...parsed.errors,...ts.getPreEmitDiagnostics(program)];
const rendered = ts.formatDiagnosticsWithColorAndContext(diagnostics,{getCurrentDirectory:()=>root,getCanonicalFileName:x=>x,getNewLine:()=> '\n'});
writeFileSync(out + '/diagnostics.txt',rendered,{flag:'wx'});
const unchanged = [...inputs].every(([path,before]) => digest(readFileSync(path)) === before);
const result = {startedAt,finishedAt:new Date().toISOString(),diagnostics:diagnostics.length,compilerVersion:ts.version,
  inputCount:inputs.size,inputsUnchanged:unchanged,sourceOverlay:[...replacements].map(([path,source])=>({path,source,sha256:digest(readFileSync(source))})),
  scope:'Full desktop tsconfig using exact external source replacements and new regression; no emit, runtime or checkout edit.'};
writeFileSync(out + '/inputs.json',JSON.stringify(Object.fromEntries([...inputs].sort()),null,2)+'\n',{flag:'wx'});
writeFileSync(out + '/result.json',JSON.stringify(result,null,2)+'\n',{flag:'wx'});
console.log(JSON.stringify(result)); if (rendered) console.log(rendered);
process.exitCode = diagnostics.length || !unchanged ? 1 : 0;
