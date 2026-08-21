/* instrument — behaviour. Optional.

   The kit draws state and declares roles. A role is a promise: `role="listbox"`
   tells assistive technology that the arrow keys work. Until the promise is
   kept, the component is not "unfinished", it LIES — and that is worse than
   never declaring the role at all.

   This file keeps the promise and does nothing else.

   ── What it does NOT do ───────────────────────────────────────────────────

   It does not choose appearance. It sets attributes that are ALREADY in the
   markup contract — tabindex, aria-selected, aria-checked, aria-expanded —
   and what they look like is for CSS to decide. The moment the script starts
   painting, the kit loses its main property: the application can no longer
   override the look without touching the behaviour.

   There are exactly two exceptions, and both are named where they stand:
   `--fill` on the slider is a data channel rather than styling (syncSlider),
   and the markup of a toast is built by JS, because a queue and a timer are
   beyond CSS (toast). Nowhere else does the kit write to style or className.

   It is not required. Without it everything that needs no keyboard works as
   before: disclosure on <details>, the popover on the Popover API, the modal
   on <dialog>, validation on :user-invalid.

   ── Including it ──────────────────────────────────────────────────────────

     <script type="module" src="src/kit.js"></script>

   A module rather than a classic script: it has named exports, and Svelte and
   React wrappers will sit on them later. There is still no build step.

   An application that drives the markup itself opts out of the self-start
   with `data-instrument="manual"` on <html> and calls start() by hand — the
   explanation is at the end of the file. Three handles, and all three exist
   for one purpose: so that an attribute never has two owners.

     data-instrument="manual"   on <html> — do not start by yourself
     start(root, {observe})     do not raise an observer
     data-roving="manual"       on a group — the application drives tabindex

   ── Why delegation rather than initialisation ─────────────────────────────

   The listeners sit on the document and the elements are looked up at the
   moment of a keypress. For an agent interface that is not an optimisation
   but the only workable option: queue rows, steps and tree nodes ARRIVE while
   the work runs, and any init(el) at startup would miss everything that
   appeared later.

   The one thing that cannot be done lazily is the initial tabindex: a group
   without it is not reachable by Tab at all. So a MutationObserver watches
   for groups appearing, rather than a human.
   ========================================================================= */

/* Everything the kit SAYS to a human is collected here and only here.
 *
 * Four phrases wired into the code mean a library declared framework-agnostic
 * speaks to a screen reader in one language with nothing to reach in and
 * change it. Two of the four have an attribute override on the button, two
 * have none at all.
 *
 * The attribute stays and still wins: it has a different job — to say
 * something else for ONE button. What changes here is the language of the
 * whole screen:
 *
 *     import { strings } from '@keshon/instrument/js';
 *     strings.copied = 'Copié';
 *
 * An object rather than a translation function: a four-string dictionary that
 * needs substitution in one place. A full i18n here would be machinery
 * heavier than the task, and a key nobody translates is a promise the kit
 * does not keep.
 */
export const strings = {
  toasts: 'Notifications',
  copied: 'Copied',
  copyFailed: 'Could not copy',
  tagRemoved: (label) => `Tag ${label} removed`,
};

/* The roles that carry a keyboard contract, and everything worth knowing
   about them.

   The list is closed. It matches the markup-contract table in the design
   principles — if a row appears there, it has to appear here too, or the kit
   starts promising the unkeepable again. */
const GROUPS = {
  listbox: {
    item: '[role="option"]',
    axis: 'vertical',
    // Selection follows focus: a single-select listbox behaves that way per
    // APG, and a task queue is exactly that case.
    follows: 'aria-selected',
  },
  tree: {
    item: '[role="treeitem"]',
    axis: 'vertical',
    follows: 'aria-selected',
  },
  menu: {
    item: '[role^="menuitem"]',
    axis: 'vertical',
    // A menu item has no selection: it is an action, not a choice.
    follows: null,
  },
  radiogroup: {
    item: '[role="radio"]',
    axis: 'horizontal',
    follows: 'aria-checked',
  },
  tablist: {
    item: '[role="tab"]',
    axis: 'horizontal',
    follows: 'aria-selected',
  },
};

const GROUP_SELECTOR = Object.keys(GROUPS).map((r) => `[role="${r}"]`).join(',');

/* Multi-select removes "selection follows focus".
 *
 * Per APG a single-select listbox selects whatever focus lands on — and a task
 * queue is exactly that case. A multi-select one behaves differently: an arrow
 * ONLY moves focus, and Space toggles. Otherwise walking a filter bar with the
 * arrows would switch on everything walked past.
 *
 * The flag is looked for on the group rather than on the item:
 * aria-multiselectable is the group's attribute, and it does not change the
 * role — a listbox stays a listbox. So the role dictionary needed no change:
 * multiplicity is a layer over an entry, not a sixth line in it.
 *
 * `follows` moves into `multi` rather than being dropped: the toggle needs the
 * attribute name to know what to write. Menus never get here — there `follows`
 * is empty, there is no selection at all, and nothing to layer over. */
function specOf(group) {
  const spec = GROUPS[group.getAttribute('role')];
  if (!spec || !spec.follows) return spec || null;
  if (group.getAttribute('aria-multiselectable') !== 'true') return spec;
  return { ...spec, follows: null, multi: spec.follows };
}

/* The axis comes from aria-orientation when it is there: horizontal trees
   and vertical tab lists exist, and markup is entitled to say so. */
function axisOf(group, spec) {
  return group.getAttribute('aria-orientation') || spec.axis;
}

