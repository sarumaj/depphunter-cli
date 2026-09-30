// Small DOM helpers shared by the UI modules.

export const $ = id => document.getElementById(id);

export const fmt = new Intl.NumberFormat();

export function escapeHTML(s) {
  return String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** h('div', {class, style, onclick, …}, ...children) builds an element. */
export function h(tag, attributes = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attributes)) {
    if (k === 'class') el.className = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'style') el.style.cssText = v;
    else el.setAttribute(k, v);
  }
  for (const c of children.flat()) if (c != null && c !== false) el.append(c);
  return el;
}

/**
 * Opens something that wants the pointer - a menu, the backpack - once the pointer is
 * really free, and returns a function that calls the opening off if it has not
 * happened yet.
 *
 * Letting go of a pointer lock is asynchronous. Until `pointerlockchange` says it has
 * gone, pointer events still go to the locked canvas, and a menu already open would
 * read one of them as a click outside itself and shut again - which is a menu that
 * opens on the second press. So nothing is shown, and no click-outside handler can
 * see it, until the lock has been released. A browser that never reports the release
 * would leave the menu shut for good, so after `wait` milliseconds it opens anyway.
 *
 * Implements: REQ-UI-014
 */
export function whenUnlocked(open, wait = 250) {
  if (!document.pointerLockElement) {
    open();
    return () => {};
  }
  let done = false;
  const settle = run => {
    if (done) return;
    done = true;
    document.removeEventListener('pointerlockchange', change);
    clearTimeout(timer);
    if (run) open();
  };
  const change = () => { if (!document.pointerLockElement) settle(true); };
  document.addEventListener('pointerlockchange', change);
  const timer = setTimeout(() => settle(true), wait);
  document.exitPointerLock?.();
  return () => settle(false);
}

/** A toolbar button's count badge: hidden at nothing, else showing `text`. */
export function badge(el, count, text = count) {
  el.hidden = !count;
  el.textContent = text;
}

/** The dot a finding's row is marked with, colored by the row's severity class. */
export const sevDot = (tag = 'span') => h(tag, { class: 'sev-dot' });

/**
 * A finding as a row of the backpack or the findings list: its severity's dot, the
 * `lines` that say what and where it is, a word for its `state`, and one `button`.
 */
export function findingItem(attributes, lines, state, button) {
  return h('li', attributes, sevDot(), h('span', { class: 'body' }, lines), h('span', { class: 'state' }, state), button);
}

/**
 * The button that puts a finding in the backpack or takes it out again: `+`, or `✓`
 * while `kept()`. A press goes no further than the button, since its row opens the
 * finding, and calls `onToggle` with whether the finding was kept. The rest of the
 * options are the button's own attributes.
 *
 * A list drawn again on every change to the backpack needs nothing more. A `live`
 * button, the side panel's, redraws itself after a press instead, and is also marked
 * `kept` while the finding is in the backpack, since its row does not say so.
 */
export function catchToggle({ kept, onToggle, live = false, class: className, ...attributes }) {
  const look = () => ({
    className: live && kept() ? `${className} kept` : className,
    title: kept() ? 'In the backpack - press to take it out' : 'Put it in the backpack',
    text: kept() ? '✓' : '+',
  });
  const first = look();
  const button = h('button', {
    class: first.className, type: 'button', ...attributes, title: first.title,
    onclick: e => {
      e.stopPropagation();
      onToggle(kept());
      if (!live) return;
      const now = look();
      button.className = now.className;
      button.textContent = now.text;
      button.title = now.title;
    },
  }, first.text);
  return button;
}

/**
 * Opens and shuts a popover that a toolbar button owns - the backpack, the
 * photographs, a menu - by their ids, `panel` and `button`, and returns
 * `setOpen(on, back = true, then)`. `then` runs once it is shown, and `onShow(on)`
 * whenever it is shown or hidden.
 *
 * From the street the pointer is held at the reticle and the popover wants it, so
 * opening first readies the walker (`hold`) and then waits for the pointer to be
 * free (whenUnlocked); a second call before that calls the opening off. Shutting it
 * gives the walker the pointer back, unless `back` is false: a popover shut because
 * the pointer went to another control would snatch the mouse out of it. A
 * `streetOnly` popover opens and shuts at once from the map.
 *
 * Implements: REQ-UI-014
 */
export function drawer({ panel, button, walker, hold = () => {}, onShow = () => {}, streetOnly = false }) {
  let pending = () => {};
  return (on, back = true, then) => {
    pending();
    const show = () => {
      $(panel).hidden = !on;
      $(button).setAttribute('aria-expanded', on);
      onShow(on);
      then?.();
    };
    if (streetOnly && !walker()?.active) return show();
    if (on) {
      hold();
      pending = whenUnlocked(show);
      return;
    }
    show();
    if (back && walker()?.active) walker().lockPointer();
  };
}
