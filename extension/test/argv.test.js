// The command line the extension builds from its settings. No server needed: what is
// checked is that each setting reaches depphunter as the flag it names, and that a
// setting left alone adds nothing - which is what lets a folder's .depphunter.yaml,
// which flags override, keep deciding everything the user did not set here.

const assert = require('node:assert');
const { describe, it } = require('node:test');

require('./stub'); // server.js imports vscode
const { argv } = require('../out/server.js');

const ROOT = '/work/app';

/** A configuration holding only what a test sets, like one fresh out of the manifest. */
function config(values) {
  const manifest = require('../../package.json').contributes.configuration;
  const defaults = {};
  for (const section of manifest) {
    for (const [key, p] of Object.entries(section.properties)) {
      defaults[key.replace(/^depphunter\./, '')] = p.default;
    }
  }
  const all = { ...defaults, ...values };
  return { get: key => all[key] };
}

/** The part of the command line that comes from settings. */
function fromSettings(values) {
  const args = argv(config(values), ROOT);
  const fixed = args.lastIndexOf('--embed') + 2;
  assert.strictEqual(args.at(-1), ROOT, 'the folder goes last');
  return args.slice(fixed, -1);
}

describe('the command line', () => {
  it('always runs headless, on a free port, embeddable', () => {
    const args = argv(config({}), ROOT);
    assert.deepStrictEqual(args.slice(0, 3), ['--no-open', '--addr', '127.0.0.1:0']);
    assert.ok(args.includes('--embed'));
  });

  it('adds nothing but --watch for settings left at their defaults', () => {
    assert.deepStrictEqual(fromSettings({}), ['--watch']);
  });

  it('turns every setting into its flag', () => {
    assert.deepStrictEqual(fromSettings({
      config: 'ci/depphunter.yaml',
      watch: false,
      exclude: ['vendor', ' ', '**/*.gen.go'],
      maxFileSize: 1048576,
      resolveDepth: -1,
      online: true,
      explain: true,
      cache: false,
      history: false,
      historyCommits: 500,
      style: 'galaxy',
      theme: 'dark',
      colorBy: 'churn',
      heightScale: 'log',
      expandDepth: 2,
      showStd: true,
      findings: ['reports/*.json'],
      vulns: false,
      lsp: true,
      lspTimeout: '90s',
      editorCommand: 'vim +{line} {file}',
      args: ['--exclude', 'dist'],
    }), [
      '--config', 'ci/depphunter.yaml',
      '--exclude', 'vendor', '--exclude', '**/*.gen.go',
      '--max-file-size', '1048576',
      '--resolve-depth', '-1',
      '--online', '--explain', '--no-cache', '--no-history',
      '--history-commits', '500',
      '--style', 'galaxy', '--theme', 'dark', '--color-by', 'churn', '--height-scale', 'log',
      '--expand-depth', '2', '--show-std',
      '--findings', 'reports/*.json', '--no-vulns',
      '--lsp', '--lsp-timeout', '90s',
      '--editor', 'vim +{line} {file}',
      '--exclude', 'dist',
    ]);
  });

  it('passes zero, which is a value and not an unset number', () => {
    assert.deepStrictEqual(fromSettings({ watch: false, resolveDepth: 0, expandDepth: 0 }),
      ['--resolve-depth', '0', '--expand-depth', '0']);
  });
});