/* The items of a group.
 *
 * closest() is mandatory: nested groups exist (a submenu inside a menu, a
 * group inside a tree), and without it an arrow in the parent would hop across
 * somebody else's items. Invisible ones are dropped — neither the platform nor
 * we put focus on them.
 *
 * `aria-disabled` is NOT dropped, and that is no oversight. An item declared
 * unavailable has to stay discoverable: a person walking with the arrows must
 * learn that the action exists and why it does not work right now. An item
 * dropped from the list is never learned about at all — it is simply absent,
 * and the difference between "you cannot" and "there is none" disappears.
 *
 * It follows that the roving tabindex has to be written to it: an item absent
 * from the list never receives `-1`, while a native `<button>` holds `0` — and
 * the result is the exact inverse of the promise: the arrows skip it and Tab
 * stops on it.
 *
 * `disabled` is another matter: the platform removes it from the tab order
 * itself and refuses it focus, and there is nothing here to argue about. */
function itemsOf(group, spec, withHidden) {
  return [...group.querySelectorAll(spec.item)].filter(
    (el) =>
      el.closest(GROUP_SELECTOR) === group &&
      !el.disabled &&
      (withHidden || el.offsetParent !== null),
  );
}

/** The item is declared unavailable: walk to it, but do nothing on it. */
function isDisabled(el) {
  return el.getAttribute('aria-disabled') === 'true';
}

/* Who drives the roving tabindex.
 *
 * Of the seven attributes the kit writes, six leave through a cancellable
 * event: the application calls preventDefault and the markup is left alone.
 * The seventh, tabindex, written unconditionally, is the one place an
 * attribute could end up with two owners: React, having rendered tabIndex
 * from state, and the kit, rewriting it in the next microtask.
 *
 * An event is the wrong tool here, and that is worth recording so it is not
 * proposed again. roving() is called from refresh(), that is on EVERY batch of
 * document mutations — an event per group per update would be noise, and the
 * application would answer the same thing every time. The answer here is not
 * about one particular move but about ownership as a whole, which makes it a
 * declaration rather than a decision: an attribute on the group, like
 * everything else in this kit.
 *
 * The kit does not stop moving focus because of it: the application took the
 * attribute, it did not forbid a human to walk with the arrows. A cancelled
 * inst:select behaves in exactly the same way. */
function rovingOwned(group) {
  return group.dataset.roving !== 'manual';
}

/* The roving tabindex: exactly one item of a group is reachable by Tab.
 *
 * That is the whole of it. A list of two hundred rows each with tabindex="0"
 * costs two hundred presses to get past; a list where none of them has it is
 * unreachable altogether. */
function roving(group, spec, current) {
  const items = itemsOf(group, spec);
  if (!items.length) return;
  /* The anchor of the tab order prefers an available item.
   *
   * Walking to an unavailable one with the arrows is promised behaviour, but
   * making it the ONLY Tab stop would greet somebody entering the group for
   * the first time with the news that nothing here can be done. A selected
   * item still outranks it: if the markup says `aria-selected="true"`, there
   * is nothing to argue about. */
  const active = current && items.includes(current)
    ? current
    : items.find((el) => el.getAttribute(spec.follows || 'aria-selected') === 'true')
      || items.find((el) => !isDisabled(el))
      || items[0];
  if (rovingOwned(group)) {
    for (const el of items) el.tabIndex = el === active ? 0 : -1;
  }
  return active;
}

/* Move focus and, if the role calls for it, the selection with it.
 *
 * The selection is cleared from the WHOLE group rather than toggled on the
 * target: two selected items in a single-select list is a state the markup
 * cannot get back out of. */
function move(group, spec, to) {
  if (rovingOwned(group)) {
    for (const el of itemsOf(group, spec)) el.tabIndex = el === to ? 0 : -1;
  }
  /* In a multi-select group an arrow does NOT select. Otherwise walking a
     filter bar would switch on every filter walked past, and "selection
     follows focus" from a single-select list would turn into "everything gets
     selected". There it is Space that toggles, and it arrives here by another
     road: through click.                                                    */
  if (!spec.multi) select(group, spec, to);
  to.focus();
}

/* Move the selection onto an item and say so.
 *
 * Separate from move, because mouse and keyboard arrive here differently: an
 * arrow moves focus and selection together, a click moves selection only.
 *
 * A cancelled event leaves the markup untouched, but focus still goes where it
 * was going: the application declined to drive the state itself, it did not
 * forbid a human to move. */
function select(group, spec, to) {
  /* An unavailable item moves focus and does not move state. The check sits
     here rather than in two handlers: both roads arrive through select() —
     arrow-and-Space and the mouse click — so this fork belongs where the
     multi-select fork already is. */
  if (isDisabled(to)) return;

  /* Multi-select toggles ONE item and leaves its neighbours alone — that is
     what makes it different. The branch sits here rather than in the key
     handler because both roads arrive through select(): arrow-and-Space and
     the mouse click alike. One fork, one place for it.                    */
  if (spec.multi) {
    const on = to.getAttribute(spec.multi) === 'true';
    const value = to.dataset.value ?? to.textContent.trim();
    if (!emit(to, 'select', { value, selected: !on })) return;
    to.setAttribute(spec.multi, String(!on));
    return;
  }
  if (!spec.follows || to.getAttribute(spec.follows) === 'true') return;
  if (!emit(to, 'select', { value: to.dataset.value ?? to.textContent.trim() })) return;
  const items = itemsOf(group, spec);
  for (const el of items) {
    el.setAttribute(spec.follows, String(el === to));
  }
  panels(group, spec, items, to);
}

