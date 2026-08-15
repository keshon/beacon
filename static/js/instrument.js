/* instrument — поведение. Опционально.

   Кит рисует состояние и объявляет роли. Роль — это обещание: `role="listbox"`
   говорит вспомогательной технологии, что стрелки работают. Пока обещание не
   выполнено, компонент не «не доделан», он ОБМАНЫВАЕТ — и это хуже, чем не
   объявлять роль вовсе.

   Этот файл выполняет обещание и больше ничего не делает.

   ── Что он НЕ делает ──────────────────────────────────────────────────────

   Не рисует. Ни одного присвоения style, ни одного добавления класса
   оформления. Он ставит атрибуты, которые УЖЕ есть в разметочном контракте —
   tabindex, aria-selected, aria-checked, aria-expanded, — а как они выглядят,
   решает CSS. Как только скрипт начнёт красить, кит потеряет главное своё
   свойство: приложение перестанет иметь возможность переопределить вид, не
   трогая поведение.

   Не обязателен. Без него всё, что не требует клавиатуры, работает как
   работало: раскрытие на <details>, поповер на Popover API, модалка на
   <dialog>, валидация на :user-invalid.

   ── Подключение ───────────────────────────────────────────────────────────

     <script type="module" src="src/kit.js"></script>

   Модуль, а не классический скрипт: у него есть именованные экспорты, и на
   них потом сядут обёртки Svelte и React. Сборки по-прежнему нет.

   ── Почему делегирование, а не инициализация ──────────────────────────────

   Слушатели висят на документе, а элементы ищутся в момент нажатия. Для
   агентного интерфейса это не оптимизация, а единственный рабочий вариант:
   строки очереди, шаги и узлы дерева ПРИБЫВАЮТ во время работы, и любой
   init(el) при старте промахнулся бы мимо всего, что появилось позже.

   Единственное, что нельзя сделать лениво, — начальный tabindex: группа без
   него не достижима по Tab вообще. Поэтому за появлением групп следит
   MutationObserver, а не человек.
   ========================================================================= */

/* Роли, у которых есть контракт клавиатуры, и всё, что о них надо знать.

   Список закрыт. Он совпадает с таблицей разметочного контракта в
   конституции — если там появится строка, она обязана появиться и здесь,
   иначе кит снова начнёт обещать невыполнимое. */
