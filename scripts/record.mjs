#!/usr/bin/env node
// Records a showcase video of depphunter running on this repository.
//
//     node scripts/record.mjs [--out DIR] [--scenes map,intro,...] [--config FILE]
//                             [--preview] [--headed] [--plan] [--encode]
//
// It builds depphunter into the temporary directory, serves this repository, drives
// the map and walk mode through the scenes below, and encodes the frames with the
// end card (endcard.html) into DIR/depphunter-showcase.mp4; DIR is showcase/ at the
// root unless --out says otherwise. The frames stay in DIR/frames, and --encode
// encodes them again without recording. It needs Playwright with
// Chromium in this repository's node_modules (npm install --no-save playwright &&
// npx playwright install chromium) and ffmpeg with libx264.
//
// Frames are taken one at a time rather than filmed. The page's clock
// (requestAnimationFrame, performance.now, timers) is frozen and stepped by exactly
// 1/FPS, and each frame is captured after its step, so the video is smooth at FPS
// however long a frame takes to draw. Without a GPU walk mode takes seconds a frame,
// and the full video hours; --headed draws in a visible browser, which uses one.
// --preview records at half the resolution and a third of the rate, to check what
// the scenes do before paying for them; the page is laid out the same either way.
//
// What happens is not written here: scripts/record-cfg.json (or --config) names the
// scenes in order and lists each one's steps, and this script only knows how to
// carry out a step. Each step is an object whose "do" names the action:
//
//   caption  text, sub          the caption at the bottom ("" hides it)
//   hold     s                  let s seconds pass
//   glide    to, s              the pointer to [x, y] (fractions of the view) or a selector
//   click    target, s          glide to target and click it
//   type     text               type, one key every other frame
//   set      target, value      set a field's value, no frame of its own
//   choose   target, value      set a field's value and show it for a frame
//   pick     target, value, s, down
//                               click a select open, go down its list and click value
//   key      key                press a key
//   cursor   hidden             hide or show the drawn pointer
//   hide     targets            take the overlays matching these selectors out of shot
//   show     targets            ... and put them back
//   walk     hide               enter walk mode, if not in it yet, with hide out of shot;
//                               the first time, the arrival over the city is filmed
//   catch    tool, distance, held, read
//                               walk up to the nearest bug and catch it with tool
//   douse    s, back            put out the nearest fire with the extinguisher
//   photo    show               photograph the tallest building near, then view it
//   grapple  hops, near, far, rise
//                               grapple roof to roof, hops times, to towers near..far
//                               apart and at most rise higher or lower
//   fly      s, speed, turn, above
//                               fly the jet backpack in a banking climb
//
// Every step's length is known before it runs, so the progress line gives the
// share done, the time a frame takes and how long is left, and --plan prints what
// each scene will take without recording anything.
import { chromium } from 'playwright';
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';

const REPO = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..');

// ------------------------------------------------------------------ arguments

const argv = process.argv.slice(2);
const opt = (name, fallback) => { const i = argv.indexOf(`--${name}`); return i >= 0 ? argv[i + 1] : fallback; };
const flag = name => argv.includes(`--${name}`);
const PREVIEW = flag('preview');
// Not in the temporary directory: an ffmpeg installed as a snap has a private /tmp
// of its own and would find no frames there.
const OUT = path.resolve(opt('out', path.join(REPO, 'showcase')));
const FPS = Number(opt('fps', PREVIEW ? 10 : 30));
// The page is always laid out at 1280x720, so a preview looks like the video; a
// preview only draws it at half the resolution, which is a quarter of the pixels.
const VIEW = { width: 1280, height: 720 };
const SCALE = PREVIEW ? 0.5 : 1;
const CONFIG = JSON.parse(fs.readFileSync(path.resolve(opt('config', path.join(REPO, 'scripts/record-cfg.json'))), 'utf8'));
const SCENES = opt('scenes', CONFIG.order.join(',')).split(',');
for (const name of SCENES) if (!CONFIG.scenes[name]) throw new Error(`no scene called ${name} in the config`);
const PORT = Number(opt('port', 0)); // 0: whatever port is free
const STEP = 1000 / FPS;
const EYE = 0.45; // walk.js: eye height above the feet

// ------------------------------------------------------------------ findings

// Reports for the map to place: lint issues on real Go files (bugs on buildings),
// advisories against the Go modules nothing calls (bugs on the island), and one the
// code reaches (a building on fire). They are made up; the files are not.
function writeReports(dir) {
  const files = execFileSync('git', ['ls-files', 'internal/*.go', 'cmd/*.go'], { cwd: REPO, encoding: 'utf8' })
    .split('\n').filter(f => f && !f.endsWith('_test.go') && !f.includes('/testdata/'));
  const msgs = [
    ['errcheck', 'Error return value of `w.Write` is not checked', 'error'],
    ['govet', 'printf: fmt.Sprintf format %d has arg of wrong type', 'error'],
    ['gosec', 'G304: Potential file inclusion via variable', 'error'],
    ['staticcheck', 'SA4006: this value of `err` is never used', 'warning'],
    ['unused', 'func `helper` is unused', 'warning'],
    ['ineffassign', 'ineffectual assignment to `n`', 'warning'],
    ['revive', 'exported function should have comment', 'info'],
  ];
  let seed = 7;
  const rnd = () => ((seed = (seed * 1103515245 + 12345) % 2 ** 31) / 2 ** 31);
  const issues = files.filter(() => rnd() < 0.6).map((f, i) => {
    const [linter, text, severity] = msgs[i % msgs.length];
    return { FromLinter: linter, Text: text, Severity: severity, Pos: { Filename: f, Line: 10 + Math.floor(rnd() * 200), Column: 2 } };
  });
  fs.writeFileSync(path.join(dir, 'golangci.json'), JSON.stringify({ Issues: issues, Report: { Linters: [] } }));

  const main = 'github.com/sarumaj/depphunter-cli';
  const lines = [
    { osv: { id: 'GO-0000-0001', summary: 'Decoder reads past the end of a buffer', database_specific: { severity: 'CRITICAL' } } },
    { finding: { osv: 'GO-0000-0001', fixed_version: 'v0.3.8', trace: [
      { module: 'golang.org/x/text', version: 'v0.3.7', package: 'golang.org/x/text/encoding/unicode', function: 'Decode' },
      { module: main, package: `${main}/internal/scan`, function: 'Scan', position: { filename: 'internal/scan/scan.go', line: 48, column: 1 } },
    ] } },
  ];
  ['golang.org/x/sys', 'golang.org/x/mod', 'github.com/spf13/viper', 'github.com/spf13/cobra', 'gopkg.in/yaml.v3',
    'github.com/fsnotify/fsnotify', 'golang.org/x/sync'].forEach((m, i) => {
    const id = `GO-2026-${1000 + i}`;
    lines.push({ osv: { id, summary: `Advisory in ${m}`, affected: [{ package: { ecosystem: 'Go', name: m } }],
      database_specific: { severity: ['CRITICAL', 'HIGH', 'MODERATE', 'LOW'][i % 4] } } });
    lines.push({ finding: { osv: id, fixed_version: 'v9.9.9', trace: [{ module: m, version: 'v0.0.1' }] } });
  });
  fs.writeFileSync(path.join(dir, 'govuln.json'), lines.map(l => JSON.stringify(l)).join('\n'));
  console.log(`reports: ${issues.length} lint issues, 8 advisories`);
}