/* The panel of the selected tab.
 *
 * The one named by aria-controls is shown and the rest are hidden. There is
 * nothing to guess: the link is stated in the markup, and without this line a
 * tab switches to the eye while the content stays as it was.
 *
 * Tabs only: a list row and a menu item have no panel. */
function panels(group, spec, items, to) {
  if (spec.item !== '[role="tab"]') return;
  const doc = group.ownerDocument;
  for (const el of items) {
    const id = el.getAttribute('aria-controls');
    if (!id) continue;
    const panel = doc.getElementById(id);
    if (panel) panel.hidden = el !== to;
  }
}

function step(items, from, delta) {
  const i = items.indexOf(from);
  if (i < 0) return items[0];
  // Wrapping: a list has no "end" past which there is nowhere to go. APG
  // allows either behaviour, but in a task queue wrapping saves the person
  // from a dead end on the last row.
  return items[(i + delta + items.length) % items.length];
}

/* Expanding and collapsing a tree node.
 *
 * The forward arrow opens a collapsed node and, on an open one, goes to the
 * first child. The back arrow is symmetrical. This is the contract of the
 * treeitem role, and without it aria-expanded in the markup is decoration. */
/* The descendants of a node in a flat tree: everything that follows with a
 * greater level, up to the first row at the same level or lower. Nesting here
 * lives in aria-level rather than in the markup: a tree of ten thousand nodes
 * cannot be built any other way. */
function descendants(group, spec, item) {
  const items = itemsOf(group, spec, true);
  const level = Number(item.getAttribute('aria-level') || 1);
  const out = [];
  for (let i = items.indexOf(item) + 1; i < items.length; i++) {
    if (Number(items[i].getAttribute('aria-level') || 1) <= level) break;
    out.push(items[i]);
  }
  return out;
}

/* Collapse or expand a node.
 *
 * The kit does not draw — it sets hidden, that is a state the platform already
 * has. The event is cancellable: an application that renders its tree from
 * data will remove the rows itself and does not need a second hand. */
function setExpanded(group, spec, item, open) {
  if (!emit(item, 'expand', { open })) return;
  item.setAttribute('aria-expanded', String(open));
  for (const d of descendants(group, spec, item)) {
    if (open) {
      const level = Number(d.getAttribute('aria-level') || 1);
      const parent = closestParent(group, spec, d, level);
      d.hidden = parent ? parent.getAttribute('aria-expanded') === 'false' : false;
    } else {
      d.hidden = true;
    }
  }
  /* The roving tabindex is reset HERE, not by the observer.
   *
   * The observer is subscribed to childList, and collapsing a node means
   * setting the hidden attribute — an edit it does not see. Without this line
   * the anchor of the tab order stays on a hidden descendant: it holds
   * tabindex="0", every visible node holds -1, and the whole tree drops out of
   * the tab order — precisely the state roving() was written against. */
  roving(group, spec, item);
}

/* The nearest ancestor of a row by level — needed so that expanding does not
 * reveal the descendants of a node collapsed deeper down. */
function closestParent(group, spec, item, level) {
  const items = itemsOf(group, spec, true);
  for (let i = items.indexOf(item) - 1; i >= 0; i--) {
    const l = Number(items[i].getAttribute('aria-level') || 1);
    if (l < level) return items[i];
  }
  return null;
}

function treeArrow(group, spec, item, forward) {
  const expandable = item.hasAttribute('aria-expanded');
  const open = item.getAttribute('aria-expanded') === 'true';

  if (forward) {
    if (expandable && !open) {
      setExpanded(group, spec, item, true);
      return true;
    }
    if (expandable && open) {
      const items = itemsOf(group, spec);
      const next = items[items.indexOf(item) + 1];
      if (next) move(group, spec, next);
      return true;
    }
    return false;
  }

  if (expandable && open) {
    setExpanded(group, spec, item, false);
    return true;
  }
  // A collapsed node sends focus to its parent: the level comes from
  // aria-level, because the nesting in the markup may be flat.
  const level = Number(item.getAttribute('aria-level') || 1);
  const items = itemsOf(group, spec);
  for (let i = items.indexOf(item) - 1; i >= 0; i--) {
    if (Number(items[i].getAttribute('aria-level') || 1) < level) {
      move(group, spec, items[i]);
      return true;
    }
  }
  return false;
}

/* Moving between the columns of a cascader.
 *
 * A column is an ordinary listbox: one Tab stop, the arrows along it,
 * selection following focus. What a listbox has no notion of is the column
 * BESIDE it, and the cross-axis arrows are free there — a vertical group
 * gives ArrowLeft and ArrowRight no meaning at all.
 *
 * Scoped by CLASS rather than by role, and that distinction matters. Teaching
 * every listbox in the kit to walk sideways would be changing a role's
 * promise for the sake of one component; a listbox that happens to stand
 * inside `.inst-cascader` gains a behaviour of the cascader instead. The role
 * dictionary is untouched, and a listbox anywhere else answers the arrows
 * exactly as it did.
 *
 * Forward lands on what the next column already has selected, or on its first
 * item; back does the same. Nothing is opened and nothing is rendered here:
 * the column beside is the application's, and if it is empty there is nowhere
 * to go and the key falls through to the platform. */
