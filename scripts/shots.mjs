#!/usr/bin/env node
// Takes the same pictures of the page twice and says whether anything moved.
//
//     node scripts/shots.mjs --baseline DIR [--scenes a,b] [--bin PATH] [--port N] [--keep-temp]
//     node scripts/shots.mjs --compare DIR  [--scenes a,b] [--bin PATH] [--port N] [--keep-temp]
//     node scripts/shots.mjs --list
//
// A change to the page that means to change nothing - a helper pulled out, a module
// split in two - is proven by the page looking and reading exactly as it did. The
// tests cannot say that: they load the modules in Node, where nothing is drawn and
// most of app.js never runs. So this drives the real page through a fixed list of
// scenes (SCENES below) and keeps two things from each, a screenshot and the markup:
//
//   DIR/<scene>.png    the viewport, WebGL and all
//   DIR/<scene>.html   the page's markup, then every form control's value and the
//                      focused element, which the markup leaves out
//
// --baseline writes them into DIR. --compare takes them again into DIR.now and
// compares each scene with DIR's: the markup byte for byte, the screenshots pixel
// for pixel, with a picture of the difference (DIR.now/<scene>.diff.png) wherever
// one differs. A scene that differs is taken again, twice at most, since the
// compositor now and then shades a pixel of an edge one level apart; what differs
// on every take is reported. It prints a line per scene and exits 1 if any differs.
// --help lists the options; CHROMIUM names a browser to use instead of Playwright's
// own.
//
// The page is served by a depphunter binary, which embeds web/static: without --bin
// this checkout is built first, so a comparison is always of the tree as it stands.
// A baseline of the tree before a change and a --compare after it is the proof a
// refactor of the page owes (CONTRIBUTING.md); two runs of the same tree have to
// agree exactly, and that is what everything below is for:
//
//   - the repository: not this checkout, which is what is being changed, but a small
//     one written out by fixture() below, with a git history at fixed dates and
//     scanner reports that put bugs on its files and its packages and a fire in it;
//   - the drawing: SwiftShader, a 1280x800 viewport at one pixel per pixel, reduced
//     motion, English in UTC;
//   - time: the page's clock (Date, timers, requestAnimationFrame, performance.now)
//     is frozen at CLOCK and only ever moves forward by fixed steps, after the
//     network has gone quiet, so what a scene shows does not depend on how fast the
//     machine is (settle);
//   - chance: Math.random is a seeded generator, so a scattered shot goes the same
//     way every time;
//   - what the markup cannot help carrying: the server's address and token, the
//     fixture's path and object URLs are written as ORIGIN, TOKEN, FIXTURE and blob:…
//     in the dumps.
//
// The introductions are marked as seen, and every scene starts from a fresh browser
// context and a server of its own, so no scene sees what the one before it left in
// local storage or in the server's session.
//
// Some scenes need more than the page's own controls give: a photograph in the stash
// is only taken with the camera, and a shot is fired with a mouse the walker holds
// locked. The copy of app.js served to this browser also exports a handful of its
// module-scope names on window.__shots, as record.mjs does for its director; the page
// is otherwise the one being shipped.
//
// Where things go, as in record.mjs: the binary, the fixture and the reports go in
// one temporary directory, depphunter-shots-*, removed when the run ends however it
// ends (--keep-temp leaves it); the 3D models a checkout without Git LFS lacks are
// fetched once into the user cache directory, depphunter/scripts.

import { chromium } from 'playwright';
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

// ------------------------------------------------------------------ arguments

const OPTIONS = {
  baseline: { type: 'string', value: 'DIR', help: 'take every scene into DIR' },
  compare: { type: 'string', value: 'DIR', help: 'take them into DIR.now and compare with DIR' },
  scenes: { type: 'string', value: 'A,B', help: 'only these scenes (default: all, in order)' },
  list: { type: 'boolean', help: 'print the scenes and exit' },
  bin: { type: 'string', value: 'PATH', help: 'depphunter binary to serve with (default: build this checkout)' },
  port: { type: 'string', value: 'N', help: 'port to serve on (default: 0, any free one)' },
  'keep-temp': { type: 'boolean', help: 'leave the temporary directory behind' },
  help: { type: 'boolean', help: 'print this and exit' },
};
const ARGS = parseArgs({ options: Object.fromEntries(Object.entries(OPTIONS).map(([k, o]) => [k, { type: o.type }])) }).values;
if (ARGS.help || !(ARGS.baseline || ARGS.compare || ARGS.list)) {
  const rows = Object.entries(OPTIONS).map(([k, o]) => [`--${k}${o.value ? ` ${o.value}` : ''}`, o.help]);
  const w = Math.max(...rows.map(r => r[0].length));
  console.log(`usage: node scripts/shots.mjs --baseline DIR | --compare DIR [options]\n\n${rows.map(([a, h]) => `  ${a.padEnd(w)}  ${h}`).join('\n')}` +
    '\n\nCHROMIUM=PATH uses that browser instead of Playwright\'s own.');
  process.exit(ARGS.help ? 0 : 2);
}
const PORT = Number(ARGS.port ?? 0);
if (!Number.isFinite(PORT)) throw new Error(`--port takes a number, not ${ARGS.port}`);

