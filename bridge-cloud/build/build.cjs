const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { minify } = require('terser');
const obfuscator = require('javascript-obfuscator');

(async () => {
  const source = fs.readFileSync(path.join(__dirname, '../index.js'), 'utf8');
  // AST-based removal also handles inline console calls, preserving surrounding code.
  const minified = await minify(source, {
    compress: { drop_console: true, passes: 2 },
    mangle: true,
    format: { comments: false },
  });
  const code = obfuscator.obfuscate(minified.code, {
    compact: true,
    controlFlowFlattening: true,
    controlFlowFlatteningThreshold: 0.75,
    deadCodeInjection: true,
    deadCodeInjectionThreshold: 0.4,
    stringArray: true,
    stringArrayEncoding: ['base64'],
    stringArrayThreshold: 0.8,
    identifierNamesGenerator: 'hexadecimal',
    renameGlobals: false,
    selfDefending: true,
    // Console calls already removed; avoid adding console stubs.
    disableConsoleOutput: false,
    target: 'node',
  }).getObfuscatedCode();
  new vm.Script(code);
  const out = process.env.OUT_DIR || path.join(__dirname, '../../../deploy/dist/cloud');
  fs.mkdirSync(out, { recursive: true });
  fs.writeFileSync(path.join(out, 'index.js'), code);
  fs.copyFileSync(path.join(__dirname, '../package.json'), path.join(out, 'package.json'));
  console.log(`Cloud build: ${source.length} -> ${code.length} bytes`);
})().catch(err => { console.error(err); process.exit(1); });