function cascadeArrow(group, forward) {
  const set = group.closest('.inst-cascader');
  if (!set) return false;
  const cols = [...set.querySelectorAll('.inst-cascader-col')];
  const next = cols[cols.indexOf(group) + (forward ? 1 : -1)];
  if (!next) return false;
  const spec = specOf(next);
  if (!spec) return false;
  const items = itemsOf(next, spec);
  if (!items.length) return false;
  move(next, spec, items.find((n) => n.getAttribute('aria-selected') === 'true') || items[0]);
  return true;
}

const NEXT = { vertical: 'ArrowDown', horizontal: 'ArrowRight' };
const PREV = { vertical: 'ArrowUp', horizontal: 'ArrowLeft' };

function onKeydown(e) {
  if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;

  const item = e.target.closest?.(
    '[role="option"],[role="treeitem"],[role^="menuitem"],[role="radio"],[role="tab"]',
  );
  if (!item) return;

  const group = item.closest(GROUP_SELECTOR);
  if (!group) return;
  const spec = specOf(group);
  if (!spec) return;

  const axis = axisOf(group, spec);
  const items = itemsOf(group, spec);
  if (!items.length) return;

  // A tree takes the cross-axis arrows for expansion — but only those.
  if (group.getAttribute('role') === 'tree') {
    const forward = e.key === 'ArrowRight';
    if (forward || e.key === 'ArrowLeft') {
      if (treeArrow(group, spec, item, forward)) e.preventDefault();
      return;
    }
  }


  // A column of a cascader takes them for the column beside it.
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
    if (cascadeArrow(group, e.key === 'ArrowRight')) {
      e.preventDefault();
      return;
    }
  }

  switch (e.key) {
    case NEXT[axis]:
      e.preventDefault();
      move(group, spec, step(items, item, 1));
      return;
    case PREV[axis]:
      e.preventDefault();
      move(group, spec, step(items, item, -1));
      return;
    case 'Home':
      e.preventDefault();
      move(group, spec, items[0]);
      return;
    case 'End':
      e.preventDefault();
      move(group, spec, items[items.length - 1]);
      return;
    case ' ':
    case 'Enter':
      // A radio and a tab were already selected by the arrow — there is
      // nothing left to press. For the rest the click is handed to the
      // platform: the application's handler is on it, and inventing a second
      // event would be pointless.
      if (spec.follows && group.getAttribute('role') !== 'radiogroup') {
        move(group, spec, item);
      }
      if (e.key === ' ') e.preventDefault();
      // Synthesising a click on an unavailable item would call the
      // application's handler on behalf of a person who pressed "you cannot".
      if (!isDisabled(item)) item.click();
      return;
    default:
  }
}

/* Focus arriving by mouse has to move the tabindex too: otherwise the next
 * Tab leaves from somewhere other than where the person just pointed. */
function onFocusin(e) {
  const group = e.target.closest?.(GROUP_SELECTOR);
  if (!group) return;
  const spec = specOf(group);
  if (!spec) return;
  const item = e.target.closest(spec.item);
  if (item) roving(group, spec, item);
}

/* ── Toasts ────────────────────────────────────────────────────────────────

   Here the script does what CSS cannot do in principle: it counts time and
   holds a queue. The look is still entirely in CSS — not one assignment to
   style.

   The region is opened as a popover ONCE and never closed. The reason is not
   convenience: a closed popover is display: none, and a live region in
   display: none is not announced — a screen reader would hear no toast at
   all.
   ───────────────────────────────────────────────────────────────────────── */

const TOAST_LIMIT = 4;
const TOAST_DURATION = 5000;

function toastRegion(doc) {
  // Our own region is the one living in the top layer. Without [popover] any
  // markup carrying this class is picked up: a page SHOWING the region as an
  // example would receive real notifications into it.
  let region = doc.querySelector('.inst-toasts[popover]');
  if (region) {
    ensureOpen(region);
    return region;
  }
  region = doc.createElement('div');
  region.className = 'inst-toasts';
  region.setAttribute('popover', 'manual');
  // The live region is polite: a toast reports a result rather than
  // interrupting the work. An error does interrupt — role="alert" sits on the
  // error itself.
  region.setAttribute('aria-live', 'polite');
  region.setAttribute('aria-label', strings.toasts);
  doc.body.append(region);
  ensureOpen(region);
  return region;
}

/* Open the region once.
 *
 * It cannot be moved to the TOP of the top layer, and that was checked:
 * hidePopover() followed by showPopover() — both within one task and across a
 * task — does not raise the region above a modal opened with showModal(). The
 * modal dialog stays higher.
 *
 * So there is no point pretending otherwise: a toast lives above any z-index
 * and any overflow: hidden, but NOT above a modal. The limitation is recorded
 * in the reference along with what to do instead.                         */
function ensureOpen(region) {
  try {
    if (!region.matches(':popover-open')) region.showPopover();
  } catch {
    // A region with no popover attribute means the application decided
    // otherwise, and that is its right: the toasts will work, just not in the
    // top layer.
  }
}

/* Remove once the transition has played out.
 *
 * transitionend with a timer as backstop: under prefers-reduced-motion the
 * transition collapses to 0.01ms and the event may not arrive at all — without
 * the backstop the toast would stay on screen forever. */
function removeToast(el) {
  if (el.dataset.state === 'leaving') return;
  el.dataset.state = 'leaving';
  let done = false;
  const finish = () => {
    if (done) return;
    done = true;
    el.remove();
  };
  el.addEventListener('transitionend', finish, { once: true });
  setTimeout(finish, 400);
}

