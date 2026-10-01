// The source of the file the side panel shows (Panel.source): highlighted text with a
// find bar above it, a preview for a picture, a clip or a recording, and for a file
// with no text its size and, when asked, a hex dump of its first bytes.

import hljs from './vendor/highlight.min.js';
import powershell from './vendor/highlight-powershell.min.js';

import { fileSize } from './model.js';
import { BinaryFile, fetchBytes, fetchSource, fileURL } from './data.js';
import { fmt, h, escapeHTML } from './dom.js';

hljs.registerLanguage('powershell', powershell);

const HLJS = {
  Go: 'go', JavaScript: 'javascript', TypeScript: 'typescript', Python: 'python', Rust: 'rust',
  Java: 'java', Kotlin: 'kotlin', 'C#': 'csharp', C: 'c', 'C++': 'cpp', Ruby: 'ruby', PHP: 'php',
  Shell: 'bash', YAML: 'yaml', JSON: 'json', Markdown: 'markdown', HTML: 'xml', XML: 'xml', CSS: 'css',
  SQL: 'sql', Swift: 'swift', Lua: 'lua', Make: 'makefile', Perl: 'perl', R: 'r', Scala: 'scala',
  'Objective-C': 'objectivec', TOML: 'ini', GraphQL: 'graphql', Docker: 'dockerfile', PowerShell: 'powershell',
};
const MAX_HIGHLIGHT = 300_000; // bytes; larger files are shown as plain text

/**
 * The panel's source methods, on Panel.prototype (panel.js): they read and fill the
 * panel's body and keep its find text across redraws.
 */
export const sourceView = {
  // Implements: REQ-MAP-027, REQ-MAP-028, REQ-MAP-044, REQ-MAP-062
  async source(node, sequence, keepTop = 0) {
    const file = node.kind === 'symbol' ? node.parentNode : node;
    // A picture, a clip or a recording is shown as itself rather than read as text,
    // and without asking for the text first: the element fetches what it needs.
    const kind = mediaKind(file.path), url = kind && fileURL(file.path);
    if (url) {
      const section = h('div', { class: 'p-section' }, h('h4', {}, 'Preview'));
      section.append(media(kind, url, file, () => section.replaceWith(this.binary(file, 'application/octet-stream'))));
      this.body.append(section);
      return;
    }
    const pre = h('pre', { class: 'code' }, h('span', { class: 'ln' }, 'Loading…'));
    // Find in the file, above it: what was being looked for is kept across a live
    // update that redraws the panel, and found again in the new text.
    const find = findBar(pre, this.findText, q => { this.findText = q; });
    this.body.append(h('div', { class: 'p-section' }, h('div', { class: 'p-head' }, h('h4', {}, 'Source'), find.el), pre));
    let text;
    try {
      text = await fetchSource(file.path);
    } catch (e) {
      if (e instanceof BinaryFile) {
        if (sequence !== this.seq) return;
        pre.parentElement.replaceWith(this.binary(file, e.type));
        // Now it is known to have no text, the corner offers a hex editor for it.
        this.binaryPath = file.path;
        this.pinOpen(this.node);
        return;
      }
      text = `(${e.message})`;
    }
    // The selection changed while the file was being read - or, far more often, a
    // live update redrew the panel for the same node and started its own read. The
    // <pre> this began with is gone, but the text is still good: while the panel is
    // showing the same file and the redraw's read has not come back, fill the pane
    // that is there rather than leave it on "Loading..." while updates keep arriving.
    if (sequence !== this.seq) {
      if (this.node?.id === node.id) {
        const live = this.body.querySelector('pre.code');
        if (live && !live.dataset.filled) paint(live, text, file);
      }
      return;
    }
    paint(pre, text, file);
    find.run(false);

    const outline = file.children.filter(c => c.kind === 'symbol');
    if (outline.length) {
      pre.parentElement.before(h('div', { class: 'p-section' },
        h('h4', {}, 'Symbols ', h('span', { class: 'n' }, fmt.format(outline.length))),
        h('ul', { class: 'p-list' }, outline.map(s => this.item('li', { onclick: () => this.onSelect(s) },
          h('span', { class: 'name' }, s.name), h('span', { class: 'meta' }, s.symbolKind), h('span', { class: 'meta' }, `:${s.line}`))))));
    }
    if (keepTop) this.body.scrollTop = keepTop; // a live update: stay where the reader was
    else if (node.kind === 'symbol') this.gotoLine(pre, node.line);
  },

  /**
   * What is shown for a file that is not text: nothing of its content until asked.
   * Its bytes say little and there can be a great many of them, so the section says
   * what the file is and how big, and a button - which says as much - shows the first
   * RAW_BYTES of them, as offsets, hexadecimal and whatever is printable.
   *
   * Implements: REQ-MAP-062
   */
  binary(file, type) {
    const size = file.bytes || 0;
    const note = h('p', { class: 'p-binary' },
      `Binary file${type && type !== 'application/octet-stream' ? ` (${type})` : ''}`
      + `${size ? `, ${formatBytes(size)}` : ''} - it has no text to show, so its content is not shown.`);
    const show = h('button', {
      class: 'p-reveal', type: 'button',
      title: `Shows the first ${formatBytes(RAW_BYTES)} as hexadecimal. It is rarely readable, and it is not highlighted or searched.`,
      onclick: async () => {
        show.disabled = true;
        show.textContent = 'Reading…';
        const pre = h('pre', { class: 'code hex' });
        try {
          const bytes = await fetchBytes(file.path, RAW_BYTES);
          pre.innerHTML = hexDump(bytes).map(l => `<span class="ln">${escapeHTML(l)}</span>`).join('');
          if (size > bytes.length) pre.append(h('span', { class: 'ln meta' }, `… the first ${formatBytes(bytes.length)} of ${formatBytes(size)}`));
          show.replaceWith(pre);
        } catch (e) {
          show.replaceWith(h('p', { class: 'p-binary' }, `(${e.message})`));
        }
      },
    }, '⚠ Show raw bytes');
    return h('div', { class: 'p-section' }, h('h4', {}, 'Content'), note, show);
  },

  gotoLine(pre, line) {
    const el = pre.children[line - 1];
    if (!el) return;
    el.classList.add('hit');
    el.scrollIntoView({ block: 'center' });
  },
};

