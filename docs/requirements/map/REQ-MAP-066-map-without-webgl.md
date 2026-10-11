---
id: REQ-MAP-066
title: The map is drawn without WebGL
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

Where the browser gives the page no WebGL context, the isometric map **shall**
still be drawn, on a 2D canvas: every box in its color, shaded by its faces and
hidden by whatever stands in front of it, with the selection's outline, the
dependency arcs and the findings' pins; it **shall** be panned, zoomed, rotated,
pointed at, labeled and exported as it is otherwise. Walk mode **shall not** be
offered there: its button is disabled and says why, and neither V nor a
double-click on open ground enters it; the introduction and the help **shall**
leave the street out, the help saying why, the walker's figure **shall not** be
drawn on the map, and the status line **shall** say the map is drawn without
WebGL. Where the context is lost while the map is open, the map **shall** carry
on in the same way from where it is, leaving walk mode if the reader was in it,
and **shall** stay so until the page is loaded again. The address parameter
`flat` **shall** ask for the same drawing where WebGL is to be had.

## Rationale

WebGL can be turned off, blocklisted for a driver or lost to a remote desktop,
and a map that then shows nothing at all is no use to the people most likely to
be reading one over such a connection. The map's facts are its boxes and their
colors; the facades, the streets, the water and the street itself are shaders,
and are what is given up.

## Acceptance criteria

1. With WebGL unavailable, the map opens and shows its boxes in their colors, a
   nearer box hiding what is behind it.
2. Selecting a module there outlines it and draws its arcs; labels and hover
   cards work as they do with WebGL.
3. The Walk button is disabled and explains itself, and V does nothing.
4. `?flat` draws the map the same way in a browser that has WebGL.
5. Without WebGL the introduction has no card about walking, the help shows a
   note in place of walk mode's sections, and no figure stands on the map.
6. Losing the WebGL context with the map open leaves the map drawn, pannable
   and zoomable, with the Walk button disabled and the status line saying so.