/**
 * Show a toast.
 *
 * The kit builds the markup, but all of it is described in the reference: if
 * the standard one is not enough, assemble your own node and put it into
 * `.inst-toasts` yourself — the queue and the timer will pick it up the same
 * way.
 */
export function toast(options = {}) {
  const {
    title = '',
    text = '',
    tone = '',
    duration = TOAST_DURATION,
    action = null, // { label, onClick }
    doc = document,
  } = options;

  const region = toastRegion(doc);

  const el = doc.createElement('div');
  el.className = 'inst-toast';
  if (tone) el.dataset.tone = tone;
  // An error has to interrupt: it reports that the action did NOT happen.
  el.setAttribute('role', tone === 'error' ? 'alert' : 'status');

  const body = doc.createElement('div');
  body.className = 'inst-toast-body';
  if (title) {
    const t = doc.createElement('div');
    t.className = 'inst-toast-title';
    t.textContent = title;
    body.append(t);
  }
  if (text) {
    const s = doc.createElement('div');
    s.className = 'inst-toast-text';
    s.textContent = text;
    body.append(s);
  }
  el.append(body);

  if (action) {
    const wrap = doc.createElement('div');
    wrap.className = 'inst-toast-actions';
    const btn = doc.createElement('button');
    btn.className = 'inst-btn inst-btn--sm inst-btn--ghost';
    btn.type = 'button';
    btn.textContent = action.label;
    btn.addEventListener('click', () => {
      action.onClick?.();
      removeToast(el);
    });
    wrap.append(btn);
    el.append(wrap);
  }

  region.append(el);

  // The ceiling of the queue. Twenty toasts in a row are not a message but a
  // wall: the oldest leave so that the newest can be seen.
  const live = [...region.children].filter((x) => x.dataset.state !== 'leaving');
  for (const old of live.slice(0, Math.max(0, live.length - TOAST_LIMIT))) {
    removeToast(old);
  }

  if (duration > 0) {
    let timer = setTimeout(() => removeToast(el), duration);
    /* The timer freezes under the cursor and on focus. A toast that vanishes
       exactly as it starts being read is a lost message, and WCAG 2.2.1 asks
       for precisely this: time must be pausable. */
    const hold = () => clearTimeout(timer);
    const resume = () => { timer = setTimeout(() => removeToast(el), duration); };
    el.addEventListener('pointerenter', hold);
    el.addEventListener('pointerleave', resume);
    el.addEventListener('focusin', hold);
    el.addEventListener('focusout', resume);
  }

  return el;
}

/* ==========================================================================
   Drawn promises

   The same law as for roles, applied to appearance: if the kit DREW an
   affordance, it has to keep it. The copy button, the remove cross, the
   ew-resize cursor on an axis label — these are promises made to the user in
   pictures. A promise not kept is not "unfinished", it lies.

   Every behaviour here:
   · draws nothing — it sets attributes from the markup contract;
   · sends a cancellable event, so the application can step in or take the
     work over;
   · works by delegation, so nodes may arrive while the work runs.
   ========================================================================== */

/** A cancellable event of the kit. false means the application took over. */
function emit(el, name, detail) {
  return el.dispatchEvent(
    new CustomEvent('inst:' + name, { bubbles: true, cancelable: true, detail }),
  );
}

/* ── The live region for short messages ──────────────────────────────────
   Separate from the toasts: "copied" is a confirmation of an action rather
   than a notification, and it has no business queueing. */
function announce(doc, text) {
  let live = doc.querySelector('[data-inst-live]');
  if (!live) {
    live = doc.createElement('div');
    live.className = 'inst-u-visually-hidden';
    live.setAttribute('aria-live', 'polite');
    live.dataset.instLive = '';
    doc.body.append(live);
  }
  live.textContent = '';
  // An empty string and then the text: a repeat of the same message is not
  // announced otherwise — the region decides nothing has changed.
  //
  // A timer rather than requestAnimationFrame: a hidden tab gets no frames,
  // and the announcement would never happen at all.
  setTimeout(() => { live.textContent = text; }, 0);
}

/* ── Copying ─────────────────────────────────────────────────────────────

   The source: data-copy on the button, otherwise the text of the nearest
   inst-copyable or inst-code minus the text of the button itself.

   textContent rather than innerText: the latter depends on layout and returns
   an empty string for hidden content — a collapsed code block would be copied
   into nothing. */
function copySource(btn) {
  if (btn.dataset.copy !== undefined && btn.dataset.copy !== '') return btn.dataset.copy;
  const host = btn.closest('.inst-copyable, .inst-code, .inst-copy-host');
  if (!host) return '';
  const clone = host.cloneNode(true);
  for (const b of clone.querySelectorAll('.inst-copy')) b.remove();
  return clone.textContent.trim();
}

async function onCopy(btn) {
  const text = copySource(btn);
  if (!text || !emit(btn, 'copy', { text })) return;

  let ok = true;
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // An insecure origin rejects the write silently. Answering with silence
    // would be a lie: the button would look as though it had worked.
    ok = false;
  }

  btn.dataset.copied = ok ? 'true' : 'false';
  // Success and failure take DIFFERENT labels: one shared between them would
  // announce "copied" where the write was rejected — exactly the lie that
  // tracking the failure exists to remove.
  const said = ok
    ? btn.dataset.copiedLabel || strings.copied
    : btn.dataset.failedLabel || strings.copyFailed;
  announce(btn.ownerDocument, said);
  clearTimeout(+btn.dataset.instTimer || 0);
  btn.dataset.instTimer = setTimeout(() => {
    delete btn.dataset.copied;
  }, 1400);
}

