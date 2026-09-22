// What the walker has caught, and what became of it.
//
// Catching a bug in walk mode reads out what the scanner said about it and then, up
// to now, let it go. The point of catching one is to come back to it, so a caught bug
// goes in the backpack and stays there until the thing it was reporting is gone from
// the source.
//
// That last part is why the backpack is worth having rather than being a list of
// links: every live update re-reads the scanner reports, and a finding that is no
// longer in them has been fixed. reconcile is what notices, and an entry it marks
// fixed stays in the backpack, struck through, until it is cleared - seeing what you
// caught turn green is the whole reward for having caught it.
//
// It is kept in localStorage, per repository, so closing the tab does not empty it.
// Nothing here is authoritative: the findings document is, and the backpack is a
// record of what someone did about it.

const KEY = 'depphunter.backpack';
const MAX = 500; // a backpack, not a database

export class Backpack {
  /**
   * repo names the store, so two maps open side by side do not share one backpack.
   * onChange is called whenever the contents change, for whatever draws them.
   */
  constructor(repo, onChange = () => {}) {
    this.key = `${KEY}:${repo}`;
    this.onChange = onChange;
    this.items = read(this.key);
  }

  get counts() {
    let fixed = 0;
    for (const it of this.items) if (it.fixed) fixed++;
    return { total: this.items.length, fixed, open: this.items.length - fixed };
  }

  /** The ids of everything in the backpack, which is what stays caught on the map. */
  get ids() {
    return new Set(this.items.map(it => it.id));
  }

  has(id) {
    return this.items.some(it => it.id === id);
  }

  /** Puts a caught finding in. Catching the same one twice changes nothing. */
  add(finding, node) {
    if (this.has(finding.id)) return false;
    this.items.unshift({
      id: finding.id,
      severity: finding.severity || 'unknown',
      title: finding.title || finding.id,
      where: finding.package || finding.path || '',
      line: finding.line || 0,
      nodeId: node?.id || '',
      caughtAt: Date.now(),
      fixed: false,
      fixedAt: 0,
    });
    if (this.items.length > MAX) this.items.length = MAX;
    this.save();
    return true;
  }

  remove(id) {
    const before = this.items.length;
    this.items = this.items.filter(it => it.id !== id);
    if (this.items.length !== before) this.save();
  }

  /** Empties it, or just the part of it that is done with. */
  clear(fixedOnly = false) {
    this.items = fixedOnly ? this.items.filter(it => !it.fixed) : [];
    this.save();
  }

  /**
   * Takes the list as somebody else has it - the editor's side panel, which drops
   * items from the same backpack through the server. `quiet` travels to onChange, so
   * that whoever redraws knows not to hand the same list straight back up again.
   */
  replace(items) {
    if (!Array.isArray(items)) return false;
    const clean = items.filter(it => it && typeof it.id === 'string').slice(0, MAX);
    if (JSON.stringify(clean) === JSON.stringify(this.items)) return false;
    this.items = clean;
    this.save(true);
    return true;
  }

  /**
   * Compares what is in the backpack against the newest scanner reports. A finding
   * that is still reported is still there; one that is not has been fixed, and is
   * marked rather than dropped. Called with null - findings turned off, or a report
   * that failed to read - nothing is decided, because absence of a report is not
   * absence of the finding.
   */
  reconcile(index) {
    if (!index) return;
    const live = new Set(index.all.map(f => f.id));
    let changed = false;
    for (const it of this.items) {
      const fixed = !live.has(it.id);
      if (fixed === it.fixed) continue;
      it.fixed = fixed;
      it.fixedAt = fixed ? Date.now() : 0;
      changed = true;
    }
    if (changed) this.save();
    return changed;
  }

  save(quiet = false) {
    try {
      localStorage.setItem(this.key, JSON.stringify(this.items));
    } catch {
      // A full or blocked store is not worth a broken map; the backpack is then
      // just for this session.
    }
    this.onChange(this, quiet);
  }
}

function read(key) {
  try {
    const items = JSON.parse(localStorage.getItem(key) || '[]');
    return Array.isArray(items) ? items.filter(it => it && typeof it.id === 'string') : [];
  } catch {
    return [];
  }
}