const GROUPS = {
  listbox: {
    item: '[role="option"]',
    axis: 'vertical',
    // Выделение следует за фокусом: одиночный listbox по APG ведёт себя так,
    // и очередь задач — ровно этот случай.
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
    // У пункта меню выделения нет: он действие, а не выбор.
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

/* Ось берётся из aria-orientation, если он есть: горизонтальное дерево и
   вертикальные вкладки существуют, и разметка вправе так сказать. */
function axisOf(group, spec) {
  return group.getAttribute('aria-orientation') || spec.axis;
}

/* Элементы группы.
 *
 * closest() обязателен: вложенные группы существуют (подменю внутри меню,
 * группа внутри дерева), и без него стрелка в родителе прыгала бы по чужим
 * пунктам. Отключённые и невидимые выбрасываются — фокус на них не ставится
 * ни платформой, ни нами. */
function itemsOf(group, spec, withHidden) {
  return [...group.querySelectorAll(spec.item)].filter(
    (el) =>
      el.closest(GROUP_SELECTOR) === group &&
      el.getAttribute('aria-disabled') !== 'true' &&
      !el.disabled &&
      (withHidden || el.offsetParent !== null),
  );
}

/* Бегущий tabindex: ровно один элемент группы достижим по Tab.
 *
 * Это и есть весь смысл. Список из двухсот строк, у каждой tabindex="0",
 * заставляет двести раз нажать Tab, чтобы уйти дальше; список, где ни у одной
 * его нет, недостижим вовсе — так кит и жил до сих пор. */
function roving(group, spec, current) {
  const items = itemsOf(group, spec);
  if (!items.length) return;
  const active = current && items.includes(current)
    ? current
    : items.find((el) => el.getAttribute(spec.follows || 'aria-selected') === 'true') || items[0];
  for (const el of items) el.tabIndex = el === active ? 0 : -1;
  return active;
}

/* Перевести фокус и, если роль этого требует, состояние выбора.
 *
 * Выбор снимается со ВСЕЙ группы, а не переключается на цели: два выделенных
 * пункта в одиночном списке — состояние, из которого разметка уже не выйдет. */
function move(group, spec, to) {
  for (const el of itemsOf(group, spec)) el.tabIndex = el === to ? 0 : -1;
  select(group, spec, to);
  to.focus();
}

/* Перенести выбор на пункт и сказать об этом.
 *
 * Отдельно от move, потому что мышь и клавиатура приходят сюда по-разному:
 * стрелка двигает фокус и выбор вместе, щелчок — только выбор.
 *
 * Отменённое событие оставляет разметку нетронутой, но фокус всё равно уходит
 * куда шёл: приложение отказалось вести состояние само, а не запретило
 * человеку перемещаться. */
function select(group, spec, to) {
  if (!spec.follows || to.getAttribute(spec.follows) === 'true') return;
  if (!emit(to, 'select', { value: to.dataset.value ?? to.textContent.trim() })) return;
  const items = itemsOf(group, spec);
  for (const el of items) {
    el.setAttribute(spec.follows, String(el === to));
  }
  panels(group, spec, items, to);
}

/* Панель выбранной вкладки.
 *
 * Показывается та, на которую указывает aria-controls, остальные прячутся.
 * Гадать не о чем: связь названа в разметке, и без этой строки вкладка
 * переключалась на вид, а содержимое оставалось прежним.
 *
 * Только для вкладок: у строки списка и пункта меню панели нет. */
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
  // По кругу: у списка нет «конца», после которого некуда деться. APG
  // допускает оба поведения, но в очереди задач заворот избавляет от
  // тупика на последней строке.
  return items[(i + delta + items.length) % items.length];
}

/* Разворот и сворачивание узла дерева.
 *
 * Стрелка «вперёд» на свёрнутом узле раскрывает его, на раскрытом — уходит к
 * первому потомку. «Назад» симметрично. Это контракт роли treeitem, и без
 * него aria-expanded в разметке — украшение. */
/* Потомки узла в плоском дереве: всё, что идёт следом с бо́льшим уровнем,
 * до первой строки того же или меньшего. Вложенность здесь живёт в
 * aria-level, а не в разметке: дерево на десять тысяч узлов иначе не
 * построить. */
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

/* Свернуть или раскрыть узел.
 *
 * Кит не рисует — он ставит hidden, то есть состояние, которое и так есть в
 * платформе. Событие отменяемо: приложение, которое рисует дерево из данных,
 * снимет строки само, и вторая рука ему не нужна. */
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
}

/* Ближайший предок строки по уровню — нужен, чтобы при раскрытии не показать
 * потомков узла, свёрнутого глубже. */
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
  // Свёрнутый узел уводит фокус к родителю: уровень берётся из aria-level,
  // потому что вложенность в разметке может быть плоской.
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
  const spec = GROUPS[group.getAttribute('role')];
  if (!spec) return;

  const axis = axisOf(group, spec);
  const items = itemsOf(group, spec);
  if (!items.length) return;

  // Дерево забирает поперечные стрелки под раскрытие — но только их.
  if (group.getAttribute('role') === 'tree') {
    const forward = e.key === 'ArrowRight';
    if (forward || e.key === 'ArrowLeft') {
      if (treeArrow(group, spec, item, forward)) e.preventDefault();
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
      // Радио и вкладка уже выбраны стрелкой — нажимать нечего. Остальным
      // клик отдаётся платформе: обработчик приложения висит на нём, и
      // выдумывать второе событие незачем.
      if (spec.follows && group.getAttribute('role') !== 'radiogroup') {
        move(group, spec, item);
      }
      if (e.key === ' ') e.preventDefault();
      item.click();
      return;
    default:
  }
}

/* Фокус, пришедший мышью, тоже обязан перевесить tabindex: иначе следующий
 * Tab уйдёт не оттуда, куда человек только что ткнул. */
