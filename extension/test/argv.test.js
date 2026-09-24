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
      links: false,
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
      // The view settings go as seeds rather than as flags: a repository that has
      // saved a view of its own keeps it, which is what makes the map's Save button
      // mean something in an editor that has these set.
      '--ui-default', 'style=galaxy', '--ui-default', 'theme=dark',
      '--ui-default', 'color_by=churn', '--ui-default', 'height_scale=log',
      '--ui-default', 'expand_depth=2', '--ui-default', 'show_std=true',
      '--findings', 'reports/*.json', '--no-vulns', '--no-links',
      '--lsp', '--lsp-timeout', '90s',
      '--editor', 'vim +{line} {file}',
      '--exclude', 'dist',
    ]);
  });

  it('passes zero, which is a value and not an unset number', () => {
    assert.deepStrictEqual(fromSettings({ watch: false, resolveDepth: 0, expandDepth: 0 }),
      ['--resolve-depth', '0', '--ui-default', 'expand_depth=0']);
  });

  it('never sends a view setting as a flag, which would beat the saved view', () => {
    // The map writes what it is set to into the repository's own ui: section, and a
    // flag beats that file - so a view setting sent as one would make Save quietly
    // stop working for whatever the editor happens to have set.
    const flags = fromSettings({
      watch: false, style: 'galaxy', theme: 'dark', colorBy: 'churn',
      heightScale: 'log', expandDepth: 2, showStd: true,
    });
    for (const beats of ['--style', '--theme', '--color-by', '--height-scale', '--expand-depth', '--show-std']) {
      assert.ok(!flags.includes(beats), `${beats} would overrule the saved view`);
    }
    assert.equal(flags.filter(a => a === '--ui-default').length, 6);
  });
});
