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

import { StartError, start } from './server';

const RELEASES = 'https://github.com/sarumaj/depphunter-cli/releases';

interface Session {
  readonly root: string;
  readonly name: string;
  readonly url: string;
  readonly child: ChildProcess;
}

const sessions = new Map<string, Session>();
let log: vscode.OutputChannel;
let status: vscode.StatusBarItem;

export function activate(context: vscode.ExtensionContext): void {
  log = vscode.window.createOutputChannel('depphunter');
  status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  status.command = 'depphunter.open';
  context.subscriptions.push(log, status, { dispose: stopAll });

  context.subscriptions.push(
    vscode.commands.registerCommand('depphunter.open', (resource?: vscode.Uri) => open(resource)),
    vscode.commands.registerCommand('depphunter.restart', () => restart()),
    vscode.commands.registerCommand('depphunter.stop', () => stop()),
    vscode.commands.registerCommand('depphunter.showLog', () => log.show()),
    vscode.workspace.onDidChangeWorkspaceFolders(e => {
      for (const folder of e.removed) end(folder.uri.fsPath);
    }),
    vscode.workspace.onDidChangeConfiguration(e => {
      if (e.affectsConfiguration('depphunter')) offerRestart();
    }),
  );
}

export function deactivate(): void {
  stopAll();
}

async function open(resource?: vscode.Uri): Promise<void> {
  const folder = await pick(resource);
  if (!folder) return;
  const session = sessions.get(folder.root) ?? await launch(folder);
  if (session) await show(session);
}

async function launch(folder: { root: string; name: string }): Promise<Session | undefined> {
  try {
    const running = await vscode.window.withProgress(
      { location: vscode.ProgressLocation.Notification, title: `depphunter: mapping ${folder.name}…`, cancellable: true },
      (_progress, token) => start(folder.root, log, token),
    );
    const session: Session = { ...folder, ...running };
    sessions.set(folder.root, session);
    // It may still stop on its own - a bad argument, a port taken, the user killing
    // it - and a remembered address that answers nothing is worse than none.
    running.child.on('exit', () => {
      if (sessions.get(folder.root) === session) end(folder.root);
    });
    refreshStatus();
    return session;
  } catch (err) {
    if (err instanceof vscode.CancellationError) return undefined;
    await report(err);
    return undefined;
  }
}

async function show(session: Session): Promise<void> {
  // asExternalUri is what makes this work over a remote or a tunnel: the server
  // listens on the loopback address of the machine the extension host is on, which is
  // not the machine the browser is on, and this forwards the port and rewrites the
  // address. Locally it hands back what it was given.
  const address = (await vscode.env.asExternalUri(vscode.Uri.parse(session.url))).toString();
  const where = vscode.workspace.getConfiguration('depphunter', vscode.Uri.file(session.root)).get<string>('openIn');
  if (where === 'externalBrowser') {
    await vscode.env.openExternal(vscode.Uri.parse(address));
    return;
  }
  try {
    await vscode.commands.executeCommand('simpleBrowser.show', address);
  } catch {
    // Not every build of every editor ships the built-in browser.
    log.appendLine('the built-in browser is not available here; opening the map outside the editor');
    await vscode.env.openExternal(vscode.Uri.parse(address));
  }
}

async function restart(): Promise<void> {
  const root = await pickRunning('Restart which map?');
  if (!root) return;
  const was = sessions.get(root);
  end(root);
  const session = await launch({ root, name: was?.name ?? (path.basename(root) || root) });
  if (session) await show(session);
}

async function stop(): Promise<void> {
  const folder = await pickRunning('Stop which map?');
  if (folder) end(folder);
}

function end(root: string): void {
  const session = sessions.get(root);
  if (!session) return;
  sessions.delete(root);
  session.child.removeAllListeners('exit');
  session.child.kill();
  refreshStatus();
}

function stopAll(): void {
  for (const root of [...sessions.keys()]) end(root);
}

// Which folder to map: the one that was right-clicked, the only one there is, or the
// one the user says.
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

function refreshStatus(): void {
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
async function offerRestart(): Promise<void> {
  if (sessions.size === 0) return;
  const answer = await vscode.window.showInformationMessage(
    'depphunter settings changed. Restart the server to use them?', 'Restart');
  if (answer === 'Restart') await restart();
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
