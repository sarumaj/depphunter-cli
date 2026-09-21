// The Maps view in depphunter's own corner of the activity bar: every folder that can
// be mapped, and whether a server is running for it.
//
// Its elements are the folders' URIs, which is also what the explorer hands a command
// when a folder is right-clicked there, so the same open, restart and stop commands
// serve both without either having to know where it was called from.

import * as path from 'node:path';
import * as vscode from 'vscode';

/** What the view needs to know about a running server. */
export interface Running {
  readonly name: string;
  readonly url: string;
}

export class MapsView implements vscode.TreeDataProvider<vscode.Uri> {
  private readonly changed = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly running: () => ReadonlyMap<string, Running>) {}

  refresh(): void {
    this.changed.fire();
  }

  dispose(): void {
    this.changed.dispose();
  }

  /**
   * The workspace folders, then any folder inside one that was mapped from the
   * explorer: a server that is running belongs in the list whether or not it was
   * started for a workspace folder.
   */
  getChildren(element?: vscode.Uri): vscode.Uri[] {
    if (element) return [];
    const folders = (vscode.workspace.workspaceFolders ?? []).map(f => f.uri);
    const listed = new Set(folders.map(u => u.fsPath));
    const others = [...this.running().keys()].filter(root => !listed.has(root)).sort();
    return [...folders, ...others.map(root => vscode.Uri.file(root))];
  }

  getTreeItem(uri: vscode.Uri): vscode.TreeItem {
    const root = uri.fsPath;
    const session = this.running().get(root);
    const folder = vscode.workspace.workspaceFolders?.find(f => f.uri.fsPath === root);
    const item = new vscode.TreeItem(
      session?.name ?? folder?.name ?? (path.basename(root) || root),
      vscode.TreeItemCollapsibleState.None);
    item.id = root;
    item.contextValue = session ? 'running' : 'stopped';
    item.iconPath = new vscode.ThemeIcon(session ? 'globe' : 'folder');
    // The address is shown without its query: the session token is in there.
    const address = session?.url.replace(/\?.*/, '');
    item.description = address ? `running · ${address.replace(/^https?:\/\//, '').replace(/\/$/, '')}` : '';
    item.tooltip = address ? `${root}\n${address}` : root;
    item.command = { command: 'depphunter.open', title: 'Open the Map', arguments: [uri] };
    return item;
  }
}