// ------------------------------------------------------------------ models

// The 3D models are Git LFS objects. A checkout without `git lfs pull` holds only
// their pointers, and the map then draws stand-ins; fetch the real files instead.
async function models() {
  const out = {};
  for (const name of ['hand', 'bug', 'props']) {
    const file = path.join(REPO, 'web/static', `${name}.glb`);
    const head = fs.readFileSync(file).subarray(0, 4).toString();
    if (head === 'glTF') continue; // the real thing is already embedded
    const cache = path.join(os.tmpdir(), `depphunter-${name}.glb`);
    if (!fs.existsSync(cache)) {
      const url = `https://media.githubusercontent.com/media/sarumaj/depphunter-cli/main/web/static/${name}.glb`;
      console.log(`fetching ${name}.glb (the checkout holds an LFS pointer)`);
      const res = await fetch(url);
      if (!res.ok) throw new Error(`${url}: ${res.status}; run git lfs pull instead`);
      fs.writeFileSync(cache, Buffer.from(await res.arrayBuffer()));
    }
    out[name] = fs.readFileSync(cache);
  }
  return out;
}

// ------------------------------------------------------------------ actions

// Where the story goes on foot: beside the building with the most bugs in its street.
let stage = null;
async function findStage() {
  stage = await dh(d => {
    const w = d.walker;
    const count = new Map();
    for (const b of d.bugs?.bugs || []) if (!b.flying && b.lap.kind === 'street' && b.box.kind === 'building') count.set(b.box, (count.get(b.box) || 0) + 1);
    const box = [...count.entries()].sort((a, b) => b[1] - a[1])[0]?.[0];
    if (!box) return null;
    const keep = { ...w.p };
    w.teleport(box);
    const spot = { x: w.p.x, z: w.p.z, feet: w.p.feet, yaw: w.p.yaw, pitch: 0 };
    Object.assign(w.p, keep);
    return { spot, box: { x: box.x, z: box.z, top: box.y + box.h }, bugs: count.get(box) };
  });
  say(`  stage: ${JSON.stringify(stage)}`);
}

/**
 * The nearest bug walking a street, which is the one a walker can stand beside:
 * one on a wall or a roof, or in the air, is out of a net's reach from the ground.
 * Returns its index and the spot `dist` out from its building, in the street it
 * walks, or null.
 */
async function streetBug(from, dist) {
  return dh((d, a) => {
    const w = d.walker;
    let best = null, bd = Infinity;
    (d.bugs?.bugs || []).forEach((b, i) => {
      if (b.caught || b.flying || b.lap.kind !== 'street') return;
      // Out from the building the bug circles, so the spot is in its street and not
      // inside the building across it; a spot that is not at the bug's level is not.
      const ox = b.pos.x - b.box.x, oz = b.pos.z - b.box.z, len = Math.hypot(ox, oz) || 1;
      const spot = { x: b.pos.x + (ox / len) * a.dist, z: b.pos.z + (oz / len) * a.dist };
      spot.feet = w.height(spot.x, spot.z);
      if (Math.abs(spot.feet - (b.pos.y - 0.04)) > 0.2) return;
      const far = Math.hypot(b.pos.x - a.x, b.pos.z - a.z);
      if (far < bd) { bd = far; best = { i, spot }; }
    });
    return best;
  }, { x: from.x, z: from.z, dist });
}
/**
 * Where to stand to face bug i now: `dist` out from its building, level with it.
 * The bug keeps walking its lap, so the spot moves with it, around corners too.
 */
const spotOf = (i, dist) => dh((d, a) => {
  const b = d.bugs.bugs[a.i];
  const ox = b.pos.x - b.box.x, oz = b.pos.z - b.box.z, len = Math.hypot(ox, oz) || 1;
  return { x: b.pos.x + (ox / len) * a.dist, z: b.pos.z + (oz / len) * a.dist, feet: b.pos.y - 0.04 };
}, { i, dist });
/**
 * Turns the walker, as little as it takes, until the crosshair is on bug i as the
 * game itself finds it (walker.updateAim marches the ray through the bent view):
 * angles worked out on the flat map can leave it on the wall behind a small bug,
 * and a tool fired then tags the wall. Returns whether it is on the bug.
 */
const aimOn = i => dh((d, a) => {
  const w = d.walker, p = w.p, bug = d.bugs.bugs[a.i];
  const yaw = p.yaw, pitch = p.pitch;
  const offsets = [];
  for (let y = -15; y <= 15; y++) for (let x = -15; x <= 15; x++) offsets.push([x * 0.008, y * 0.008]);
  offsets.sort((u, v) => Math.hypot(...u) - Math.hypot(...v));
  for (const [dy, dp] of offsets) {
    w.scene.setWalker(p.x, p.feet, p.z, a.eye, yaw + dy, pitch + dp);
    w.updateAim();
    if (w.aim.bug === bug) { p.yaw = yaw + dy; p.pitch = pitch + dp; return true; }
  }
  w.scene.setWalker(p.x, p.feet, p.z, a.eye, yaw, pitch);
  w.updateAim();
  return false;
}, { i, eye: EYE });
const bugAt = i => dh((d, i) => { const b = d.bugs.bugs[i]; return { x: b.pos.x, y: b.pos.y + 0.05, z: b.pos.z, caught: b.caught }; }, i);
/**
 * The next grapple hop: from tower `from` (a box index; null picks the first tower
 * too) to one near..far away and at most `rise` higher or lower, not yet stood on,
 * ahead along `dir` where it can be. Returns where to stand on the first roof, the
 * yaw and pitch at which the gun's sight line (walker.lookingAt) meets the other
 * tower just under its roof, and both towers, or null.
 */
