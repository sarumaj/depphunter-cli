// depphunter in the editor: one server per folder, and the map in the built-in
// browser beside the code.
//
// The server is a child process of the extension host, so it lives as long as the
// window does and no longer. Opening the map again while one is already running
// reuses it: analyzing a large repository takes a while, and the point of --watch is
// that it only has to happen once.

import { ChildProcess } from 'node:child_process';
import * as path from 'node:path';
import * as vscode from 'vscode';

import { Api, PackItem } from './api';
import { BackpackView } from './backpack';
import * as panel from './panel';
import { StartError, start } from './server';
import { binDirFor, exposeOnPath } from './terminal';
import { DependencyTree, Row } from './tree';
import { MapsView } from './view';

const RELEASES = 'https://github.com/sarumaj/depphunter-cli/releases';

interface Session {
  readonly root: string;
  readonly name: string;
  readonly url: string;
  readonly child: ChildProcess;
}

const sessions = new Map<string, Session>();
// Servers still starting, by folder. A second open of the folder waits on the entry,
// and stopAll cancels it: until a server is in sessions, nothing else can stop it.
const starting = new Map<string, { done: Promise<Session | undefined>; cancel: vscode.CancellationTokenSource }>();
let log: vscode.OutputChannel;
let status: vscode.StatusBarItem;
let view: MapsView;
let tree: DependencyTree;
let treeView: vscode.TreeView<Row>;
let backpack: BackpackView;
/**
 * The session the two lower views are showing. One window may map several folders;
 * the panel shows the one whose map was opened last, which is the one being looked at.
 */
let attached: { root: string; api: Api; stream: vscode.Disposable; greeted?: boolean } | undefined;
/** What the map has selected, so a panel that was hidden can catch up when it opens. */
let selected = '';
/** Where this build was installed, which is where a released one keeps its binary. */
let home: string | undefined;

// Implements: REQ-EXT-001, REQ-EXT-027, REQ-EXT-033
export function activate(context: vscode.ExtensionContext): void {
  home = context.extensionPath;
  // The editor's terminals get the binary on their PATH (terminal.ts), from the start
  // and again whenever the setting that decides which binary it is changes.
  const onPath = (): void => {
    const cfg = vscode.workspace.getConfiguration('depphunter');
    if (context.environmentVariableCollection) {
      exposeOnPath(context.environmentVariableCollection, binDirFor(cfg.get<string>('path'), home, cfg.get<boolean>('addToPath') ?? true));
    }
  };
  onPath();
  settings = settingsOf(context.extension?.packageJSON);
  log = vscode.window.createOutputChannel('depphunter');
  status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  status.command = 'depphunter.open';
  view = new MapsView(() => sessions);
  tree = new DependencyTree();
  backpack = new BackpackView();
  treeView = vscode.window.createTreeView('depphunter.tree', { treeDataProvider: tree, showCollapseAll: true });
  context.subscriptions.push(log, status, view, tree, backpack, treeView,
    // A tree that was hidden was not revealed, so opening the panel would show
    // nothing picked out although the map has had something selected all along.
    treeView.onDidChangeVisibility(e => e.visible && revealSelected(selected)),
    { dispose: stopAll }, { dispose: detach });

  context.subscriptions.push(
    vscode.commands.registerCommand('depphunter.open', (resource?: vscode.Uri) => open(resource)),
    vscode.commands.registerCommand('depphunter.restart', (resource?: vscode.Uri) => restart(resource)),
    vscode.commands.registerCommand('depphunter.stop', (resource?: vscode.Uri) => stop(resource)),
    vscode.commands.registerCommand('depphunter.showLog', () => log.show()),
    vscode.commands.registerCommand('depphunter.openSettings', () =>
      vscode.commands.executeCommand('workbench.action.openSettings', '@ext:sarumaj.depphunter')),
    vscode.commands.registerCommand('depphunter.openExternal', (resource?: vscode.Uri) => openExternal(resource)),
    vscode.commands.registerCommand('depphunter.refresh', () => refreshPanel()),
    vscode.commands.registerCommand('depphunter.select', (row: Row) => attached?.api.select(row.node.id).catch(noted)),
    vscode.commands.registerCommand('depphunter.openFile', (row: Row) => openFile(row)),
    vscode.commands.registerCommand('depphunter.showFinding', (it: PackItem) => showFinding(it)),
    vscode.commands.registerCommand('depphunter.dropFinding', (it: PackItem) => dropFinding(it)),
    vscode.commands.registerCommand('depphunter.export', () => exportGraph()),
    vscode.commands.registerCommand('depphunter.resolution', () => showResolution()),
    vscode.commands.registerCommand('depphunter.exportBackpack', () => exportBackpack()),
    vscode.window.registerTreeDataProvider('depphunter.maps', view),
    vscode.window.registerTreeDataProvider('depphunter.backpack', backpack),
    vscode.workspace.onDidChangeWorkspaceFolders(e => {
      for (const folder of e.removed) end(folder.uri.fsPath);
      view.refresh();
    }),
    vscode.workspace.onDidChangeConfiguration(e => {
      if (e.affectsConfiguration('depphunter.path') || e.affectsConfiguration('depphunter.addToPath')) onPath();
      if (e.affectsConfiguration('depphunter')) void offerRestart(e);
    }),
  );
}

