// HTML labels over the map: region names (directories, islands) and, for a selection,
// the names at both ends of its arcs. Placed greedily by priority without overlaps.

const MAX_LABELS = 160;

export class Labels {
  constructor(root, scene) {
    this.root = root;
    this.scene = scene;
    this.candidates = [];
    this.pool = [];
  }

  /** Chooses what may be labelled: the layout's boxes plus the focus (arcs, selection). */
  // Implements: REQ-MAP-040, REQ-MAP-041
  set(boxes, focus, selected) {
    const c = [];
    for (const b of boxes) {
      const n = b.node;
      if (b.kind === 'land' && n.kind === 'ecosystem') c.push({ b, text: n.name, cls: 'island', prio: 0 });
      else if (b.kind === 'terrace' && n.kind === 'dir') c.push({ b, text: n.name + '/', cls: '', prio: 1 + n.depth });
      else if (b.kind === 'district') c.push({ b, text: n.name + '/', cls: 'secondary', prio: 2 + n.depth });
    }
    if (focus) {
      for (const a of focus.arcs) {
        for (const b of [a.from, a.to]) {
          if (b.kind === 'building' || b.kind === 'package' || b.kind === 'symbol') c.push({ b, text: b.node.name, cls: '', prio: -1 });
        }
      }
      c.push({ b: focus.selBox, text: selected.name, cls: 'island', prio: -2 });
    }
    this.candidates = c.sort((a, b) => a.prio - b.prio);
    this.scene.requestRender();
  }

  /** Positions labels for the current camera; called after every render. */
  // Implements: REQ-MAP-040, REQ-MAP-042
  draw() {
    const { root, scene } = this;
    const placed = [];
    const seen = new Set();
    let used = 0;
    for (const l of this.candidates) {
      if (used >= MAX_LABELS) break;
      const key = l.b.node.id + l.cls;
      if (seen.has(key)) continue;
      seen.add(key);
      const b = l.b, top = b.y + b.h;
      const corners = [[-1, -1], [1, -1], [-1, 1], [1, 1]].map(([sx, sz]) => scene.project(b.x + sx * b.w / 2, top, b.z + sz * b.d / 2));
      if (corners.some(c => !c)) continue;
      const xs = corners.map(c => c.x);
      const width = l.text.length * 6.6 + 14;
      // Region labels need their region to be big enough on screen to be worth naming.
      if (l.prio > 0 && Math.max(...xs) - Math.min(...xs) < Math.min(width, 90)) continue;
      // Regions are named at their top-most corner on screen, i.e. their back edge.
      const anchor = l.prio > 0 && b.kind !== 'district' ? corners.reduce((a, c) => (c.y < a.y ? c : a)) : scene.project(b.x, top, b.z);
      const rect = { x: anchor.x - width / 2, y: anchor.y - 20, w: width, h: 18 };
      if (rect.x > root.clientWidth || rect.y > root.clientHeight || rect.x + rect.w < 0 || rect.y + rect.h < 0) continue;
      if (placed.some(p => p.x < rect.x + rect.w && rect.x < p.x + p.w && p.y < rect.y + rect.h && rect.y < p.y + p.h)) continue;
      placed.push(rect);

      let el = this.pool[used];
      if (!el) {
        el = document.createElement('div');
        this.pool.push(el);
        root.append(el);
      }
      el.className = 'label ' + l.cls;
      el.textContent = l.text;
      el.style.left = anchor.x + 'px';
      el.style.top = anchor.y - 2 + 'px';
      el.hidden = false;
      used++;
    }
    for (let i = used; i < this.pool.length; i++) this.pool[i].hidden = true;
  }

  /** The labels on screen, for painting them into a screenshot. */
  visible() {
    return this.pool.filter(el => !el.hidden);
  }
}