const planHop = args => dh((d, a) => {
  const w = d.walker, p = w.p, top = b => b.y + b.h;
  const reach = w.secondary?.reach ?? 50;
  const tall = w.boxes.map((b, i) => ({ b, i })).filter(o => o.b.kind === 'building' && o.b.h > 1);
  const sight = (A, B) => {
    const dx = B.x - A.x, dz = B.z - A.z, len = Math.hypot(dx, dz) || 1, ux = dx / len, uz = dz / len;
    const out = (box, m) => Math.max(0, Math.min((box.w / 2 - m) / (Math.abs(ux) || 1e-9), (box.d / 2 - m) / (Math.abs(uz) || 1e-9)));
    const s = out(A, 0.35), f = out(B, 0);
    const stand = { x: A.x + ux * s, z: A.z + uz * s, feet: top(A) };
    const face = { x: B.x - ux * f, z: B.z - uz * f };
    const yaw = Math.atan2(-(face.x - stand.x), -(face.z - stand.z));
    const base = Math.atan2(top(B) - 0.2 - (stand.feet + a.eye), Math.hypot(face.x - stand.x, face.z - stand.z));
    let best = null;
    for (let k = -40; k <= 25; k++) {
      const pitch = base + k * 0.004;
      w.scene.setWalker(stand.x, stand.feet, stand.z, a.eye, yaw, pitch);
      const hit = w.lookingAt(reach);
      if (hit && hit.box === B && hit.point.y < top(B) - 0.05 && (!best || hit.point.y > best.y)) best = { pitch, y: hit.point.y };
    }
    return best && { stand, yaw, pitch: best.pitch, dist: len, dir: { x: ux, z: uz } };
  };
  const mid = (a.near + a.far) / 2;
  const pairs = [];
  const starts = a.from === null ? tall.slice().sort((x, y) => top(y.b) - top(x.b)).slice(0, 40) : [{ b: w.boxes[a.from], i: a.from }];
  for (const A of starts) for (const B of tall) {
    if (A.i === B.i || a.visited.includes(B.i)) continue;
    const dist = Math.hypot(A.b.x - B.b.x, A.b.z - B.b.z), rise = top(B.b) - top(A.b);
    if (dist < a.near || dist > a.far || Math.abs(rise) > a.rise) continue;
    let score = -Math.abs(dist - mid) * 0.15 - Math.abs(rise) * 0.5;
    if (a.from === null) score += top(A.b) * 0.5;
    if (a.dir) score += ((B.b.x - A.b.x) * a.dir.x + (B.b.z - A.b.z) * a.dir.z) / dist * 3;
    pairs.push({ A, B, score });
  }
  pairs.sort((x, y) => y.score - x.score);
  let plan = null;
  for (const { A, B } of pairs.slice(0, 80)) {
    const s = sight(A.b, B.b);
    if (s) { plan = { ...s, ai: A.i, bi: B.i, top: top(B.b) }; break; }
  }
  w.scene.setWalker(p.x, p.feet, p.z, a.eye, p.yaw, p.pitch);
  return plan;
}, { ...args, eye: EYE });
const at = (sel) => (Array.isArray(sel) ? { x: VIEW.width * sel[0], y: VIEW.height * sel[1] } : center(sel));