export function deactivate(): void {
  stopAll();
  detach();
  panel.closeAll();
}

// ---------------------------------------------------------------- the panel

/**
 * Points the dependency tree and the backpack at a running server, and follows what
 * it says afterwards.
 *
 * The graph is fetched once and redrawn on a --watch update; the selection and the
 * catch arrive on the server's event stream, which is also how a building picked on
 * the map turns into a revealed row here.
 * Implements: REQ-EXT-002, REQ-EXT-004, REQ-EXT-008, REQ-EXT-009, REQ-EXT-010
 */
async function attach(session: Session): Promise<void> {
  detach();
  const api = new Api(session.url);
  const stream = api.watch(event => {
    if (attached?.api !== api) return;
    switch (event.name) {
      case 'graph':
        void refreshGraph(api);
        break;
      case 'selection':
        if (event.data.origin !== api.origin) revealSelected(event.data.id);
        break;
      case 'backpack':
        void refreshSession(api);
        break;
      case 'hello': {
        // A connection that did not resume may have missed announcements while it
        // was down, and nothing announces them twice - so ask again, which is cheap:
        // the graph answers 304 when it has not moved. The first greeting is not a
        // reconnection; attach has just read both.
        const first = !attached.greeted;
        attached.greeted = true;
        if (!first && !event.data.resumed) void Promise.all([refreshGraph(api), refreshSession(api)]);
        break;
      }
    }
  });
  attached = { root: session.root, api, stream };
  treeView.title = `Dependencies: ${session.name}`;
  await Promise.all([refreshGraph(api), refreshSession(api)]);
}

// Implements: REQ-EXT-002
function detach(): void {
  attached?.stream.dispose();
  attached = undefined;
  selected = '';
  tree.setGraph(undefined);
  backpack.setItems([]);
  treeView.title = 'Dependencies';
}

async function refreshGraph(api: Api, force = false): Promise<void> {
  try {
    // null: the server says the panel already has this graph, so there is nothing
    // to rebuild. Re-indexing a hundred thousand nodes to arrive at the same tree
    // is the sort of work nobody sees and everybody pays for.
    const graph = await api.graph(force);
    if (graph && attached?.api === api) tree.setGraph(graph);
  } catch (err) {
    noted(err);
  }
}

async function refreshSession(api: Api): Promise<void> {
  try {
    const state = await api.session();
    if (attached?.api !== api) return;
    backpack.setItems(state.backpack ?? []);
    revealSelected(state.selected);
  } catch (err) {
    noted(err);
  }
}

// The Refresh command: somebody pressed it, so the graph is read again whether or
// not the server thinks the panel already has it.
function refreshPanel(): void {
  if (attached) void Promise.all([refreshGraph(attached.api, true), refreshSession(attached.api)]);
}

/**
 * Opens the tree down to what the map has selected, without stealing the focus.
 * Implements: REQ-EXT-008
 */
function revealSelected(id: string): void {
  selected = id;
  const row = id ? tree.graphModel?.rowFor(id) : undefined;
  if (row && treeView.visible) void treeView.reveal(row, { select: true, focus: false, expand: true });
}

