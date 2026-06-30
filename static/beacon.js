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

    function applyAppearancePrefs() {
        var root = document.documentElement;
        try {
            var up = localStorage.getItem('beaconColorizeUp') === '1';
            var down = localStorage.getItem('beaconColorizeDown') === '1';
            root.setAttribute('data-colorize-up', up ? '1' : '0');
            root.setAttribute('data-colorize-down', down ? '1' : '0');
        } catch (e) {
            root.setAttribute('data-colorize-up', '0');
            root.setAttribute('data-colorize-down', '0');
        }
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
            applyAppearancePrefs: applyAppearancePrefs,
        };
    } else {
        window.Beacon.csrfToken = csrfToken;
        window.Beacon.apiFetch = apiFetch;
        window.Beacon.initCollapse = initCollapse;
        window.Beacon.applyAppearancePrefs = applyAppearancePrefs;
    }

    document.addEventListener('DOMContentLoaded', function () {
        initCollapse(document);
    });
})();
