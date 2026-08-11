// Dashboard live updates: uptime strips, SSE, client-side layout toggle.
(function () {
    'use strict';

    var TICK_WIDTH = 3;
    var TICK_GAP = 2;
    var COMPACT_MAX = 45;
    var HISTORY_MAX = 500;
    var storageKey = 'beaconDashboardView';

    var historyCache = Object.create(null);
    var currentView = null;
    var activeES = null;
    var resizeObserver = null;

    function readBootstrap() {
        var el = document.getElementById('dashboard-uptime-bootstrap');
        if (!el) return;
        try {
            var parsed = JSON.parse(el.textContent || '{}');
            if (!parsed || typeof parsed !== 'object') return;
            Object.keys(parsed).forEach(function (id) {
                historyCache[id] = (parsed[id] || []).slice();
            });
        } catch (e) {}
    }

    // Штрих истории: класс кита, исход — тоном из закрытого словаря.
    function barEl(ok) {
        var d = document.createElement('span');
        d.className = 'inst-history-tick';
        d.setAttribute('data-tone', ok ? 'ok' : 'error');
        return d;
    }

    function segmentCapacity(strip) {
        var w = strip.clientWidth;
        if (w <= 0) return COMPACT_MAX;
        return Math.max(1, Math.floor((w + TICK_GAP) / (TICK_WIDTH + TICK_GAP)));
    }

    function isFillStrip(strip) {
        return strip.classList.contains('uptime-strip--fill');
    }

    function maxForStrip(strip) {
        return isFillStrip(strip) ? segmentCapacity(strip) : COMPACT_MAX;
    }

    function slicePoints(points, limit) {
        if (!points || !points.length) return [];
        if (points.length <= limit) return points;
        return points.slice(points.length - limit);
    }

    function renderStrip(strip, points) {
        var slice = slicePoints(points, maxForStrip(strip));
        strip.textContent = '';
        for (var i = 0; i < slice.length; i++) {
            strip.appendChild(barEl(!!slice[i].success));
        }
    }

    function appendToCache(monitorId, success) {
        if (!historyCache[monitorId]) historyCache[monitorId] = [];
        var buf = historyCache[monitorId];
        buf.push({ success: !!success });
        if (buf.length > HISTORY_MAX) {
            historyCache[monitorId] = buf.slice(buf.length - HISTORY_MAX);
        }
    }

    function refreshFillStrips() {
        document.querySelectorAll('.uptime-strip--fill[data-monitor-id]').forEach(function (strip) {
            var id = strip.getAttribute('data-monitor-id');
            if (id) renderAllStripsForMonitor(id);
        });
    }

    function renderAllStripsForMonitor(id) {
        var pts = historyCache[id];
        if (!pts || !pts.length) return;
        document.querySelectorAll('.uptime-strip[data-monitor-id="' + id + '"]').forEach(function (strip) {
            renderStrip(strip, pts);
        });
    }

    function wireFillStrips() {
        if (typeof ResizeObserver === 'undefined') return;
        if (!resizeObserver) {
            resizeObserver = new ResizeObserver(function (entries) {
                entries.forEach(function (entry) {
                    var strip = entry.target;
                    var id = strip.getAttribute('data-monitor-id');
                    if (id) renderAllStripsForMonitor(id);
                });
            });
        }
        document.querySelectorAll('.uptime-strip--fill[data-monitor-id]').forEach(function (strip) {
            resizeObserver.observe(strip);
        });
    }

    function bootstrapUptime() {
        Object.keys(historyCache).forEach(function (id) {
            renderAllStripsForMonitor(id);
        });
        wireFillStrips();
    }

    // Тон из словаря кита; слово рядом с точкой обязательно — цвет не имеет
    // права быть единственным носителем.
    function statusBadgeHtml(status) {
        var labels = { up: 'Up', down: 'Down', unknown: 'Unknown' };
        var tones = { up: 'ok', down: 'error', unknown: 'neutral' };
        var s = status && tones[status] ? status : 'unknown';
        return (
            '<span class="inst-badge" data-tone="' + tones[s] + '">' +
            '<span class="inst-dot"></span>' + labels[s] + '</span>'
        );
    }

    function updateRowFromSSE(data) {
        if (!data.monitor_id) return;
        appendToCache(data.monitor_id, !!data.success);
        document.querySelectorAll('[data-dashboard-monitor="' + data.monitor_id + '"]').forEach(function (root) {
            var lat = root.querySelector('.dashboard-latency');
            var lc = root.querySelector('.dashboard-lastcheck');
            var stCell = root.querySelector('.dashboard-status');
            if (lat && data.latency_ms != null) lat.textContent = data.latency_ms;
            if (lc && data.last_check != null) lc.textContent = data.last_check;
            if (stCell && data.status) stCell.innerHTML = statusBadgeHtml(data.status);
        });
        renderAllStripsForMonitor(data.monitor_id);
    }

    var retryMs = 1000;

    function connectSSE() {
        if (activeES) {
            activeES.close();
            activeES = null;
        }
        var es = new EventSource('/api/stream/checks');
        activeES = es;
        es.onmessage = function (ev) {
            retryMs = 1000;
            try {
                var data = JSON.parse(ev.data);
                updateRowFromSSE(data);
            } catch (e) {}
        };
        es.onerror = function () {
            es.close();
            if (activeES === es) activeES = null;
            setTimeout(connectSSE, retryMs);
            retryMs = Math.min(retryMs * 2, 30000);
        };
    }

    function normalizeView(mode) {
        if (mode === 'grid') mode = 'list';
        if (mode !== 'cards' && mode !== 'list' && mode !== 'table') mode = 'cards';
        return mode;
    }

    function resolveInitialView() {
        try {
            var params = new URLSearchParams(window.location.search);
            var fromUrl = params.get('view');
            if (fromUrl) return normalizeView(fromUrl);
        } catch (e) {}
        try {
            var saved = localStorage.getItem(storageKey);
            if (saved) return normalizeView(saved);
        } catch (e) {}
        return 'cards';
    }

    function setView(mode) {
        mode = normalizeView(mode);
        if (mode === currentView) return;
        currentView = mode;
        document.documentElement.setAttribute('data-dashboard-view', mode);
        try {
            localStorage.setItem(storageKey, mode);
        } catch (e) {}
        try {
            var url = new URL(window.location.href);
            url.searchParams.set('view', mode);
            history.replaceState(null, '', url.toString());
        } catch (e) {}
        if (mode === 'cards') {
            requestAnimationFrame(refreshFillStrips);
        }
    }

    function initViewToggle() {
        if (!document.getElementById('dashboardViews')) return;
        currentView = resolveInitialView();
        document.documentElement.setAttribute('data-dashboard-view', currentView);
        document.querySelectorAll('[data-dashboard-view]').forEach(function (b) {
            if (b.getAttribute('role') !== 'radio') return;
            var on = b.getAttribute('data-dashboard-view') === currentView;
            b.setAttribute('aria-checked', String(on));
            b.tabIndex = on ? 0 : -1;
        });
        try {
            var url = new URL(window.location.href);
            if (url.searchParams.get('view') !== currentView) {
                url.searchParams.set('view', currentView);
                history.replaceState(null, '', url.toString());
            }
        } catch (e) {}
        // Отметку и клавиатуру ведёт кит по role="radiogroup"; приложение
        // слушает результат и записывает выбор.
        var group = document.querySelector('[data-dashboard-views]');
        if (group) {
            group.addEventListener('inst:select', function (e) {
                var v = e.target.getAttribute('data-dashboard-view');
                if (v) setView(v);
            });
        }
    }

    function init() {
        readBootstrap();
        initViewToggle();
        requestAnimationFrame(function () {
            bootstrapUptime();
            if (currentView === 'cards') {
                refreshFillStrips();
            }
        });
        connectSSE();
        window.addEventListener('pagehide', function () {
            if (activeES) {
                activeES.close();
                activeES = null;
            }
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