const VIEW = { width: 1280, height: 800 };
// The moment the page believes it is. Well after every commit in the fixture, so
// "3 months ago" reads the same on any day the harness is run.
const CLOCK = Date.parse('2026-06-01T12:00:00Z');
// How far the clock is moved after each step: enough for a debounced search (120 ms)
// and a panel's slide. It is jumped rather than run through, and then two frames are
// drawn: run through, it is 36 frames that SwiftShader takes over a minute to draw
// and nobody looks at.
const STEP = 600;
const FRAME = 16;
const FRAMES = 2 * FRAME + 2;
// How many times --compare takes a scene that differs before it says so (main).
const TAKES = 3;

// ------------------------------------------------------------------ leaving

// As in record.mjs: run once, newest first, however the run ends. A server left
// behind keeps its port, and a browser left behind keeps its memory.
const cleanups = [];
let cleaned = false;
const onExit = callback => cleanups.push(callback);
const cleanup = () => {
  if (cleaned) return;
  cleaned = true;
  for (const callback of cleanups.reverse()) { try { callback(); } catch { /* leaving anyway */ } }
};
process.on('exit', cleanup);
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => { cleanup(); process.exit(130); });
process.on('uncaughtException', e => { console.error(e); cleanup(); process.exit(1); });
process.on('unhandledRejection', e => { console.error(e); cleanup(); process.exit(1); });

// ------------------------------------------------------------------ where things go

let temp = null;
/** This run's temporary directory, made on first use and removed on exit (--keep-temp). */
function temporaryDirectory() {
  if (temp) return temp;
  temp = fs.mkdtempSync(path.join(os.tmpdir(), 'depphunter-shots-'));
  onExit(ARGS['keep-temp'] ? () => console.log(`kept ${temp}`) : () => fs.rmSync(temp, { recursive: true, force: true }));
  return temp;
}

/** The user cache directory's corner for these scripts, where os.UserCacheDir puts it in Go. */
function cacheDirectory() {
  const home = os.homedir();
  const base = process.env.XDG_CACHE_HOME
    || (process.platform === 'darwin' ? path.join(home, 'Library', 'Caches')
      : process.platform === 'win32' ? (process.env.LOCALAPPDATA || path.join(home, 'AppData', 'Local'))
        : path.join(home, '.cache'));
  const directory = path.join(base, 'depphunter', 'scripts');
  fs.mkdirSync(directory, { recursive: true });
  return directory;
}

// ------------------------------------------------------------------ the fixture

/**
 * Writes the repository every scene is taken of into `directory`, and the scanner
 * reports about it beside it, and returns their paths.
 *
 * Small enough to analyze in a second, and with one of everything the panels show:
 * Go packages that import each other and three modules, a JavaScript corner with npm
 * dependencies, a Markdown file, a binary file for the hex view, and three commits at
 * fixed dates by two authors for the history colors. The lint issues put bugs on
 * files, the advisories put them on packages, and one advisory the code reaches sets
 * internal/text/decode.go alight.
 */
