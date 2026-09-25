// What the camera has photographed, until you decide what to do with it.
//
// The camera used to write a file the moment it was used, which put a download in the
// browser's bar for every idle press of the shutter and gave no way to look at what
// had been taken. A photograph is a thing to collect, the way a caught finding is: it
// goes in here, it can be looked at, and the ones worth keeping are saved one at a
// time.
//
// Unlike the backpack, this is a session and nothing more. A backpack entry is a
// finding id, a few dozen bytes, and it belongs in the store that outlives the tab; a
// photograph is a megabyte of PNG, and a browser's store is neither large enough nor
// the right place for it. So the pictures live in memory, the count is capped, and the
// panel says as much - anything worth keeping is saved to a file, which is the whole
// point of having the button.

const MAX = 24; // beyond this the oldest is let go of, so a long session cannot grow

// Implements: REQ-HUNT-034, REQ-HUNT-036
export class Stash {
  /** onChange is called whenever the contents change, for whatever draws them. */
  constructor(onChange = () => {}) {
    this.onChange = onChange;
    this.items = [];
    this.nth = 0;
  }

  get count() { return this.items.length; }

  /**
   * Keeps one. `blob` is the PNG as rendered, `where` a line about what was in the
   * frame. Returns the entry, whose `n` is what the walker is told.
   */
  add(blob, where = '') {
    const item = {
      id: `photo-${++this.nth}`,
      n: this.nth,
      where,
      at: Date.now(),
      blob,
      url: URL.createObjectURL(blob),
    };
    this.items.unshift(item);
    // The oldest goes rather than the newest being refused: somebody taking their
    // twenty-fifth photograph wants that one, not a message about the other twenty-four.
    while (this.items.length > MAX) this.let_go(this.items.pop());
    this.onChange(this);
    return item;
  }

  remove(id) {
    const i = this.items.findIndex(it => it.id === id);
    if (i < 0) return;
    this.let_go(this.items.splice(i, 1)[0]);
    this.onChange(this);
  }

  clear() {
    for (const it of this.items) this.let_go(it);
    this.items = [];
    this.onChange(this);
  }

  /**
   * Writes one out as a file. The name carries the repository and the photograph's
   * number, so a handful saved from one session sort the way they were taken.
   *
   * Implements: REQ-HUNT-035
   */
  save(id, repo) {
    const it = this.items.find(x => x.id === id);
    if (!it) return null;
    const a = document.createElement('a');
    a.href = it.url;
    a.download = `${repo}-photo-${String(it.n).padStart(2, '0')}.png`;
    a.click();
    return a.download;
  }

  // The URL is the only thing holding the bitmap; letting go of it is what frees it.
  let_go(it) {
    if (it) URL.revokeObjectURL(it.url);
  }
}
