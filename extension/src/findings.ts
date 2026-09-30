// Every finding the scanners reported, beside the backpack, and a way to catch each one
// from here.
//
// The backpack view lists what was caught on the map. What was not caught yet is the
// rest of the scanners' document, and walking the map up to a bug is not the only way
// to decide that a finding is worth coming back to - so this lists all of them, and
// each one that is not in the backpack yet has a button that puts it there.
//
// Putting it there is the map's own catch. The page is the one that owns the backpack
// (web/static/backpack.js keeps it in the browser's store), and a catch there records
// a finding against the building its bug stands at (web/static/findings.js place).
// This side cannot run those modules, so placeFinding and packItemFor below say the
// same thing in TypeScript, and extension/test/findings.test.js holds them to the
// page's modules on the same graph: an entry caught here is the entry a catch on the
// map would have made. It goes up the way a removal from the backpack view does
// (PUT /api/backpack), and the map takes it from the server's announcement.

import * as vscode from 'vscode';

import { Finding, PackItem } from './api';
import { rank, severityIcon } from './backpack';
import { ListView } from './list';

/** The node a finding's bug stands at: its package, else its file, else the nearest directory on the map, else the repository. */
export function placeFinding(f: Finding, has: (id: string) => boolean): string {
  if (f.package) {
    const id = `p:${f.ecosystem}:${f.package}`;
    if (has(id)) return id;
  }
  if (f.path) {
    const id = `f:${f.path}`;
    if (has(id)) return id;
    // A path the map does not draw - a lock file, a vendored copy - is still about
    // somewhere: the nearest directory that is on the map takes it.
    let directory = f.path;
    for (let i = 0; i < 64; i++) {
      const cut = directory.lastIndexOf('/');
      directory = cut < 0 ? '.' : directory.slice(0, cut);
      if (has(`d:${directory}`)) return `d:${directory}`;
      if (directory === '.') break;
    }
  }
  return 'd:.';
}

/** The backpack entry a catch of `f` at `nodeId` makes, field for field what the map's Backpack.add writes. */
export function packItemFor(f: Finding, nodeId: string, now = Date.now()): PackItem {
  return {
    id: f.id,
    severity: f.severity || 'unknown',
    title: f.title || f.id,
    where: f.package || f.path || '',
    line: f.line || 0,
    nodeId,
    caughtAt: now,
    fixed: false,
    fixedAt: 0,
  };
}

/** A package advisory by its package and version, anything else by its file. */
export function nameOf(f: Finding): string {
  if (f.package) return f.version ? `${f.package}@${f.version}` : f.package;
  return f.path || 'this repository';
}

// Implements: REQ-EXT-035, REQ-EXT-036
export class FindingsView extends ListView<Finding> {
  private items: Finding[] = [];
  private caught = new Set<string>();

  /**
   * `has` answers whether the graph on show has a node, which is what a finding is
   * placed by; nothing is placed until one is.
   */
  constructor(private readonly has: (id: string) => boolean = () => false) {
    super();
  }

  setFindings(items: Finding[]): void {
    this.items = items;
    this.refresh();
  }

  /** The ids in the backpack, which are the rows shown as caught. */
  setCaught(ids: Iterable<string>): void {
    const next = new Set(ids);
    if (next.size === this.caught.size && [...next].every(id => this.caught.has(id))) return;
    this.caught = next;
    this.refresh();
  }

  get contents(): Finding[] {
    return this.items;
  }

  isCaught(id: string): boolean {
    return this.caught.has(id);
  }

  /** The node the finding's bug stands at on the map, for selecting it and for the entry a catch makes. */
  nodeOf(f: Finding): string {
    return placeFinding(f, this.has);
  }

  // Worst first, then by name, as the map's list has them.
  getChildren(element?: Finding): Finding[] {
    if (element) return [];
    const text = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
    const key = (f: Finding) => (f.package || f.path || '').toLowerCase();
    return [...this.items].sort((a, b) =>
      rank(b.severity) - rank(a.severity) ||
      text(key(a), key(b)) ||
      text(a.title || '', b.title || '') ||
      text(a.id, b.id));
  }

  getTreeItem(f: Finding): vscode.TreeItem {
    const caught = this.caught.has(f.id);
    const item = new vscode.TreeItem(nameOf(f), vscode.TreeItemCollapsibleState.None);
    item.id = `finding:${f.id}`;
    const where = f.package ? f.path : f.line ? `line ${f.line}` : '';
    item.description = [f.ref && f.ref !== f.title ? `${f.ref} ${f.title}` : f.title, where, caught ? 'in the backpack' : f.severity || 'unknown']
      .filter(Boolean).join(' · ');
    item.iconPath = severityIcon(f.severity, caught);
    // `caught` offers taking it out again, `uncaught` putting it in (package.json menus).
    item.contextValue = caught ? 'caught' : 'uncaught';
    item.tooltip = new vscode.MarkdownString(
      `**${f.severity || 'unknown'}** ${f.title}\n\n\`${f.ref || f.id}\` · ${nameOf(f)}${f.source ? `\n\nreported by ${f.source}` : ''}` +
      (caught ? '\n\nIn the backpack.' : ''));
    item.command = {
      command: 'depphunter.showFinding', title: 'Show on the Map',
      arguments: [packItemFor(f, this.nodeOf(f))],
    };
    return item;
  }
}
