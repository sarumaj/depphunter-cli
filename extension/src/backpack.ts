// What has been caught, beside the code rather than inside the map.
//
// The backpack is the map's: a finding caught in walk mode goes in it and stays until
// the scanners stop reporting it, and the browser is where it is kept, because it has
// to outlive a server that only runs while somebody is looking. What the server holds
// is this session's copy (internal/server/session.go), and this is a view of that -
// so what was caught while walking can be worked through here, in the editor, which
// is where the fixing happens.

import * as vscode from 'vscode';

import { PackItem } from './api';

export class BackpackView implements vscode.TreeDataProvider<PackItem> {
  private readonly changed = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.changed.event;
  private items: PackItem[] = [];

  setItems(items: PackItem[]): void {
    this.items = items;
    this.changed.fire();
  }

  get contents(): PackItem[] {
    return this.items;
  }

  dispose(): void {
    this.changed.dispose();
  }

  getChildren(element?: PackItem): PackItem[] {
    // Worst first, and the ones that are done with at the bottom: the order they
    // would be worked through in.
    return element ? [] : [...this.items].sort((a, b) =>
      Number(a.fixed ?? false) - Number(b.fixed ?? false) ||
      rank(b.severity) - rank(a.severity) ||
      (b.caughtAt ?? 0) - (a.caughtAt ?? 0));
  }

  getTreeItem(it: PackItem): vscode.TreeItem {
    const item = new vscode.TreeItem(it.title || it.id, vscode.TreeItemCollapsibleState.None);
    item.id = it.id;
    item.description = [it.where && it.line ? `${it.where}:${it.line}` : it.where, it.fixed ? 'fixed' : it.severity]
      .filter(Boolean).join(' · ');
    item.iconPath = it.fixed
      // Gone from the latest scan. It stays in the backpack until it is cleared,
      // because seeing what you caught turn green is the point of having caught it.
      ? new vscode.ThemeIcon('pass', new vscode.ThemeColor('testing.iconPassed'))
      : new vscode.ThemeIcon(icon(it.severity), new vscode.ThemeColor(color(it.severity)));
    item.contextValue = 'finding';
    item.tooltip = new vscode.MarkdownString(
      `**${it.severity || 'unknown'}** ${it.title}\n\n\`${it.id}\`${it.where ? `\n\n${it.where}` : ''}`);
    item.command = { command: 'depphunter.showFinding', title: 'Show on the Map', arguments: [it] };
    return item;
  }
}

const ORDER = ['unknown', 'info', 'low', 'medium', 'moderate', 'high', 'critical'];
const rank = (s: string) => ORDER.indexOf((s || 'unknown').toLowerCase());

function icon(severity: string): string {
  switch ((severity || '').toLowerCase()) {
    case 'critical':
    case 'high':
      return 'error';
    case 'medium':
    case 'moderate':
    case 'low':
      return 'warning';
    default:
      return 'info';
  }
}

function color(severity: string): string {
  switch ((severity || '').toLowerCase()) {
    case 'critical':
    case 'high':
      return 'list.errorForeground';
    case 'medium':
    case 'moderate':
    case 'low':
      return 'list.warningForeground';
    default:
      return 'foreground';
  }
}
