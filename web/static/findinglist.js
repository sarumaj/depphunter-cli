// Every finding the map shows, as a list, and a way to catch each one from there.
//
// From above, a finding is a pin over a building, and reading one means finding the
// pin first: a repository with sixty advisories against its lock files is sixty
// buildings to go and point at. The list is the same findings with the map taken out
// of the way - worst first, each one a row that lights its building when pointed at,
// opens its details when picked, and goes in the backpack from its own button.
//
// Going in the backpack from here is catching it. It is the same act as netting its
// bug in the street (catchFinding in backpack.js): the same entry, recorded against
// the same building, handed up to the server the same way - and so its bug stops
// walking, since what is in the backpack is what stays caught. Only the walk up to it
// is left out, which is the whole of what the list is for.

import { rankOf, whereOf } from './findings.js';
import { h } from './dom.js';

/** What a row is sorted and labeled by: the package a finding is against, or its file. */
export const nameOf = f => f.package || f.path || '';

/**
 * The rows of the list: every finding whose building the map draws under the current
 * filters (`visible`, from filter.js), worst first and then by name, each with the
 * node its bug stands at. The name breaks ties the way a reader scans a list; the
 * title and the id after it only keep the order the same from one redraw to the next.
 *
 * Implements: REQ-HUNT-049
 */
export function findingRows(index, visible = () => true) {
  if (!index) return [];
  const rows = [];
  for (const finding of index.all) {
    const node = index.place(finding);
    if (visible(node)) rows.push({ finding, node });
  }
  const text = (a, b) => (a < b ? -1 : a > b ? 1 : 0);
  return rows.sort((a, b) =>
    rankOf(b.finding.severity) - rankOf(a.finding.severity)
    || text(nameOf(a.finding).toLowerCase(), nameOf(b.finding).toLowerCase())
    || text(a.finding.title || '', b.finding.title || '')
    || text(a.finding.id, b.finding.id));
}

/**
 * The list's rows on the page. `hooks`:
 *   caught(id)          whether a finding is in the backpack
 *   onPoint(row)        a row is pointed at or focused (null: none any more)
 *   onOpen(row)         a row is picked: its building is selected and its details read
 *   onToggle(row, kept) its button is pressed: in the backpack if it is not (kept is
 *                       false), out again if it is
 *
 * Implements: REQ-HUNT-049, REQ-HUNT-050, REQ-HUNT-051
 */
export class FindingList {
  constructor(list, hooks) {
    this.list = list;
    this.hooks = hooks;
    this.rows = [];
  }

  /** Draws `rows` (findingRows), and returns how many of them are still to catch. */
  draw(rows) {
    // A redraw replaces every row, the one with the keyboard on it among them - and
    // pressing a row's button is itself a redraw, since it changes the backpack. So
    // the focus is carried over to the same finding's row, or its button.
    const active = globalThis.document?.activeElement;
    const held = active && this.list.contains?.(active)
      ? { id: active.closest('li')?.dataset.finding, button: active.tagName === 'BUTTON' }
      : null;
    this.rows = rows;
    this.list.replaceChildren(...rows.map(row => this.row(row)));
    if (held) {
      const li = [...this.list.children].find(row => row.dataset.finding === held.id);
      (held.button ? li?.querySelector('.keep') : li)?.focus();
    }
    return rows.filter(row => !this.hooks.caught(row.finding.id)).length;
  }

  row(row) {
    const f = row.finding;
    const kept = !!this.hooks.caught(f.id);
    const severity = f.severity || 'unknown';
    // What it is against, and where that is when the name has not already said so:
    // the lock file a package's advisory was read from, or the line in a file.
    const name = f.package ? whereOf(f) : f.path || 'this repository';
    const location = f.package
      ? [f.ecosystem, f.path].filter(Boolean).join(' · ')
      : f.line ? `line ${f.line}` : '';
    const button = h('button', {
      class: 'keep', type: 'button',
      'aria-pressed': String(kept),
      'aria-label': kept ? `In the backpack: take ${f.title} out` : `Put ${f.title} in the backpack`,
      title: kept ? 'In the backpack - press to take it out' : 'Put it in the backpack',
      onclick: e => {
        e.stopPropagation(); // the row itself opens the details
        this.hooks.onToggle(row, kept);
      },
    }, kept ? '✓' : '+');
    const li = h('li', {
      class: `sev-${severity}${kept ? ' kept' : ''}`,
      tabindex: '0',
      'aria-label': `${severity}: ${f.title}, ${whereOf(f)}${kept ? ', in the backpack' : ''}`,
      onclick: () => this.hooks.onOpen(row),
      onmouseenter: () => this.hooks.onPoint(row),
      onmouseleave: () => this.hooks.onPoint(null),
      onfocus: () => this.hooks.onPoint(row),
      onblur: () => this.hooks.onPoint(null),
      onkeydown: e => this.key(e, li, row),
    },
      h('span', { class: 'sev-dot' }),
      h('span', { class: 'body' },
        h('span', { class: 'n' }, name),
        h('span', { class: 't' }, f.ref && f.ref !== f.title ? `${f.ref} · ${f.title}` : f.title || f.id),
        location ? h('span', { class: 'w' }, location) : null),
      h('span', { class: 'state' }, kept ? 'in the backpack' : severity),
      button);
    li.dataset.finding = f.id;
    return li;
  }

  /**
   * The keyboard: the arrows, Home and End move between rows, Enter and Space open
   * one, and Insert or `+` puts it in the backpack (or takes it out) - so the list can
   * be worked through without a pointer. A key the list uses goes no further: `+` is
   * also the map's own key for opening a level.
   *
   * Implements: REQ-HUNT-051
   */
  key(e, li, row) {
    if (e.target !== li) return; // the button handles its own keys
    const rows = [...this.list.children];
    const at = rows.indexOf(li);
    let to = -1;
    switch (e.key) {
      case 'ArrowDown': to = Math.min(rows.length - 1, at + 1); break;
      case 'ArrowUp': to = Math.max(0, at - 1); break;
      case 'Home': to = 0; break;
      case 'End': to = rows.length - 1; break;
      case 'Enter': case ' ': this.hooks.onOpen(row); break;
      case '+': case 'Insert': this.hooks.onToggle(row, !!this.hooks.caught(row.finding.id)); break;
      default: return;
    }
    e.preventDefault();
    e.stopPropagation();
    if (to >= 0) rows[to].focus();
  }
}