// Each action is `run` (does it) and `length` (the seconds it will take, which is
// what the progress line counts against; a step that can end early is counted at
// its longest).
const CLICK = 0.7, TAKE = 2.5, APPROACH = 2.5, SETTLE = 0.3, OPENS = 1.3;
const HOP_AIM = 1.4, GRAPPLE_BITE = 1.2, GRAPPLE_REEL = 4;
const LIST_OPEN = 0.5, LIST_READ = 0.35;
const ARRIVAL = 5; // walk.js ARRIVAL: the first walk's flight in
const actions = {
  caption: { length: () => 0, run: st => caption(st.text ?? '', st.sub ?? '') },
  hold: { length: st => st.s, run: st => hold(st.s) },
  glide: {
    length: st => st.s ?? 0.8,
    run: async st => { const p = await at(st.to); await glide(p.x, p.y, st.s ?? 0.8); },
  },
  click: { length: st => (st.s ?? CLICK) + 2 / FPS, run: st => clickAt(st.target, st.s ?? CLICK) },
  type: { length: st => (2 * st.text.length) / FPS, run: st => typeSlowly(st.text) },
  set: { length: () => 0, run: st => choose(st.target, st.value) },
  choose: { length: () => 1 / FPS, run: st => frame(() => choose(st.target, st.value)) },
  key: { length: () => 1 / FPS, run: st => frame(() => page.keyboard.press(st.key)) },
  // A select, chosen the way a person does it: a click opens the list, the pointer
  // goes down it to the option, and a click there takes it. The list is drawn into
  // the page, because a native one is not in a screenshot (and, left open, would be
  // in every frame after it).
  pick: {
    length: st => (st.s ?? CLICK) + 1 / FPS + LIST_OPEN + (st.down ?? 0.6) + 1 / FPS + LIST_READ,
    async run(st) {
      const c = await center(st.target);
      await glide(c.x, c.y, st.s ?? CLICK);
      let opt;
      await frame(async () => {
        await page.evaluate(({ x, y }) => window.__ripple(x, y), c);
        opt = await openList(st.target, st.value);
      });
      await hold(LIST_OPEN);
      await glide(opt.x, opt.y, st.down ?? 0.6);
      await frame(async () => {
        await page.evaluate(({ x, y }) => window.__ripple(x, y), opt);
        await page.evaluate(() => document.getElementById('promo-list')?.classList.add('taken'));
      });
      await hold(LIST_READ);
      await page.evaluate(() => document.getElementById('promo-list')?.remove());
      await choose(st.target, st.value);
    },
  },
  // Overlays out of shot, for a clean picture: the walk HUD's key list, a hover card,
  // the status line. `show` brings back what `hide` took away.
  hide: { length: () => 0, run: st => overlays(st.targets, true) },
  show: { length: () => 0, run: st => overlays(st.targets, false) },
  cursor: { length: () => 0, run: st => hideCursor(!!st.hidden) },

  // Into walk mode, with `hide` out of shot from the first frame. The first walk on
  // the page is flown in (walk.js startArrival); it is landed in the street the story
  // goes on in, and filmed until it is down.
  walk: {
    length: () => CLICK + 2 / FPS + ARRIVAL,
    async run(st) {
      if (await dh(d => d.walker.active)) { if (st.hide) await overlays(st.hide, true); return ready(); }
      const c = await center('#walk');
      await glide(c.x, c.y, CLICK);
      await frame(() => page.mouse.down());
      await frame(async () => {
        await page.mouse.up();
        if (st.hide) await overlays(st.hide, true);
        await hideCursor(true);
        if (!stage) await findStage();
        await dh((d, spot) => {
          const a = d.walker.arrival;
          if (a && spot) { Object.assign(a.land, spot, { pitch: 0.05 }); a.t = 0; }
        }, stage?.spot);
      });
      for (let k = 0; k < frames_(ARRIVAL + 1) && await dh(d => !!d.walker.arrival); k++) await frame();
    },
  },


  // Walks up to the nearest bug, aims at it, uses the tool and lets the catch play
  // out. A catch opens its finding and holds the walker, as the map does; after
  // `read` seconds of it the walk goes on.
  catch: {
    length: st => APPROACH + SETTLE + (st.held ?? 0) + TAKE + OPENS + (st.read ?? 0) + 0.4,
    async run(st) {
      await ready();
      let me = await state();
      const found = await streetBug(me, st.distance);
      if (!found) { say(`  ${st.tool}: no bug left in the streets`); return; }
      const { i, spot } = found;
      await dh((d, id) => { if (d.walker.primary.id !== id) d.walker.setTool(id); }, st.tool);
      const from = { ...me };
      // Walked, roughly: a fifth of the way a second, however far, within limits.
      const walkTime = Math.min(APPROACH, Math.max(0.8, Math.hypot(spot.x - me.x, spot.z - me.z) / 5));
      // Toward the bug's spot as it is each frame, not as it was when chosen: a bug
      // that rounds a corner meanwhile would leave the walker facing a wall.
      await hold(walkTime, async t => {
        const e = ease(t), to = await spotOf(i, st.distance);
        const here = { x: lerp(from.x, to.x, e), z: lerp(from.z, to.z, e), feet: lerp(from.feet, to.feet, e) };
        const want = lookAngles(here, await bugAt(i));
        return view({ ...here, yaw: turn(from.yaw, want.yaw, e), pitch: lerp(from.pitch, want.pitch, e) });
      });
      // Then stays with it, a step behind so a turn at a corner is walked, not jumped.
      const track = async () => {
        const to = await spotOf(i, st.distance);
        me = await state();
        const here = { x: lerp(me.x, to.x, 0.3), z: lerp(me.z, to.z, 0.3), feet: lerp(me.feet, to.feet, 0.3) };
        await view({ ...here, ...lookAngles(here, await bugAt(i)) });
        return aimOn(i);
      };
      await hold(SETTLE, track);
      if (st.held) {
        await dh(d => { d.walker.firing = true; d.walker.fire(); });
        await hold(st.held, track);
        await dh(d => { d.walker.firing = false; });
      } else {
        await frame(async () => { await track(); await dh(d => d.walker.fire()); });
      }
      for (let k = 0; k < frames_(TAKE); k++) {
        await frame(track);
        if ((await bugAt(i)).caught) break;
      }
      const caught = (await bugAt(i)).caught;
      say(`  ${st.tool}: bug ${i} ${caught ? 'caught' : 'missed'}`);
      // The finding opens once the catch has played out (TAKE_MS in bugs.js, about a
      // second), not at once: wait for it, so it is read here rather than landing on
      // whatever the next step is doing.
      for (let k = 0; caught && k < frames_(OPENS) && !(await dh(d => d.walker.frozen)); k++) await frame();
      if (await dh(d => d.walker.frozen)) {
        await hold(st.read ?? 0);
        await resume();
      }
      await hold(0.4);
    },
  },

  douse: {
    length: st => (st.back != null ? 1.4 : 0) + st.s,
    async run(st) {
      await ready();
      const target = await dh(d => {
        const ids = [...(d.fires?.lit?.keys() || [])];
        const box = d.walker.boxes.find(b => b.kind === 'building' && ids.includes(b.node?.id));
        if (!box) return null;
        const keep = { ...d.walker.p };
        d.walker.teleport(box);
        const spot = { x: d.walker.p.x, z: d.walker.p.z, feet: d.walker.p.feet };
        Object.assign(d.walker.p, keep);
        return { id: box.node.id, x: box.x, z: box.z, y: box.y, h: box.h, spot };
      });
      if (!target) { say('  douse: nothing is burning'); return; }
      say(`  douse: ${target.id} is alight`);
      await dh(d => { if (d.walker.primary.id !== 'extinguisher') d.walker.setTool('extinguisher'); });
      // Back off the wall a little, so the building and its flames are in frame.
      const away = Math.atan2(target.spot.x - target.x, target.spot.z - target.z);
      const r0 = Math.hypot(target.spot.x - target.x, target.spot.z - target.z) + (st.back ?? 1.4);
      const spot = { x: target.x + Math.sin(away) * r0, z: target.z + Math.cos(away) * r0 };
      spot.feet = await groundAt(spot.x, spot.z);
      const roof = { x: target.x, y: target.y + target.h * 0.85, z: target.z };
      const from = await state();
      await hold(1.4, t => {
        const e = ease(t);
        const here = { x: lerp(from.x, spot.x, e), z: lerp(from.z, spot.z, e), feet: lerp(from.feet, spot.feet, e) };
        const want = lookAngles(here, roof);
        return view({ ...here, yaw: turn(from.yaw, want.yaw, e), pitch: lerp(from.pitch, want.pitch, e) });
      });
      await dh(d => { d.walker.firing = true; });
      for (let k = 0; k < frames_(st.s); k++) {
        await frame();
        if (k % 5 === 0 && !(await dh((d, id) => d.fires.lit.has(id), target.id))) break;
      }
      await dh(d => { d.walker.firing = false; });
      say(`  douse: ${(await dh((d, id) => d.fires.lit.has(id), target.id)) ? 'still burning' : 'out'}`);
    },
  },

  // The camera on the tallest building near, then the photographs: G opens them, and
  // "show" puts one back up on the camera for `show` seconds.
  photo: {
    length: st => 1.2 + 0.4 + 1 / FPS + 1.4 + 1 / FPS + 0.8 + 0.9 + 2 / FPS + (st.show ?? 3.2),
    async run(st) {
      await ready();
      await dh(d => { if (d.walker.primary.id !== 'camera') d.walker.setTool('camera'); });
      const subject = await dh(d => {
        const p = d.walker.p;
        const near = d.walker.boxes.filter(b => b.kind === 'building' && Math.hypot(b.x - p.x, b.z - p.z) < 14);
        const b = near.sort((a, c) => (c.y + c.h) - (a.y + a.h))[0];
        return b && { x: b.x, y: b.y + b.h * 0.7, z: b.z };
      });
      const from = await state();
      const want = subject ? lookAngles(from, subject) : from;
      await hold(1.2, t => view({ yaw: turn(from.yaw, want.yaw, ease(t)), pitch: lerp(from.pitch, want.pitch, ease(t)) }));
      await hold(0.4);
      await frame(() => dh(d => d.walker.fire()));
      await hold(1.4);
      await hideCursor(false);
      await frame(() => page.keyboard.press('g'));
      await hold(0.8);
      const button = '#stash-list li button.link:not(.keep)';
      if (await page.locator(button).count()) {
        await clickAt(button, 0.9);
        await hideCursor(true);
        await hold(st.show ?? 3.2);
      } else {
        say('  photo: nothing in the stash');
        await frame(() => page.keyboard.press('g'));
      }
      await dh(d => { if (d.walker.showing) d.walker.endShow(); });
      await resume();
    },
  },

  // Roof to roof across the city, `hops` times: stand at the edge of a tower's roof,
  // aim the grapple at the top of a facade near..far away, and ride the line up onto
  // that roof. Every aim is checked with the gun's own sight line first, so the hook
  // bites the tower meant and not a roof or a tree in between.
  grapple: {
    length: st => 1.8 + (st.hops ?? 2) * (HOP_AIM + 0.3 + GRAPPLE_BITE + GRAPPLE_REEL) + 1.6,
    async run(st) {
      await ready();
      await dh(d => { if (d.walker.secondary?.id !== 'grapple') d.walker.setTool('grapple'); });
      const hops = st.hops ?? 2;
      let from = null, dir = null;
      const visited = [];
      for (let h = 0; h < hops; h++) {
        const plan = await planHop({ from, visited, dir, near: st.near ?? 6, far: st.far ?? 22, rise: st.rise ?? 3 });
        if (!plan) { say(`  grapple: no tower in reach for hop ${h + 1}`); break; }
        const aim = { yaw: plan.yaw, pitch: plan.pitch };
        if (from === null) {
          // Arriving on the first roof: a turn from the city onto the next tower.
          await hold(1.8, t => view({ ...plan.stand, yaw: turn(aim.yaw + 1.6, aim.yaw, ease(t)), pitch: lerp(-0.25, aim.pitch, ease(t)) }));
        } else {
          // A walk to the roof's edge, turning onto the next tower on the way.
          const me = await state();
          await hold(HOP_AIM, t => { const e = ease(t);
            return view({ x: lerp(me.x, plan.stand.x, e), z: lerp(me.z, plan.stand.z, e), feet: plan.stand.feet,
              yaw: turn(me.yaw, aim.yaw, e), pitch: lerp(me.pitch, aim.pitch, e) }); });
        }
        await hold(0.3, () => view({ ...plan.stand, ...aim }));
        await frame(() => dh(d => d.walker.useSecondary()));
        // The hook's flight, then the reel, with the view held on where it bit.
        let bit = false;
        for (let k = 0; k < frames_(GRAPPLE_BITE) && !bit; k++) { await frame(); bit = await dh(d => !!d.walker.pull); }
        for (let k = 0; bit && k < frames_(GRAPPLE_REEL); k++) { await frame(); if (!(await dh(d => !!d.walker.pull))) break; }
        const me = await state();
        const onto = await dh((d, a) => { const b = d.walker.boxes[a.bi];
          return Math.abs(a.x - b.x) <= b.w / 2 && Math.abs(a.z - b.z) <= b.d / 2 && Math.abs(a.feet - (b.y + b.h)) < 0.3; }, { bi: plan.bi, ...me });
        say(`  grapple hop ${h + 1}: ${bit ? 'bit' : 'did not bite'}, ${onto ? 'onto' : 'not on'} tower ${plan.bi} (${plan.dist.toFixed(1)} away, roof at ${plan.top.toFixed(2)}, feet at ${me.feet.toFixed(2)})`);
        if (!onto) break;
        visited.push(plan.ai);
        dir = plan.dir;
        from = plan.bi;
      }
      // Looking back down over the edge at the way come.
      const me = await state();
      await hold(1.6, t => view({ yaw: me.yaw + 0.6 * ease(t), pitch: lerp(me.pitch, -0.45, ease(t)) }));
    },
  },

  // The jet backpack: a burst of thrust, then a banking climb over the roofs.
  fly: {
    length: st => 1 / FPS + st.s,
    async run(st) {
      await ready();
      await dh(d => { if (d.walker.secondary?.id !== 'jetpack') d.walker.setTool('jetpack'); });
      const top = await dh(d => Math.max(...d.walker.boxes.map(b => b.y + b.h)));
      const from = await state();
      const lift = Math.max(from.feet + 4, top + (st.above ?? 3));
      let yaw = from.yaw, here = { ...from };
      await frame(() => dh(d => d.walker.useSecondary()));
      await hold(st.s, t => {
        yaw += (st.turn ?? 0.35) / FPS;
        here = {
          x: here.x - Math.sin(yaw) * (st.speed ?? 5) / FPS,
          z: here.z - Math.cos(yaw) * (st.speed ?? 5) / FPS,
          feet: lerp(from.feet, lift, ease(Math.min(1, t * 2.5))),
        };
        return view({ ...here, yaw, pitch: lerp(0.05, -0.32, ease(Math.min(1, t * 2))) });
      });
    },
  },
};