function onFocusin(e) {
  const group = e.target.closest?.(GROUP_SELECTOR);
  if (!group) return;
  const spec = GROUPS[group.getAttribute('role')];
  if (!spec) return;
  const item = e.target.closest(spec.item);
  if (item) roving(group, spec, item);
}

/* ── Тосты ──────────────────────────────────────────────────────────────────

   Здесь скрипт делает то, чего CSS не может в принципе: считает время и
   держит очередь. Вид по-прежнему целиком в CSS — ни одного присвоения style.

   Область открывается popover'ом ОДИН раз и не закрывается. Причина не в
   удобстве: закрытый popover это display: none, а живой регион в display:
   none не озвучивается — скринридер не услышал бы ни одного тоста.
   ───────────────────────────────────────────────────────────────────────── */

const TOAST_LIMIT = 4;
const TOAST_DURATION = 5000;

function toastRegion(doc) {
  // Своя область — та, что живёт в верхнем слое. Без [popover] под руку
  // попадала любая разметка с этим классом: страница, ПОКАЗЫВАЮЩАЯ область
  // примером, получала в неё настоящие уведомления.
  let region = doc.querySelector('.inst-toasts[popover]');
  if (region) {
    ensureOpen(region);
    return region;
  }
  region = doc.createElement('div');
  region.className = 'inst-toasts';
  region.setAttribute('popover', 'manual');
  // Живой регион вежливый: тост сообщает результат, а не прерывает работу.
  // Ошибка перебивает — на ней самой стоит role="alert".
  region.setAttribute('aria-live', 'polite');
  region.setAttribute('aria-label', 'Уведомления');
  doc.body.append(region);
  ensureOpen(region);
  return region;
}

/* Открыть область один раз.
 *
 * Перевести её НАВЕРХ верхнего слоя нельзя, и это проверено: hidePopover() с
 * последующим showPopover() — и в одной задаче, и через задачу — не поднимает
 * область над модалкой, открытой showModal(). Модальный диалог остаётся выше.
 *
 * Значит и притворяться незачем: тост живёт выше любого z-index и любого
 * overflow: hidden, но НЕ выше модалки. Ограничение записано в справочнике
 * вместе с тем, что делать вместо.                                        */
function ensureOpen(region) {
  try {
    if (!region.matches(':popover-open')) region.showPopover();
  } catch {
    // Область без атрибута popover — приложение решило иначе, и это его
    // право: тосты будут работать, просто не в верхнем слое.
  }
}

/* Убрать с доигранным переходом.
 *
 * transitionend со страховкой по таймеру: под prefers-reduced-motion переход
 * схлопывается до 0.01ms и событие может не прийти вовсе — без страховки
 * тост остался бы на экране навсегда. */
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
 * Показать тост.
 *
 * Разметку строит кит, но она вся описана в справочнике: если стандартной не
 * хватает, соберите свой узел и положите в `.inst-toasts` сами — очередь и
 * таймер подхватят его так же.
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
  // Ошибка обязана перебить: она сообщает, что действие НЕ выполнено.
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

  // Потолок очереди. Двадцать тостов подряд — не сообщение, а стена: самые
  // старые уходят, чтобы новое было видно.
  const live = [...region.children].filter((x) => x.dataset.state !== 'leaving');
  for (const old of live.slice(0, Math.max(0, live.length - TOAST_LIMIT))) {
    removeToast(old);
  }

  if (duration > 0) {
    let timer = setTimeout(() => removeToast(el), duration);
    /* Таймер замирает под курсором и на фокусе. Тост, исчезнувший ровно
       тогда, когда его начали читать, — это потерянное сообщение, и WCAG
       2.2.1 требует ровно этого: у времени должна быть пауза. */
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
   Нарисованные обещания

   Тот же закон, что у ролей, применённый к оформлению: если кит НАРИСОВАЛ
   affordance, он обязан её выполнить. Кнопка копирования, крестик снятия,
   курсор ew-resize на подписи оси — это обещания, данные пользователю
   картинкой. Невыполненное обещание не «не доделано», оно обманывает.

   Каждое поведение здесь:
   · ничего не рисует — ставит атрибуты из разметочного контракта;
   · шлёт отменяемое событие, чтобы приложение могло вмешаться или взять
     работу на себя;
   · работает через делегирование, поэтому узлы могут прибывать во время
     работы.
   ========================================================================== */

/** Отменяемое событие кита. false — приложение перехватило работу. */
function emit(el, name, detail) {
  return el.dispatchEvent(
    new CustomEvent('inst:' + name, { bubbles: true, cancelable: true, detail }),
  );
}

/* ── Живая область для коротких сообщений ────────────────────────────────
   Отдельная от тостов: «скопировано» — это подтверждение действия, а не
   уведомление, и вставать в очередь ему незачем. */
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
  // Пустая строка и сразу текст: повтор того же сообщения иначе не
  // объявляется — регион считает, что ничего не изменилось.
  //
  // Таймер, а не requestAnimationFrame: кадров в скрытой вкладке нет, и
  // объявление не состоялось бы вовсе.
  setTimeout(() => { live.textContent = text; }, 0);
}

