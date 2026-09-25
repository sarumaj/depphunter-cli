#!/usr/bin/env node
// Takes the pictures the introduction is told with.
//
// The cards used to be prose alone, and prose is the worst medium there is for a
// gesture. "Hold R, point at one and let go" is four words and a shrug; a picture of
// the wheel open with the cursor on a wedge is the whole of it. What the introduction
// has to get across is mostly gestures and mechanics - a wheel flicked open, a line
// pulling a walker up a wall, a roof alight and spreading - and none of them survive
// being described.
//
// They are taken from the running map rather than drawn, for the same reason the help
// does not have illustrations in it: a drawing of a feature is a thing somebody has to
// remember to redraw, and nobody ever does. This is a command instead, and the point
// of it is that anybody can run it again:
//
//     node scripts/tour-shots.mjs [--repo PATH] [--bin ./depphunter]
//
// It builds the binary if it has to, serves this repository with a report that has
// something reachable in it (so there is a fire to photograph), drives the map through
// each scene, and writes web/static/tour/*.webp.
//
// Small on purpose. Every one of these is embedded in the binary and inlined again,
// base64, into any standalone HTML export - so a screenshot is not a screenshot here,
// it is a permanent cost paid by every copy of the map anybody ever exports. They are
// cropped to the thing being shown, scaled to SHOT_W, and written as WebP at a quality
// that keeps flat color clean and lets the photographic noise go. That comes to tens
// of kilobytes each rather than the half-megabyte a raw full-frame PNG costs.

import { chromium } from 'playwright';
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';

const HERE = path.dirname(new URL(import.meta.url).pathname);
const REPO = path.resolve(HERE, '..');
const OUT = path.join(REPO, 'web/static/tour');

// What a card's picture is drawn at. Wide enough to read a HUD in, narrow enough that
// the dialog does not have to grow around it.
const SHOT_W = 720;
const QUALITY = 0.82;
// The window the map is driven in. Larger than the picture, so a crop can be taken
// from the middle of a scene rather than the whole of a cramped one.
const VIEW = { width: 1280, height: 800 };

const arg = (name, fallback) => {
  const at = process.argv.indexOf(name);
  return at > 0 ? process.argv[at + 1] : fallback;
};

/**
 * A report with a reachable vulnerability in it, so that the fire card has a fire to
 * photograph. Written to a temporary file: it describes this repository and nothing
 * else, and it is not worth keeping.
 *
 * It names a real package and a real file, because what the picture has to show is the
 * map doing its actual job. A made-up path would place the fire nowhere.
 */
function report(into) {
  const main = 'github.com/sarumaj/depphunter-cli';
  const frame = (module, pkg, fn, at) => ({
    module, version: at ? '' : 'v0.3.7', package: pkg, function: fn,
    ...(at ? { position: { filename: at, line: 48, column: 1 } } : {}),
  });
  const lines = [
    { osv: { id: 'GO-0000-0001', summary: 'Decoder reads past the end of a buffer', database_specific: { severity: 'CRITICAL' } } },
    { finding: { osv: 'GO-0000-0001', fixed_version: 'v0.3.8', trace: [
      frame('golang.org/x/text', 'golang.org/x/text/encoding/unicode', 'Decode'),
      frame(main, `${main}/internal/scan`, 'Scan', 'internal/scan/scan.go'),
    ] } },
  ];
  fs.writeFileSync(into, lines.map(l => JSON.stringify(l)).join('\n'));
  return into;
}

/**
 * Where the fire is on screen, as a crop around it, or null if nothing is alight.
 *
 * Aiming the camera at a roof by pressing keys for a fixed number of milliseconds is
 * a guess, and it was a wrong one: the first fire card came out as a blue wall with
 * two orange wisps at the top edge, which is a picture of a building and not of a
 * fire. So the frame is searched for the fire instead. Flames are the only warm thing
 * on a map of blue-gray walls, green lawns and pale sky - red leading, blue trailing -
 * which makes them cheap to find and impossible to confuse with the city.
 *
 * The crop is squared off around whatever was found, with room to breathe, so the
 * card shows a roof alight rather than a flame filling the frame.
 */
