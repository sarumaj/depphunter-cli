// What has been caught, beside the code rather than inside the map.
//
// A finding caught in walk mode stays in the backpack until the scanners stop
// reporting it, and the browser keeps it, since it has to outlive a server that runs
// only while somebody is looking. The server holds this session's copy
// (internal/server/session.go); this is a view of that, so the catch can be worked
// through in the editor, where the fixing happens.

import * as vscode from 'vscode';

import { PackItem } from './api';
import { ListView } from './list';

// Implements: REQ-EXT-010
export class BackpackView extends ListView<PackItem> {
  private items: PackItem[] = [];

  setItems(items: PackItem[]): void {
    this.items = items;
    this.refresh();
  }

  get contents(): PackItem[] {
    return this.items;
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
    // Gone from the latest scan. It stays in the backpack until it is cleared,
    // because seeing what you caught turn green is the point of having caught it.
    item.iconPath = severityIcon(it.severity, !!it.fixed);
    item.contextValue = 'finding';
    item.tooltip = new vscode.MarkdownString(
      `**${it.severity || 'unknown'}** ${it.title}\n\n\`${it.id}\`${it.where ? `\n\n${it.where}` : ''}`);
    item.command = { command: 'depphunter.showFinding', title: 'Show on the Map', arguments: [it] };
    return item;
  }
}

const NOTE = { icon: 'info', color: 'foreground' };
const WARNING = { icon: 'warning', color: 'list.warningForeground' };
const ERROR = { icon: 'error', color: 'list.errorForeground' };

/**
 * Every severity a scanner reports, by its lowercase name: where it ranks (worst
 * highest), and the icon and color its rows are drawn with.
 */
const SEVERITY: Record<string, { rank: number; icon: string; color: string }> = {
  unknown: { rank: 0, ...NOTE },
  info: { rank: 1, ...NOTE },
  low: { rank: 2, ...WARNING },
  medium: { rank: 3, ...WARNING },
  moderate: { rank: 4, ...WARNING },
  high: { rank: 5, ...ERROR },
  critical: { rank: 6, ...ERROR },
};

/** A severity's entry, whatever its case; none for one no scanner here reports. No severity at all is unknown. */
function severityOf(severity: string): (typeof SEVERITY)[string] | undefined {
  const name = (severity || 'unknown').toLowerCase();
  return Object.hasOwn(SEVERITY, name) ? SEVERITY[name] : undefined;
}

/** Where a severity ranks, worst highest; below every known one (-1) when it is not known. */
export const rank = (severity: string) => severityOf(severity)?.rank ?? -1;

/** The codicon a severity is drawn with. */
export function icon(severity: string): string {
  return severityOf(severity)?.icon ?? NOTE.icon;
}

/** The theme color of that icon. */
export function color(severity: string): string {
  return severityOf(severity)?.color ?? NOTE.color;
}

/**
 * A finding's icon in a list: its severity's, or a green tick once it is `done` with -
 * fixed since it was caught, or in the backpack already.
 */
export function severityIcon(severity: string, done: boolean): vscode.ThemeIcon {
  return done
    ? new vscode.ThemeIcon('pass', new vscode.ThemeColor('testing.iconPassed'))
    : new vscode.ThemeIcon(icon(severity), new vscode.ThemeColor(color(severity)));
}
