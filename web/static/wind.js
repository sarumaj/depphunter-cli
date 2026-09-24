// What the walker can spend before they have to walk it off.
//
// Running and jumping were free, and free is what made them the only way anybody
// moved: sprinting everywhere is faster than walking everywhere and costs nothing to
// choose, so the walk in walk mode never happened. A gauge is what turns that into a
// decision - a dash across a junction is worth what it costs, a sprint the length of
// a district is not, and the difference is legible before it is made rather than
// after.
//
// It behaves like a tool's tank, deliberately: it empties while it is being used,
// fills while it is not, and is drawn next to the one that says the same thing about
// the jet backpack. What it does not share is a tool - a walker always has legs, so
// this is always shown.
//
// Running the last of it out leaves the walker winded, which is a state rather than a
// number. Without it, a walker at nought recovers one frame's worth, sprints it away
// again on the next, and the gauge flickers instead of stopping them; with it they
// walk until a quarter of it is back. Flying is not the legs' work and costs nothing:
// the jet has its own tank, and paying twice for the same flight would only mean
// landing to catch a breath the walker never took.

const FULL = 9;        // seconds of flat-out running a walker has in them
const JUMP = 0.16;     // ... and what one jump takes out of it, as a share
const RECOVER = 7;     // seconds of not running it takes to get all of it back
const SECOND_WIND = 0.25; // how much has to be back before they can run again

/**
 * The walker's wind, and the gauge in the HUD that shows it. Like Health it holds no
 * timers: walk.js calls it once a frame with what the walker is doing.
 */
export class Wind {
  /** `hud` is the walk HUD, whose .w-stamina it draws into. */
  constructor(hud) {
    this.hud = hud;
    this.share = 1;
    this.spent = false; // winded: walking only, until SECOND_WIND of it is back
    this.shown = -1;    // what the gauge is currently saying, so it is only written when it moves
    this.shownSpent = null;
  }

  /** What a jump costs, so walk.js and the tests can say it without guessing. */
  static get jumpCost() { return JUMP; }

  /** Whether there is enough in the walker to start or keep running. */
  get ready() { return !this.spent && this.share > 0; }

  /** Back to a full chest, on the way into walk mode. */
  reset() {
    this.share = 1;
    this.spent = false;
    this.draw();
  }

  /**
   * One frame of it. `running` is whether the walker is actually running - flying and
   * standing about are both worth the same, which is nothing.
   */
  breathe(dt, running) {
    this.set(this.share + (running ? -dt / FULL : dt / RECOVER));
  }

  /**
   * Takes a one-off cost - a jump - and says whether the walker had it. A jump with
   * nothing left simply does not happen, which reads as tired rather than as broken.
   */
  spend(cost) {
    if (!this.ready || this.share < cost) return false;
    this.set(this.share - cost);
    return true;
  }

  set(share) {
    this.share = Math.max(0, Math.min(1, share));
    if (this.share <= 0) this.spent = true;
    else if (this.share >= SECOND_WIND) this.spent = false;
    this.draw();
  }

  /**
   * The gauge, drawn like the fuel one because it is the same kind of thing. Written
   * only when the number has moved: this is called on every frame, and a style set
   * every frame is a layout every frame.
   */
  draw() {
    const box = this.box ||= this.hud.querySelector('.w-stamina');
    if (!box) return;
    const left = Math.round(this.share * 100);
    if (left === this.shown && this.spent === this.shownSpent) return;
    this.shown = left;
    this.shownSpent = this.spent;
    (this.fill ||= box.querySelector('.w-stamina-fill')).style.width = `${left}%`;
    box.dataset.state = this.spent ? 'empty' : left > 35 ? 'well' : 'low';
    box.title = this.spent
      ? 'Out of breath: you can walk, but not run or jump, until you have got some of it back'
      : `Wind: ${left}% left. Running spends it, a jump costs a little of it, and it comes back while you are not doing either.`;
  }
}
