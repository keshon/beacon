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
        };
        return;
    }

    window.Beacon.csrfToken = csrfToken;
    window.Beacon.apiFetch = apiFetch;
})();
