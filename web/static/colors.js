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
    land: v('--land'),
    terraceA: v('--terrace-a'),
    terraceB: v('--terrace-b'),
    district: v('--district'),
    pkg: v('--pkg'),
    pkgUnresolved: v('--pkg-unresolved'),
    edgeOut: v('--edge-out'),
    edgeIn: v('--edge-in'),
    dim: v('--dim'),
    select: v('--text'),
  };
}

// Stable language -> colour mapping for one repository (see model.rankLanguages).
export function languageColors(model, pal) {
  const map = new Map();
  model.languages.forEach(({ lang }, i) => map.set(lang, i < SLOTS ? pal.series[i] : pal.other));
  return {
    of: lang => map.get(lang) || pal.other,
    legend: () => {
      const top = model.languages.slice(0, SLOTS).map(l => ({ label: l.lang, color: map.get(l.lang), lang: l.lang }));
      const rest = model.languages.slice(SLOTS);
      if (rest.length || model.root.langLoc.has('')) top.push({ label: 'Other', color: pal.other, lang: null });
      return top;
    },
  };
}

export function sequential(pal, t) {
  const i = Math.min(pal.seq.length - 1, Math.max(0, Math.round(t * (pal.seq.length - 1))));
  return pal.seq[i];
}
