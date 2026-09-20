// Side panel describing the selected node: stats, dependencies, dependents, source.

import hljs from './vendor/highlight.min.js';
import powershell from './vendor/highlight-powershell.min.js';

hljs.registerLanguage('powershell', powershell);
import { ancestors, boundaryEdges } from './model.js';
import { fetchSource } from './data.js';
import { ago, formatDate } from './history.js';
import { fmt, h, escapeHTML } from './dom.js';

// indexHost keeps the part of an index URL that identifies it on a stat tile.
const indexHost = url => url.replace(/^https?:\/\//, '').replace(/\/.*$/, '');

const HLJS = {
  Go: 'go', JavaScript: 'javascript', TypeScript: 'typescript', Python: 'python', Rust: 'rust',
  Java: 'java', Kotlin: 'kotlin', 'C#': 'csharp', C: 'c', 'C++': 'cpp', Ruby: 'ruby', PHP: 'php',
  Shell: 'bash', YAML: 'yaml', JSON: 'json', Markdown: 'markdown', HTML: 'xml', XML: 'xml', CSS: 'css',
  SQL: 'sql', Swift: 'swift', Lua: 'lua', Make: 'makefile', Perl: 'perl', R: 'r', Scala: 'scala',
  'Objective-C': 'objectivec', TOML: 'ini', GraphQL: 'graphql', Docker: 'dockerfile', PowerShell: 'powershell',
};
const MAX_HIGHLIGHT = 300_000; // bytes; larger files are shown as plain text


export class Panel {
  constructor(root, body, { model, colorOf, onSelect, onOpen, openLabel, historyOf, linkKind, onClose }) {
    Object.assign(this, { root, body, model, colorOf, onSelect, onOpen, openLabel, historyOf, linkKind, onClose });
    this.seq = 0;
    // Which branches of the dependency trees are open, by direction and path, so a
    // live update redraws the panel without closing what the reader opened.
    this.open = new Set();
  }

  close() {
    this.root.hidden = true;
    this.root.parentElement.classList.remove('panel-open');
    this.node = null;
    this.onClose?.();
  }

  /** keepScroll: stay where the reader was (a live update re-showing the same node). */
  show(node, keepScroll = false) {
    const top = keepScroll && this.node?.id === node.id ? this.body.scrollTop : 0;
    this.node = node;
    this.root.hidden = false;
    this.root.parentElement.classList.add('panel-open');
    const seq = ++this.seq;
    this.body.replaceChildren(...[
      this.crumbs(node),
      h('h2', { class: 'p-title' }, node.kind === 'file' ? h('span', { class: 'swatch', style: `background:${this.colorOf(node.lang)}` }) : null,
        node.name, h('span', { class: 'badge' }, node.symbolKind || node.kind),
        node.unresolved ? h('span', { class: 'badge warn', title: 'Not found in any manifest' }, '⚠ unresolved') : null,
        node.floating ? h('span', { class: 'badge warn', title: 'Not fixed to one version: it moves when installed again' }, '⚠ floating') : null,
        node.transitive ? h('span', { class: 'badge', title: 'No file here imports it: a dependency pulled it in' }, 'transitive') : null,
        node.indexUnknown ? h('span', { class: 'badge warn', title: 'Only this repository names this index; nothing on your machine does' }, '⚠ index') : null),
      this.openButton(node),
      this.stats(node),
      node.kind === 'dir' ? this.languageMix(node) : null,
      ...this.dependencies(node),
      this.history(node),
    ].filter(Boolean));
    this.body.scrollTop = top;
    if (node.kind === 'file' || node.kind === 'symbol') this.source(node, seq, top);
  }

  /** A list row or breadcrumb: clickable, and reachable by keyboard. */
  item(tag, attrs, ...children) {
    const go = attrs.onclick;
    return h(tag, {
      ...attrs, tabindex: '0', role: 'button',
      onkeydown: e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); go(); } },
    }, ...children);
  }

  openButton(node) {
    const file = node.kind === 'symbol' ? node.parentNode : node;
    if (file.kind !== 'file' || !this.openLabel) return null;
    return h('div', { class: 'p-actions' },
      h('button', { onclick: () => this.onOpen(file.path, node.line || 1), title: `${this.openLabel} (O)` }, this.openLabel + ' ↗'));
  }

  crumbs(node) {
    const parts = ancestors(node).map(a => this.item('a', { onclick: () => this.onSelect(a) }, a.name));
    const out = [];
    parts.forEach((p, i) => { if (i) out.push(' / '); out.push(p); });
    return h('div', { class: 'crumbs' }, out);
  }

  stats(n) {
    const stat = (v, k) => h('div', { class: 'stat' }, h('div', { class: 'v' }, v), h('div', { class: 'k' }, k));
    switch (n.kind) {
      case 'dir':
        return h('div', { class: 'stats' },
          stat(fmt.format(n.fileCount), 'files'), stat(fmt.format(n.totalLoc), 'lines'),
          stat(fmt.format(n.children.filter(c => c.kind === 'dir').length), 'sub-directories'));
      case 'file':
        return h('div', { class: 'stats' },
          stat(fmt.format(n.loc || 0), 'lines'), stat(n.lang || 'unknown', 'language'),
          stat(fmt.format(n.children.length), 'symbols'));
      case 'symbol':
        return h('div', { class: 'stats' }, stat(n.symbolKind, 'kind'), stat(`line ${n.line}`, 'defined at'));
      case 'package':
        return h('div', { class: 'stats' },
          stat(n.version || '-', n.floating ? 'version (floating)' : 'version'),
          n.requested ? stat(n.requested, 'requested') : null,
          stat(fmt.format(n.importers), 'importing files'),
          stat(n.parentNode?.name || '', 'ecosystem'),
          n.index ? stat(indexHost(n.index), n.indexUnknown ? 'index (unknown here)' : 'index') : null);
      case 'ecosystem':
        return h('div', { class: 'stats' }, stat(fmt.format(n.children.length), 'packages'));
    }
    return null;
  }

  languageMix(n) {
    const total = n.totalLoc || 1;
    const rows = [...n.langLoc.entries()].sort((a, b) => b[1] - a[1]);
    const top = rows.slice(0, 7), rest = rows.slice(7).reduce((a, [, v]) => a + v, 0);
    if (rest) top.push(['Other', rest]);
    const color = l => l === 'Other' ? this.colorOf(null) : this.colorOf(l);
    return h('div', { class: 'p-section' },
      h('h4', {}, 'Languages ', h('span', { class: 'n' }, 'by lines')),
      h('div', { class: 'bar', role: 'img', 'aria-label': 'Language breakdown' },
        top.filter(([, v]) => v > 0).map(([l, v]) => h('div', { style: `flex:${v};background:${color(l)}`, title: `${l || 'unknown'}: ${fmt.format(v)} lines` }))),
      h('div', { class: 'bar-legend' },
        top.map(([l, v]) => h('span', {}, h('i', { class: 'swatch', style: `background:${color(l)}` }), `${l || 'unknown'} ${Math.round(v / total * 100)}%`))),
    );
  }

  // neighbors are what a node depends on ('out') or what depends on it ('in'),
  // grouped per node so a file importing the same package twice is one row.
  neighbors(node, dir) {
    const { out, in: inc } = boundaryEdges(this.model, node, this.linkKind());
    const edges = dir === 'out' ? out : inc;
    const key = dir === 'out' ? 'to' : 'from';
    const m = new Map();
    for (const e of edges) {
      const node = this.model.byId.get(e[key]);
      const g = m.get(node.id) || { node, count: 0, line: e.line };
      g.count++;
      m.set(node.id, g);
    }
    return [...m.values()].sort((a, b) => b.count - a.count || a.node.name.localeCompare(b.node.name));
  }

  // tree renders one direction as rows that open into the next: a dependency's own
  // dependencies, and theirs, which is the shape a supply chain has once versions are
  // pinned and transitive packages are on the map. Nothing is fetched - the edges are
  // in the model - so opening a row costs only layout.
  tree(title, hint, root, dir) {
    const groups = this.neighbors(root, dir);
    const ul = h('ul', { class: 'p-list tree' });
    this.insertRows(ul, null, groups, dir, [root.id], 0);
    return h('div', { class: 'p-section' },
      h('h4', {}, h('span', { class: 'swatch', style: `background:${hint}` }), title,
        h('span', { class: 'n' }, fmt.format(groups.length))),
      groups.length ? ul : h('div', { class: 'empty' }, 'None'));
  }

  // insertRows puts one level of rows after `after` (or at the end of the list).
  insertRows(ul, after, groups, dir, ancestors, depth) {
    let anchor = after;
    for (const g of groups) {
      const row = this.treeRow(ul, g, dir, ancestors, depth);
      if (anchor) anchor.after(row);
      else ul.append(row);
      anchor = row;
      // Only now is the row in the list, so its own children have somewhere to go:
      // this is what reopens the branches a redraw inherited.
      if (row.reopen) {
        row.reopen();
        anchor = ul.lastChild === row ? row : this.lastOf(ul, row);
      }
    }
  }

  // lastOf is the last row belonging to a row's branch, which is where the next
  // sibling goes once that branch has been reopened.
  lastOf(ul, row) {
    const prefix = row.dataset.branch + '>';
    let last = row;
    for (let el = row.nextElementSibling; el; el = el.nextElementSibling) {
      if (!el.dataset.branch || !el.dataset.branch.startsWith(prefix)) break;
      last = el;
    }
    return last;
  }

  treeRow(ul, g, dir, ancestors, depth) {
    const node = g.node, branch = [...ancestors, node.id], key = dir + '|' + branch.join('>');
    // A package that depends on something that depends back on it would open for
    // ever: the repeat is shown and left closed.
    const cyclic = ancestors.includes(node.id);
    const children = cyclic ? [] : this.neighbors(node, dir);
    const twisty = children.length
      ? h('button', { class: 'twisty', 'aria-label': `Show what ${node.name} ${dir === 'out' ? 'depends on' : 'is used by'}` }, '▸')
      : h('span', { class: 'twisty leaf' }, cyclic ? '↻' : '');

    const row = this.item('li', {
      class: 'tree-row', style: `padding-left:${8 + depth * 14}px`,
      onclick: () => this.onSelect(node),
      title: cyclic ? `${node.path || node.name} (already open further up)` : node.path || node.name,
    },
      twisty,
      h('span', { class: 'swatch', style: `background:${this.swatchOf(node)}` }),
      h('span', { class: 'name' }, node.kind === 'symbol' ? node.name : node.path || node.name),
      h('span', { class: 'meta' }, node.kind === 'package' ? node.parentNode.name
        : node.kind === 'symbol' ? node.parentNode.path : node.kind),
      g.count > 1 ? h('span', { class: 'meta' }, `×${g.count}`) : null);
    row.dataset.branch = key;

    if (!children.length) return row;
    const toggle = event => {
      if (event) event.stopPropagation();
      this.open.has(key) ? this.collapse(ul, key) : this.expand(ul, row, children, dir, branch, depth);
    };
    twisty.addEventListener('click', toggle);
    // The row itself selects; the arrow keys open and close it, as a tree does.
    row.addEventListener('keydown', e => {
      if (e.key === 'ArrowRight' && !this.open.has(key)) { e.preventDefault(); toggle(); }
      if (e.key === 'ArrowLeft' && this.open.has(key)) { e.preventDefault(); toggle(); }
    });
    row.setAttribute('aria-expanded', 'false');
    if (this.open.has(key)) {
      // An update redraws the panel; what was open stays open. The children can only
      // be inserted once the row itself is in the list, which insertRows does next.
      row.reopen = () => this.expand(ul, row, children, dir, branch, depth);
    }
    return row;
  }

  expand(ul, row, children, dir, branch, depth) {
    const key = row.dataset.branch;
    this.open.add(key);
    row.setAttribute('aria-expanded', 'true');
    row.querySelector('.twisty').textContent = '▾';
    this.insertRows(ul, row, children, dir, branch, depth + 1);
  }

  collapse(ul, key) {
    this.open.delete(key);
    const row = ul.querySelector(`[data-branch="${CSS.escape(key)}"]`);
    if (row) {
      row.setAttribute('aria-expanded', 'false');
      row.querySelector('.twisty').textContent = '▸';
    }
    // Everything opened below it goes with it, however deep.
    for (const el of [...ul.children]) {
      if (el.dataset.branch && el.dataset.branch.startsWith(key + '>')) {
        this.open.delete(el.dataset.branch);
        el.remove();
      }
    }
  }

  swatchOf(node) {
    const file = node.kind === 'symbol' ? node.parentNode : node;
    return file.kind === 'file' ? this.colorOf(file.lang) : 'var(--pkg)';
  }

  dependencies(n) {
    const refs = this.linkKind() === 'reference';
    if (n.kind === 'ecosystem' || (n.kind === 'symbol' && !refs)) return [];
    const usedBy = this.tree('Used by', 'var(--edge-in)', n, 'in');
    if (refs) return [this.tree('Uses', 'var(--edge-out)', n, 'out'), usedBy];
    // Some ecosystems (Go) import directories, not files: point at the package instead.
    const pkg = n.kind === 'file' && n.parentNode;
    const pkgUsers = pkg ? (this.model.edgesTo.get(pkg.id) || []).length : 0;
    if (pkgUsers) {
      usedBy.append(h('div', { class: 'hint' }, `Its package is imported ${pkgUsers}× - `,
        h('a', { class: 'link', onclick: () => this.onSelect(pkg) }, `select ${pkg.path}/`)));
    }
    return [this.tree('Depends on', 'var(--edge-out)', n, 'out'), usedBy];
  }

  // Git history of a file or directory in the selected range, with top authors.
  history(n) {
    if (n.kind !== 'file' && n.kind !== 'dir') return null;
    const hist = this.historyOf(n);
    if (!hist) return null;
    const title = h('h4', {}, 'Git history ', h('span', { class: 'n' }, `since ${formatDate(hist.since)}`));
    const m = hist.metric;
    if (!m) return h('div', { class: 'p-section' }, title, h('div', { class: 'empty' }, 'Not committed'));
    const stat = (v, k) => h('div', { class: 'stat' }, h('div', { class: 'v' }, v), h('div', { class: 'k' }, k));
    const top = [...m.authors.entries()].sort((a, b) => b[1] - a[1]);
    const most = top.length ? top[0][1] : 1;
    const others = top.slice(5).reduce((a, [, c]) => a + c, 0);
    return h('div', { class: 'p-section' }, title,
      h('div', { class: 'stats' },
        stat(fmt.format(m.commits), 'commits'), stat(fmt.format(m.churn), 'lines changed'),
        stat(ago(m.last), 'last change'), stat(fmt.format(m.authors.size), 'authors')),
      top.length ? h('ul', { class: 'p-list authors' }, top.slice(0, 5).map(([a, c]) =>
        h('li', { title: `${hist.authors[a]}: ${c} commits` },
          h('span', { class: 'name' }, hist.authors[a]),
          h('span', { class: 'share' }, h('i', { style: `width:${Math.round(c / most * 100)}%` })),
          h('span', { class: 'meta' }, fmt.format(c)))),
        others ? h('li', {}, h('span', { class: 'name meta' }, `${top.length - 5} more`), h('span', { class: 'meta' }, fmt.format(others))) : null)
        : h('div', { class: 'empty' }, 'No commits in range'),
    );
  }

  async source(node, seq, keepTop = 0) {
    const file = node.kind === 'symbol' ? node.parentNode : node;
    const pre = h('pre', { class: 'code' }, h('span', { class: 'ln' }, 'Loading…'));
    this.body.append(h('div', { class: 'p-section' }, h('h4', {}, 'Source'), pre));
    let text;
    try {
      text = await fetchSource(file.path);
    } catch (e) {
      text = `(${e.message})`;
    }
    if (seq !== this.seq) return; // selection changed meanwhile

    const lang = HLJS[file.lang];
    let lines;
    if (lang && hljs.getLanguage(lang) && text.length < MAX_HIGHLIGHT) {
      lines = splitHighlighted(hljs.highlight(text, { language: lang, ignoreIllegals: true }).value);
    } else {
      lines = text.split('\n').map(escapeHTML);
    }
    if (lines.length && lines[lines.length - 1] === '') lines.pop();
    pre.innerHTML = lines.map(l => `<span class="ln">${l || ' '}</span>`).join('');

    const outline = file.children.filter(c => c.kind === 'symbol');
    if (outline.length) {
      pre.parentElement.before(h('div', { class: 'p-section' },
        h('h4', {}, 'Symbols ', h('span', { class: 'n' }, fmt.format(outline.length))),
        h('ul', { class: 'p-list' }, outline.map(s => this.item('li', { onclick: () => this.onSelect(s) },
          h('span', { class: 'name' }, s.name), h('span', { class: 'meta' }, s.symbolKind), h('span', { class: 'meta' }, `:${s.line}`))))));
    }
    if (keepTop) this.body.scrollTop = keepTop; // a live update: stay where the reader was
    else if (node.kind === 'symbol') this.gotoLine(pre, node.line);
  }

  gotoLine(pre, line) {
    const el = pre.children[line - 1];
    if (!el) return;
    el.classList.add('hit');
    el.scrollIntoView({ block: 'center' });
  }
}


// hljs spans may cross newlines (block comments, template strings); re-open them per
// line so every line is a self-contained, correctly colored fragment.
function splitHighlighted(html) {
  const out = [];
  let open = [];
  for (const line of html.split('\n')) {
    const prefix = open.join('');
    for (const m of line.matchAll(/<span[^>]*>|<\/span>/g)) {
      if (m[0] === '</span>') open.pop(); else open.push(m[0]);
    }
    out.push(prefix + line + '</span>'.repeat(open.length));
  }
  return out;
}