function fixture(directory) {
  const root = path.join(directory, 'shop');
  const write = (file, text) => {
    fs.mkdirSync(path.dirname(path.join(root, file)), { recursive: true });
    fs.writeFileSync(path.join(root, file), text);
  };
  const lines = (count, make) => Array.from({ length: count }, (_, i) => make(i)).join('\n');
  const main = 'example.com/shop';
  write('go.mod', `module ${main}\n\ngo 1.22\n\nrequire (\n\tgithub.com/google/uuid v1.6.0\n\tgolang.org/x/text v0.3.7\n\tgopkg.in/yaml.v3 v3.0.1\n)\n`);
  write('cmd/shop/main.go', `// Command shop serves the shop.\npackage main\n\nimport (\n\t"fmt"\n\n\t"${main}/internal/cart"\n\t"${main}/internal/store"\n)\n\nfunc main() {\n\ts := store.Open("shop.yaml")\n\tc := cart.New(s)\n\tfmt.Println(c.Total())\n}\n`);
  write('internal/cart/cart.go', `// Package cart adds up what is being bought.\npackage cart\n\nimport "${main}/internal/store"\n\n// Cart is a list of lines.\ntype Cart struct {\n\tstore *store.Store\n\tlines []Line\n}\n\n// Line is one product and how many.\ntype Line struct {\n\tProduct string\n\tCount   int\n}\n\n// New starts an empty cart.\nfunc New(s *store.Store) *Cart { return &Cart{store: s} }\n\n${lines(24, i => `// Step${i} is step ${i}.\nfunc (c *Cart) Step${i}() int { return len(c.lines) + ${i} }\n`)}\n`);
  write('internal/cart/price.go', `package cart\n\n// Total adds the lines up.\nfunc (c *Cart) Total() int {\n\ttotal := 0\n\tfor _, l := range c.lines {\n\t\ttotal += c.store.Price(l.Product) * l.Count\n\t}\n\treturn total\n}\n`);
  write('internal/cart/cart_test.go', `package cart\n\nimport "testing"\n\nfunc TestNew(t *testing.T) {\n\tif New(nil).Total() != 0 {\n\t\tt.Fatal("not empty")\n\t}\n}\n`);
  write('internal/store/store.go', `// Package store reads the catalog.\npackage store\n\nimport (\n\t"os"\n\n\t"github.com/google/uuid"\n\t"gopkg.in/yaml.v3"\n\n\t"${main}/internal/text"\n)\n\n// Store is the catalog.\ntype Store struct {\n\tID     string\n\tPrices map[string]int\n}\n\n// Open reads a catalog.\nfunc Open(file string) *Store {\n\ts := &Store{ID: uuid.NewString()}\n\tdata, _ := os.ReadFile(file)\n\t_ = yaml.Unmarshal(text.Decode(data), &s.Prices)\n\treturn s\n}\n\n// Price is what one costs.\nfunc (s *Store) Price(product string) int { return s.Prices[product] }\n`);
  write('internal/text/decode.go', `// Package text reads what the catalog was written in.\npackage text\n\nimport "golang.org/x/text/encoding/unicode"\n\n${lines(46, i => `// line ${i}`)}\n\n// Decode turns UTF-16 into UTF-8.\nfunc Decode(b []byte) []byte {\n\tout, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(b)\n\treturn out\n}\n`);
  write('web/package.json', `${JSON.stringify({ name: 'shop-web', version: '1.0.0', dependencies: { lodash: '4.17.20', preact: '10.19.0' } }, null, 2)}\n`);
  write('web/app.js', `import { h, render } from 'preact';\nimport chunk from 'lodash/chunk';\nimport { basket } from './basket.js';\n\nrender(h('ul', {}, chunk(basket(), 2).map(p => h('li', {}, p.join(', ')))), document.body);\n`);
  write('web/basket.js', `export function basket() {\n  return ['tea', 'milk', 'bread', 'jam'];\n}\n`);
  write('README.md', '# shop\n\nA shop. See [the guide](docs/guide.md) and [the missing page](docs/missing.md).\n');
  write('docs/guide.md', '# Guide\n\nRun `go run ./cmd/shop`.\n');
  write('assets/logo.bin', Buffer.from(Array.from({ length: 300 }, (_, i) => (i * 37 + 11) % 256)));

  // Three commits at fixed dates by two people, so that commits, churn, age and
  // authors each have something to color.
  const git = (args, date, who = ['Ada', 'ada@example.com']) => execFileSync('git', args, {
    cwd: root, stdio: 'ignore',
    env: {
      ...process.env, GIT_AUTHOR_NAME: who[0], GIT_AUTHOR_EMAIL: who[1], GIT_COMMITTER_NAME: who[0], GIT_COMMITTER_EMAIL: who[1],
      GIT_AUTHOR_DATE: date, GIT_COMMITTER_DATE: date, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1',
    },
  });
  git(['init', '-q', '-b', 'main'], '2026-01-05T09:00:00Z');
  git(['add', '.'], '2026-01-05T09:00:00Z');
  git(['commit', '-q', '-m', 'The shop'], '2026-01-05T09:00:00Z');
  fs.appendFileSync(path.join(root, 'internal/cart/price.go'), '\n// Discount is none yet.\nfunc (c *Cart) Discount() int { return 0 }\n');
  git(['commit', '-q', '-am', 'A discount'], '2026-03-10T15:30:00Z', ['Grace', 'grace@example.com']);
  fs.appendFileSync(path.join(root, 'web/basket.js'), '\nexport const empty = () => [];\n');
  git(['commit', '-q', '-am', 'An empty basket'], '2026-04-20T11:00:00Z', ['Grace', 'grace@example.com']);

  const golangci = path.join(directory, 'golangci.json');
  const issue = (file, line, linter, text, severity) => ({ FromLinter: linter, Text: text, Severity: severity, Pos: { Filename: file, Line: line, Column: 2 } });
  fs.writeFileSync(golangci, JSON.stringify({
    Issues: [
      issue('internal/cart/cart.go', 22, 'errcheck', 'Error return value of `c.store.Save` is not checked', 'error'),
      issue('internal/cart/cart.go', 30, 'staticcheck', 'SA4006: this value of `n` is never used', 'warning'),
      issue('internal/store/store.go', 22, 'gosec', 'G304: Potential file inclusion via variable', 'error'),
      issue('cmd/shop/main.go', 12, 'revive', 'exported function should have comment', 'info'),
    ],
    Report: { Linters: [] },
  }));
  const govulncheck = path.join(directory, 'govulncheck.json');
  const osv = (id, summary, module, severity) => ({ osv: { id, summary, affected: [{ package: { ecosystem: 'Go', name: module } }], database_specific: { severity } } });
  fs.writeFileSync(govulncheck, [
    osv('GO-2026-0001', 'Decoder reads past the end of a buffer', 'golang.org/x/text', 'CRITICAL'),
    { finding: { osv: 'GO-2026-0001', fixed_version: 'v0.3.8', trace: [
      { module: 'golang.org/x/text', version: 'v0.3.7', package: 'golang.org/x/text/encoding/unicode', function: 'Decode' },
      { module: main, package: `${main}/internal/text`, function: 'Decode', position: { filename: 'internal/text/decode.go', line: 50, column: 1 } },
    ] } },
    osv('GO-2026-0002', 'Unbounded alias expansion', 'gopkg.in/yaml.v3', 'HIGH'),
    { finding: { osv: 'GO-2026-0002', fixed_version: 'v3.0.2', trace: [{ module: 'gopkg.in/yaml.v3', version: 'v3.0.1' }] } },
    osv('GO-2026-0003', 'Predictable identifiers', 'github.com/google/uuid', 'LOW'),
    { finding: { osv: 'GO-2026-0003', fixed_version: 'v1.6.1', trace: [{ module: 'github.com/google/uuid', version: 'v1.6.0' }] } },
  ].map(line => JSON.stringify(line)).join('\n'));
  return { root, reports: [golangci, govulncheck] };
}

// ------------------------------------------------------------------ the server and the browser

/** The binary to serve with: --bin as given, or this checkout built into the temporary directory. */
function binary() {
  if (ARGS.bin) {
    const bin = path.resolve(ARGS.bin);
    if (!fs.existsSync(bin)) throw new Error(`--bin ${bin}: no such file`);
    return bin;
  }
  const bin = path.join(temporaryDirectory(), process.platform === 'win32' ? 'depphunter.exe' : 'depphunter');
  console.log('building depphunter...');
  execFileSync('go', ['build', '-o', bin, './cmd/depphunter'], { cwd: REPO, stdio: 'inherit' });
  return bin;
}

/**
 * Serves `repository` with the given reports, and resolves to its URL, and a `stop`
 * that resolves once the server has exited, as soon as the history and the findings
 * have been read too: a page that loaded while they were still pending would poll for
 * them on its own clock, which is the one the harness holds still. A server still
 * running is stopped when the run ends.
 *
 * Every take of a scene gets a server of its own. The server keeps a session - what
 * is selected, what is in the backpack - for every page that opens the map, and one
 * scene's selection would otherwise decide where the next one walks in.
 */
async function serve(bin, repository, findings) {
  const server = spawn(bin, [repository, '--addr', `127.0.0.1:${PORT}`, '--no-open', '--no-cache',
    ...findings.flatMap(f => ['--findings', f])]);
  const exited = new Promise(resolve => server.once('exit', resolve));
  const stop = () => {
    if (server.exitCode === null && server.signalCode === null) server.kill();
    return exited;
  };
  onExit(stop);
  let url = '', said = '', ended = null;
  const grab = d => {
    said += String(d);
    const m = /http:\/\/\S+/.exec(said);
    if (m && !url) url = m[0];
  };
  server.stdout.on('data', grab);
  server.stderr.on('data', grab);
  server.on('exit', code => { ended = code; });
  server.on('error', e => { ended = e.message; });
  for (let i = 0; i < 240 && !url && ended === null; i++) await sleep(500);
  if (!url) {
    throw new Error(`depphunter ${ended === null ? 'said nothing about where it was serving in 2 minutes'
      : `stopped (${ended}) before serving`}; it said:\n${said.trim() || '(nothing)'}`);
  }
  for (const name of ['history', 'findings']) {
    for (let i = 0; ; i++) {
      const api = new URL(`api/${name}`, url);
      api.search = new URL(url).search;
      const response = await fetch(api);
      if (response.status !== 202) break;
      if (i > 240) throw new Error(`api/${name} was still pending after 2 minutes`);
      await sleep(500);
    }
  }
  return { url, stop };
}

/** Headless Chromium drawing WebGL with SwiftShader, closed when the run ends. */
async function launch() {
  const executable = process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {};
  const browser = await chromium.launch({
    ...executable,
    args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist',
      '--font-render-hinting=none', '--disable-lcd-text'],
  });
  onExit(() => { browser.close().catch(() => {}); });
  return browser;
}

// As in record.mjs: a checkout without `git lfs pull` holds pointers for the 3D
// models, and the map then draws stand-ins; the real files are fetched once into the
// cache directory and served to `context` in their place.
async function routeModels(context) {
  for (const name of ['hand', 'bug', 'props']) {
    const file = path.join(REPO, 'web/static', `${name}.glb`);
    if (fs.readFileSync(file).subarray(0, 4).toString() === 'glTF') continue; // the real thing is embedded
    const cached = path.join(cacheDirectory(), `${name}.glb`);
    if (!fs.existsSync(cached)) {
      const url = `https://media.githubusercontent.com/media/sarumaj/depphunter-cli/main/web/static/${name}.glb`;
      console.log(`fetching ${name}.glb (the checkout holds an LFS pointer)`);
      const response = await fetch(url);
      if (!response.ok) throw new Error(`${url}: ${response.status}; run git lfs pull instead`);
      fs.writeFileSync(`${cached}.part`, Buffer.from(await response.arrayBuffer()));
      fs.renameSync(`${cached}.part`, cached);
    }
    const body = fs.readFileSync(cached);
    await context.route(`**/${name}.glb*`, route => route.fulfill({ contentType: 'model/gltf-binary', body }));
  }
}

const sleep = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));

