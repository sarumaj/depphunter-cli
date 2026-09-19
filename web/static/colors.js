// Colour roles. Every value is read from CSS custom properties (style.css), so the
// light and dark themes are each chosen deliberately, not derived by inversion.

const SLOTS = 7; // categorical slots for languages; the rest fold into "Other"

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
    edgeOut: v('--edge-out'),
    edgeIn: v('--edge-in'),
    dim: v('--dim'),
    select: v('--text'),
  };
}

// assignSlots gives the largest languages the categorical slots and keeps earlier
// assignments (prev) while those languages exist, so live updates never repaint a
// language: colour follows the language, not its rank.
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
