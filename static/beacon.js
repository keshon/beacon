// Root namespace and shared HTTP helpers for Beacon frontend modules.
(function () {
    'use strict';

    function csrfToken() {
        var match = document.cookie.match(/(?:^|;\s*)beacon_csrf=([^;]+)/);
        return match ? decodeURIComponent(match[1]) : '';
    }

    function apiFetch(url, options) {
        options = options || {};
        var method = (options.method || 'GET').toUpperCase();
        var headers = Object.assign({}, options.headers || {});
        if (method !== 'GET' && method !== 'HEAD') {
            var token = csrfToken();
            if (token) {
                headers['X-CSRF-Token'] = token;
            }
        }
        options.headers = headers;
        return fetch(url, options);
    }

    function initCollapse(root) {
        var scope = root || document;
        scope.querySelectorAll('[data-beacon-collapse]').forEach(function (trigger) {
            if (trigger._beaconCollapseWired) return;
            trigger._beaconCollapseWired = true;
            var sel = trigger.getAttribute('data-beacon-collapse-target');
            if (!sel) return;
            var panel = scope.querySelector(sel);
            if (!panel) return;
            trigger.addEventListener('click', function () {
                var open = panel.classList.toggle('show');
                trigger.setAttribute('aria-expanded', open ? 'true' : 'false');
            });
        });
    }

    if (!window.Beacon) {
        window.Beacon = {
            notify: {
                globalDefaults: { alert_mode: 'repeat', templates: {} },
            },
            policy: {},
            policyModal: {},
            settings: null,
            csrfToken: csrfToken,
            apiFetch: apiFetch,
            initCollapse: initCollapse,
        };
    } else {
        window.Beacon.csrfToken = csrfToken;
        window.Beacon.apiFetch = apiFetch;
        window.Beacon.initCollapse = initCollapse;
    }

    document.addEventListener('DOMContentLoaded', function () {
        initCollapse(document);
    });
})();
