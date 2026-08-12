/* Тема и плотность. Атрибут ставит приложение, рисует кит.

   Шесть тем плюс «по системе»: последнее — отсутствие атрибута, а не
   отдельная тема. Значение хранится, чтобы выбор пережил перезагрузку. */
(function () {
    var root = document.documentElement;
    var sel = document.getElementById('beaconTheme');
    if (!sel) return;

    var saved = null;
    try { saved = localStorage.getItem('beaconTheme'); } catch (e) { }
    sel.value = saved || 'system';

    sel.addEventListener('change', function () {
        var v = sel.value;
        if (v === 'system') {
            root.removeAttribute('data-theme');
        } else {
            root.setAttribute('data-theme', v);
        }
        try { localStorage.setItem('beaconTheme', v); } catch (e) { }
    });

    // Density is a document attribute. The kit runs the mark and the arrow
    // keys through role="radiogroup"; the application only records the choice.
    var dens = document.getElementById('beaconDensity');
    if (dens) {
        var savedD = null;
        try { savedD = localStorage.getItem('beaconDensity'); } catch (e) { }
        dens.querySelectorAll('[role="radio"]').forEach(function (b) {
            var on = (b.getAttribute('data-value') || '') === (savedD || '');
            b.setAttribute('aria-checked', String(on));
            b.tabIndex = on ? 0 : -1;
        });
        dens.addEventListener('inst:select', function (e) {
            var v = e.target.getAttribute('data-value') || '';
            if (v) { root.setAttribute('data-density', v); } else { root.removeAttribute('data-density'); }
            try { localStorage.setItem('beaconDensity', v); } catch (e2) { }
        });
    }

    // Row colouring lives with theme and density: same scope (this browser),
    // same storage, same popover. The pre-paint script in head_theme.html sets
    // the attributes before the first frame; this only handles changes.
    var colorize = [
        { el: document.getElementById('ui_colorize_up'), key: 'beaconColorizeUp', attr: 'data-colorize-up' },
        { el: document.getElementById('ui_colorize_down'), key: 'beaconColorizeDown', attr: 'data-colorize-down' },
    ];
    colorize.forEach(function (c) {
        if (!c.el) return;
        try {
            c.el.checked = localStorage.getItem(c.key) === '1';
        } catch (e) {}
        c.el.addEventListener('change', function () {
            var on = c.el.checked ? '1' : '0';
            document.documentElement.setAttribute(c.attr, on);
            try {
                localStorage.setItem(c.key, on);
            } catch (e) {}
        });
    });
})();
