// Fire is a vulnerability somebody can actually reach.
//
// A scanner against a lock file will tell you a dependency is vulnerable. That is not
// the same as saying this project can suffer it, and most of the time it is not even
// close: the advisory is against a function nothing here calls. govulncheck is the one
// that does the work of telling those apart - it follows the call graph and reports
// which vulnerable functions are reachable from this code and from where - and the
// scanner readers keep the answer (findings.Reached, and Path/Line for the call site).
//
// So the map draws the two differently, and that difference is the whole point of the
// analogy. A vulnerability nothing reaches walks a lap as a bug: it is on the map, it
// is worth knowing about, it can be caught when you get to it. One that is reachable
// burns. It is the only finding on this map that gets worse while you look at it, and
// it is the only one that deserves to.
//
// Where the fire goes is the second half. The advisory names the package and the one
// call site in this repository that reaches it, and those are the first two things
// alight: the package's island, and the building that calls in. From there it travels
// the imports the map already draws, against their direction - to the files that
// import the burning one, and then to the files that import those. That is not
// decoration. A vulnerable function reachable from a file is reachable from everything
// that calls that file, which is exactly the thing a list of forty advisories cannot
// show you and a fire front can.
//
// Putting it out is the other lesson. Cooling a building holds the line there, but the
// fire goes on arriving from behind it; put out the package the fire started from and
// everything it lit goes out with it. That is what upgrading the dependency does, and
// it is a better argument for doing so than a number in a report.
//
// Nothing here draws anything or knows that a scene exists. What burns, how hot, and
// what spreads where is all decided in this file, so all of it can be reasoned about -
// and tested - without a browser.

/** A vulnerability a scanner proved this project can reach. Only these burn. */
export const reachable = f => f.kind === 'vulnerability' && !!f.reached;

// How fire behaves, in seconds and in heat, where heat runs 0 (out) to 1 (well alight).
//
// The numbers are set so that a walker who does nothing watches a district go up over
// a minute or two - long enough to see it move and choose where to stand, short enough
// that it is a thing happening rather than a thing on a map. Cooling is faster than
// catching, because a tool you hold down should feel like it is winning.
export const CATCHES = 0.35;     // heat per second, once alight
export const DOUSES = 0.9;       // heat per second under the extinguisher
export const SPREADS_AT = 0.75;  // how well alight it has to be to pass the fire on
export const SPREADS_EVERY = 4;  // seconds between one building lighting the next
export const COOLS_FOR = 25;     // seconds a doused building will not catch again
// However big the map, this many buildings alight at once is as much as the eye can
// read and as much as the scene should be asked to draw.
export const MOST = 60;

/**
 * What the fires start from: every reachable vulnerability, with the package it is
 * against and the building that calls into it. Both may be missing from the layout -
 * a vendored file, a package the filters have hidden - and a seat with neither is no
 * use to anybody, so it is dropped.
 *
 * `index` is what findings.js built; the node a finding sits on is already worked out
 * there, but both ends are wanted here rather than the one it chose.
 */
export function seatsOf(index, model) {
  const seats = [];
  for (const f of index?.all || []) {
    if (!reachable(f)) continue;
    const pkg = f.package ? model.byId.get(`p:${f.ecosystem}:${f.package}`) : null;
    const file = f.path ? model.byId.get(`f:${f.path}`) : null;
    if (!pkg && !file) continue;
    seats.push({ f, pkg, file, from: (pkg || file).id, into: (file || pkg).id });
  }
  return seats;
}

/**
 * The fires on one map.
 *
 * Every burning building is one entry in `lit`: what it is, how hot, which seat lit it
 * and how long since it last passed the fire on. A building that has been put out is
 * remembered in `cooled` until it is worth catching again, so a walker who holds the
 * line somewhere gets to keep it for a while rather than watching it relight behind
 * them as they turn round.
 */
export class Fires {
  constructor(model) {
    this.model = model;
    this.seats = [];
    this.lit = new Map();     // nodeId -> { seat, heat, since, spreadAt }
    this.cooled = new Map();  // nodeId -> seconds left before it will catch again
    this.out = new Set();     // seats whose fire is out for good, by finding id
  }

  /** Nothing is burning and nothing is remembered: a new map, or a new set of reports. */
  light(index) {
    this.seats = seatsOf(index, this.model);
    this.lit.clear();
    this.cooled.clear();
    // A seat already put out stays out across a relayout, the way a caught bug does.
    for (const seat of this.seats) {
      if (this.out.has(seat.f.id)) continue;
      this.ignite(seat.from, seat, true);
    }
  }

