// Root namespace for Beacon frontend modules.
(function () {
    'use strict';
    if (!window.Beacon) {
        window.Beacon = {
            notify: {
                globalDefaults: { alert_mode: 'repeat', templates: {} },
            },
            policy: {},
            policyModal: {},
            settings: null,
        };
    }
})();
