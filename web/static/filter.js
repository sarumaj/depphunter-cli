// Filters decide what exists on the map; search finds nodes among what is visible.

/**
 * filters: {hiddenLangs: Set<string>, hiddenEcos: Set<string>, path: string}
 * `path` is a comma-separated glob list; plain patterns include, "!pattern" excludes.
 * Returns {visible(node), counts: Map<dirId, {fileCount, totalLoc}>, hiddenFiles}.
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
      if (filters.hiddenEcos.has(e.id) || !used) hidden.add(p.id); else left++;
    }
    if (filters.hiddenEcos.has(e.id) || !left) hidden.add(e.id);
  }
  return { visible, counts, hiddenFiles };
}

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
  for (const it of items) { it.lname = it.name.toLowerCase(); it.lcontext = it.context.toLowerCase(); }
  return items;
}

/** Best `limit` matches for query among items passing `visible`. */
export function search(items, query, visible, limit = 12) {
  const q = query.trim().toLowerCase();
  if (!q) return [];
  const out = [];
  for (const it of items) {
    // A match on the name beats a match on the path; kinds break ties (files first).
    let s = fuzzy(q, it.lname, it.name);
    if (s !== null) s += 40;
    else if ((s = fuzzy(q, it.lcontext, it.context)) === null) continue;
    s += { file: 3, dir: 2, symbol: 1, package: 0 }[it.node.kind];
    if (out.length === limit && s <= out[limit - 1].score) continue;
    if (!visible(it.node)) continue;
    out.push({ ...it, score: s });
    out.sort((a, b) => b.score - a.score || a.context.length - b.context.length);
    if (out.length > limit) out.pop();
  }
  return out;
}

// Subsequence match scored like common fuzzy finders: consecutive runs and matches at
// word boundaries (after / . _ - or a lower→upper case change) earn bonuses, gaps cost.
// Returns null when q is not a subsequence.
function fuzzy(q, lower, orig) {
  let score = 0, ti = 0, prev = -2;
  for (let qi = 0; qi < q.length; qi++) {
    const ch = q[qi];
    const idx = lower.indexOf(ch, ti);
    if (idx < 0) return null;
    const before = orig[idx - 1];
    const boundary = idx === 0 || '/._- '.includes(before) ||
      (before && before === before.toLowerCase() && orig[idx] !== orig[idx].toLowerCase());
    score += 1 + (idx === prev + 1 ? 5 : 0) + (boundary ? 8 : 0) - Math.min(3, idx - ti);
    prev = idx;
    ti = idx + 1;
  }
  if (lower.startsWith(q)) score += 15;
  return score - lower.length * 0.05;
}