// Find in the file (findBar). As many matches as are marked at once, which is more
// than anybody steps through, and a limit on the work a one-letter search does in a
// file of a hundred thousand lines.
const MAX_FOUND = 5000;

/**
 * Every place `query` occurs in `lines`, ignoring case, in reading order: its line
 * (0-based) and where in the line it starts and ends. Literal text, not a pattern -
 * somebody looking for `a.b(` means those four characters. Overlapping occurrences
 * are not counted twice.
 *
 * Implements: REQ-MAP-063
 */
export function findMatches(lines, query, limit = MAX_FOUND) {
  const out = [];
  const q = (query || '').toLowerCase();
  if (!q) return out;
  for (let i = 0; i < lines.length && out.length < limit; i++) {
    const line = lines[i].toLowerCase();
    for (let at = line.indexOf(q); at >= 0 && out.length < limit; at = line.indexOf(q, at + q.length)) {
      out.push({ line: i, start: at, end: at + q.length });
    }
  }
  return out;
}

/**
 * The find bar over a file's source: a search field, how many matches and which one
 * is current, and buttons to step between them. Enter steps on, Shift+Enter back,
 * Escape clears. Matches are highlighted where they are without touching the lines
 * themselves (the CSS Custom Highlight API), so the syntax coloring and a symbol's
 * own marking are left as they were; where a browser has no such thing, the lines
 * that match are marked instead. `remember` hears every change of what is looked for.
 *
 * Returns the bar and `run`, which finds again - after the text arrives, say. `step`
 * false leaves the current match where it is instead of scrolling to the first.
 *
 * Implements: REQ-MAP-063
 */
export function findBar(pre, initial = '', remember = () => {}) {
  let found = [], at = -1;
  const input = h('input', {
    class: 'p-find', type: 'search', placeholder: 'Find in file', 'aria-label': 'Find in this file',
    spellcheck: 'false', autocomplete: 'off',
  });
  input.value = initial || '';
  const count = h('span', { class: 'p-find-count', 'aria-live': 'polite' });
  const previous = h('button', { type: 'button', class: 'p-find-step', title: 'Previous match (Shift+Enter)', 'aria-label': 'Previous match' }, '↑');
  const next = h('button', { type: 'button', class: 'p-find-step', title: 'Next match (Enter)', 'aria-label': 'Next match' }, '↓');
  const lines = () => pre.children.filter ? pre.children : [...pre.children];

  const show = () => {
    const els = lines();
    for (const el of els) { el.classList.remove('found'); el.classList.remove('current'); }
    for (const m of found) els[m.line]?.classList.add('found');
    const current = found[at];
    if (current) els[current.line]?.classList.add('current');
    count.textContent = !input.value ? '' : found.length ? `${at + 1} of ${found.length}${found.length >= MAX_FOUND ? '+' : ''}` : 'No matches';
    previous.disabled = next.disabled = found.length < 2;
    highlight(els, found, current);
  };
  const go = i => {
    if (!found.length) return;
    at = (i + found.length) % found.length;
    show();
    lines()[found[at].line]?.scrollIntoView?.({ block: 'center' });
  };
  const run = (step = true) => {
    // Nothing to search until the text is in: the placeholder line is not the file.
    if (!pre.dataset.filled) { found = []; at = -1; count.textContent = ''; return; }
    found = findMatches(lines().map(el => el.textContent), input.value);
    at = found.length ? 0 : -1;
    // Stepping needs something to step to; with nothing found the count and the
    // marks still have to change, or the last search's would stay on screen.
    if (step && found.length) go(0); else show();
  };

  input.addEventListener('input', () => { remember(input.value); run(); });
  input.addEventListener('keydown', e => {
    if (e.key === 'Enter') { e.preventDefault(); go(at + (e.shiftKey ? -1 : 1)); }
    else if (e.key === 'Escape' && input.value) {
      e.preventDefault();
      e.stopPropagation(); // the first Escape clears the search, not the panel
      input.value = '';
      remember('');
      run();
    }
  });
  previous.addEventListener('click', () => go(at - 1));
  next.addEventListener('click', () => go(at + 1));
  return { el: h('div', { class: 'p-findbar', role: 'search' }, input, count, previous, next), run, input };
}

