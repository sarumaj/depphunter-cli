---
id: REQ-PERF-011
title: Frames drawn smaller while they fall behind
scope: perf
type: non-functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

While frames follow one another (walking, or moving the map) and fall behind
45 frames a second, the system **shall** draw them with fewer pixels, down to
half the canvas's resolution on a side, and **shall** draw them finer again
once they keep up; the map at rest, a screenshot and a photograph **shall** be
drawn at the canvas's full resolution.

## Rationale

The facades and the streets are painted per pixel, so on a slow GPU the frame
rate follows the pixel count. A smooth picture that is slightly soft while it
moves is better than a sharp one that stutters, and a picture that stays on
screen can afford every pixel.

## Acceptance criteria

1. Frames that keep up are drawn at full resolution.
2. Frames that fall behind are drawn smaller, never below half a side.
3. Frames that keep up again grow back, but not within moments to a size that
   was too slow.
4. A pause between frames is not taken for a slow frame.
5. The map at rest, screenshots and photographs are drawn at full resolution.
