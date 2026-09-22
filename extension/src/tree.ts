// The dependency tree, in the panel the activity-bar icon unfolds.
//
// The map answers "what does this look like"; a tree answers "what is under this",
// and the second question is the one asked with a keyboard while reading code. It is
// the same graph either way - directories hold files, files import packages and other
// files, packages depend on packages - so this draws the one document the server
// serves and nothing of its own.
//
// Opening a row costs nothing: every edge is already in the graph the panel fetched
// once. A branch that leads back to something already open on it is shown once more,
// marked, and left closed - dependency graphs contain cycles, and a tree that
// followed one would not end.
//
// Picking a row is the same act as clicking a building: it is sent to the server, and
// the map follows. A selection made on the map comes back the same way and is
// revealed here.

import * as vscode from 'vscode';

import { Graph, GraphNode } from './api';

/** One row: a graph node in one place in the tree, which is not the only place. */
export interface Row {
  readonly node: GraphNode;
  readonly parent?: Row;
  /** This node is already open further up this branch, so it is a leaf here. */
  readonly cycle?: boolean;
}

/** The graph, indexed for the questions a tree asks of it. */
export class Model {
  readonly byId = new Map<string, GraphNode>();
  private readonly children = new Map<string, GraphNode[]>();
  private readonly depends = new Map<string, GraphNode[]>();
  private readonly imports = new Map<string, GraphNode[]>();
  readonly roots: GraphNode[] = [];

  constructor(readonly graph: Graph) {
    for (const n of graph.nodes) this.byId.set(n.id, n);
    for (const n of graph.nodes) {
      // Symbols are what is inside a file, not something it depends on, and a tree of
      // them is what the editor's own outline is for.
      if (n.kind === 'symbol') continue;
      if (!n.parent) this.roots.push(n);
      else push(this.children, n.parent, n);
    }
    for (const e of graph.edges) {
      const to = this.byId.get(e.to);
      if (!to || to.kind === 'symbol') continue;
      // An edge out of a symbol belongs to the file that holds it: the tree has no
      // row for the symbol, and the import is the file's either way.
      const from = this.byId.get(e.from);
      const owner = from?.kind === 'symbol' ? from.parent ?? e.from : e.from;
      if (e.kind === 'import') push(this.imports, owner, to);
      else if (e.kind === 'depends') push(this.depends, owner, to);
    }
    for (const map of [this.children, this.depends, this.imports]) {
      for (const list of map.values()) dedupe(list);
    }
    sort(this.roots);
  }

  /** What sits under a node: what it holds, or what it depends on. */
  childrenOf(id: string): GraphNode[] {
    const node = this.byId.get(id);
    if (!node) return [];
    switch (node.kind) {
      case 'dir':
      case 'ecosystem':
        return this.children.get(id) ?? [];
      case 'file':
        return this.imports.get(id) ?? [];
      case 'package':
        return this.depends.get(id) ?? [];
      default:
        return [];
    }
  }

  /** The chain from a root down to a node, as the tree holds it. */
  rowFor(id: string): Row | undefined {
    const chain: GraphNode[] = [];
    for (let n = this.byId.get(id); n; n = n.parent ? this.byId.get(n.parent) : undefined) {
      // A symbol has no row; its file is as close as the tree gets.
      if (n.kind !== 'symbol') chain.unshift(n);
      if (chain.length > 64) return undefined; // a parent chain that loops
    }
    let row: Row | undefined;
    for (const node of chain) row = row ? { node, parent: row } : { node };
    return row;
  }
}

export class DependencyTree implements vscode.TreeDataProvider<Row> {
  private readonly changed = new vscode.EventEmitter<Row | undefined>();
  readonly onDidChangeTreeData = this.changed.event;
  private model: Model | undefined;

  /** Replaces what the tree draws; undefined empties it (no server running). */
  setGraph(graph: Graph | undefined): void {
    this.model = graph ? new Model(graph) : undefined;
    this.changed.fire(undefined);
  }

  get graphModel(): Model | undefined {
    return this.model;
  }

  dispose(): void {
    this.changed.dispose();
  }