async function openFile(row: Row): Promise<void> {
  const file = row.node.kind === 'file' ? row.node : undefined;
  if (!file?.path || !attached) return;
  const uri = vscode.Uri.file(path.join(attached.root, file.path));
  await vscode.window.showTextDocument(uri, { preview: true });
}

/**
 * A caught finding, back where it was caught: the map selects what it belongs to.
 * Implements: REQ-EXT-012
 */
async function showFinding(it: PackItem): Promise<void> {
  if (!attached || !it.nodeId) return;
  await attached.api.select(it.nodeId).catch(noted);
  revealSelected(it.nodeId);
}

// Implements: REQ-EXT-011
async function dropFinding(it: PackItem): Promise<void> {
  if (!attached) return;
  const left = backpack.contents.filter(other => other.id !== it.id);
  try {
    await attached.api.setBackpack(left);
    backpack.setItems(left);
  } catch (err) {
    await report(err);
  }
}

// ---------------------------------------------------------------- exports

// Implements: REQ-EXT-014
const GRAPH_FORMATS = [
  { label: 'JSON', detail: 'the graph document', format: 'json', ext: 'json' },
  { label: 'GraphML', detail: 'Gephi, yEd, NetworkX', format: 'graphml', ext: 'graphml' },
  { label: 'DOT', detail: 'Graphviz dependency graph', format: 'dot', ext: 'dot' },
  { label: 'HTML', detail: 'a self-contained map to share', format: 'html', ext: 'html' },
];

// Implements: REQ-EXT-013
const PACK_FORMATS = [
  { label: 'Markdown', detail: 'a checklist to paste into an issue', format: 'md', ext: 'md' },
  { label: 'CSV', detail: 'for a spreadsheet', format: 'csv', ext: 'csv' },
  { label: 'JSON', detail: 'the items as the map records them', format: 'json', ext: 'json' },
];

const exportGraph = () => save('api/export', GRAPH_FORMATS, name => name);
const exportBackpack = () => save('api/backpack', PACK_FORMATS, name => `${name}-backpack`);

/**
 * Asks what format, asks where, and writes what the server produced.
 * Implements: REQ-EXT-013, REQ-EXT-014
 */
async function save(
  endpoint: string,
  formats: { label: string; detail: string; format: string; ext: string }[],
  name: (root: string) => string,
): Promise<void> {
  if (!attached) {
    void vscode.window.showInformationMessage('Open a map first: there is nothing to export yet.');
    return;
  }
  const chosen = await vscode.window.showQuickPick(formats, { title: 'Export as', matchOnDetail: true });
  if (!chosen) return;
  const base = name(path.basename(attached.root) || 'depphunter');
  const uri = await vscode.window.showSaveDialog({
    defaultUri: vscode.Uri.file(path.join(attached.root, `${base}.${chosen.ext}`)),
    filters: { [chosen.label]: [chosen.ext] },
  });
  if (!uri) return;
  try {
    const body = await attached.api.download(`/${endpoint}?format=${chosen.format}`);
    await vscode.workspace.fs.writeFile(uri, body);
    const open = await vscode.window.showInformationMessage(`Exported to ${path.basename(uri.fsPath)}.`, 'Open');
    if (open === 'Open') await vscode.commands.executeCommand('vscode.open', uri);
  } catch (err) {
    await report(err);
  }
}

/**
 * How the analysis reached the dependencies it drew, as a document beside the code.
 * The server renders it (internal/trace), so this and what `--explain` writes to the
 * log are one report in two shapes rather than two accounts that can disagree.
 * Implements: REQ-EXT-016
 */
async function showResolution(): Promise<void> {
  if (!attached) {
    void vscode.window.showInformationMessage('Open a map first: there is nothing to report on yet.');
    return;
  }
  try {
    const body = await attached.api.download('/api/resolution?format=md');
    const doc = await vscode.workspace.openTextDocument({
      content: body.toString('utf8'), language: 'markdown',
    });
    // Rendered where the editor can render it; a build or a configuration without
    // the Markdown preview still gets the document itself.
    try {
      await vscode.commands.executeCommand('markdown.showPreview', doc.uri);
    } catch {
      await vscode.window.showTextDocument(doc);
    }
  } catch (err) {
    await report(err);
  }
}

