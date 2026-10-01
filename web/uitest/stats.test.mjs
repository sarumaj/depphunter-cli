// The rendering readout (stats.js).

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { Stats, wanted } = await import('../static/map/stats.js');

describe('rendering readout', () => {
  // Verifies: REQ-PERF-014
  it('is shown only when the address asks for it', () => {
    assert.equal(wanted('?token=abc&stats'), true);
    assert.equal(wanted('?stats'), true);
    assert.equal(wanted('?token=abc'), false);
    assert.equal(wanted(''), false);
  });

  // Verifies: REQ-PERF-014
  it('reads out the frame rate, CPU time, resolution, draws and triangles', () => {
    const readout = { scene: { resolution: { scale: 0.85 } }, times: [], written: 0, text: { textContent: '' } };
    for (let i = 0; i <= 60; i++) Stats.prototype.frame.call(readout, 1000 + i * 20, 4, { calls: 17, triangles: 259186 });
    assert.equal(readout.text.textContent, ['50 fps   4.0 ms cpu', 'resolution 85%', '17 draws   259k triangles'].join('\n'));
  });
});
