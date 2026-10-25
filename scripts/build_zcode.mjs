import { build } from 'esbuild';
import { existsSync, readFileSync, readdirSync, mkdirSync, writeFileSync, copyFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const upstream = resolve(root, 'libs/platform/zcode/upstream');
execFileSync(process.execPath, [resolve(upstream, 'dynamic-workflow/scripts/generate-libs.mjs')], {stdio:'inherit'});
const names = ['core', 'contracts', 'dynamic-workflow', 'shared-types', 'shared', 'model-option-map', 'zcode-cua'];
const packages = new Map(names.map(name => {
  const path = resolve(upstream, name);
  return [JSON.parse(readFileSync(resolve(path, 'package.json'), 'utf8')).name, path];
}));

// 上游是未发布的工作区源码；按原 exports 解析 src，避免修改第三方源码或生成目录。
const sources = {
  name: 'zcode-source-packages',
  setup(builder) {
    builder.onResolve({filter:/^@zcode\//}, args => {
      const parts = args.path.split('/');
      const path = packages.get(parts.slice(0, 2).join('/'));
      if (!path) return;
      const metadata = JSON.parse(readFileSync(resolve(path, 'package.json'), 'utf8'));
      const key = parts.length === 2 ? '.' : './' + parts.slice(2).join('/');
      const exported = metadata.exports[key];
      const target = typeof exported === 'string' ? exported : exported?.import;
      if (!target) throw new Error('ZCode 未导出模块：' + args.path);
      const source = resolve(path, target.replace('./dist/', './src/'));
      return {path:existsSync(source) ? source : source.replace(/\.js$/, '.ts')};
    });
  },
};

const outfile = resolve(root, process.argv[3] || process.env.NW_AGENT_OUT || 'agent-dist/worker.mjs');
const result = await build({
  entryPoints:[resolve(root, process.argv[2] || 'libs/platform/zcode/worker.mjs')],
  outfile,
  bundle:true, platform:'node', format:'esm', target:'node24', sourcemap:false,
  legalComments:'external', plugins:[sources], logLevel:'info', metafile:true,
  banner:{js:"import { createRequire as __zcodeCreateRequire } from 'node:module'; import { fileURLToPath as __zcodeFileURL } from 'node:url'; import { dirname as __zcodeDirname } from 'node:path'; const require = __zcodeCreateRequire(import.meta.url); const __filename = __zcodeFileURL(import.meta.url); const __dirname = __zcodeDirname(__filename);"},
});

// 随运行文件发布实际打包依赖的完整许可与版权，不把无关工作区依赖带入运行镜像。
const dependencies = new Map();
for (const input of Object.keys(result.metafile.inputs)) {
  if (!input.includes('node_modules/')) continue;
  let directory = dirname(resolve(root, input));
  let metadata;
  while (directory !== dirname(directory)) {
    if (existsSync(resolve(directory, 'package.json'))) {
      metadata = JSON.parse(readFileSync(resolve(directory, 'package.json'), 'utf8'));
      if (metadata.name && metadata.version) break;
    }
    directory = dirname(directory);
  }
  if (!metadata?.name) throw new Error('无法定位运行依赖声明：' + input);
  dependencies.set(`${metadata.name}@${metadata.version}`, {directory, metadata});
}
const licenses = resolve(dirname(outfile), 'licenses');
mkdirSync(licenses, {recursive:true});
let notices = 'ZCode 原生运行时：3.16.2 main，来源 https://github.com/zai-org/ZCode，Copyright 2026 Z.AI，Apache-2.0。\n\n';
notices += 'Fig autocomplete 2.692.3 生成注册表 — 原始源码 MIT（npm 元数据标示 ISC）。\nVisual Studio Code IPC/common 派生工具 — MIT。\n\n';
copyFileSync(resolve(upstream, 'LICENSE'), resolve(licenses, 'ZCode-Apache-2.0.txt'));
copyFileSync(resolve(root, 'docs/licenses/ZCode-runtime-NOTICE.txt'), resolve(licenses, 'ZCode-NOTICE.txt'));
for (const name of ['Fig-autocomplete-MIT.txt','VSCode-MIT.txt']) copyFileSync(resolve(root, 'docs/licenses', name), resolve(licenses, name));
for (const [name, {directory, metadata}] of [...dependencies].sort()) {
  const files = readdirSync(directory).filter(file => /^(licen[cs]e|notice|thirdpartynotice)/i.test(file));
  if (!files.length) throw new Error('缺少运行依赖许可：' + name);
  notices += `${name} — ${typeof metadata.license === 'string' ? metadata.license : JSON.stringify(metadata.license)}\n`;
  for (const file of files) copyFileSync(resolve(directory, file), resolve(licenses, name.replaceAll('/', '-') + '-' + file));
}
writeFileSync(resolve(dirname(outfile), 'THIRD_PARTY_NOTICES.txt'), notices);