/**
 * The map in the browser outside the editor, whatever depphunter.openIn says. The
 * setting is where it opens by default; this is for the one time it is wanted
 * somewhere with more screen, or a second monitor, or a browser's own dev tools.
 * Implements: REQ-EXT-015
 */
async function openExternal(resource?: vscode.Uri): Promise<void> {
  const folder = await pick(resource);
  if (!folder) return;
  const session = await ensure(folder);
  if (!session) return;
  if (attached?.root !== session.root) await attach(session);
  await vscode.env.openExternal(vscode.Uri.parse(await reachable(session.url)));
}

/** An error worth a line in the log and nothing more: the panel is not the task. */
function noted(err: unknown): void {
  log.appendLine(`side panel: ${err instanceof Error ? err.message : String(err)}`);
}

async function open(resource?: vscode.Uri): Promise<void> {
  const folder = await pick(resource);
  if (!folder) return;
  const session = await ensure(folder);
  if (!session) return;
  // launch attaches the panel to what it started; this is the other way in, where
  // the server was already running - possibly for a different folder than the one
  // the panel is showing.
  if (attached?.root !== session.root) await attach(session);
  await show(session);
}

/**
 * The server for a folder: the one running, the one starting, or a new one.
 * Implements: REQ-EXT-026
 */
function ensure(folder: { root: string; name: string }): Promise<Session | undefined> {
  const session = sessions.get(folder.root);
  if (session) return Promise.resolve(session);
  return starting.get(folder.root)?.done ?? launch(folder);
}

function launch(folder: { root: string; name: string }): Promise<Session | undefined> {
  const pending = starting.get(folder.root);
  if (pending) return pending.done;
  const cancel = new vscode.CancellationTokenSource();
  const done = launching(folder, cancel).finally(() => {
    starting.delete(folder.root);
    cancel.dispose();
  });
  starting.set(folder.root, { done, cancel });
  return done;
}

async function launching(folder: { root: string; name: string }, cancel: vscode.CancellationTokenSource): Promise<Session | undefined> {
  try {
    const running = await vscode.window.withProgress(
      { location: vscode.ProgressLocation.Notification, title: `depphunter: mapping ${folder.name}…`, cancellable: true },
      async (_progress, token) => {
        // Cancelled from the notification by the user, or by stopAll through cancel.
        const link = token.onCancellationRequested(() => cancel.cancel());
        try {
          if (cancel.token.isCancellationRequested) throw new vscode.CancellationError();
          return await start(folder.root, home, log, cancel.token);
        } finally {
          link.dispose();
        }
      },
    );
    const session: Session = { ...folder, ...running };
    sessions.set(folder.root, session);
    // It may still stop on its own - a bad argument, a port taken, the user killing
    // it - and a remembered address that answers nothing is worse than none. The
    // listener goes on before attach, so an exit during attach's requests is heard.
    running.child.on('exit', (code, signal) => {
      if (sessions.get(folder.root) !== session) return;
      end(folder.root);
      // end() closes the tab, so the user is told why. stop and restart remove this
      // listener before killing the server; only an unexpected exit reaches here.
      void vscode.window.showWarningMessage(
        `depphunter for ${folder.name} stopped (${signal ?? `exit code ${code}`}).`, 'Show Log', 'Restart',
      ).then(answer => {
        if (answer === 'Show Log') log.show();
        if (answer === 'Restart') void restart(vscode.Uri.file(folder.root));
      });
    });
    // Awaited, so that the panel is filled by the time the map is on screen rather
    // than a moment afterwards. It is one request to a server on this machine, and
    // it reports its own failures to the log.
    await attach(session);
    refreshStatus();
    return sessions.get(folder.root) === session ? session : undefined;
  } catch (err) {
    if (err instanceof vscode.CancellationError) return undefined;
    await report(err);
    return undefined;
  }
}

