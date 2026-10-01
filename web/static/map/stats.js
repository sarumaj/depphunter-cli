// How the map is drawing, in a corner of it: shown when the address carries ?stats
// (or &stats), for telling what a change of rendering does on a given machine.
//
// frames a second, the time a frame takes on the CPU, the resolution it is drawn at
// (resolution.js), draw calls and triangles.
//
// Implements: REQ-PERF-014

/** Whether the address asks for the readout. */
export const wanted = search => new URLSearchParams(search).has('stats');

// Frames averaged over, and how often the text is written.
const WINDOW = 60, EVERY = 500;

export class Stats {
  constructor(scene, parent) {
    this.scene = scene;
    this.times = [];
    this.written = 0;
    this.el = document.createElement('div');
    this.el.className = 'render-stats';
    this.text = document.createElement('pre');
    this.el.append(this.text);
    parent.appendChild(this.el);
  }

  /** A frame drawn at `now` that took `ms` on the CPU; `info`: the renderer's counts for it. */
  frame(now, ms, info) {
    this.times.push([now, ms]);
    if (this.times.length > WINDOW) this.times.shift();
    if (now - this.written < EVERY || this.times.length < 2) return;
    this.written = now;
    const span = now - this.times[0][0];
    const fps = span > 0 ? (this.times.length - 1) * 1000 / span : 0;
    const cpu = this.times.reduce((a, [, t]) => a + t, 0) / this.times.length;
    this.text.textContent = [
      `${fps.toFixed(0)} fps   ${cpu.toFixed(1)} ms cpu`,
      `resolution ${(this.scene.resolution.scale * 100).toFixed(0)}%`,
      `${info.calls} draws   ${(info.triangles / 1000).toFixed(0)}k triangles`,
    ].join('\n');
  }
}