/* ── Removing a tag ──────────────────────────────────────────────────────

   Focus has to move to a neighbour: a removed element takes it nowhere, and a
   keyboard user ends up at the start of the document. */
function onTagRemove(btn) {
  const tag = btn.closest('.inst-tag');
  if (!tag) return;
  const label = tag.textContent.trim();
  if (!emit(tag, 'remove', { value: tag.dataset.value ?? label })) return;

  const next = tag.nextElementSibling?.querySelector?.('.inst-tag-remove')
    || tag.previousElementSibling?.querySelector?.('.inst-tag-remove')
    || tag.parentElement;
  tag.remove();
  if (next instanceof HTMLElement) {
    if (!next.matches('.inst-tag-remove') && next.tabIndex < 0) next.tabIndex = -1;
    next.focus();
  }
  announce(btn.ownerDocument, strings.tagRemoved(label));
}

/* ── Dragging an axis label ──────────────────────────────────────────────

   The ew-resize cursor on inst-num-axis means "drag from here". The step comes
   from the field itself, the acceleration from modifiers: Shift ×10, Alt ×0.1.
   The value leaves through input and change events, as with native input, so
   frameworks see it without a line of glue. */
function onAxisDown(e, axis) {
  const field = axis.closest('.inst-num-field');
  const input = field?.querySelector('input[type="number"]');
  if (!input || input.disabled || input.readOnly) return;

  const step = Number(input.step) || 1;
  const startX = e.clientX;
  const startValue = Number(input.value) || 0;
  let moved = false;

  axis.setPointerCapture(e.pointerId);
  e.preventDefault();

  const onMove = (ev) => {
    const mul = ev.shiftKey ? 10 : ev.altKey ? 0.1 : 1;
    const delta = Math.round((ev.clientX - startX) / 2) * step * mul;
    if (!delta && !moved) return;
    moved = true;
    let v = startValue + delta;
    if (input.min !== '') v = Math.max(Number(input.min), v);
    if (input.max !== '') v = Math.min(Number(input.max), v);
    // The float tail is eaten by the step: 0.1 + 0.2 gives
    // 0.30000000000000004 otherwise.
    const decimals = (String(step).split('.')[1] || '').length;
    input.value = decimals ? v.toFixed(decimals) : String(v);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  };

  const onUp = () => {
    axis.releasePointerCapture?.(e.pointerId);
    axis.removeEventListener('pointermove', onMove);
    axis.removeEventListener('pointerup', onUp);
    axis.removeEventListener('pointercancel', onUp);
    if (moved) input.dispatchEvent(new Event('change', { bubbles: true }));
  };

  axis.addEventListener('pointermove', onMove);
  axis.addEventListener('pointerup', onUp);
  axis.addEventListener('pointercancel', onUp);
}

/* ── The slider value: into <output> and into the fill of the track ──────
   The link to <output> is already declared by the for attribute. Synchronising
   it by hand means writing the same line in every application.

   --fill is the same thing for the FILL: the track is painted up to the thumb,
   and the fraction is computed here because CSS cannot read the value of an
   input. The markup declares the initial value itself (a data channel), and
   the script keeps it in agreement while dragging: without the script the fill
   stays at the value the page arrived with, and that is the truth rather than
   zero.

   The style is INLINE, and this is the one place the kit writes to style —
   that is, the one thing an application cannot override without !important.
   That is deliberate: the property has one owner, and the markup declares it
   the same way. There is nowhere to move to a class — the fraction is
   continuous. */
function syncSlider(input) {
  const min = Number(input.min === '' ? 0 : input.min);
  const max = Number(input.max === '' ? 100 : input.max);
  // A degenerate range: max === min. The division would give NaN and the fill
  // would disappear entirely instead of honestly showing the edge.
  const share = max > min ? (Number(input.value) - min) / (max - min) : 0;
  input.style.setProperty('--fill', `${Math.min(1, Math.max(0, share)) * 100}%`);

  if (!input.id) return;
  for (const out of input.ownerDocument.querySelectorAll(`output[for~="${CSS.escape(input.id)}"]`)) {
    // Comparing before writing is not a micro-optimisation but a termination
    // condition.
    //
    // Assigning textContent replaces the text node EVEN when the text is the
    // same, that is it makes a childList edit. The observer wakes on it and
    // calls refresh(), refresh() writes the same text again — and a page with
    // a slider never goes idle again. Measured: the tab stops responding, and
    // the pixel sweep hung dead on that page.
    if (out.textContent !== input.value) out.textContent = input.value;
  }
}

/* ── "Select all" in a table ─────────────────────────────────────────────
   The checkbox in the header of the selection column. The in-between state is
   indeterminate: "some rows are selected" cannot be said with checked, nor
   with its absence. */
function rowBoxes(table) {
  return [...table.querySelectorAll('tbody .inst-col-select input[type="checkbox"], tbody td:first-child input[type="checkbox"]')];
}

function syncSelectAll(table) {
  const head = table.querySelector('thead .inst-col-select input[type="checkbox"]');
  if (!head) return;
  const boxes = rowBoxes(table);
  const on = boxes.filter((b) => b.checked).length;
  head.checked = on > 0 && on === boxes.length;
  head.indeterminate = on > 0 && on < boxes.length;
}