// Implements: REQ-EXT-020
async function show(session: Session): Promise<void> {
  const address = await reachable(session.url);
  const where = vscode.workspace.getConfiguration('depphunter', vscode.Uri.file(session.root)).get<string>('openIn');
  if (where === 'externalBrowser') {
    await vscode.env.openExternal(vscode.Uri.parse(address));
    return;
  }
  if (where === 'simpleBrowser') {
    try {
      await vscode.commands.executeCommand('simpleBrowser.show', address);
      return;
    } catch {
      // Not every build of every editor ships the built-in browser.
      log.appendLine('the built-in browser is not available here; opening the map in a tab of its own');
    }
  }
  panel.open(session.root, `depphunter: ${session.name}`, address);
  void check(session.url, address);
}

// Both take the folder the Maps view hands them; from the command palette they ask.
async function restart(resource?: vscode.Uri): Promise<void> {
  const root = resource?.fsPath ?? await pickRunning('Restart which map?');
  if (!root) return;
  // One already starting is let finish, so that it is ended rather than orphaned.
  await starting.get(root)?.done;
  const was = sessions.get(root);
  end(root);
  const session = await launch({ root, name: was?.name ?? (path.basename(root) || root) });
  if (session) await show(session);
}

async function stop(resource?: vscode.Uri): Promise<void> {
  const folder = resource?.fsPath ?? await pickRunning('Stop which map?');
  if (folder) end(folder);
}

// Implements: REQ-EXT-029
function end(root: string): void {
  // The tab goes with the server: what it holds is a page on a port that is about to
  // stop answering, and an error page is worse than no tab.
  panel.close(root);
  if (attached?.root === root) detach();
  const session = sessions.get(root);
  if (!session) return;
  sessions.delete(root);
  session.child.removeAllListeners('exit');
  session.child.kill();
  refreshStatus();
}

// Implements: REQ-EXT-026
function stopAll(): void {
  for (const { cancel } of starting.values()) cancel.cancel();
  for (const root of [...sessions.keys()]) end(root);
}

// Which folder to map: the one that was right-clicked, the only one there is, or the
// one the user says.
// Implements: REQ-EXT-027
async function pick(resource?: vscode.Uri): Promise<{ root: string; name: string } | undefined> {
  if (resource?.scheme === 'file') {
    // A folder in the explorer, which may be one inside a workspace folder rather
    // than the workspace folder itself: mapping a subdirectory is a fair thing to ask.
    return { root: resource.fsPath, name: path.basename(resource.fsPath) || resource.fsPath };
  }
  const folders = vscode.workspace.workspaceFolders ?? [];
  if (folders.length === 0) {
    void vscode.window.showErrorMessage('depphunter maps a folder, and this window has none open.');
    return undefined;
  }
  const chosen = folders.length === 1 ? folders[0] : await vscode.window.showWorkspaceFolderPick();
  return chosen ? { root: chosen.uri.fsPath, name: chosen.name } : undefined;
}

async function pickRunning(prompt: string): Promise<string | undefined> {
  const running = [...sessions.values()];
  if (running.length === 0) {
    void vscode.window.showInformationMessage('No depphunter server is running.');
    return undefined;
  }
  if (running.length === 1) return running[0].root;
  const chosen = await vscode.window.showQuickPick(
    running.map(s => ({ label: s.name, description: s.root })),
    { title: prompt },
  );
  return chosen?.description;
}

// Implements: REQ-EXT-030
function refreshStatus(): void {
  view.refresh();
  const running = [...sessions.values()];
  if (running.length === 0) {
    status.hide();
    return;
  }
  status.text = '$(globe) depphunter';
  status.tooltip = running.map(s => `${s.name}: ${s.url.replace(/\?.*/, '')}`).join('\n');
  status.show();
}

// A server reads its settings once, at startup, so changing them changes nothing
// until it is started again. Say so, rather than leaving the user to wonder.
/** The extension's own settings, from its manifest; empty when it cannot be read. */
let settings: string[] = [];

function settingsOf(manifest: unknown): string[] {
  const configuration = (manifest as { contributes?: { configuration?: unknown } } | undefined)?.contributes?.configuration;
  const sections = (Array.isArray(configuration) ? configuration : [configuration]) as { properties?: object }[];
  return sections.flatMap(section => Object.keys(section?.properties ?? {}));
}

/** Whether a restart prompt is on screen, so that editing settings.json does not stack them. */
let offering = false;

