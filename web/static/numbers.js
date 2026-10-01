// Small arithmetic the map's modules share.

export const clamp = (v, low, high) => Math.min(high, Math.max(low, v));

/**
 * Exponential easing towards a value: `tau` is how long it takes to close most of
 * the gap, in seconds, whatever dt happens to be. An undefined current value starts
 * where it is going, so nothing animates in from zero on the first frame.
 */
export const ease = (current, want, tau, deltaTime) =>
  current === undefined ? want : current + (want - current) * (1 - Math.exp(-Math.max(0, deltaTime) / tau));
