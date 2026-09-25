// Color roles. Every value is read from CSS custom properties (style.css), so the
// light and dark themes are each chosen deliberately, not derived by inversion.

import { bulk } from './model.js';

const SLOTS = 7; // categorical slots for languages; the rest fold into "Other"

// Implements: REQ-A11Y-001, REQ-MAP-051
export function readPalette() {
  const cs = getComputedStyle(document.documentElement);
  const v = name => cs.getPropertyValue(name).trim();
  return {
    series: Array.from({ length: SLOTS }, (_, i) => v(`--series-${i + 1}`)),
    other: v('--series-other'),
    seq: Array.from({ length: 7 }, (_, i) => v(`--seq-${i + 1}`)), // near-surface -> strong
    water: v('--water'),
    sky: v('--sky'),        // walk mode: horizon, zenith and sea
    skyTop: v('--sky-top'),
    sea: v('--sea'),
    land: v('--land'),
    terraceA: v('--terrace-a'),
    terraceB: v('--terrace-b'),
    district: v('--district'),
    noData: v('--no-data'),
    pkg: v('--pkg'),
    pkgUnresolved: v('--pkg-unresolved'),
    pkgFloating: v('--pkg-floating'),
    edgeOut: v('--edge-out'),
    edgeIn: v('--edge-in'),
    dim: v('--dim'),
    select: v('--text'),
    avatar: v('--avatar') || v('--text'), // the walker's marker: ink, so it is nobody's language
  };
}

// assignSlots gives the largest languages the categorical slots and keeps earlier
// assignments (prev) while those languages exist, so live updates never repaint a
// language: color follows the language, not its rank.
// Implements: REQ-MAP-036
export function assignSlots(model, prev = []) {
  const present = new Set(model.languages.map(l => l.lang));
  const slots = Array.from({ length: SLOTS }, (_, i) => (present.has(prev[i]) ? prev[i] : null));
  for (const { lang } of model.languages) {
    const free = slots.indexOf(null);
    if (free < 0) break;
    if (!slots.includes(lang)) slots[free] = lang;
  }
  return slots;
}

// Implements: REQ-MAP-014, REQ-A11Y-001
export function languageColors(model, pal, slots) {
  const map = new Map();
  slots.forEach((lang, i) => lang && map.set(lang, pal.series[i]));
  return {
    of: lang => map.get(lang) || pal.other,
    legend: () => {
      const entries = slots.map((lang, i) => lang && { label: lang, color: pal.series[i], lang }).filter(Boolean);
      const other = model.languages.some(l => !map.has(l.lang)) || model.root.langLoc.has('');
      if (other) entries.push({ label: 'Other', color: pal.other, lang: null });
      return entries;
    },
  };
}

export function sequential(pal, t) {
  const i = Math.min(pal.seq.length - 1, Math.max(0, Math.round(t * (pal.seq.length - 1))));
  return pal.seq[i];
}

/**
 * The base color of one layout box: in a history mode (`hm` the loaded metrics) the
 * sequential ramp over `historyT`, in size mode the ramp over `sizeT` of the drawn
 * size, and otherwise the language (buildings, symbols) or the fixed role colors.
 * Implements: REQ-MAP-037, REQ-MAP-051, REQ-MAP-058, REQ-HIST-010, REQ-HIST-013
 */
export function boxColor(b, { mode, pal, langs, sizeT, hm, historyT }) {
  const n = b.node;
  if (hm && (b.kind === 'district' || b.kind === 'building' || b.kind === 'symbol')) {
    const t = historyT(mode, b.kind === 'symbol' ? n.parentNode : n, hm);
    return t === null ? pal.noData : sequential(pal, t);
  }
  switch (b.kind) {
    case 'land': return pal.land;
    case 'terrace': return n.kind === 'file' ? pal.terraceB : (n.depth % 2 ? pal.terraceB : pal.terraceA);
    case 'district':
      return mode === 'size' ? sequential(pal, sizeT(n.totalBulk / Math.max(1, n.fileCount))) : pal.district;
    case 'building':
      return mode === 'size' ? sequential(pal, sizeT(bulk(n))) : langs.of(n.lang);
    case 'symbol': {
      const f = n.parentNode;
      return mode === 'size' ? sequential(pal, sizeT(bulk(f))) : langs.of(f.lang);
    }
    // A package nothing pins is worth seeing from across the map.
    // Implements: REQ-SUP-004
    case 'package': return n.unresolved ? pal.pkgUnresolved : n.floating ? pal.pkgFloating : pal.pkg;
  }
  return pal.other;
}