  getChildren(element?: Row): Row[] {
    if (!this.model) return [];
    if (!element) return this.model.roots.map(node => ({ node }));
    if (element.cycle) return [];
    const open = new Set<string>();
    for (let r: Row | undefined = element; r; r = r.parent) open.add(r.node.id);
    return sort([...this.model.childrenOf(element.node.id)])
      .map(node => ({ node, parent: element, cycle: open.has(node.id) }));
  }

  getParent(element: Row): Row | undefined {
    return element.parent;
  }

  getTreeItem(row: Row): vscode.TreeItem {
    const n = row.node;
    const leaf = row.cycle || this.model?.childrenOf(n.id).length === 0;
    const item = new vscode.TreeItem(n.name || n.id,
      leaf ? vscode.TreeItemCollapsibleState.None : vscode.TreeItemCollapsibleState.Collapsed);
    item.iconPath = icon(n);
    item.description = description(n, !!row.cycle);
    item.tooltip = tooltip(n, !!row.cycle);
    item.contextValue = n.kind;
    item.command = { command: 'depphunter.select', title: 'Show on the Map', arguments: [row] };
    return item;
  }
}

function icon(n: GraphNode): vscode.ThemeIcon {
  switch (n.kind) {
    case 'dir':
      return new vscode.ThemeIcon('folder');
    case 'file':
      return new vscode.ThemeIcon('file-code');
    case 'ecosystem':
      return new vscode.ThemeIcon('globe');
    default:
      // A package nothing here vouches for, or one that is not fixed to a version,
      // is the one worth picking out of a list of a hundred.
      if (n.indexUnknown || n.unresolved) return new vscode.ThemeIcon('warning');
      if (n.floating) return new vscode.ThemeIcon('package', new vscode.ThemeColor('list.warningForeground'));
      return new vscode.ThemeIcon('package');
  }
}

function description(n: GraphNode, cycle: boolean): string {
  const parts: string[] = [];
  if (n.version) parts.push(n.version);
  if (n.kind === 'file' && n.loc) parts.push(`${n.loc} lines`);
  if (n.transitive) parts.push('transitive');
  if (n.floating) parts.push('floating');
  if (n.private) parts.push('private');
  if (cycle) parts.push('↻ already above');
  return parts.join(' · ');
}

function tooltip(n: GraphNode, cycle: boolean): vscode.MarkdownString {
  const md = new vscode.MarkdownString();
  md.appendMarkdown(`**${n.name}**\n\n`);
  const row = (k: string, v: string | undefined) => v && md.appendMarkdown(`${k}: \`${v}\`\n\n`);
  row('Path', n.path);
  row('Language', n.lang);
  row('Version', n.version);
  row('Requested', n.requested);
  row('Index', n.index?.replace(/^https?:\/\//, ''));
  if (n.private) {
    md.appendMarkdown('Yours: never named to a public index, never sent to the vulnerability database\n\n');
  }
  if (n.indexUnknown) md.appendMarkdown('⚠ nothing on this machine configures that index\n\n');
  if (n.unresolved) md.appendMarkdown('⚠ no manifest declares it\n\n');
  if (n.floating) md.appendMarkdown('⚠ not pinned to one version\n\n');
  if (cycle) md.appendMarkdown('Already open further up this branch, so it stops here.\n\n');
  return md;
}

function push<T>(map: Map<string, T[]>, key: string, value: T): void {
  const list = map.get(key);
  if (list) list.push(value);
  else map.set(key, [value]);
}

function dedupe(list: GraphNode[]): void {
  const seen = new Set<string>();
  let kept = 0;
  for (const n of list) {
    if (seen.has(n.id)) continue;
    seen.add(n.id);
    list[kept++] = n;
  }
  list.length = kept;
}

// Directories, then files, then everything external, and alphabetically within each -
// the order a file tree is read in.
const RANK: Record<string, number> = { dir: 0, file: 1, ecosystem: 2, package: 3, symbol: 4 };

function sort(nodes: GraphNode[]): GraphNode[] {
  return nodes.sort((a, b) =>
    (RANK[a.kind] ?? 9) - (RANK[b.kind] ?? 9) || a.name.localeCompare(b.name));
}
