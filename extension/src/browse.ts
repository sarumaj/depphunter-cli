// What a package points at, opened from the editor: its page and its repository in
// the system's browser, and the folder it is installed in.

import * as path from 'node:path';
import * as vscode from 'vscode';

/**
 * Opens a page in the system's browser. Anything but http(s) is refused: what is
 * opened comes from the graph, which a repository's manifests wrote.
 *
 * Implements: REQ-EXT-037
 */
export async function openLink(url: string | undefined): Promise<boolean> {
  if (!url || !/^https?:\/\//i.test(url)) return false;
  return vscode.env.openExternal(vscode.Uri.parse(url));
}

const NEW_WINDOW = 'Open in New Window';

/** What the system's file manager is called where the editor runs. */
function fileManager(): string {
  if (process.platform === 'darwin') return 'Reveal in Finder';
  if (process.platform === 'win32') return 'Reveal in File Explorer';
  return 'Open Containing Folder';
}

/**
 * Shows the folder a package is installed in. One inside the workspace - a
 * node_modules, a vendor directory - is revealed in the Explorer; one outside it, in
 * a shared cache, can be opened in a window of its own or shown in the system's file
 * manager. It is not offered as a workspace folder: making a window of one folder a
 * workspace of two restarts the extensions, and with them every map.
 *
 * Implements: REQ-EXT-037
 */
export async function revealFolder(folder: string, name: string): Promise<void> {
  const uri = vscode.Uri.file(folder);
  const inside = (vscode.workspace.workspaceFolders ?? []).some(f => {
    const relative = path.relative(f.uri.fsPath, folder);
    return !relative.startsWith('..') && !path.isAbsolute(relative);
  });
  if (inside) {
    await vscode.commands.executeCommand('revealInExplorer', uri);
    return;
  }
  const answer = await vscode.window.showInformationMessage(
    `${name} is installed in ${folder}, outside the workspace.`, NEW_WINDOW, fileManager());
  if (answer === NEW_WINDOW) {
    await vscode.commands.executeCommand('vscode.openFolder', uri, { forceNewWindow: true });
  } else if (answer) {
    await vscode.commands.executeCommand('revealFileInOS', uri);
  }
}