// Implements: REQ-EXT-031
async function offerRestart(e: vscode.ConfigurationChangeEvent): Promise<void> {
  // depphunter.openIn is read each time a map is shown, and depphunter.addToPath only
  // touches the terminals; nothing running needs either.
  const notServer = ['depphunter.openIn', 'depphunter.addToPath'];
  const onlyShown = notServer.some(key => e.affectsConfiguration(key))
    && settings.length > 0 && !settings.some(key => !notServer.includes(key) && e.affectsConfiguration(key));
  const affected = [...sessions.keys()].filter(root => e.affectsConfiguration('depphunter', vscode.Uri.file(root)));
  if (onlyShown || affected.length === 0 || offering) return;
  offering = true;
  try {
    const answer = await vscode.window.showInformationMessage(
      'depphunter settings changed. Restart the server to use them?', 'Restart');
    if (answer !== 'Restart') return;
    // The servers running then, not the ones the change was made under: one may have
    // been stopped, or restarted, while the question was on screen.
    for (const root of affected) {
      if (sessions.has(root)) await restart(vscode.Uri.file(root));
    }
  } finally {
    offering = false;
  }
}

/**
 * The address to put in front of the reader.
 *
 * asExternalUri is what makes this work over a remote or a tunnel - it forwards the
 * port and rewrites the address, since the server listens on the loopback of the
 * extension host's machine and not the browser's. What it does not reliably hand back
 * is the query, which in embed mode is where the session token lives (no cookie
 * survives inside somebody else's frame). An address that lost it loads to
 * "unauthorized" and nothing else, so the token is put back rather than trusted to
 * survive the trip.
 * Implements: REQ-EXT-023
 */
async function reachable(url: string): Promise<string> {
  const token = new URL(url).searchParams.get('token');
  const external = await vscode.env.asExternalUri(vscode.Uri.parse(url));
  if (!token) return external.toString();
  const out = new URL(external.toString(true)); // skipEncoding: keeps '=' and '&' literal
  for (const key of [...out.searchParams.keys()]) {
    if (key === 'token' || key.startsWith('token=')) out.searchParams.delete(key);
  }
  out.searchParams.set('token', token);
  return out.toString();
}

/**
 * Asks the server, once, whether the address actually opens - the map is about to be
 * shown in a frame, where a refusal is a page of text nobody can do anything with and
 * no hint as to why. Here it can be named, in the log, next to the command line that
 * produced it.
 */
async function check(local: string, address: string): Promise<void> {
  // The server is probed on its loopback address, which the extension host can
  // reach; the rewritten address is for the browser's machine, which over a remote or
  // a tunnel is another one. Its path and query are used, since whether the token
  // survived the rewrite is what is being checked.
  if (typeof fetch !== 'function') return; // Node before 18: nothing to ask with
  const probe = new URL(local);
  const shown = new URL(address);
  probe.pathname = shown.pathname;
  probe.search = shown.search;
  try {
    const res = await fetch(probe, { redirect: 'manual' });
    if (res.status === 200) return;
    log.appendLine(`the map answered ${res.status} at ${address.replace(/token=[^&]*/, 'token=...')}`);
    log.appendLine(res.status === 401 || res.status === 303
      ? 'the session token did not reach the page: the server may have been started without --embed, '
        + 'or the address lost its query on the way. The command line above is what was run.'
      : 'see the command line above for what was started.');
  } catch (err) {
    log.appendLine(`the map could not be reached: ${err instanceof Error ? err.message : String(err)}`);
  }
}

async function report(err: unknown): Promise<void> {
  const message = err instanceof Error ? err.message : String(err);
  if (err instanceof StartError && err.notFound) {
    const answer = await vscode.window.showErrorMessage(
      `${message} Install it, or set "depphunter.path" to where it is.`,
      'Get depphunter', 'Open Settings');
    if (answer === 'Get depphunter') await vscode.env.openExternal(vscode.Uri.parse(RELEASES));
    if (answer === 'Open Settings') {
      await vscode.commands.executeCommand('workbench.action.openSettings', 'depphunter.path');
    }
    return;
  }
  const answer = await vscode.window.showErrorMessage(message, 'Show Log');
  if (answer === 'Show Log') log.show();
}