/* ── Копирование ─────────────────────────────────────────────────────────

   Источник: data-copy у кнопки, иначе текст ближайшего inst-copyable или
   inst-code без текста самой кнопки.

   textContent, а не innerText: второй зависит от раскладки и у скрытого
   содержимого возвращает пустую строку — свёрнутый блок кода копировался бы
   в ничто. */
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
    // Незащищённый origin отклоняет запись молча. Молчать в ответ — значит
    // соврать: кнопка выглядела бы сработавшей.
    ok = false;
  }

  btn.dataset.copied = ok ? 'true' : 'false';
  // Удача и неудача берут РАЗНЫЕ подписи: одна на двоих объявляла бы
  // «скопировано» там, где запись отклонена, — то есть ровно то враньё,
  // ради устранения которого неудача вообще отслеживается.
  const said = ok
    ? btn.dataset.copiedLabel || 'Скопировано'
    : btn.dataset.failedLabel || 'Не удалось скопировать';
  announce(btn.ownerDocument, said);
  clearTimeout(+btn.dataset.instTimer || 0);
  btn.dataset.instTimer = setTimeout(() => {
    delete btn.dataset.copied;
  }, 1400);
}

/* ── Снятие тега ─────────────────────────────────────────────────────────

   Фокус обязан уйти на соседа: удалённый элемент уводит его в никуда, и
   человек с клавиатуры оказывается в начале документа. */
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
  announce(btn.ownerDocument, `Метка ${label} снята`);
}

/* ── Перетаскивание подписи оси ──────────────────────────────────────────

   Курсор ew-resize на inst-num-axis означает «отсюда тянут». Шаг берётся из
   самого поля, ускорение — из модификаторов: Shift ×10, Alt ×0.1. Значение
   уходит событиями input и change, как у нативного ввода, поэтому фреймворки
   видят его без единой строки клея. */
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
    // Хвост float съедается шагом: 0.1 + 0.2 иначе даёт 0.30000000000000004.
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

/* ── Значение слайдера: в <output> и в заполнение дорожки ────────────────
   Связь с <output> уже объявлена атрибутом for. Синхронизировать её вручную —
   писать одну и ту же строку в каждом приложении.

   --fill — то же самое для ЗАЛИВКИ: дорожка красится до бегунка, и доля
   считается здесь, потому что CSS не умеет прочитать value у input. Разметка
   объявляет начальное значение сама (канал данных), а скрипт держит его в
   согласии при перетаскивании: без скрипта заливка остаётся на том значении,
   с которым страница пришла, и это правда, а не ноль. */
function syncSlider(input) {
  const min = Number(input.min === '' ? 0 : input.min);
  const max = Number(input.max === '' ? 100 : input.max);
  // Вырожденный диапазон: max === min. Деление дало бы NaN, и заливка
  // пропала бы вовсе вместо того, чтобы честно показать край.
  const share = max > min ? (Number(input.value) - min) / (max - min) : 0;
  input.style.setProperty('--fill', `${Math.min(1, Math.max(0, share)) * 100}%`);

  if (!input.id) return;
  for (const out of input.ownerDocument.querySelectorAll(`output[for~="${CSS.escape(input.id)}"]`)) {
    out.textContent = input.value;
  }
}