// ------------------------------------------------------------------ encoding

/**
 * The frames in OUT/frames and the end card, as OUT/depphunter-showcase.mp4. The end
 * card is drawn from endcard.html at the root, at the recording's size.
 */
async function encode({ frames: count, fps, scale }) {
  const size = { width: VIEW.width * scale, height: VIEW.height * scale };
  const browser = await chromium.launch();
  const card = await browser.newPage({ viewport: VIEW, deviceScaleFactor: scale });
  await card.goto(`file://${path.join(REPO, 'endcard.html')}`);
  await card.waitForTimeout(400);
  await card.screenshot({ path: path.join(OUT, 'endcard.png') });
  await browser.close();

  const CARD = 4.5;
  const total = count / fps + CARD;
  const video = path.join(OUT, 'depphunter-showcase.mp4');
  try {
    execFileSync('ffmpeg', ['-v', 'error', '-y',
      '-framerate', String(fps), '-i', path.join(OUT, 'frames', 'f%05d.jpg'),
      '-loop', '1', '-framerate', String(fps), '-t', String(CARD), '-i', path.join(OUT, 'endcard.png'),
      '-filter_complex', `[0:v]scale=${size.width}:${size.height},format=yuv420p,setsar=1[a];` +
        `[1:v]scale=${size.width}:${size.height},format=yuv420p,setsar=1[b];` +
        `[a][b]concat=n=2:v=1:a=0,fade=t=in:st=0:d=0.6,fade=t=out:st=${(total - 0.8).toFixed(2)}:d=0.8[v]`,
      '-map', '[v]', '-c:v', 'libx264', '-preset', 'slow', '-crf', '18', '-pix_fmt', 'yuv420p',
      '-movflags', '+faststart', '-r', String(fps), video], { stdio: 'inherit' });
  } catch (e) {
    const tmp = [os.tmpdir(), '/tmp'].some(t => OUT.startsWith(t + path.sep));
    throw new Error(`ffmpeg could not encode ${path.join(OUT, 'frames')}: ${e.message.split('\n')[0]}. ` +
      (e.code === 'ENOENT' ? 'ffmpeg is not installed, or not on the PATH. '
        : tmp ? 'An ffmpeg installed as a snap cannot read files in /tmp; use --out somewhere in your home. ' : '') +
      'The frames are kept, and --encode encodes them again without recording.');
  }
  console.log(`wrote ${video} (${total.toFixed(1)} s)`);
}