function onTableToggle(box) {
  const table = box.closest('.inst-table');
  if (!table) return;
  const inHead = !!box.closest('thead');

  if (inHead) {
    if (!emit(table, 'selectall', { checked: box.checked })) return;
    for (const b of rowBoxes(table)) {
      if (b.disabled) continue;
      b.checked = box.checked;
      b.closest('tr')?.setAttribute('aria-selected', String(box.checked));
    }
    box.indeterminate = false;
  } else {
    box.closest('tr')?.setAttribute('aria-selected', String(box.checked));
  }
  syncSelectAll(table);
}

/* ── Inserting a substitution into a field ───────────────────────────────

   A button labelled "{{name}}" that puts nothing into the field is the same
   unkept promise as a role without a keyboard.

   It is inserted AT THE CARET rather than at the end: a person puts the cursor
   where the substitution is wanted and expects it there. A selection is
   replaced. After the insert focus returns to the field — otherwise there is
   nowhere to type the next letter. */
function onInsert(btn) {
  const doc = btn.ownerDocument;
  const sel = btn.dataset.insertInto;
  const field = sel ? doc.querySelector(sel) : btn.closest('.inst-field')?.querySelector('textarea, input');
  if (!field) return;

  const text = btn.dataset.insert ?? btn.textContent.trim();
  if (!emit(btn, 'insert', { text, field })) return;

  const start = field.selectionStart ?? field.value.length;
  const end = field.selectionEnd ?? field.value.length;
  field.value = field.value.slice(0, start) + text + field.value.slice(end);
  field.focus();
  const caret = start + text.length;
  field.setSelectionRange?.(caret, caret);
  // A native event rather than one of ours: frameworks see the change without
  // a line of glue.
  field.dispatchEvent(new Event('input', { bubbles: true }));
}

/* ── Expand all / collapse all ───────────────────────────────────────────

   A list of six <details> costs six presses to read in full and six more to
   put back. Every file manager and every mail client has this button for
   exactly that reason.

   The state is read off the CONTENT rather than remembered: if even one node
   is closed, a press opens everything. A button that remembers its own "I am
   collapsing next" sooner or later disagrees with what the person opened by
   hand.

   The scope is the nearest panel, or the element named by a selector in
   `data-details-all`. */
function onDetailsAll(btn) {
  const doc = btn.ownerDocument;
  const sel = btn.dataset.detailsAll;
  const scope = (sel && doc.querySelector(sel)) || btn.closest('.inst-panel') || btn.parentElement;
  if (!scope) return;

  const all = [...scope.querySelectorAll('details')];
  if (!all.length) return;

  const expand = all.some((d) => !d.open);
  if (!emit(btn, 'details-all', { expand, count: all.length })) return;
  for (const d of all) d.open = expand;

  btn.setAttribute('aria-expanded', String(expand));
  const label = expand ? btn.dataset.labelCollapse : btn.dataset.labelExpand;
  if (label) btn.textContent = label;
}

function onClick(e) {
  const copy = e.target.closest?.('.inst-copy');
  if (copy) { onCopy(copy); return; }

  const detailsAll = e.target.closest?.('[data-details-all]');
  if (detailsAll) { onDetailsAll(detailsAll); return; }

  const tagX = e.target.closest?.('.inst-tag-remove');
  if (tagX) { onTagRemove(tagX); return; }

  const insert = e.target.closest?.('.inst-insert');
  if (insert) { onInsert(insert); return; }

  /* A click on a tree twist expands the node.
   *
   * The twist is drawn pressable — `cursor: pointer` and a 90° turn on
   * `aria-expanded` — and with no branch under it a click only selects,
   * leaving the tree impossible to open with a mouse at all. A drawn promise
   * has to be kept, or the styling lies about the behaviour.
   *
   * The selection is NOT moved with it: the twist and the row are two
   * different targets, and being neighbours is no reason to glue their
   * meanings together. `inst:expand` is cancellable, as it is on the keyboard:
   * an application that renders its tree from data will remove the rows
   * itself. */
  const twist = e.target.closest?.('.inst-tree-twist');
  if (twist) {
    const node = twist.closest('[role="treeitem"]');
    const tree = node?.closest('[role="tree"]');
    const treeSpec = tree && specOf(tree);
    if (node && treeSpec && node.hasAttribute('aria-expanded')) {
      setExpanded(tree, treeSpec, node, node.getAttribute('aria-expanded') !== 'true');
      return;
    }
  }

  // A click on a tab or an option. Without this the kit draws the selected
  // tab, the arrows switch it and the mouse does not: focusin only moves the
  // roving tabindex.
  const group = e.target.closest?.(GROUP_SELECTOR);
  if (!group) return;
  const spec = specOf(group);
  const item = spec && e.target.closest(spec.item);
  if (item && itemsOf(group, spec).includes(item)) select(group, spec, item);
}

function onChange(e) {
  const t = e.target;
  if (!(t instanceof HTMLElement)) return;
  if (t.matches?.('.inst-table input[type="checkbox"]')) onTableToggle(t);
}

function onInput(e) {
  const t = e.target;
  if (t instanceof HTMLElement && t.matches?.('.inst-slider')) syncSlider(t);
}

/* A popover opened — items appeared in the layout that were not in it.
 *
 * Opening a popover does not edit childList, so the observer does not wake on
 * it, and itemsOf() filters closed items out by offsetParent. Without this
 * subscription the menu items hold their native tabIndex 0 until the first
 * focusin: Tab stops on every one of them, and the group stops being a single
 * control exactly where the registry granted it an exemption on the promise
 * of the opposite.
 *
 * The listener is installed in the CAPTURE phase: toggle does not bubble, and
 * there is no other way to catch it on the root. This is cheaper than
 * attributes: true across the whole document — such an observer would wake on
 * every tabindex the kit writes itself, that is on its own edits.
 *
 * Guarded by tools/behavior.js, the popover section: it opens the popover and
 * reads the group WITHOUT setting focus. The first Tab inside repairs the
 * items by itself, so a check that focuses first would see an order where
 * there is none. */
