// What the walker can take before the hunt is over.
//
// Two things on the map are dangerous, and both of them were there already. A long
// drop is one: the grapple gun and the jet backpack put roofs within reach, and a roof
// is only worth arriving on if leaving it the wrong way costs something. The bugs are
// the other. A finding is a bug on a wall; standing among them while the crosshair is
// somewhere else means being bitten, and how hard depends on what the scanner said -
// a note about a style rule stings, a critical advisory takes a third of you.
//
// Neither is a single blow. A fall has a height it is free below, and a bite is one
// of several the walker can stand, so what either does is turn walking about into a
// thing with a cost - not a way to lose a map by surprise.
//
// The other direction is the hunt itself: every bug in the backpack raises the ceiling
// and heals by the same amount, so a walker who has been catching things is a walker
// who can stand in a swarm. Dying is not a loss of anything kept - the backpack is
// what the session is for, and it keeps what was caught - only of the walk, which is
// begun again at full health.

const BASE = 100;        // what the walker starts with, before anything is caught
const PER_CATCH = 8;     // ... and what each bug in the backpack adds to the ceiling
const MAX_CATCH = 150;   // as far as catching can raise it
// A fall is free up to this, in map units - a storey is 0.3, so about a house - and
// costs this much per unit beyond it. Off a tall tower that is fatal, off a terrace
// wall it is nothing, and the walk mostly happens in between.
// Implements: REQ-WALK-027
const SAFE_FALL = 3, PER_UNIT = 26;
// A bite, by what the finding it came from was called. Several of any of them are
// survivable, which is the point: being bitten is a reason to swing at what is biting
// you rather than a thing that happens once and ends the walk.
// Implements: REQ-WALK-028
const BITE = { critical: 34, high: 22, medium: 13, low: 7, info: 4, unknown: 9 };
const HURT_MS = 420; // how long the screen wears a hit
// Mending: how long after the last thing that hurt before it starts, and how much a
// second of not being hurt is worth.
//
// It is slow, and it waits. A walker who is being bitten or standing in a fire mends
// nothing at all, so neither stops being dangerous for having been survived once; a
// walker who has got clear is back on their feet in half a minute. That gap is the
// whole of it - the map is a place to spend an afternoon in rather than a run to be
// restarted, and without mending a walk is only ever as long as the worst mistake in
// it, which turns every roof into a thing to avoid rather than a thing to stand on.
//
// It does not raise the ceiling. That is the backpack's, and catching things is meant
// to stay the only way to be worth more than you started.
const MENDS_AFTER = 5000, MENDS = 4;

/**
 * The walker's condition, and the bar in the HUD that shows it. It holds no timers of
 * its own: walk.js calls it as things happen and reads `dead` on the way past.
 */
export class Health {
  /** `hud` is the walk HUD, whose .w-health it draws into. */
  constructor(hud) {
    this.hud = hud;
    this.max = BASE;
    this.hp = BASE;
    this.hurtAt = 0;
    this.shown = -1;    // what the bar is currently saying, so it is only written when it moves
    this.shownMax = -1;
  }

  get dead() { return this.hp <= 0; }

  /** What a bite takes, so the HUD and the tests can say it without guessing. */
  static biteFor(severity) { return BITE[severity] ?? BITE.unknown; }

  /**
   * Back to full, with a ceiling set by how much is already in the backpack. Called
   * on the way into walk mode, which is the only way back in after dying.
   *
   * Implements: REQ-WALK-029, REQ-WALK-030
   */
  reset(caught = 0) {
    this.max = BASE + Math.min(MAX_CATCH, caught * PER_CATCH);
    this.hp = this.max;
    this.hurtAt = 0;
    this.draw();
  }

  /**
   * One more in the backpack: the ceiling goes up and the walker is mended by as
   * much. Catching is how you get through a bad street, and this is what makes that
   * true rather than a figure of speech.
   *
   * Implements: REQ-WALK-029
   */
  caught(total) {
    const was = this.max;
    this.max = BASE + Math.min(MAX_CATCH, total * PER_CATCH);
    this.hp = Math.min(this.max, this.hp + Math.max(0, this.max - was));
    this.draw();
  }

  /**
   * Time going by, for a walker nothing has touched lately. Returns what it put back,
   * so the caller can say so if it ever wants to; nothing while anything is still
   * happening to them.
   *
   * Given the turn rather than the clock, so it mends by how much time the walk has
   * spent and not by how long the page has been open - a walker held still while a
   * panel is read is not quietly healing behind it.
   */
  mend(dt, now = performance.now()) {
    if (this.dead || this.hp >= this.max) return 0;
    if (now - this.hurtAt < MENDS_AFTER) return 0;
    const put = Math.min(this.max - this.hp, MENDS * dt);
    this.hp += put;
    this.draw(now);
    return put;
  }

  /** Takes `amount` off, and says whether that was the end of it. */
  hurt(amount) {
    if (amount <= 0 || this.dead) return false;
    this.hp = Math.max(0, this.hp - amount);
    this.hurtAt = performance.now();
    this.draw();
    return this.dead;
  }

  /**
   * What a drop of `height` map units costs. Returns what was taken, so the caller
   * can say so; nothing at all for anything shorter than a house.
   *
   * Implements: REQ-WALK-027
   */
  fall(height) {
    const damage = Math.round(Math.max(0, height - SAFE_FALL) * PER_UNIT);
    this.hurt(damage);
    return damage;
  }

  /** What one bite from a bug of that severity costs, taken. */
  bite(severity) {
    const damage = Health.biteFor(severity);
    this.hurt(damage);
    return damage;
  }

  /**
   * The bar: how much is left, in the color of how bad it is, and the screen washed
   * red for a moment after anything lands. Called on every change and once a frame,
   * because the wash has to come off again by itself - so the elements are found once
   * and the numbers are only written when they have moved. A style written every
   * frame is a layout every frame, for a bar that changes a few times a walk.
   */
  draw(now = performance.now()) {
    const box = this.box ||= this.hud.querySelector('.w-health');
    if (!box) return;
    this.hud.classList.toggle('hit', now - this.hurtAt < HURT_MS);
    const left = Math.ceil(this.hp);
    if (left === this.shown && this.max === this.shownMax) return;
    this.shown = left;
    this.shownMax = this.max;
    const share = this.max > 0 ? this.hp / this.max : 0;
    (this.text ||= box.querySelector('.w-hp')).textContent = left;
    (this.fill ||= box.querySelector('.w-health-fill')).style.width = `${(share * 100).toFixed(1)}%`;
    box.dataset.state = share > 0.6 ? 'well' : share > 0.3 ? 'hurt' : 'bad';
  }
}