/**
 * A page on `url` in a context of its own, with the clock frozen at CLOCK, chance
 * seeded, the introductions seen, and app.js's module scope reachable on
 * window.__shots. Resolves once the map has been drawn.
 */
async function open(browser, url) {
  const context = await browser.newContext({
    viewport: VIEW, deviceScaleFactor: 1, reducedMotion: 'reduce', colorScheme: 'light', locale: 'en-US', timezoneId: 'UTC',
  });
  await routeModels(context);
  // The names record.mjs reads as well (scripts/record.mjs); app.js has to keep them.
  await context.route('**/app.js*', async route => {
    const response = await route.fetch();
    const body = await response.text();
    await route.fulfill({ response, body: `${body}
window.__shots = { get walker() { return walker; }, get stash() { return stash; }, get state() { return state; },
  get scene() { return scene; } };` });
  });
  await context.addInitScript(() => {
    localStorage.setItem('depphunter.introduced', '1');
    localStorage.setItem('depphunter.introduced.walk', '1');
    // Whether the pointer lock is granted, and when, is up to the browser and the
    // machine's load, and a locked pointer turns the parked mouse (settle) into a
    // look to one side. So it is asked for and never answered: the walker waits for
    // it, as it would in a browser still thinking it over.
    Element.prototype.requestPointerLock = () => new Promise(() => {});
    // mulberry32: small, fast, and the same numbers in every browser.
    let seed = 0x5eed;
    Math.random = () => {
      seed = (seed + 0x6d2b79f5) | 0;
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  });
  const page = await context.newPage();
  // Installed, the clock runs until it is paused, for however long the pausing takes;
  // pausing it a second ahead makes where it stops the same in every run.
  await page.clock.install({ time: CLOCK - 1000 });
  await page.clock.pauseAt(CLOCK);
  // The held clock's performance.now still counts from when the page really started
  // loading, and its animation frames fall on a grid laid from there too, both of
  // which differ from run to run by a millisecond or two - and the clouds, the water,
  // the hands and a shot in flight all move with them. So both count from the frozen
  // Date instead: a frame is a timer that falls due on the next multiple of FRAME.
  await context.addInitScript(frame => {
    const start = Date.now();
    performance.now = () => Date.now() - start;
    window.requestAnimationFrame = callback =>
      setTimeout(() => callback(performance.now()), frame - (performance.now() % frame));
    window.cancelAnimationFrame = handle => clearTimeout(handle);
  }, FRAME);
  page.on('pageerror', e => console.error('  page error:', e.message));
  // What is on its way over the network. The event stream never finishes, so it is
  // not counted; everything else is waited for before the clock moves (settle). So is
  // a body that is never read - the source of a file that turns out to be binary is
  // left where it is - once it has had two seconds to finish.
  page.inFlight = new Set();
  page.on('request', r => { if (!r.url().includes('api/events')) page.inFlight.add(r); });
  for (const done of ['requestfinished', 'requestfailed']) page.on(done, r => page.inFlight.delete(r));
  page.on('response', r => setTimeout(() => page.inFlight.delete(r.request()), 2000));
  await page.goto(url, { waitUntil: 'load' });
  await page.waitForFunction(() => window.__shots?.state && document.fonts.status === 'loaded'
    && document.getElementById('live')?.hidden === false && !document.getElementById('live').classList.contains('off'));
  // The 3D models are parsed, and their textures decoded, after they have arrived and
  // on the browser's own time. A bug placed one step later walks one step less, so
  // they are given a few seconds to be ready before the clock first moves.
  await quiet(page);
  await sleep(3000);
  await settle(page, 3);
  return page;
}

/**
 * Lets the page catch up with what was just done to it: waits in real time for the
 * network to go quiet, then jumps the clock on by STEP, firing every timer that falls
 * due once, and draws two frames; `times` over. The clock only moves while nothing
 * is in flight, so a response always lands between two steps rather than somewhere
 * inside one.
 */
async function settle(page, times = 1) {
  for (let i = 0; i < times; i++) {
    await quiet(page);
    await page.clock.fastForward(STEP);
    await page.clock.runFor(FRAMES);
  }
  // The pointer is parked in the same place every time, over nothing that reacts to
  // it, so no scene carries a hover left over from its last click.
  await page.mouse.move(VIEW.width - 2, VIEW.height - 2);
  await page.clock.runFor(FRAMES);
}

/** Waits, in real time, until nothing has been in flight for 300 milliseconds. */
async function quiet(page) {
  for (let calm = 0, waited = 0; calm < 3; waited++) {
    if (waited > 600) throw new Error(`still waiting after a minute for ${[...page.inFlight].map(r => r.url()).join(', ')}`);
    await sleep(100);
    calm = page.inFlight.size ? 0 : calm + 1;
  }
}

// ------------------------------------------------------------------ steps

/** Selects a node by name through the map's own search; `kind` is its badge (file, package...). */
async function pick(page, name, kind) {
  await page.fill('#search', name);
  await settle(page);
  const row = page.locator('#search-results li[data-i]').filter({ has: page.locator('.name', { hasText: name }) })
    .filter({ has: page.locator('.badge', { hasText: kind }) }).first();
  await row.dispatchEvent('pointerdown');
  await settle(page, 2);
}

/** Clicks what `selector` names and lets the page settle. */
async function click(page, selector, times = 1) {
  await page.locator(selector).first().click();
  await settle(page, times);
}

/** Presses a key on the page (not in a field) and lets it settle. */
async function press(page, key, times = 1) {
  await page.evaluate(() => document.activeElement?.blur?.());
  await page.keyboard.press(key);
  await settle(page, times);
}

/** Switches the theme through its own control. */
async function theme(page, value) {
  await page.selectOption('#theme', value);
  await settle(page);
}

/** Puts a photograph in the stash, as the camera would: a flat two-tone PNG. */
async function photograph(page) {
  await page.evaluate(async () => {
    const canvas = document.createElement('canvas');
    canvas.width = 160;
    canvas.height = 90;
    const context = canvas.getContext('2d');
    context.fillStyle = '#6a8caf';
    context.fillRect(0, 0, 160, 90);
    context.fillStyle = '#d9a441';
    context.fillRect(40, 30, 50, 60);
    const blob = await new Promise(resolve => canvas.toBlob(resolve, 'image/png'));
    window.__shots.stash.add(blob, 'internal/cart/cart.go');
  });
  await settle(page);
}

/**
 * Enters walk mode, lands the flight in at once (walk.js endArrival) and lets the
 * walker find its feet. Headless, the pointer cannot be locked, so the walker is let
 * go by hand, as record.mjs does.
 */
async function walk(page) {
  await press(page, 'v');
  await page.evaluate(() => {
    const walker = window.__shots.walker;
    if (walker.arrival) walker.endArrival(true);
    if (walker.frozen) walker.setFrozen(false);
  });
  await settle(page, 2);
}

/** Puts the tool `id` in the hand it belongs in (walk.js setTool). */
async function hold(page, id) {
  await page.evaluate(id => {
    const walker = window.__shots.walker;
    if (walker.primary.id !== id && walker.secondary?.id !== id) walker.setTool(id);
  }, id);
  await settle(page, 2);
}

/**
 * Fires the tool `id` - the right hand's, or the left's for a secondary one - and
 * holds the clock `frames` frames later, with the shot in the air. The scene returns
 * HELD, so the picture is taken there and not after the flight has ended.
 */
async function shoot(page, id, frames) {
  await hold(page, id);
  await page.evaluate(id => {
    const walker = window.__shots.walker;
    if (walker.secondary?.id === id) walker.useSecondary();
    else walker.fire();
  }, id);
  for (let i = 0; i < frames; i++) await page.clock.runFor(FRAME);
  return HELD;
}

// ------------------------------------------------------------------ the scenes

// What a scene returns when the picture has to be taken as it stands, without the
// clock moving on (a shot in flight).
const HELD = Symbol('held');

// Each scene starts from a freshly loaded map. The panels are here first, since they
// are what a change to the markup moves; walk mode after, whose frames cost the most.
const SCENES = {
  'map': async () => {},
  'panel-file': async page => { await pick(page, 'cart.go', 'file'); },
  'panel-package': async page => { await pick(page, 'gopkg.in/yaml.v3', 'package'); },
  'panel-finding': async page => {
    await press(page, 'l');
    await click(page, '#findings-list li', 2);
  },
  'panel-catch': async page => {
    await pick(page, 'cart.go', 'file');
    await click(page, '#panel .catch');
  },
  'panel-source-find': async page => {
    await pick(page, 'price.go', 'file');
    await page.fill('#panel .p-find', 'total');
    await settle(page, 2);
  },
  'panel-hex': async page => {
    await pick(page, 'logo.bin', 'file');
    await click(page, '#panel button:has-text("Show raw bytes")', 2);
  },
  'panel-directory': async page => { await pick(page, 'internal', 'dir'); },
  'findings': async page => { await press(page, 'l'); },
  'findings-catch': async page => {
    await press(page, 'l');
    await click(page, '#findings-list li .keep');
  },
  'findings-dark': async page => {
    await theme(page, 'dark');
    await press(page, 'l');
  },
  'backpack-empty': async page => { await press(page, 'b'); },
  'backpack': async page => {
    await press(page, 'l');
    await click(page, '#findings-list li:nth-child(1) .keep');
    await click(page, '#findings-list li:nth-child(3) .keep');
    await press(page, 'b');
  },
  'backpack-dark': async page => {
    await theme(page, 'dark');
    await pick(page, 'cart.go', 'file');
    await click(page, '#panel .catch');
    await press(page, 'b');
  },
  'stash': async page => {
    await photograph(page);
    await photograph(page);
    await press(page, 'g');
  },
  'export': async page => { await click(page, '#export-btn'); },
  'filters': async page => { await click(page, '#filters-btn'); },
  'legend-churn': async page => {
    await page.selectOption('#color-by', 'churn');
    await settle(page);
  },
  'dark': async page => {
    await theme(page, 'dark');
    await pick(page, 'store.go', 'file');
  },
  'walk': async page => { await walk(page); },
  'walk-backpack': async page => {
    await pick(page, 'cart.go', 'file');
    await click(page, '#panel .catch');
    await walk(page);
    await press(page, 'b', 2);
  },
  'walk-stash': async page => {
    await walk(page);
    await photograph(page);
    await press(page, 'g', 2);
  },
  'walk-findings': async page => {
    await walk(page);
    await press(page, 'l', 2);
  },
  'walk-export': async page => {
    await walk(page);
    await press(page, 'x', 2);
  },
  // Every tool in its hand (tools.js TOOL_IDS), then the four that throw something
  // caught in the air: a dart, a bobber on its line, a nail and a grapple on its rope.
  ...Object.fromEntries(['rod', 'net', 'camera', 'bubbles', 'extinguisher', 'dart', 'nailer', 'grapple', 'jetpack',
    'skimmers', 'parachute'].map(id => [`hold-${id}`, async page => { await walk(page); await hold(page, id); }])),
  'shot-dart': async page => { await walk(page); return shoot(page, 'dart', 6); },
  'shot-bobber': async page => { await walk(page); return shoot(page, 'rod', 6); },
  'shot-nail': async page => { await walk(page); return shoot(page, 'nailer', 2); },
  'shot-grapple': async page => { await walk(page); return shoot(page, 'grapple', 6); },
};

// ------------------------------------------------------------------ taking and comparing

/**
 * The viewport as a PNG, straight from the compositor. Playwright's own screenshot
 * waits for the page to draw a frame first, and a page whose clock is held still
 * never does.
 *
 * The map draws only when something changed, and the water and the sky move with the
 * clock they were last drawn at. Something that arrived by itself - the plants and the
 * bugs, fetched and parsed while the scene went on - can have asked for the last
 * frame one step earlier in one run than in another. So a frame is asked for last of
 * all, at the same moment in every run, once the page has had a second to finish; and
 * the picture is taken once that frame has had time to reach the screen. A HELD scene
 * is taken as it stands, since walk mode draws every frame anyway.
 */
async function screenshot(page, held) {
  if (!held) await settle(page);
  await sleep(1000);
  await page.evaluate(() => window.__shots.scene.renderNow());
  await sleep(500);
  const session = await page.context().newCDPSession(page);
  const { data } = await session.send('Page.captureScreenshot', { format: 'png' });
  await session.detach();
  return Buffer.from(data, 'base64');
}

/**
 * The page as markup, then what the markup leaves out: each form control's value and
 * where the focus is. The server's address and token, the fixture's path and object
 * URLs, which differ between runs by nature, are masked.
 */
async function dump(page, fixturePath, url) {
  const text = await page.evaluate(() => {
    const controls = [...document.querySelectorAll('input, select, textarea')]
      .map(el => `${el.id || el.name || el.className || el.tagName.toLowerCase()} = ${JSON.stringify(el.type === 'checkbox' ? el.checked : el.value)}`);
    const active = document.activeElement;
    const focus = active && active !== document.body
      ? `${active.tagName.toLowerCase()}${active.id ? `#${active.id}` : ''}${active.className ? `.${String(active.className).split(' ').join('.')}` : ''}`
      : 'body';
    return `${document.documentElement.outerHTML}\n\n<!-- controls\n${controls.join('\n')}\nfocus = ${focus}\n-->\n`;
  });
  const { origin, searchParams } = new URL(url);
  const token = searchParams.get('token');
  return text.split(origin).join('ORIGIN').split(fixturePath).join('FIXTURE')
    .split(token || '\0').join('TOKEN')
    .replace(/blob:ORIGIN\/[0-9a-f-]+/g, 'blob:…');
}

/**
 * How many pixels of two same-sized PNGs differ, and a picture of where: the new one
 * dimmed, with every pixel that changed in red. Worked out in the browser, which
 * already decodes PNGs, rather than with a dependency that would.
 */
async function difference(page, before, after) {
  return page.evaluate(async ({ a, b }) => {
    const load = async data => {
      const image = new Image();
      await new Promise(resolve => { image.onload = resolve; image.src = `data:image/png;base64,${data}`; });
      const canvas = document.createElement('canvas');
      canvas.width = image.width;
      canvas.height = image.height;
      const context = canvas.getContext('2d');
      context.drawImage(image, 0, 0);
      return { canvas, context, pixels: context.getImageData(0, 0, image.width, image.height) };
    };
    const [x, y] = [await load(a), await load(b)];
    if (x.canvas.width !== y.canvas.width || x.canvas.height !== y.canvas.height) return { differ: -1, total: 0, png: null };
    const out = y.context.createImageData(y.canvas.width, y.canvas.height);
    let differ = 0;
    for (let i = 0; i < out.data.length; i += 4) {
      const same = x.pixels.data[i] === y.pixels.data[i] && x.pixels.data[i + 1] === y.pixels.data[i + 1]
        && x.pixels.data[i + 2] === y.pixels.data[i + 2] && x.pixels.data[i + 3] === y.pixels.data[i + 3];
      if (!same) differ++;
      const gray = (y.pixels.data[i] + y.pixels.data[i + 1] + y.pixels.data[i + 2]) / 12 + 170;
      out.data.set(same ? [gray, gray, gray, 255] : [255, 0, 0, 255], i);
    }
    y.context.putImageData(out, 0, 0);
    return { differ, total: out.data.length / 4, png: y.canvas.toDataURL('image/png').split(',')[1] };
  }, { a: before.toString('base64'), b: after.toString('base64') });
}

/** Compares one scene just taken with the baseline's, and says how it went. */
async function verdictOn(page, base, name, png, html) {
  const was = { png: path.join(base, `${name}.png`), html: path.join(base, `${name}.html`) };
  if (!fs.existsSync(was.png) || !fs.existsSync(was.html)) return { same: false, text: 'NOT IN THE BASELINE' };
  const sameMarkup = fs.readFileSync(was.html, 'utf8') === html;
  const beforePng = fs.readFileSync(was.png);
  let pixels = 'same pixels';
  if (!beforePng.equals(png)) {
    const d = await difference(page, beforePng, png);
    if (d.differ === -1) pixels = 'a different size';
    else if (d.differ) {
      pixels = `${d.differ} pixels differ (${(100 * d.differ / d.total).toFixed(3)}%)`;
      fs.writeFileSync(path.join(`${base}.now`, `${name}.diff.png`), Buffer.from(d.png, 'base64'));
    }
  }
  if (pixels === 'same pixels') fs.rmSync(path.join(`${base}.now`, `${name}.diff.png`), { force: true });
  const same = sameMarkup && pixels === 'same pixels';
  return { same, text: same ? 'same' : `DIFFERS: ${sameMarkup ? 'same markup' : 'different markup'}, ${pixels}` };
}

async function main() {
  if (ARGS.list) {
    console.log(Object.keys(SCENES).join('\n'));
    return 0;
  }
  const base = path.resolve(ARGS.baseline ?? ARGS.compare);
  const out = ARGS.baseline ? base : `${base}.now`;
  const names = ARGS.scenes ? ARGS.scenes.split(',') : Object.keys(SCENES);
  for (const name of names) if (!SCENES[name]) throw new Error(`no scene called ${name}; --list lists them`);
  if (ARGS.compare && !fs.existsSync(base)) throw new Error(`${base}: no baseline there; take one with --baseline`);
  fs.rmSync(out, { recursive: true, force: true });
  fs.mkdirSync(out, { recursive: true });

  const { root, reports } = fixture(temporaryDirectory());
  const bin = binary();
  const browser = await launch();
  let differing = 0;
  for (const name of names) {
    const started = Date.now();
    let verdict = 'taken';
    // The compositor now and then shades a pixel or two of an edge one level apart
    // between runs. A scene that differs is taken again, up to TAKES times in all, so
    // that only what differs every time is reported: a real change always does.
    for (let take = 1; take <= (ARGS.compare ? TAKES : 1); take++) {
      const { url, stop } = await serve(bin, root, reports);
      const page = await open(browser, url);
      const held = await SCENES[name](page) === HELD;
      const png = await screenshot(page, held);
      const html = await dump(page, root, url);
      fs.writeFileSync(path.join(out, `${name}.png`), png);
      fs.writeFileSync(path.join(out, `${name}.html`), html);
      let result = { same: true };
      if (ARGS.compare) {
        result = await verdictOn(page, base, name, png, html);
        verdict = result.same && take > 1 ? `same (take ${take})` : result.text;
      }
      await page.context().close();
      await stop();
      if (result.same) break;
      if (take === TAKES) differing++;
    }
    console.log(`  ${name.padEnd(20)} ${verdict}  (${((Date.now() - started) / 1000).toFixed(0)} s)`);
  }
  if (ARGS.compare) {
    console.log(differing ? `${differing} of ${names.length} scenes differ; see ${out}` : `all ${names.length} scenes are the same`);
    return differing ? 1 : 0;
  }
  console.log(`baseline in ${out}`);
  return 0;
}

// Exiting is what stops the server and removes the temporary directory (onExit).
main().then(code => process.exit(code), err => { console.error(err); process.exit(1); });
