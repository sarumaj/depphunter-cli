// Just enough of the editor's API to run the built extension outside an editor.
//
// What this is for: everything the extension does that can break without anyone
// noticing happens between it and the depphunter binary - the arguments it starts the
// server with, and reading back the address the server prints. Neither is checked by
// the Go tests or by the type checker, and an argument depphunter stops accepting
// would only show up when somebody opened the map. So the real out/extension.js runs
// here, against a real server, with the editor stubbed out from under it.

const Module = require('node:module');

/** What the extension asked the editor to do, oldest first. */
const calls = [];

/** The settings the stub hands back; a test can change them between runs. */
const settings = {
  path: process.env.DEPPHUNTER || 'depphunter',
  watch: true,
  style: 'default',
  findings: [],
  args: [],
  openIn: 'webview',
  dropQuery: false,
  editorCommand: '',
};

/** The commands the extension registered, by id. */
const commands = new Map();

let root = process.cwd();

const vscode = {
  Uri: {
    parse: s => ({ scheme: s.split(':')[0], fsPath: s, toString: () => s }),
    file: p => ({ scheme: 'file', fsPath: p, toString: () => 'file://' + p }),
  },
  env: {
    appRoot: '', // no editor to find a launcher in, so --editor is left off
    // The real one forwards the port and rewrites the address over a remote or a
    // tunnel. settings.dropQuery makes it drop the query while doing so, which is
    // what takes the session token with it.
    asExternalUri: async u => (settings.dropQuery
      ? vscode.Uri.parse(u.toString().replace(/\?.*/, ''))
      : u),
    openExternal: async u => { calls.push(['openExternal', u.toString()]); return true; },
  },
  workspace: {
    get workspaceFolders() { return [{ uri: { fsPath: root, scheme: 'file' }, name: 'workspace' }]; },
    fs: {
      writeFile: async (uri, data) => { calls.push(['writeFile', uri.fsPath, data]); },
    },
    getConfiguration: () => ({ get: key => settings[key] }),
    openTextDocument: async options => {
      const doc = { ...options, uri: vscode.Uri.parse('untitled:Untitled-1') };
      calls.push(['openTextDocument', doc]);
      return doc;
    },
    getWorkspaceFolder: () => undefined,
    onDidChangeWorkspaceFolders: () => ({ dispose() {} }),
    onDidChangeConfiguration: () => ({ dispose() {} }),
  },
  window: {
    createOutputChannel: () => ({
      appendLine: s => log(s + '\n'), append: log, show() {}, dispose() {},
    }),
    createWebviewPanel: (type, title, _column, options) => {
      const panel = {
        viewType: type, title, options, reveal() { calls.push(['panel.reveal', panel.title]); },
        webview: { set html(v) { panel.html = v; calls.push(['panel.html', v]); } },
        onDidDispose: fn => { panel.disposed = fn; return { dispose() {} }; },
        dispose() { calls.push(['panel.dispose', panel.title]); panel.disposed?.(); },
      };
      calls.push(['createWebviewPanel', panel]);
      return panel;
    },
    createStatusBarItem: () => ({
      show() { calls.push(['status.show']); }, hide() { calls.push(['status.hide']); },
      dispose() {}, set text(v) { calls.push(['status.text', v]); },
      set tooltip(v) { calls.push(['status.tooltip', v]); }, set command(_v) {},
    }),
    showErrorMessage: async m => { calls.push(['error', m]); return undefined; },
    showInformationMessage: async m => { calls.push(['info', m]); return undefined; },
    showWarningMessage: async m => { calls.push(['warning', m]); return undefined; },
    showWorkspaceFolderPick: async () => vscode.workspace.workspaceFolders[0],
    showQuickPick: async items => items[0],
    registerTreeDataProvider: (id, provider) => {
      calls.push(['registerTreeDataProvider', id, provider]);
      return { dispose() {} };
    },
    createTreeView: (id, options) => {
      const view = {
        id, visible: true, title: id,
        reveal: async (element, opts) => { calls.push(['reveal', id, element, opts]); },
        onDidChangeVisibility: fn => { view.visibilityListener = fn; return { dispose() {} }; },
        dispose() {},
      };
      calls.push(['registerTreeDataProvider', id, options.treeDataProvider]);
      calls.push(['createTreeView', id, view]);
      return view;
    },
    showSaveDialog: async options => { calls.push(['showSaveDialog', options]); return undefined; },
    showTextDocument: async doc => { calls.push(['showTextDocument', doc]); return { document: doc }; },
    withProgress: (_options, task) => task(
      { report() {} }, { onCancellationRequested: () => ({ dispose() {} }) }),
  },
  commands: {
    registerCommand: (id, fn) => { commands.set(id, fn); return { dispose() {} }; },
    executeCommand: async (id, ...args) => { calls.push(['executeCommand', id, ...args]); },
  },
  EventEmitter: class EventEmitter {
    constructor() { this.listeners = []; }
    get event() { return fn => { this.listeners.push(fn); return { dispose() {} }; }; }
    fire(v) { for (const fn of this.listeners) fn(v); }
    dispose() { this.listeners = []; }
  },
  TreeItem: class TreeItem {
    constructor(label, collapsibleState) { this.label = label; this.collapsibleState = collapsibleState; }
  },
  TreeItemCollapsibleState: { None: 0, Collapsed: 1, Expanded: 2 },
  ThemeIcon: class ThemeIcon { constructor(id, color) { this.id = id; this.color = color; } },
  ThemeColor: class ThemeColor { constructor(id) { this.id = id; } },
  MarkdownString: class MarkdownString {
    constructor(value = '') { this.value = value; }
    appendMarkdown(text) { this.value += text; return this; }
  },
  Disposable: class Disposable {
    constructor(fn) { this.dispose = fn; }
  },
  StatusBarAlignment: { Right: 2 },
  ViewColumn: { Active: -1 },
  ProgressLocation: { Notification: 15 },
  CancellationError: class CancellationError extends Error {},
  CancellationTokenSource: class CancellationTokenSource {
    constructor() {
      const listeners = [];
      this.token = {
        isCancellationRequested: false,
        onCancellationRequested: fn => { listeners.push(fn); return { dispose() {} }; },
      };
      this.cancel = () => {
        if (this.token.isCancellationRequested) return;
        this.token.isCancellationRequested = true;
        for (const fn of listeners) fn();
      };
    }
    dispose() {}
  },
};

function log(text) {
  if (process.env.VERBOSE) process.stderr.write(text);
}

// require('vscode') resolves to nothing outside an editor, where it is injected into
// the module cache by the extension host. Do the same.
const resolve = Module._resolveFilename;
Module._resolveFilename = function (request, ...rest) {
  return request === 'vscode' ? 'vscode' : resolve.call(this, request, ...rest);
};
require.cache['vscode'] = { id: 'vscode', filename: 'vscode', loaded: true, exports: vscode };

module.exports = {
  vscode, calls, commands, settings,
  setRoot: dir => { root = dir; },
  /** The last call of a kind, or undefined. */
  last: (kind, id) => [...calls].reverse().find(c => c[0] === kind && (!id || c[1] === id)),
};