/* ── «Выбрать всё» в таблице ─────────────────────────────────────────────
   Чекбокс в шапке колонки выбора. Промежуточное состояние — indeterminate:
   «часть строк выбрана» невыразимо ни через checked, ни через его отсутствие. */
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

/* ── Вставка подстановки в поле ──────────────────────────────────────────

   Кнопка с надписью «{{name}}», которая ничего не кладёт в поле, — то же
   невыполненное обещание, что роль без клавиатуры.

   Вставляется НА МЕСТО КАРЕТКИ, а не в конец: человек ставит курсор туда,
   где нужна подстановка, и ждёт её там. Выделенное заменяется. После
   вставки фокус возвращается в поле — иначе следующую букву некуда набрать. */
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
  // Нативное событие, а не своё: рамки видят изменение без единой строки клея.
  field.dispatchEvent(new Event('input', { bubbles: true }));
}

/* ── Раскрыть все / свернуть все ─────────────────────────────────────────

   Список из шести <details> — это шесть нажатий, чтобы прочитать его целиком,
   и ещё шесть, чтобы вернуть как было. Кнопка есть у каждого файлового
   менеджера и почтового клиента ровно поэтому.

   Состояние определяется по СОДЕРЖИМОМУ, а не запоминается: если хоть один
   узел закрыт, нажатие раскрывает всё. Кнопка, которая помнит своё «сейчас
   сверну», рано или поздно расходится с тем, что человек открыл руками.

   Область — ближайшая панель, либо элемент, названный в `data-details-all`
   селектором. */
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

  // Щелчок по вкладке или варианту. Без этого кит рисовал выбранную вкладку,
  // стрелки её переключали, а мышь — нет: focusin переставляет только
  // бегущий tabindex.
  const group = e.target.closest?.(GROUP_SELECTOR);
  if (!group) return;
  const spec = GROUPS[group.getAttribute('role')];
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

function onPointerDown(e) {
  const axis = e.target.closest?.('.inst-num-axis');
  if (axis && e.button === 0) onAxisDown(e, axis);
}

/** Расставить бегущий tabindex и привести к данным то, что от них зависит. */
export function refresh(root = document) {
  for (const group of root.querySelectorAll?.(GROUP_SELECTOR) || []) {
    const spec = GROUPS[group.getAttribute('role')];
    if (spec) roving(group, spec);
  }
  // Начальное состояние: <output> и «выбрать всё» обязаны совпадать с
  // разметкой ДО первого взаимодействия, иначе первый же кадр врёт.
  for (const s of root.querySelectorAll?.('.inst-slider') || []) syncSlider(s);
  for (const t of root.querySelectorAll?.('.inst-table') || []) syncSelectAll(t);
}

let observer = null;

/** Подключить поведение. Вызывается сам при загрузке модуля. */
export function start(root = document) {
  root.addEventListener('keydown', onKeydown);
  root.addEventListener('focusin', onFocusin);
  root.addEventListener('click', onClick);
  root.addEventListener('change', onChange);
  root.addEventListener('input', onInput);
  root.addEventListener('pointerdown', onPointerDown);
  refresh(root);

  // Группы прибывают во время работы — это агентный интерфейс, в нём строки
  // очереди появляются по одной. Наблюдатель дешевле, чем требование звать
  // refresh() руками после каждого рендера, и не забывается.
  const target = root.body || root;
  if (target && typeof MutationObserver === 'function') {
    let queued = false;
    observer = new MutationObserver(() => {
      if (queued) return;
      queued = true;
      queueMicrotask(() => {
        queued = false;
        refresh(root);
      });
    });
    observer.observe(target, { childList: true, subtree: true });
  }
}

/** Отключить. Нужно тестам и горячей перезагрузке, а не приложению. */
export function stop(root = document) {
  root.removeEventListener('keydown', onKeydown);
  root.removeEventListener('focusin', onFocusin);
  root.removeEventListener('click', onClick);
  root.removeEventListener('change', onChange);
  root.removeEventListener('input', onInput);
  root.removeEventListener('pointerdown', onPointerDown);
  observer?.disconnect();
  observer = null;
}

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => start(), { once: true });
  } else {
    start();
  }
}