// --encode: the frames of an earlier recording, encoded again without recording.
if (flag('encode')) {
  const dir = path.join(OUT, 'frames');
  const count = fs.existsSync(dir) ? fs.readdirSync(dir).filter(f => /^f\d{5}\.jpg$/.test(f)).length : 0;
  if (!count || !fs.existsSync(path.join(dir, 'f00000.jpg'))) {
    throw new Error(`no frames to encode in ${dir}: ${count ? 'the first one, f00000.jpg, is missing' : 'it is empty or not there'}` +
      (fs.existsSync(`${dir}.previous`) ? `; the recording before the last one is in ${dir}.previous` : ''));
  }
  const meta = path.join(OUT, 'recording.json');
  await encode(fs.existsSync(meta) ? JSON.parse(fs.readFileSync(meta, 'utf8'))
    : { frames: count, fps: FPS, scale: SCALE });
  process.exit(0);
}

// ------------------------------------------------------------------ the plan

for (const name of SCENES) {
  for (const st of CONFIG.scenes[name]) if (!actions[st.do]) throw new Error(`${name}: no action called ${st.do}`);
}
const lengthOf = name => CONFIG.scenes[name].reduce((s, st) => s + Math.round(actions[st.do].length(st) * FPS), 0);
let planned = Math.max(1, SCENES.reduce((sum, name) => sum + lengthOf(name), 0));
if (flag('plan')) {
  for (const name of SCENES) console.log(`${name.padEnd(10)} ${String(lengthOf(name)).padStart(5)} frames  ${(lengthOf(name) / FPS).toFixed(1).padStart(5)} s`);
  console.log(`${'total'.padEnd(10)} ${String(planned).padStart(5)} frames  ${(planned / FPS).toFixed(1).padStart(5)} s at ${FPS} fps, plus the end card`);
  process.exit(0);
}

// ------------------------------------------------------------------ the rig

fs.mkdirSync(OUT, { recursive: true });
const BIN = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'depphunter-record-')), 'depphunter');
console.log('building depphunter...');
execFileSync('go', ['build', '-o', BIN, './cmd/depphunter'], { cwd: REPO, stdio: 'inherit' });
const frames = path.join(OUT, 'frames');
// The last recording's frames are kept aside rather than deleted: a run that fails
// before its first frame would otherwise take a finished recording with it.
if (fs.existsSync(frames) && fs.readdirSync(frames).length) {
  fs.rmSync(`${frames}.previous`, { recursive: true, force: true });
  fs.renameSync(frames, `${frames}.previous`);
}
fs.rmSync(frames, { recursive: true, force: true });
fs.mkdirSync(frames, { recursive: true });
writeReports(OUT);
const glb = await models();

const srv = spawn(BIN, [REPO, '--addr', `127.0.0.1:${PORT}`, '--no-open',
  '--findings', path.join(OUT, 'golangci.json'), '--findings', path.join(OUT, 'govuln.json')]);
// The server goes when this does, however that happens: a server left behind by an
// interrupted run keeps its port, and the next run's server cannot have it.
let browser = null;
const stop = () => { if (srv.exitCode === null) srv.kill(); };
process.on('exit', stop);
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => { stop(); process.exit(130); });
process.on('uncaughtException', e => { stop(); console.error(e); process.exit(1); });
process.on('unhandledRejection', e => { stop(); console.error(e); process.exit(1); });

let url = '', said = '', exited = null;
const grab = d => {
  said += String(d);
  const m = /http:\/\/\S+/.exec(said);
  if (m && !url) url = m[0];
};
srv.stdout.on('data', grab);
srv.stderr.on('data', grab);
srv.on('exit', code => { exited = code; });
srv.on('error', e => { exited = e.message; });
for (let i = 0; i < 240 && !url && exited === null; i++) await new Promise(r => setTimeout(r, 500));
if (!url) {
  throw new Error(`depphunter ${exited === null ? 'said nothing about where it was serving in 2 minutes'
    : `stopped (${exited}) before serving`}; it said:\n${said.trim() || '(nothing)'}`);
}
console.log('serving at', url);