async function findFire(page, view) {
  const png = (await page.screenshot()).toString('base64');
  return page.evaluate(async ({ data, view }) => {
    const img = new Image();
    await new Promise(r => { img.onload = r; img.src = 'data:image/png;base64,' + data; });
    const c = document.createElement('canvas');
    c.width = img.width; c.height = img.height;
    c.getContext('2d').drawImage(img, 0, 0);
    const px = c.getContext('2d').getImageData(0, 0, img.width, img.height).data;
    let x0 = 1e9, y0 = 1e9, x1 = -1, y1 = -1, n = 0;
    for (let i = 0; i < px.length; i += 4) {
      const [r, g, b] = [px[i], px[i + 1], px[i + 2]];
      if (!(r > g && g > b && r - b > 30)) continue;
      const at = i / 4, x = at % img.width, y = Math.floor(at / img.width);
      // The HUD is warm in places (a stamina bar, a severity dot); the city is not.
      if (y < 90 || y > img.height - 120) continue;
      x0 = Math.min(x0, x); x1 = Math.max(x1, x);
      y0 = Math.min(y0, y); y1 = Math.max(y1, y);
      n++;
    }
    if (n < 40) return null;
    // Around it, not on it: a fire with a roof under it reads as a building alight.
    const cx = (x0 + x1) / 2, cy = (y0 + y1) / 2;
    const w = Math.min(view.width, Math.max(520, (x1 - x0) * 4));
    const h = Math.round(w * 13 / 23);
    return {
      x: Math.max(0, Math.min(view.width - w, Math.round(cx - w / 2))),
      y: Math.max(90, Math.min(view.height - h - 60, Math.round(cy - h * 0.38))),
      width: Math.round(w), height: h, found: n,
    };
  }, { data: png, view });
}

/** Crops a region out of the page's last frame, scales it, and writes it as WebP. */
async function shot(page, name, clip) {
  const png = await page.screenshot({ clip });
  const webp = await page.evaluate(async ({ data, w, q }) => {
    const img = new Image();
    await new Promise(r => { img.onload = r; img.src = 'data:image/png;base64,' + data; });
    const c = document.createElement('canvas');
    c.width = w;
    c.height = Math.round(img.height * (w / img.width));
    c.getContext('2d').drawImage(img, 0, 0, c.width, c.height);
    return c.toDataURL('image/webp', q).split(',')[1];
  }, { data: png.toString('base64'), w: SHOT_W, q: QUALITY });
  const file = path.join(OUT, `${name}.webp`);
  fs.writeFileSync(file, Buffer.from(webp, 'base64'));
  console.log(`  ${name}.webp  ${(fs.statSync(file).size / 1024).toFixed(1)} kB`);
}

