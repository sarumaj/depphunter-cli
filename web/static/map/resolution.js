// How many pixels a frame is drawn with, as a share of the canvas's own (dynamic
// resolution). The facades and the streets are painted per pixel, so on a slow GPU
// the frame rate follows the pixel count: while frames come one after another -
// walking, or dragging the map - and fall behind, the picture is drawn smaller and
// stretched to the canvas, and drawn finer again once they keep up. A picture that
// stays on screen (the map at rest, a screenshot, a photograph) is drawn in full.
//
// Implements: REQ-PERF-011

/** The share of the canvas's pixels a frame is never drawn with less than, a side. */
export const FLOOR = 0.5;
// Frames slower than SLOW milliseconds shrink the picture by STEP - the median of the
// last WINDOW, so that a hitch (a collection, a file loading) is not taken for a slow
// GPU; faster than FAST for a whole window of them, it grows back, but not to a size
// that was too slow within the last MEMORY milliseconds. A gap longer than PAUSE is
// the page put away, not a slow frame. Between two changes at least SETTLE: a change
// takes a frame or two to show in the timing.
const SLOW = 1000 / 45, FAST = 1000 / 57, STEP = 0.85, WINDOW = 30, MEMORY = 8000, PAUSE = 2000, SETTLE = 500;

export class Resolution {
  constructor() {
    this.scale = 1;
    this.last = -Infinity;
    this.changed = -Infinity;
    this.gaps = [];
    this.tooSlow = { scale: Infinity, at: -Infinity };
  }

  /**
   * Records a frame drawn at `now` (milliseconds) and returns the scale for the next.
   * `continuous`: whether it was asked for while the one before was being drawn,
   * which is what makes the gap between the two the time a frame takes.
   */
  frame(now, continuous = true) {
    const gap = now - this.last;
    this.last = now;
    if (!continuous || gap > PAUSE) {
      this.gaps.length = 0;
      return this.scale;
    }
    this.gaps.push(gap);
    if (this.gaps.length > WINDOW) this.gaps.shift();
    if (now - this.changed < SETTLE) return this.scale;
    const sorted = [...this.gaps].sort((a, b) => a - b), median = sorted[sorted.length >> 1];
    if (median > SLOW && this.gaps.length >= WINDOW / 3 && this.scale > FLOOR) {
      this.tooSlow = { scale: this.scale, at: now };
      this.set(Math.max(FLOOR, this.scale * STEP), now);
    } else if (median < FAST && this.gaps.length >= WINDOW && this.scale < 1) {
      const up = Math.min(1, this.scale / STEP);
      if (up < this.tooSlow.scale - 1e-6 || now - this.tooSlow.at > MEMORY) this.set(up, now);
    }
    return this.scale;
  }

  /** Back to full resolution, and nothing learned of the frames before: a view at rest. */
  rest() {
    this.set(1, -Infinity);
    this.last = -Infinity;
  }

  set(scale, now) {
    this.scale = scale;
    this.changed = now;
    this.gaps.length = 0;
  }
}