  /** Whether anything is alight, which is what the HUD and the tracker ask. */
  get burning() { return this.lit.size; }

  /** What is alight and how hot, for whatever draws it. */
  *entries() {
    for (const [id, fire] of this.lit) yield { id, ...fire };
  }

  heatOf(id) { return this.lit.get(id)?.heat || 0; }

  /**
   * Set one building alight. `source` marks the two the advisory itself names - the
   * package and the call site - which catch immediately; everything the fire reaches
   * afterwards starts cold and has to take.
   */
  ignite(id, seat, source = false) {
    if (!id || this.lit.has(id) || this.cooled.has(id) || this.lit.size >= MOST) return false;
    if (!this.model.byId.has(id)) return false;
    this.lit.set(id, { seat, heat: source ? 0.5 : 0.05, since: 0, spreadAt: 0, source });
    return true;
  }

  /**
   * A second of fire. Everything alight gets hotter; anything hot enough and old
   * enough passes the fire to the next building along.
   */
  step(dt) {
    for (const [id, left] of this.cooled) {
      const t = left - dt;
      if (t <= 0) this.cooled.delete(id); else this.cooled.set(id, t);
    }
    // Collected first: igniting inside the walk would let one building light another
    // and that one a third within the same frame, and a fire front that crosses the
    // whole map in one tick is not a front at all.
    const next = [];
    for (const [id, fire] of this.lit) {
      fire.heat = Math.min(1, fire.heat + CATCHES * dt);
      fire.since += dt;
      if (fire.heat < SPREADS_AT) continue;
      fire.spreadAt += dt;
      if (fire.spreadAt < SPREADS_EVERY) continue;
      fire.spreadAt = 0;
      for (const to of this.onward(id, fire)) next.push([to, fire.seat]);
    }
    for (const [id, seat] of next) this.ignite(id, seat);
  }

  /**
   * Where the fire goes from a building that is well alight.
   *
   * From the package the advisory is against, to the call site it named: that is the
   * one hop the scanner itself proved. From a file, to the files that import it -
   * against the arrow, because what is reachable from a file is reachable from
   * whatever calls it. Never to a package: a project's own code cannot set its
   * dependencies alight, whatever it does with them.
   */
  onward(id, fire) {
    const node = this.model.byId.get(id);
    if (!node) return [];
    if (node.kind === 'package') return fire.seat.into === id ? [] : [fire.seat.into];
    const out = [];
    for (const e of this.model.edgesTo.get(id) || []) {
      if (e.kind !== 'import') continue;
      const from = this.model.byId.get(e.from);
      if (from && from.kind === 'file') out.push(e.from);
    }
    return out;
  }

  /**
   * The extinguisher, held on one building for dt seconds. Returns what happened:
   * 'out' when this was the last of it, 'cooled' when that building is done but the
   * fire is still elsewhere, 'cooling' while it is still being worked on, and null
   * when there was nothing there to put out.
   *
   * Putting out the building the advisory names - the package - puts out everything
   * that fire lit, wherever it has got to. That is not a mercy: it is what upgrading
   * the dependency actually does, and a walker who works that out has learned the one
   * thing this whole business is for.
   */
  douse(id, dt) {
    const fire = this.lit.get(id);
    if (!fire) return null;
    fire.heat -= DOUSES * dt;
    if (fire.heat > 0) return 'cooling';
    const seat = fire.seat;
    const atSource = id === seat.from;
    this.quench(id);
    if (!atSource) return this.alightFrom(seat) ? 'cooled' : this.finish(seat);
    for (const [other, o] of [...this.lit]) if (o.seat === seat) this.quench(other);
    return this.finish(seat);
  }

  /** One building out, and cold for a while. */
  quench(id) {
    this.lit.delete(id);
    this.cooled.set(id, COOLS_FOR);
  }

  /** Whether any of this seat's fire is still going. */
  alightFrom(seat) {
    for (const fire of this.lit.values()) if (fire.seat === seat) return true;
    return false;
  }

  /** The last of a seat's fire is out, and it does not come back. */
  finish(seat) {
    this.out.add(seat.f.id);
    return 'out';
  }

  /** The findings whose fire is out, for the backpack to remember. */
  get doused() { return [...this.out]; }

  /** What the backpack remembers, put back: these do not light again. */
  keepDoused(ids) { this.out = new Set(ids); }
}
