// Use the existing desktop TypeScript dependency for semantic name resolution.
// This emits no files and is deliberately not a full JavaScript type check.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {readFileSync} from 'node:fs';
import {dirname,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(here,'../../..');
const requireDesktop=createRequire(resolve(repo,'desktop/package.json')),ts=requireDesktop('typescript');
const args=process.argv.slice(2);assert(args.length===0||args.length===2&&args[0]==='--source-text-file');
const file=resolve(here,'installed-worker-recovery.mjs');
const options={allowJs:true,checkJs:true,noEmit:true,target:ts.ScriptTarget.ESNext,
 module:ts.ModuleKind.NodeNext,moduleResolution:ts.ModuleResolutionKind.NodeNext,
 skipLibCheck:true,types:['node'],typeRoots:[dirname(dirname(requireDesktop.resolve('@types/node/package.json')))]};
const host=ts.createCompilerHost(options),originalRead=host.readFile;
// Historical bytes retain the real module path so every import resolves exactly
// as it does for the current adapter. Nothing on disk is replaced.
if(args.length){const bytes=readFileSync(resolve(args[1]),'utf8');host.readFile=path=>resolve(path)===file?bytes:originalRead(path);}
const program=ts.createProgram([file],options,host),source=program.getSourceFile(file);
assert(source);const semantic=program.getSemanticDiagnostics(source),nameCodes=new Set([2304,2552,18004,2580,2591]);
const names=semantic.filter(item=>nameCodes.has(item.code));
const diagnostic=item=>{const location=item.file&&item.start!==undefined?item.file.getLineAndCharacterOfPosition(item.start):null;return {code:item.code,...(location?{line:location.line+1,column:location.character+1}:{}),message:ts.flattenDiagnosticMessageText(item.messageText,' ')};};
const optionsDiagnostics=program.getOptionsDiagnostics();
console.log(JSON.stringify({typescript:ts.version,file:'scripts/acceptance/worker-recovery/installed-worker-recovery.mjs',
 historicalSourceOverride:args.length>0,noEmit:true,optionsDiagnostics:optionsDiagnostics.map(diagnostic),
 semanticDiagnostics:semantic.length,undefinedNameDiagnostics:names.map(diagnostic),
 limitation:'Only unresolved-name diagnostics are checked; other JavaScript type diagnostics are outside this check.'},null,2));
process.exitCode=names.length||optionsDiagnostics.length?1:0;