async function main() {
  fs.mkdirSync(OUT, { recursive: true });
  const repo = path.resolve(arg('--repo', REPO));
  let bin = arg('--bin', '');
  if (!bin) {
    bin = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'depphunter-tourshot-')), 'depphunter');
    console.log('building depphunter...');
    execFileSync('go', ['build', '-o', bin, './cmd/depphunter'], { cwd: REPO, stdio: 'inherit' });
  }
  const reportFile = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'tourshot-')), 'govuln.json');
  report(reportFile);

  const srv = spawn(bin, [repo, '--addr', '127.0.0.1:8977', '--no-open', '--findings', reportFile]);
  let url = '';
  srv.stdout.on('data', d => { const m = /http:\/\/\S+/.exec(String(d)); if (m && !url) url = m[0]; });
  srv.stderr.on('data', d => { const m = /http:\/\/\S+/.exec(String(d)); if (m && !url) url = m[0]; });
  for (let i = 0; i < 120 && !url; i++) await new Promise(r => setTimeout(r, 500));
  if (!url) throw new Error('the map never said where it was serving');
  console.log('serving at', url);

  // Playwright's own browser where it has one, and whatever CHROMIUM names where it
  // does not. A machine that already has a Chromium - a CI image, a container built
  // around one - should not have to download a second copy to take five pictures.
  const browser = await chromium.launch(
    process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
  // Reduced motion: the first walk is not flown in (walk.js startArrival), so the street
  // is there to be photographed as soon as the tour is out of the way.
  const page = await browser.newPage({ viewport: VIEW, reducedMotion: 'reduce' });
  page.on('pageerror', e => console.error('page error:', e.message));
  await page.goto(url, { waitUntil: 'domcontentloaded' });
  await settle(page, 20000);
  await skipTour(page);

  // The middle of the view, where every scene is staged.
  const mid = { x: 180, y: 120, width: 920, height: 520 };

  console.log('taking the pictures...');
  // 1. The map itself, with a module selected so the dependency arcs are drawn.
  await pick(page, 'internal/scan/scan.go');
  await page.waitForTimeout(3000);
  await shot(page, 'map', mid);

  // 2. The street, which is the same map from inside it.
  await page.click('#walk');
  await page.waitForTimeout(3000);
  await skipTour(page);
  await page.waitForTimeout(4000);
  await shot(page, 'street', mid);

  // 3. The tool wheel, held open with the cursor on a wedge.
  await page.keyboard.down('r');
  await page.waitForTimeout(400);
  await page.mouse.move(VIEW.width / 2, VIEW.height / 2);
  await page.mouse.move(VIEW.width / 2 - 150, VIEW.height / 2 - 60);
  await page.waitForTimeout(600);
  await shot(page, 'wheel', mid);
  await page.keyboard.up('r');
  await page.waitForTimeout(1500);
  // ... and put back down whatever that just picked up. Letting go of the wheel on a
  // wedge equips it, which is the whole point of the picture and was quietly wrong
  // for the next one: the cursor lands on the jet backpack, so the walker spent the
  // fire scene in the air looking down at a roof from a hundred feet up. A scene that
  // leaves the walker changed is a scene the one after it has to know about.
  await emptyOffHand(page);

  // 4. A roof alight, which is what a reachable vulnerability looks like. Backed away
  // from and looked up at, so the roof is in frame at all, and then found rather than
  // aimed at - see findFire.
  await page.keyboard.down('s');
  await page.waitForTimeout(1600);
  await page.keyboard.up('s');
  for (let i = 0; i < 7; i++) { await page.mouse.move(VIEW.width / 2, VIEW.height / 2 - i * 45); await page.waitForTimeout(250); }
  await page.waitForTimeout(3000);
  const blaze = await findFire(page, VIEW);
  if (blaze) console.log(`  (found the fire: ${blaze.found} pixels)`);
  else console.log('  (no fire found; taking the middle of the scene)');
  await shot(page, 'fire', blaze || mid);

  // 5. The tracker, which says where the rest of it is.
  await shot(page, 'tracker', { x: 10, y: VIEW.height - 250, width: 420, height: 240 });

  await browser.close();
  srv.kill();
  console.log('done; the cards name these in web/static/tour.js');
}

/**
 * Puts down whatever the walker is carrying in their left hand.
 *
 * Q walks that row and comes round to an empty hand within its length, so this asks
 * the HUD rather than counting presses: the row is four long today and the number of
 * presses that empties it depends on where in the ring you started.
 */
async function emptyOffHand(page) {
  for (let i = 0; i < 5; i++) {
    const carrying = await page.evaluate(() => !!document.querySelector(
      '#walk-hud .w-slots .w-slot[data-kind="secondary"][aria-selected="true"]'));
    if (!carrying) return;
    await page.keyboard.press('q');
    await page.waitForTimeout(600);
  }
  console.log('  (could not put the off hand down; the fire scene may be airborne)');
}

/** Waits for the map to have drawn something and gone quiet. */
async function settle(page, ms) {
  await page.waitForTimeout(ms);
}

/** Puts any introduction that is up out of the way; these pictures are of the map. */
async function skipTour(page) {
  for (let i = 0; i < 4; i++) {
    const el = await page.$('#tour-skip');
    if (el && await el.isVisible()) { await el.click().catch(() => {}); await page.waitForTimeout(500); }
    else break;
  }
}

/** Selects a node by path, through the map's own search. */
async function pick(page, what) {
  await page.fill('#search', what);
  await page.waitForTimeout(2500);
  const row = await page.$('#search-results li, #search-results [role="option"], .search-results li');
  if (row) await row.click().catch(() => {});
  else { await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter'); }
  await page.waitForTimeout(2500);
  await page.fill('#search', '');
}

main().catch(err => { console.error(err); process.exit(1); });
