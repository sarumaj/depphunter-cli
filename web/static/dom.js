// Small DOM helpers shared by the UI modules.

export const $ = id => document.getElementById(id);

export const fmt = new Intl.NumberFormat();

export function escapeHTML(s) {
  return String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** h('div', {class, style, onclick, …}, ...children) builds an element. */
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
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
