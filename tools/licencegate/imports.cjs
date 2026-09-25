'use strict';
// tools/licencegate/imports.cjs <projectDir> <file>...
//
// The gate's one Node helper (review I9, hole 1). It loads
// <projectDir>/node_modules/typescript, parses each file with
// ts.createSourceFile (ScriptKind TSX for .tsx/.jsx), and prints JSON
// [{file, specifier, typeOnly}] for every ImportDeclaration,
// ExportDeclaration with a module specifier, `import x = require("y")`,
// import("x") and require("x") with a string-literal argument.
//
// A parser, not a regular expression: a comment reading `from "x"`, JSX text
// holding a quote and a regex literal such as /"/g all fool a pattern, and
// the first is in a shipped file today (desktop/src/components/develop/
// CanvasLayout.tsx). It reads files and prints; it runs nothing it parses.
const fs = require('fs');
const path = require('path');

const [projectDir, ...files] = process.argv.slice(2);
if (!projectDir) {
  process.stderr.write('usage: imports.cjs <projectDir> <file>...\n');
  process.exit(2);
}
const ts = require(path.join(path.resolve(projectDir), 'node_modules', 'typescript'));
// A Node core module ("fs", "node:fs") names no package: Vite replaces it
// with an empty browser stub. The gate drops these.
const { isBuiltin } = require('module');

function kindOf(file) {
  switch (path.extname(file).toLowerCase()) {
    case '.tsx':
    case '.jsx':
      return ts.ScriptKind.TSX;
    case '.ts':
    case '.mts':
    case '.cts':
      return ts.ScriptKind.TS;
    default:
      return ts.ScriptKind.JS;
  }
}

const out = [];
for (const file of files) {
  const text = fs.readFileSync(file, 'utf8');
  const sf = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, kindOf(file));
  const push = (lit, typeOnly) =>
    out.push({ file, specifier: lit.text, typeOnly: !!typeOnly, builtin: isBuiltin(lit.text) });
  const visit = (node) => {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      push(node.moduleSpecifier, node.importClause && node.importClause.isTypeOnly);
    } else if (ts.isExportDeclaration(node) && node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
      push(node.moduleSpecifier, node.isTypeOnly);
    } else if (
      ts.isImportEqualsDeclaration(node) &&
      ts.isExternalModuleReference(node.moduleReference) &&
      ts.isStringLiteral(node.moduleReference.expression)
    ) {
      push(node.moduleReference.expression, node.isTypeOnly);
    } else if (ts.isCallExpression(node) && node.arguments.length > 0 && ts.isStringLiteralLike(node.arguments[0])) {
      const callee = node.expression;
      const isImport = callee.kind === ts.SyntaxKind.ImportKeyword;
      const isRequire = ts.isIdentifier(callee) && callee.text === 'require';
      if (isImport || isRequire) push(node.arguments[0], false);
    }
    ts.forEachChild(node, visit);
  };
  visit(sf);
}
process.stdout.write(JSON.stringify(out));
