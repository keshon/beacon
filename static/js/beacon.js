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
            trigger.setAttribute('aria-expanded', panel.classList.contains('is-open') ? 'true' : 'false');
            trigger.addEventListener('click', function () {
                var open = panel.classList.toggle('is-open');
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

    // The modal is a native <dialog>. Top layer, backdrop, Escape, an inert
    // background and focus return all come from the platform; the application
    // is left with showModal() and closing on its own buttons.
    function wireModal(modalEl) {
        if (!modalEl) return null;
        if (modalEl._beaconModal) return modalEl._beaconModal;

        modalEl.addEventListener('click', function (e) {
            if (e.target.closest('[data-beacon-modal-close]')) modalEl.close();
        });

        modalEl._beaconModal = {
            open: function () { if (!modalEl.open) modalEl.showModal(); },
            close: function () { modalEl.close(); },
        };
        return modalEl._beaconModal;
    }

    var confirmModalEl = null;
    var confirmApi = null;
    var confirmResolve = null;

    function ensureConfirmModal() {
        if (confirmModalEl) return;
        confirmModalEl = document.createElement('dialog');
        confirmModalEl.className = 'inst-dialog inst-dialog--sm';
        confirmModalEl.id = 'beaconConfirmModal';
        confirmModalEl.setAttribute('aria-labelledby', 'beaconConfirmTitle');
        confirmModalEl.setAttribute('aria-describedby', 'beaconConfirmMessage');
        confirmModalEl.innerHTML =
            '<div class="inst-dialog-head">' +
                '<h2 class="inst-dialog-title" id="beaconConfirmTitle">Confirm</h2>' +
                '<button type="button" class="inst-dialog-close inst-btn inst-btn--sm inst-btn--icon inst-btn--ghost"' +
                ' data-beacon-confirm-cancel aria-label="Close">' +
                '<svg class="inst-icon" aria-hidden="true"><use href="#i-close"/></svg></button>' +
            '</div>' +
            '<div class="inst-dialog-body"><p id="beaconConfirmMessage"></p></div>' +
            '<div class="inst-dialog-foot inst-dialog-foot--end">' +
                '<button type="button" class="inst-btn" data-beacon-confirm-cancel>Cancel</button>' +
                '<button type="button" class="inst-btn inst-btn--primary" data-beacon-confirm-ok>Confirm</button>' +
            '</div>';
        document.body.appendChild(confirmModalEl);

        function confirmClose() { confirmModalEl.close(); }

        confirmApi = {
            open: function () { if (!confirmModalEl.open) confirmModalEl.showModal(); },
            close: confirmClose,
        };

        function finish(result) {
            if (!confirmResolve) return;
            var resolve = confirmResolve;
            confirmResolve = null;
            confirmApi.close();
            resolve(result);
        }

        confirmModalEl.querySelector('[data-beacon-confirm-ok]').addEventListener('click', function () {
            finish(true);
        });
        confirmModalEl.addEventListener('click', function (e) {
            if (e.target.closest('[data-beacon-confirm-cancel]')) {
                finish(false);
            }
        });
        document.addEventListener('keydown', function (e) {
            if (!confirmModalEl || confirmModalEl.hidden || !confirmResolve) return;
            if (e.key === 'Escape') {
                e.preventDefault();
                finish(false);
            }
        });
    }

    function confirm(options) {
        options = options || {};
        ensureConfirmModal();
        var titleEl = confirmModalEl.querySelector('#beaconConfirmTitle');
        var messageEl = confirmModalEl.querySelector('#beaconConfirmMessage');
        var okBtn = confirmModalEl.querySelector('[data-beacon-confirm-ok]');
        var cancelBtn = confirmModalEl.querySelector('[data-beacon-confirm-cancel].btn');
        if (titleEl) titleEl.textContent = options.title || 'Confirm';
        if (messageEl) messageEl.textContent = options.message || '';
        if (okBtn) {
            okBtn.textContent = options.confirmLabel || 'Confirm';
            okBtn.className = 'btn ' + (options.destructive ? 'btn-danger' : 'btn-primary');
        }
        if (cancelBtn) cancelBtn.textContent = options.cancelLabel || 'Cancel';
        return new Promise(function (resolve) {
            confirmResolve = resolve;
            confirmApi.open();
            if (okBtn && typeof okBtn.focus === 'function') {
                okBtn.focus();
            }
        });
    }

    function closeAllActionMenus(except) {
        document.querySelectorAll('.app-action-menu__panel:not([hidden])').forEach(function (panel) {
            if (except && except.contains(panel)) return;
            panel.hidden = true;
            var menu = panel.closest('.app-action-menu');
            var trigger = menu && menu.querySelector('.app-action-menu__trigger');
            if (trigger) trigger.setAttribute('aria-expanded', 'false');
        });
    }

    function wireActionMenus(root) {
        var scope = root || document;
        scope.querySelectorAll('.app-action-menu').forEach(function (menu) {
            if (menu._beaconActionMenuWired) return;
            menu._beaconActionMenuWired = true;
            var trigger = menu.querySelector('.app-action-menu__trigger');
            var panel = menu.querySelector('.app-action-menu__panel');
            if (!trigger || !panel) return;
            trigger.addEventListener('click', function (e) {
                e.stopPropagation();
                var willOpen = panel.hidden;
                closeAllActionMenus(menu);
                panel.hidden = !willOpen;
                trigger.setAttribute('aria-expanded', willOpen ? 'true' : 'false');
            });
        });
        if (!document._beaconActionMenuDocWired) {
            document._beaconActionMenuDocWired = true;
            document.addEventListener('click', function () {
                closeAllActionMenus(null);
            });
            document.addEventListener('keydown', function (e) {
                if (e.key === 'Escape') closeAllActionMenus(null);
            });
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
            modal: { wire: wireModal, confirm: confirm },
            wireActionMenus: wireActionMenus,
            closeAllActionMenus: closeAllActionMenus,
        };
    } else {
        window.Beacon.csrfToken = csrfToken;
        window.Beacon.apiFetch = apiFetch;
        window.Beacon.initCollapse = initCollapse;
        window.Beacon.applyAppearancePrefs = applyAppearancePrefs;
        window.Beacon.modal = window.Beacon.modal || { wire: wireModal, confirm: confirm };
        if (window.Beacon.modal && !window.Beacon.modal.confirm) {
            window.Beacon.modal.confirm = confirm;
        }
        window.Beacon.wireActionMenus = wireActionMenus;
        window.Beacon.closeAllActionMenus = closeAllActionMenus;
    }

    document.addEventListener('DOMContentLoaded', function () {
        initCollapse(document);
        wireActionMenus(document);
    });
})();