/** Marks the matches in the page, or takes the marks away (no matches). */
function highlight(lines, found, current) {
  const registry = globalThis.CSS?.highlights;
  if (!registry || typeof Highlight !== 'function' || typeof document.createRange !== 'function') return;
  const all = new Highlight(), currentHighlight = new Highlight();
  for (const m of found) {
    const range = rangeIn(lines[m.line], m.start, m.end);
    if (range) (m === current ? currentHighlight : all).add(range);
  }
  registry.set('dh-found', all);
  registry.set('dh-current', currentHighlight);
}

/** Takes the find bar's marks off the page, for a panel shown afresh or closed. */
export function clearFound() {
  globalThis.CSS?.highlights?.delete('dh-found');
  globalThis.CSS?.highlights?.delete('dh-current');
}

/** A range over characters start..end of an element's text, across however many text nodes. */
function rangeIn(el, start, end) {
  if (!el) return null;
  const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  const range = document.createRange();
  let seen = 0, begun = false;
  for (let node = walk.nextNode(); node; node = walk.nextNode()) {
    const textLength = node.nodeValue.length;
    if (!begun && start < seen + textLength) { range.setStart(node, start - seen); begun = true; }
    if (begun && end <= seen + textLength) { range.setEnd(node, end - seen); return range; }
    seen += textLength;
  }
  return null;
}

// How much of a binary file its "show raw bytes" button reads: enough to recognize
// a header or a signature by, and few enough lines to scroll past.
const RAW_BYTES = 64 * 1024;

// The files previewed as themselves, by extension: the same list the server serves
// under a media type (internal/server mediaTypes). Anything else is text or bytes.
const MEDIA = {
  image: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'avif'],
  video: ['mp4', 'm4v', 'webm', 'ogv', 'mov'],
  audio: ['mp3', 'wav', 'ogg', 'oga', 'flac', 'm4a'],
};

/**
 * 'image', 'video' or 'audio' for a file the panel previews, or null.
 *
 * Implements: REQ-MAP-062
 */
export function mediaKind(path) {
  const extension = /\.([^./]+)$/.exec(path || '')?.[1]?.toLowerCase();
  return Object.keys(MEDIA).find(k => MEDIA[k].includes(extension)) || null;
}

/** The element that previews it; `failed` replaces it when the browser cannot play it. */
function media(kind, url, file, failed) {
  const el = kind === 'image'
    ? h('img', { class: 'p-media', src: url, alt: file.name })
    : h(kind, { class: 'p-media', src: url, controls: '', preload: 'metadata' });
  el.addEventListener('error', failed, { once: true });
  return el;
}

/**
 * Bytes as a hex dump, one line per 16: the offset, the bytes in hexadecimal in two
 * groups of eight, and the printable ASCII among them with a dot for the rest - the
 * layout `hexdump -C` and `xxd` use, so it reads the way anybody who has read one
 * expects.
 *
 * Implements: REQ-MAP-062
 */
export function hexDump(bytes) {
  const out = [];
  for (let at = 0; at < bytes.length; at += 16) {
    const row = Array.from(bytes.subarray(at, at + 16));
    const hex = row.map(b => b.toString(16).padStart(2, '0'));
    const left = hex.slice(0, 8).join(' '), right = hex.slice(8).join(' ');
    const text = row.map(b => (b >= 0x20 && b < 0x7f ? String.fromCharCode(b) : '.')).join('');
    out.push(`${at.toString(16).padStart(8, '0')}  ${left.padEnd(23)}  ${right.padEnd(23)}  |${text}|`);
  }
  return out;
}

const formatBytes = fileSize;

// The file, one <span> per line so a symbol can be scrolled to, highlighted when
// hljs knows the language and the file is small enough to be worth it.
// Implements: REQ-MAP-027
function paint(pre, text, file) {
  const language = HLJS[file.lang];
  let lines;
  if (language && hljs.getLanguage(language) && text.length < MAX_HIGHLIGHT) {
    lines = splitHighlighted(hljs.highlight(text, { language: language, ignoreIllegals: true }).value);
  } else {
    lines = text.split('\n').map(escapeHTML);
  }
  if (lines.length && lines[lines.length - 1] === '') lines.pop();
  pre.innerHTML = lines.map(l => `<span class="ln">${l || ' '}</span>`).join('');
  pre.dataset.filled = '1';
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
