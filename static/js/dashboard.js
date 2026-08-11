// Dashboard live updates: SSE only.
//
// The history strip is NOT drawn here any more. The server renders it from
// hourly buckets, one tick per hour, the same hours on every row. Drawing it
// from raw samples in the browser made the strip lie about time: for a monitor
// checked every thirty seconds, twenty-four ticks were twenty-four minutes
// while the label said a day, and two rows covered different spans.
//
// What is left for the client is one honest edit: a failed check darkens the
// LAST tick, because that tick is the current hour and the hour now contains a
// failure. Everything else waits for the next render.
(function () {
    'use strict';

    var activeES = null;

    // Tone from the kit's vocabulary; the word next to the dot is required —
    // colour has no right to be the only carrier of meaning.
    function statusBadgeHtml(status) {
        var labels = { up: 'Up', down: 'Down', unknown: 'Unknown' };
        var tones = { up: 'ok', down: 'error', unknown: 'neutral' };
        var s = status && tones[status] ? status : 'unknown';
        return (
            '<span class="inst-badge" data-tone="' + tones[s] + '">' +
            '<span class="inst-dot"></span>' + labels[s] + '</span>'
        );
    }

    // A failed check makes the current hour a failed hour. Marking the last
    // tick is the whole of the live update: the rest of the strip is history
    // and history does not change.
    function markCurrentHour(monitorId) {
        document.querySelectorAll('.inst-history[data-monitor-id="' + monitorId + '"]').forEach(function (strip) {
            var last = strip.lastElementChild;
            if (!last) return;
            last.classList.remove('beacon-tick--gap');
            last.setAttribute('data-tone', 'error');
        });
    }

    function updateRowFromSSE(data) {
        if (!data.monitor_id) return;
        if (!data.success) markCurrentHour(data.monitor_id);
        document.querySelectorAll('[data-dashboard-monitor="' + data.monitor_id + '"]').forEach(function (root) {
            var lat = root.querySelector('.dashboard-latency');
            var lc = root.querySelector('.dashboard-lastcheck');
            var stCell = root.querySelector('.dashboard-status');
            if (lat && data.latency_ms != null) lat.textContent = data.latency_ms;
            if (lc && data.last_check != null) lc.textContent = data.last_check;
            if (stCell && data.status) stCell.innerHTML = statusBadgeHtml(data.status);
        });
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

    function init() {
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
