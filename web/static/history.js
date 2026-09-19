// Git history metrics for the overlay. The server sends raw per-file changes
// ([time, author, added, deleted, commit], newest first; see internal/history), so any
// time range can be evaluated here without another request.

export const MODES = {
  commits: { label: 'Commits', title: 'Commits' },
  churn: { label: 'Lines changed', title: 'Lines changed' },
  age: { label: 'Last change', title: 'Last change' },
  authors: { label: 'Authors', title: 'Authors' },
};

export const isHistoryMode = mode => mode in MODES;

/** Oldest and newest change time in the history (unix seconds). */
export function timeRange(hist) {
  let from = Infinity, to = -Infinity;
  for (const changes of Object.values(hist.files)) {
    if (!changes.length) continue;
    to = Math.max(to, changes[0][0]);
    from = Math.min(from, changes[changes.length - 1][0]);
  }
  return from === Infinity ? null : { from, to };
}

/**
 * Metrics for files and directories from changes at or after `since`. A file's
 * `last` ignores `since`: the age of a file is its latest change ever read.
 * Returns {byId: Map<id, Metric>, max: {commits, churn, authors}, range}.
 * Metric: {commits, churn, authors: Map<authorIdx, commits>, last, first, fileCount}
 * where a directory's commits are distinct commits across its files.
 */
export function computeMetrics(model, hist, since) {
  const byId = new Map();
  const max = { commits: 0, churn: 0, authors: 0 };
  const dirCommits = new Map(); // dir id -> Set of commit indexes

  for (const n of model.byId.values()) {
    if (n.kind !== 'file') continue;
    const changes = hist.files[n.path];
    if (!changes?.length) continue;
    const m = { commits: 0, churn: 0, authors: new Map(), last: changes[0][0], first: Infinity, fileCount: 1 };
    const ids = [];
    for (const [t, author, added, deleted, commit] of changes) {
      if (t < since) break; // newest first
      m.commits++;
      m.churn += added + deleted;
      m.authors.set(author, (m.authors.get(author) || 0) + 1);
      m.first = Math.min(m.first, t);
      ids.push(commit);
    }
    byId.set(n.id, m);
    max.commits = Math.max(max.commits, m.commits);
    max.churn = Math.max(max.churn, m.churn);
    max.authors = Math.max(max.authors, m.authors.size);

    for (let d = n.parentNode; d; d = d.parentNode) {
      let dm = byId.get(d.id);
      if (!dm) {
        dm = { commits: 0, churn: 0, authors: new Map(), last: 0, first: Infinity, fileCount: 0 };
        byId.set(d.id, dm);
        dirCommits.set(d.id, new Set());
      }
      dm.fileCount++;
      dm.churn += m.churn;
      dm.last = Math.max(dm.last, m.last);
      dm.first = Math.min(dm.first, m.first);
      for (const [a, c] of m.authors) dm.authors.set(a, (dm.authors.get(a) || 0) + c);
      const set = dirCommits.get(d.id);
      for (const id of ids) set.add(id);
    }
  }
  for (const [id, set] of dirCommits) byId.get(id).commits = set.size;
  return { byId, max, range: timeRange(hist) };
}

/**
 * Color position t in [0, 1] for a node in a history mode, or null when it has no
 * history in range. Directories (districts) use per-file means for counts, like
 * size mode, so they share the files' scale.
 */
export function historyT(mode, node, metrics) {
  const m = metrics.byId.get(node.id);
  if (!m) return null;
  const perFile = v => (node.kind === 'dir' ? v / Math.max(1, m.fileCount) : v);
  switch (mode) {
    case 'commits':
      return m.commits ? Math.sqrt(Math.min(1, perFile(m.commits) / Math.max(1, metrics.max.commits))) : null;
    case 'churn':
      return m.churn ? Math.sqrt(Math.min(1, perFile(m.churn) / Math.max(1, metrics.max.churn))) : null;
    case 'authors':
      return m.authors.size ? Math.min(1, m.authors.size / Math.max(1, metrics.max.authors)) : null;
    case 'age': {
      const { from, to } = metrics.range;
      // Recent changes are strong; the square root separates the last weeks.
      return 1 - Math.sqrt(Math.min(1, (to - m.last) / Math.max(1, to - from)));
    }
  }
  return null;
}

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

/** "3 days ago", relative to `now` (unix seconds). */
export function ago(t, now = Date.now() / 1000) {
  const s = t - now;
  for (const [unit, secs] of [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60]]) {
    if (Math.abs(s) >= secs) return rtf.format(Math.round(s / secs), unit);
  }
  return rtf.format(0, 'minute');
}

export const formatDate = t => new Date(t * 1000).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