function onToggle(e) {
  const el = e.target;
  if (!(el instanceof Element)) return;
  // newState comes from the popover; on <details> toggle is still a plain
  // event in some browsers — there openness is asked of the element itself.
  if (e.newState === 'open' || (!e.newState && el.open === true)) refresh(el);
}

function onPointerDown(e) {
  const axis = e.target.closest?.('.inst-num-axis');
  if (axis && e.button === 0) onAxisDown(e, axis);
}

/** Place the roving tabindex and bring whatever depends on data into line. */
export function refresh(root = document) {
  for (const group of root.querySelectorAll?.(GROUP_SELECTOR) || []) {
    const spec = specOf(group);
    if (spec) roving(group, spec);
  }
  // The initial state: <output> and "select all" have to agree with the
  // markup BEFORE the first interaction, or the very first frame lies.
  for (const s of root.querySelectorAll?.('.inst-slider') || []) syncSlider(s);
  for (const t of root.querySelectorAll?.('.inst-table') || []) syncSelectAll(t);
}

/* An observer per EVERY root, not one per module.
 *
 * start(root) is declared as an API over an arbitrary root, and two live roots
 * happen: a dialog with a subtree of its own, a preview pane, a test harness.
 * One observer per module would mean the second start() takes the reference
 * away from the first: the callback of the first root calls disconnect() on
 * SOMEBODY ELSE'S observer and subscribes it to its own target, the first root
 * stops updating silently, and stop() cannot reach the orphan — nothing holds
 * it any more.
 *
 * The key is the root itself, so a repeated start() on the same root does not
 * breed a second observer: the old one is removed where the new one is made.
 *
 * Guarded by tools/behavior.js, the roots section. */
const observers = new Map();

/** Connect the behaviour. Called by itself when the module loads.
 *
 * observe switches the MutationObserver off. It is justified where nodes
 * arrive on their own — a task queue fills one row at a time — and redundant
 * where the new nodes are known about anyway: a framework knows about its own
 * commit, and a second observer over the whole document means a full
 * querySelectorAll sweep on every render of it. Then refresh() is called from
 * a hook after painting.
 */
export function start(root = document, { observe = true } = {}) {
  root.addEventListener('keydown', onKeydown);
  root.addEventListener('focusin', onFocusin);
  root.addEventListener('click', onClick);
  root.addEventListener('change', onChange);
  root.addEventListener('input', onInput);
  root.addEventListener('pointerdown', onPointerDown);
  root.addEventListener('toggle', onToggle, true);
  refresh(root);

  // Groups arrive while the work runs — this is an agent interface, and its
  // queue rows appear one at a time. An observer is cheaper than a demand to
  // call refresh() by hand after every render, and it is never forgotten.
  const target = root.body || root;
  if (observe && target && typeof MutationObserver === 'function') {
    observers.get(root)?.disconnect();
    let queued = false;
    let mo;
    const watch = () => mo.observe(target, { childList: true, subtree: true });
    mo = new MutationObserver(() => {
      if (queued) return;
      queued = true;
      queueMicrotask(() => {
        queued = false;
        // The observer is DISCONNECTED for the duration of the sweep, and
        // that is a latch rather than an optimisation. refresh() writes to the
        // markup — the roving tabindex, the "select all" answer, the slider
        // value — and every such write can wake the observer again. The queued
        // flag does not save it: the flag is cleared BEFORE the sweep, so an
        // edit made by the sweep itself honestly starts the next round, and
        // the page never goes idle at all.
        //
        // disconnect() also discards the queue of records, so after
        // resubscribing the earlier edits do not surface.
        mo.disconnect();
        refresh(root);
        watch();
      });
    });
    observers.set(root, mo);
    watch();
  }
}

/** Disconnect. Wanted by tests and by hot reload, not by an application. */
export function stop(root = document) {
  root.removeEventListener('keydown', onKeydown);
  root.removeEventListener('focusin', onFocusin);
  root.removeEventListener('click', onClick);
  root.removeEventListener('change', onChange);
  root.removeEventListener('input', onInput);
  root.removeEventListener('pointerdown', onPointerDown);
  root.removeEventListener('toggle', onToggle, true);
  observers.get(root)?.disconnect();
  observers.delete(root);
}

/* The self-start — and the way to decline it.
 *
 * Exactly one consumer needs to decline: whoever drives the markup themselves.
 * Importing the module in a framework application raises an observer over the
 * whole document and a subscription to six events the application never asked
 * for — and the only way to undo that is to call stop() after it has all
 * already happened.
 *
 * The refusal is declared by an attribute rather than by an import option,
 * because the import is already too late: by the time the application gets
 * control the listeners are in place. The attribute is read as the module
 * runs, before the first subscription.
 *
 *     <html data-instrument="manual">
 *
 *     import { start, refresh } from '@keshon/instrument/js';
 *     start(document, { observe: false });   // and refresh() after painting
 *
 * With no attribute the behaviour is unchanged: <script type="module"> and an
 * incidental import work as before, and the promise "include the module once
 * per page" stays true. */
if (typeof document !== 'undefined' &&
    document.documentElement?.dataset.instrument !== 'manual') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => start(), { once: true });
  } else {
    start();
  }
}
