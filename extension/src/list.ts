// What the side panel's four views have in common.
//
// Each of them is a list the editor asks for rows, and each has to tell the editor
// when to ask again - a server started, a catch made on the map, a new graph. That
// is an event and its disposal, the same four lines in every view, so they are here
// once and the views say only what their rows are.

import * as vscode from 'vscode';

/**
 * A tree view's data provider that owns its change event. `refresh` has the editor ask
 * for every row again; `dispose` lets go of whoever was listening.
 */
export abstract class ListView<T> implements vscode.TreeDataProvider<T> {
  private readonly changed = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  dispose(): void {
    this.changed.dispose();
  }

  abstract getChildren(element?: T): T[];

  abstract getTreeItem(element: T): vscode.TreeItem;
}
