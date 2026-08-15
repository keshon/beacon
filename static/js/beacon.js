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
        confirmModalEl.className = 'inst-dialog';
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
            '<div class="inst-dialog-foot">' +
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
        // Отмена — та, что в подвале, а не крестик в шапке: у обеих один
        // маркер действия, и подпись меняется только у первой.
        var cancelBtn = confirmModalEl.querySelector('.inst-dialog-foot [data-beacon-confirm-cancel]');
        if (titleEl) titleEl.textContent = options.title || 'Confirm';
        if (messageEl) messageEl.textContent = options.message || '';
        if (okBtn) {
            okBtn.textContent = options.confirmLabel || 'Confirm';
            okBtn.className =
                'inst-btn ' + (options.destructive ? 'inst-btn--danger' : 'inst-btn--primary');
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

    /* Меню действий закрывается ДО того, как действие выполнится: действие
       открывает модалку, и меню осталось бы висеть поверх неё.

       Открытие, закрытие мимо, Escape и возврат фокуса — Popover API, скрипта
       они не требуют. Приложению остаётся один случай, который платформа не
       закрывает сама: щелчок ВНУТРИ поповера. */
    function closeActionMenu(el) {
        var popover = el && el.closest('[popover]');
        if (popover && popover.matches(':popover-open')) popover.hidePopover();
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
            closeActionMenu: closeActionMenu,
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
        window.Beacon.closeActionMenu = closeActionMenu;
    }

    document.addEventListener('DOMContentLoaded', function () {
        initCollapse(document);
    });
})();
