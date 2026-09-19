// Side panel describing the selected node: stats, dependencies, dependents, source.

import hljs from './vendor/highlight.min.js';
import powershell from './vendor/highlight-powershell.min.js';

hljs.registerLanguage('powershell', powershell);
import { ancestors, boundaryEdges } from './model.js';
import { fetchSource } from './data.js';
import { ago, formatDate } from './history.js';
import { fmt, h, escapeHTML } from './dom.js';

const HLJS = {
  Go: 'go', JavaScript: 'javascript', TypeScript: 'typescript', Python: 'python', Rust: 'rust',
  Java: 'java', Kotlin: 'kotlin', 'C#': 'csharp', C: 'c', 'C++': 'cpp', Ruby: 'ruby', PHP: 'php',
  Shell: 'bash', YAML: 'yaml', JSON: 'json', Markdown: 'markdown', HTML: 'xml', XML: 'xml', CSS: 'css',
  SQL: 'sql', Swift: 'swift', Lua: 'lua', Make: 'makefile', Perl: 'perl', R: 'r', Scala: 'scala',
  'Objective-C': 'objectivec', TOML: 'ini', GraphQL: 'graphql', Docker: 'dockerfile', PowerShell: 'powershell',
};
const MAX_HIGHLIGHT = 300_000; // bytes; larger files are shown as plain text


export class Panel {
  constructor(root, body, { model, colorOf, onSelect, onOpen, openLabel, historyOf, linkKind }) {
    Object.assign(this, { root, body, model, colorOf, onSelect, onOpen, openLabel, historyOf, linkKind });
    this.seq = 0;
  }

  close() {
    this.root.hidden = true;
    this.root.parentElement.classList.remove('panel-open');
    this.node = null;
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
        node.unresolved ? h('span', { class: 'badge warn', title: 'Not found in any manifest' }, '⚠ unresolved') : null),
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
          stat(n.version || '-', 'version'), stat(fmt.format(n.importers), 'importing files'),
          stat(n.parentNode?.name || '', 'ecosystem'));
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

  dependencies(n) {
    const refs = this.linkKind() === 'reference';
    if (n.kind === 'ecosystem' || (n.kind === 'symbol' && !refs)) return [];
    const { out, in: inc } = boundaryEdges(this.model, n, this.linkKind());
    const group = (edges, key) => {
      const m = new Map();
      for (const e of edges) {
        const node = this.model.byId.get(e[key]);
        const g = m.get(node.id) || { node, count: 0, line: e.line };
        g.count++;
        m.set(node.id, g);
      }
      return [...m.values()].sort((a, b) => b.count - a.count || a.node.name.localeCompare(b.node.name));
    };
    const list = (title, hint, groups) => h('div', { class: 'p-section' },
      h('h4', {}, h('span', { class: 'swatch', style: `background:${hint}` }), title, h('span', { class: 'n' }, fmt.format(groups.length))),
      groups.length
        ? h('ul', { class: 'p-list' }, groups.map(g => this.item('li', { onclick: () => this.onSelect(g.node), title: g.node.path || g.node.name },
            h('span', { class: 'swatch', style: `background:${swatchOf(g.node)}` }),
            h('span', { class: 'name' }, g.node.kind === 'symbol' ? g.node.name : g.node.path || g.node.name),
            h('span', { class: 'meta' }, g.node.kind === 'package' ? g.node.parentNode.name
              : g.node.kind === 'symbol' ? g.node.parentNode.path : g.node.kind),
            g.count > 1 ? h('span', { class: 'meta' }, `×${g.count}`) : null)))
        : h('div', { class: 'empty' }, 'None'));
    const swatchOf = node => {
      const file = node.kind === 'symbol' ? node.parentNode : node;
      return file.kind === 'file' ? this.colorOf(file.lang) : 'var(--pkg)';
    };
    const usedBy = list('Used by', 'var(--edge-in)', group(inc, 'from'));
    if (refs) return [list('Uses', 'var(--edge-out)', group(out, 'to')), usedBy];
    // Some ecosystems (Go) import directories, not files: point at the package instead.
    const pkg = n.kind === 'file' && n.parentNode;
    const pkgUsers = pkg ? (this.model.edgesTo.get(pkg.id) || []).length : 0;
    if (pkgUsers) {
      usedBy.append(h('div', { class: 'hint' }, `Its package is imported ${pkgUsers}× - `,
        h('a', { class: 'link', onclick: () => this.onSelect(pkg) }, `select ${pkg.path}/`)));
    }
    return [list('Depends on', 'var(--edge-out)', group(out, 'to')), usedBy];
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
