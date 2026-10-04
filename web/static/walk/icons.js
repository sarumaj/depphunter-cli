// What each tool looks like on the HUD: a line drawing on a 24-unit square, in the
// slot's own color, with the one part that names it at a glance - the rod's bobber,
// the extinguisher's red, a bubble - picked out as an accent. The row along the
// bottom and the wheel both draw these, so a tool looks the same wherever it is
// chosen.
//
// Implements: REQ-TOOL-080

const NS = 'http://www.w3.org/2000/svg';

// Strokes are the drawing; `accent` marks the filled part in the tool's color.
const ICONS = {
  // A rod bent under its line, the reel at the grip and the bobber hanging below.
  rod: '<path d="M3 21 16 4.5c1.4-1.8 3.6-1.2 3.9.8"/><path d="M19.9 5.3V15"/>'
    + '<circle cx="6.6" cy="16.6" r="1.6"/><circle class="accent" cx="19.9" cy="17.3" r="2.3"/>',
  // A hoop on a handle, and the bag of the net hanging out of it.
  net: '<ellipse cx="15" cy="8" rx="5.6" ry="4.6" transform="rotate(-35 15 8)"/>'
    + '<path d="M11.3 11.6 3 21"/><path d="M12.4 10.7c.6 4.4 3.4 7.4 7 7.6 1.8-3.2 1.4-6.6-.4-9.5" stroke-dasharray="1.6 1.6"/>',
  // A camera body, its lens and the bump the shutter sits on.
  camera: '<rect x="2.5" y="7" width="19" height="13" rx="2.5"/><path d="M8 7l1.6-2.6h4.8L16 7"/>'
    + '<circle cx="12" cy="13.5" r="3.6"/><circle class="accent" cx="18.2" cy="10" r="0.9"/>',
  // A wand's ring, its handle, and two bubbles on their way.
  bubbles: '<circle cx="8" cy="8" r="4.4"/><path d="M11.2 11.2 20.5 20.5"/>'
    + '<circle class="accent" cx="17.6" cy="5" r="2.1"/><circle class="accent" cx="20.6" cy="10.4" r="1.3"/>',
  // The cylinder, its valve and handle, and the hose curling off it.
  extinguisher: '<rect class="accent" x="7.5" y="8.5" width="8" height="13" rx="2.6"/>'
    + '<path d="M9.8 8.5V5.6h3.4v2.9"/><path d="M8.6 4.4h6.8"/><path d="M13.2 5.6c3.6 0 6.2 1.6 6.6 5.2"/>',
  // A dart in flight: its point, its shaft and its flights.
  dart: '<path d="M14.2 9.8 4.5 19.5"/><path d="M14.2 9.8l1.2-4.6 4.6-1.2-1.2 4.6z"/>'
    + '<path d="M7 17l-3.6-.4M7 17l.4 3.6M9.4 14.6l-3.6-.4M9.4 14.6l.4 3.6"/>',
  // A nail gun's body and grip, and the nail leaving its muzzle.
  nailer: '<path d="M3 6.5h12.5a1.5 1.5 0 0 1 1.5 1.5v3H10l-1.4 8.5H4.8L6.4 11H3z"/>'
    + '<path d="M17 9h4.5"/><path d="M19.5 7.6v2.8"/>',
  // A canopy with its scalloped edge, and the lines gathered to the harness.
  parachute: '<path d="M2.5 11a9.5 8 0 0 1 19 0"/>'
    + '<path d="M2.5 11q2.4-1.8 4.75 0t4.75 0 4.75 0 4.75 0"/>'
    + '<path d="M2.5 11 12 19.5 21.5 11M12 11v8.5"/><rect class="accent" x="10.4" y="19" width="3.2" height="2.8" rx="0.8"/>',
  // A three-pronged hook on its shaft, and the rope from the ring at its top.
  grapple: '<circle cx="12" cy="3.6" r="1.6"/><path d="M12 5.2v12.6"/>'
    + '<path d="M12 17.8c-3.6 0-6.4-2.4-6.4-6.2l2 1.4"/><path d="M12 17.8c3.6 0 6.4-2.4 6.4-6.2l-2 1.4"/>'
    + '<path d="M12 17.8v3.4"/><path d="M10.4 3.6C7 3.6 4.6 5 3 7.4"/>',
  // Two tanks on a frame, and the jets under them.
  jetpack: '<rect x="4.5" y="3" width="6" height="12" rx="3"/><rect x="13.5" y="3" width="6" height="12" rx="3"/>'
    + '<path d="M10.5 7.5h3"/><path class="accent" d="M5.8 16.5h3.4L7.5 21.5zM14.8 16.5h3.4l-1.7 5z"/>',
  // A swim ring, striped, on the water.
  ring: '<ellipse cx="12" cy="10" rx="8.5" ry="5"/><ellipse cx="12" cy="10" rx="4" ry="2"/>'
    + '<path d="M6 6.4l1.6 2.2M18 6.4l-1.6 2.2M8 14.4l1-2M16 14.4l-1-2"/>'
    + '<path class="accent-line" d="M2 19q2.5-2 5 0t5 0 5 0 5 0"/>',
  // An open hand, holding nothing.
  none: '<path d="M7.5 12V6.5a1.5 1.5 0 0 1 3 0V11V4.8a1.5 1.5 0 0 1 3 0V11V6a1.5 1.5 0 0 1 3 0v5.5V9a1.5 1.5 0 0 1 3 0v5.2c0 4-2.8 7.3-6.6 7.3-2.4 0-3.9-.9-5.2-2.6L4.4 15a1.5 1.5 0 0 1 2.3-1.9L7.5 14"/>',
};

/** The icon of a tool, by its id (tools.js), or of the empty hand (switcher.js EMPTY). */
export function toolIcon(id) {
  const svg = document.createElementNS(NS, 'svg');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('class', 'w-icon');
  svg.setAttribute('aria-hidden', 'true');
  svg.innerHTML = ICONS[id] ?? ICONS.none;
  return svg;
}

/** The ids that have an icon of their own. */
export const ICON_IDS = Object.keys(ICONS);