browser = await chromium.launch(flag('headed')
  ? { headless: false, args: ['--ignore-gpu-blocklist', `--window-size=${VIEW.width},${VIEW.height + 120}`] }
  : { args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const ctx = await browser.newContext({ viewport: VIEW, deviceScaleFactor: SCALE });
// The director's handle: app.js keeps the walker and the rest in module scope, so the
// copy served to this browser also exports them. Nothing else about it changes.
await ctx.route('**/app.js*', async route => {
  const res = await route.fetch();
  const body = await res.text();
  await route.fulfill({ response: res, body: `${body}
window.__dh = { get walker() { return walker; }, get bugs() { return bugs; }, get fires() { return fires; },
  get stash() { return stash; }, get scene() { return scene; } };` });
});
for (const [name, body] of Object.entries(glb)) {
  await ctx.route(`**/${name}.glb*`, route => route.fulfill({ contentType: 'model/gltf-binary', body }));
}
await ctx.addInitScript(() => {
  // Straight to the map: the introductions are for people, not for a camera.
  localStorage.setItem('depphunter.introduced', '1');
  localStorage.setItem('depphunter.introduced.walk', '1');
  // A drawn pointer, since screenshots leave the real one out: an arrow that follows
  // the mouse, a ripple on every press, hidden while the pointer is locked or the
  // director says so.
  addEventListener('DOMContentLoaded', () => {
    // The text caret blinks on the browser's own clock, not the page's, and a frame
    // here takes anything up to seconds: filmed, it flickered at random. It is held
    // still and blinked on the page's clock instead, at the usual half-second, which
    // plays back at the right speed; a browser that cannot hold it still hides it
    // (CSS caret-animation, Chromium 139 and later).
    const caret = document.createElement('style');
    caret.textContent = CSS.supports('caret-animation', 'manual')
      ? 'input, textarea { caret-animation: manual; } .promo-caret-off input, .promo-caret-off textarea { caret-color: transparent !important; }'
      : 'input, textarea { caret-color: transparent !important; }';
    document.head.appendChild(caret);
    // Shown again on every keystroke and focus, as a real caret is.
    let blink = 0;
    const restart = () => {
      document.documentElement.classList.remove('promo-caret-off');
      clearInterval(blink);
      blink = setInterval(() => document.documentElement.classList.toggle('promo-caret-off'), 530);
    };
    restart();
    addEventListener('input', restart, true);
    addEventListener('focusin', restart, true);
    const c = document.createElement('div');
    c.innerHTML = '<svg width="26" height="26" viewBox="0 0 26 26"><path d="M3 2 L3 21 L8.2 16.2 L11.6 23.6 L15 22.1 L11.7 14.9 L18.6 14.9 Z" fill="#fff" stroke="#111" stroke-width="1.6" stroke-linejoin="round"/></svg>';
    c.style.cssText = 'position:fixed;left:-50px;top:-50px;z-index:2147483647;pointer-events:none;transform:translate(-3px,-2px);filter:drop-shadow(0 2px 3px rgba(0,0,0,.45))';
    document.body.appendChild(c);
    const place = e => { c.style.left = `${e.clientX}px`; c.style.top = `${e.clientY}px`; };
    addEventListener('mousemove', place, true);
    window.__ripple = (x, y) => {
      const r = document.createElement('div');
      r.className = 'promo-ripple';
      r.dataset.born = String(performance.now());
      r.style.cssText = `position:fixed;left:${x - 18}px;top:${y - 18}px;width:36px;height:36px;border-radius:50%;border:3px solid rgba(56,189,248,.95);background:rgba(56,189,248,.18);z-index:2147483646;pointer-events:none`;
      document.body.appendChild(r);
    };
    addEventListener('mousedown', e => { place(e); window.__ripple(e.clientX, e.clientY); }, true);
    const tick = () => {
      c.style.display = document.pointerLockElement || window.__hideCursor ? 'none' : 'block';
      for (const r of document.querySelectorAll('.promo-ripple')) {
        const age = (performance.now() - Number(r.dataset.born)) / 450;
        if (age >= 1) r.remove();
        else { r.style.opacity = String(1 - age); r.style.transform = `scale(${0.6 + age * 0.8})`; }
      }
      requestAnimationFrame(tick);
    };
    tick();
  });
});
const page = await ctx.newPage();
page.on('pageerror', e => console.error('page error:', e.message));
await page.clock.install();
await page.goto(url, { waitUntil: 'domcontentloaded' });
await page.clock.resume(); // real time while the map loads and settles
await page.waitForTimeout(15000);
// Freeze the clock a moment ahead of the page's own time. The page's clock is not
// this process's: it starts where the page's did and can run ahead of it, and
// pausing at a time it has already passed is an error. So the time is read from
// the page, and a margin it outran is doubled and tried again.
for (let margin = 500; ; margin *= 2) {
  const now = await page.evaluate(() => Date.now());
  try {
    await page.clock.pauseAt(new Date(now + margin));
    break;
  } catch (e) {
    if (margin > 60000 || !/past/.test(e.message)) throw e;
  }
}
// Screenshots go through CDP: Playwright's own waits for an animation frame inside the
// page, which never comes while its clock is frozen.
const cdp = await ctx.newCDPSession(page);
// One render per frame. The faked requestAnimationFrame fires every 16 ms of page
// time, so a 1/30 s step would draw the scene twice (six times at 10 fps) and throw
// all but the last away. Callbacks are queued instead and run once after each step.
await page.evaluate(() => {
  let queue = [], id = 0;
  window.requestAnimationFrame = cb => { queue.push({ id: ++id, cb }); return id; };
  window.cancelAnimationFrame = h => { queue = queue.filter(q => q.id !== h); };
  window.__flush = () => {
    const now = performance.now(), run = queue;
    queue = [];
    for (const { cb } of run) { try { cb(now); } catch (e) { console.error(e); } }
  };
});

let n = 0;
let t0 = 0; // when the first frame was asked for; the first one takes longest by far
let scene = '';
let mouse = { x: VIEW.width * 0.72, y: VIEW.height * 0.62 };
await page.mouse.move(mouse.x, mouse.y);

async function frame(during) {
  if (during) await during();
  await page.clock.runFor(STEP);
  await page.evaluate(() => window.__flush());
  // Unclipped: a clip with a scale resets the emulated pixel ratio, and with it the
  // saving --preview is for. The frame comes back at the viewport's size.
  const { data } = await cdp.send('Page.captureScreenshot', { format: 'jpeg', quality: 92 });
  fs.writeFileSync(path.join(frames, `f${String(n++).padStart(5, '0')}.jpg`), Buffer.from(data, 'base64'));
  progress();
}

const clock = s => (s >= 3600 ? `${Math.floor(s / 3600)}h${String(Math.floor(s / 60) % 60).padStart(2, '0')}m`
  : s >= 60 ? `${Math.floor(s / 60)}m${String(Math.round(s % 60)).padStart(2, '0')}s` : `${Math.round(s)}s`);
let lastLine = 0;
/**
 * How far along the recording is. On a terminal it is one line rewritten in place;
 * in a log it is a line a second of video. The rate leaves the first frame out: it
 * compiles every shader in the city and can take minutes on its own.
 */
function progress(final = false) {
  if (n === 1) t0 = Date.now();
  const spent = (Date.now() - t0) / 1000;
  const each = n > 1 ? spent / (n - 1) : 0;
  const share = Math.min(1, n / planned);
  const line = `[${(share * 100).toFixed(0).padStart(3)}%] ${n}/${planned} frames, scene ${scene}` +
    (each ? `, ${each.toFixed(2)} s a frame, ${clock(spent)} spent, about ${clock(Math.max(0, planned - n) * each)} left` : '');
  if (process.stdout.isTTY) {
    process.stdout.write(`\r${line.padEnd(lastLine)}${final ? '\n' : ''}`);
    lastLine = line.length;
  } else if (final || n % FPS === 0) console.log(line);
}
const say = text => {
  if (process.stdout.isTTY && lastLine) { process.stdout.write('\n'); lastLine = 0; }
  console.log(text);
};
const frames_ = s => Math.max(1, Math.round(s * FPS));
/** Holds for `s` seconds; `each(t)` runs before every frame with t from 0 to 1. */
async function hold(s, each) {
  const k = frames_(s);
  for (let i = 0; i < k; i++) await frame(each ? () => each(i / k) : null);
}
const ease = t => (t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2);
const lerp = (a, b, t) => a + (b - a) * t;
async function glide(x, y, s = 0.8) {
  const from = { ...mouse };
  await hold(s, t => {
    const e = ease(t + 1 / frames_(s));
    mouse = { x: lerp(from.x, x, e), y: lerp(from.y, y, e) };
    return page.mouse.move(mouse.x, mouse.y);
  });
}
async function center(sel) {
  const b = await page.locator(sel).first().boundingBox();
  if (!b) throw new Error(`nothing on screen for ${sel}`);
  return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
}
async function clickAt(sel, s = 0.7) {
  const c = await center(sel);
  await glide(c.x, c.y, s);
  await frame(() => page.mouse.down());
  await frame(() => page.mouse.up());
}
async function typeSlowly(text) {
  for (const ch of text) { await frame(() => page.keyboard.type(ch)); await frame(); }
}
// selectOption and fill wait for stability in animation frames, which never come here.
const choose = (sel, value) => page.evaluate(({ sel, value }) => {
  const el = document.querySelector(sel);
  el.value = value;
  el.dispatchEvent(new Event('input', { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
}, { sel, value });
/**
 * Draws a select's list open under it, styled from the select, with the current
 * option marked and the one under the pointer lit. Returns the center of the option
 * whose value is `value`.
 */
const openList = (sel, value) => page.evaluate(({ sel, value }) => {
  const el = document.querySelector(sel), r = el.getBoundingClientRect(), cs = getComputedStyle(el);
  document.getElementById('promo-list')?.remove();
  const list = document.createElement('div');
  list.id = 'promo-list';
  const row = Math.max(22, r.height - 4);
  const bg = cs.backgroundColor && cs.backgroundColor !== 'rgba(0, 0, 0, 0)' ? cs.backgroundColor : '#fff';
  list.style.cssText = `position:fixed;left:${r.left}px;top:${r.bottom + 2}px;min-width:${r.width}px;z-index:2147483600;`
    + `background:${bg};color:${cs.color};font:${cs.font};border:1px solid rgba(127,127,127,.45);border-radius:6px;`
    + 'box-shadow:0 8px 24px rgba(0,0,0,.28);padding:4px 0;overflow:hidden';
  const style = document.createElement('style');
  style.textContent = '#promo-list div{padding:0 12px 0 22px;white-space:nowrap;position:relative}'
    + '#promo-list div.on::before{content:"\\2713";position:absolute;left:7px}'
    + '#promo-list div:hover,#promo-list.taken div.want{background:#2563eb;color:#fff}';
  list.appendChild(style);
  let want = null;
  for (const o of el.options) {
    const d = document.createElement('div');
    d.textContent = o.textContent;
    d.style.height = d.style.lineHeight = `${row}px`;
    if (o.value === el.value) d.classList.add('on');
    if (o.value === value) { d.classList.add('want'); want = d; }
    list.appendChild(d);
  }
  document.body.appendChild(list);
  if (!want) throw new Error(`${sel} has no option ${value}`);
  const b = want.getBoundingClientRect();
  return { x: b.left + Math.min(b.width / 2, 60), y: b.top + b.height / 2 };
}, { sel, value });
const caption = (text, sub = '') => page.evaluate(({ text, sub }) => {
  let el = document.getElementById('promo-cap');
  if (!el) {
    el = document.createElement('div');
    el.id = 'promo-cap';
    document.body.appendChild(el);
  }
  const k = 1;
  el.style.cssText = `position:fixed;left:50%;bottom:${30 * k}px;transform:translateX(-50%);z-index:99999;` +
    `background:rgba(15,23,42,.86);color:#f8fafc;font:600 ${26 * k}px/1.25 system-ui,sans-serif;padding:${13 * k}px ${26 * k}px;` +
    `border-radius:${14 * k}px;box-shadow:0 8px 30px rgba(0,0,0,.35);text-align:center;pointer-events:none;max-width:${1000 * k}px`;
  el.innerHTML = text + (sub ? `<div style="font:400 ${17 * k}px/1.4 system-ui,sans-serif;color:#cbd5e1;margin-top:4px">${sub}</div>` : '');
  el.style.display = text ? 'block' : 'none';
}, { text, sub });
/** Runs fn(dh, arg) in the page, where dh holds the walker, the bugs, the fires and the stash. */
const dh = (fn, arg = null) => page.evaluate(`(${fn})(window.__dh, ${JSON.stringify(arg)})`);
const hideCursor = on => page.evaluate(on => { window.__hideCursor = on; }, on);
const overlays = (targets, off) => page.evaluate(({ targets, off }) => {
  const hidden = (window.__hidden ??= new Set());
  for (const t of targets) off ? hidden.add(t) : hidden.delete(t);
  let style = document.getElementById('promo-hide');
  if (!style) { style = document.createElement('style'); style.id = 'promo-hide'; document.head.appendChild(style); }
  style.textContent = [...hidden].map(t => `${t} { display: none !important; }`).join('\n');
}, { targets, off });

// The walker, directed: where they stand and where they look. yaw and pitch follow
// walk.js (teleport): yaw turns about +y with -z ahead, pitch is up from level.
const view = v => dh((d, v) => {
  const p = d.walker.p;
  Object.assign(p, v);
  p.vy = 0;
}, v);
const lookAngles = (from, to) => {
  const dx = to.x - from.x, dz = to.z - from.z;
  return { yaw: Math.atan2(-dx, -dz), pitch: Math.atan2(to.y - (from.feet + EYE), Math.hypot(dx, dz)) };
};
const turn = (a, b, t) => { let d = b - a; while (d > Math.PI) d -= 2 * Math.PI; while (d < -Math.PI) d += 2 * Math.PI; return a + d * t; };
const state = () => dh(d => ({ ...d.walker.p, frozen: d.walker.frozen }));
const groundAt = (x, z) => dh((d, a) => d.walker.height(a.x, a.z), { x, z });
/** Makes sure the walker is walking: nothing read or open holds them still. */
async function ready() {
  if (await dh(d => d.walker.frozen)) await resume();
}
async function resume() {
  // A catch opens its finding and holds the walker, as a second hit on a building
  // does; for the camera, walking on is the director's call.
  await dh(d => { if (d.walker.frozen) d.walker.setFrozen(false); });
  await hideCursor(true);
}

// ------------------------------------------------------------------ the scenes

say(`recording ${SCENES.join(', ')}: about ${planned} frames, ${(planned / FPS).toFixed(1)} s of video at ${FPS} fps`);
for (const name of SCENES) {
  scene = name;
  say(`scene ${name} (from frame ${n})`);
  for (const st of CONFIG.scenes[name]) await actions[st.do].run(st);
}
progress(true);
console.log(`done: ${n} frames = ${(n / FPS).toFixed(1)} s at ${FPS} fps`);
srv.kill();

// ------------------------------------------------------------------ the video

await browser.close();
fs.writeFileSync(path.join(OUT, 'recording.json'), JSON.stringify({ frames: n, fps: FPS, scale: SCALE }));
await encode({ frames: n, fps: FPS, scale: SCALE });
