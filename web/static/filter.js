// Filters decide what exists on the map; search finds nodes among what is visible.

// Implements: REQ-DIST-017
import { Fzf, byLengthAsc } from './vendor/fzf.es.js';

/**
 * filters: {hiddenLangs: Set<string>, hiddenEcosystems: Set<string>, path: string}
 * `path` is a comma-separated glob list; plain patterns include, "!pattern" excludes.
 * Returns {visible(node), counts: Map<dirId, {fileCount, totalLoc}>, hiddenFiles}.
 * Implements: REQ-MAP-032, REQ-MAP-033, REQ-MAP-035, REQ-MAP-059
 */
export function computeVisibility(model, filters) {
  const { include, exclude } = parsePathFilter(filters.path);
  const hidden = new Set();
  const counts = new Map();
  let hiddenFiles = 0;

  const fileVisible = f =>
    !filters.hiddenLangs.has(f.lang || '') &&
    (!include.length || include.some(m => m(f.path))) &&
    !exclude.some(m => m(f.path));

  const walk = n => {
    if (n.kind === 'file') {
      if (fileVisible(n)) return { fileCount: 1, totalLoc: n.loc || 0 };
      hidden.add(n.id);
      hiddenFiles++;
      return null;
    }
    const c = { fileCount: 0, totalLoc: 0 };
    for (const ch of n.children) {
      if (ch.kind !== 'dir' && ch.kind !== 'file') continue;
      const r = walk(ch);
      if (r) { c.fileCount += r.fileCount; c.totalLoc += r.totalLoc; }
    }
    counts.set(n.id, c);
    if (!c.fileCount && n !== model.root) { hidden.add(n.id); return null; }
    return c;
  };
  walk(model.root);
  const visible = n => {
    for (let p = n; p; p = p.parentNode) if (hidden.has(p.id)) return false;
    return true;
  };
  // A package stays only while a visible file imports it; an island with no packages left sinks.
  for (const e of model.ecosystems) {
    let left = 0;
    for (const p of e.children) {
      const used = (model.edgesTo.get(p.id) || []).some(edge => visible(model.byId.get(edge.from)));
      if (filters.hiddenEcosystems.has(e.id) || !used) hidden.add(p.id); else left++;
    }
    if (filters.hiddenEcosystems.has(e.id) || !left) hidden.add(e.id);
  }
  return { visible, counts, hiddenFiles };
}

// Implements: REQ-MAP-034
export function parsePathFilter(text) {
  const include = [], exclude = [];
  for (let p of (text || '').split(',').map(s => s.trim()).filter(Boolean)) {
    const neg = p.startsWith('!');
    if (neg) p = p.slice(1).trim();
    if (p) (neg ? exclude : include).push(globMatcher(p));
  }
  return { include, exclude };
}

/**
 * gitignore-flavoured globs: `*` and `?` stay within a path segment, `**` spans
 * segments; a pattern without "/" matches the file name or any directory name.
 * Implements: REQ-MAP-034
 */
export function globMatcher(glob) {
  const anchored = glob.includes('/');
  const re = new RegExp('^' + glob.replace(/^\//, '')
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*\*\//g, '\u0000')     // "**/" : any number of leading directories
    .replace(/\*\*/g, '\u0001')       // "**"  : anything
    .replace(/\*/g, '[^/]*')
    .replace(/\?/g, '[^/]')
    .replace(/\u0000/g, '(?:.*/)?')
    .replace(/\u0001/g, '.*') + '$');
  return anchored ? p => re.test(p) : p => p.split('/').some(seg => re.test(seg));
}

// ---------------------------------------------------------------- fuzzy search

/**
 * A fuzzy finder (fzf's algorithm, via fzf-for-js) over files, directories, symbols
 * and packages. Each entry is matched as "name path", so a query can name a symbol,
 * a file or a directory.
 * Implements: REQ-MAP-031
 */
export function searchIndex(model) {
  const items = [];
  for (const n of model.byId.values()) {
    switch (n.kind) {
      case 'file': items.push({ node: n, name: n.name, context: n.path }); break;
      case 'dir': if (n !== model.root) items.push({ node: n, name: n.name + '/', context: n.path + '/' }); break;
      case 'symbol': items.push({ node: n, name: n.name, context: n.parentNode.path }); break;
      case 'package': items.push({ node: n, name: n.name, context: n.parentNode.name }); break;
    }
  }
  return new Fzf(items, {
    selector: it => (it.node.kind === 'file' || it.node.kind === 'dir' ? it.context : `${it.name} ${it.context}`),
    tiebreakers: [byLengthAsc],
    // v1 is linear-time; the default v2 is slower on the 100k+ entries of big repositories.
    fuzzy: items.length > 50_000 ? 'v1' : 'v2',
  });
}

/** The best `limit` matches for query among entries passing `visible`. */
export function search(finder, query, visible, limit = 12) {
  const out = [];
  if (!query.trim()) return out;
  for (const r of finder.find(query.trim())) {
    if (!visible(r.item.node)) continue;
    out.push(r.item);
    if (out.length === limit) break;
  }
  return out;
}
