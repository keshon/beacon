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

    var scrollLockCount = 0;

    function lockBodyScroll() {
        scrollLockCount += 1;
        if (scrollLockCount !== 1) return;
        document.documentElement.classList.add('beacon-modal-open');
        document.body.classList.add('beacon-modal-open');
    }

    function unlockBodyScroll() {
        if (scrollLockCount <= 0) return;
        scrollLockCount -= 1;
        if (scrollLockCount !== 0) return;
        document.documentElement.classList.remove('beacon-modal-open');
        document.body.classList.remove('beacon-modal-open');
    }

    function isScrollLocked() {
        return scrollLockCount > 0;
    }

    function preventBackgroundScroll(e) {
        if (!isScrollLocked()) return;
        if (e.type === 'touchmove') {
            if (e.target.closest('.beacon-modal:not([hidden])')) return;
            e.preventDefault();
            return;
        }
        var body = e.target.closest('.beacon-modal__body');
        if (body && body.scrollHeight > body.clientHeight) {
            var dy = e.deltaY || 0;
            var atTop = body.scrollTop <= 0;
            var atBottom = body.scrollTop + body.clientHeight >= body.scrollHeight - 1;
            if ((dy < 0 && !atTop) || (dy > 0 && !atBottom)) return;
        }
        e.preventDefault();
    }

    document.addEventListener('wheel', preventBackgroundScroll, { passive: false });
    document.addEventListener('touchmove', preventBackgroundScroll, { passive: false });

    function wireModal(modalEl) {
        if (!modalEl || modalEl._beaconModalWired) {
            return modalEl && modalEl._beaconModal ? modalEl._beaconModal : null;
        }
        modalEl._beaconModalWired = true;
        var lastActiveElement = null;

        function close() {
            modalEl.hidden = true;
            modalEl.setAttribute('aria-hidden', 'true');
            unlockBodyScroll();
            if (lastActiveElement && typeof lastActiveElement.focus === 'function') {
                lastActiveElement.focus();
            }
            lastActiveElement = null;
        }

        function open() {
            lastActiveElement = document.activeElement;
            modalEl.hidden = false;
            modalEl.setAttribute('aria-hidden', 'false');
            lockBodyScroll();
            var dialog = modalEl.querySelector('.beacon-modal__dialog');
            if (dialog && typeof dialog.focus === 'function') {
                dialog.focus();
            }
        }

        modalEl.addEventListener('click', function (e) {
            if (e.target.closest('[data-beacon-modal-close]')) {
                close();
            }
        });

        document.addEventListener('keydown', function (e) {
            if (!modalEl || modalEl.hidden) return;
            if (e.key === 'Escape') {
                e.preventDefault();
                close();
            }
        });

        modalEl._beaconModal = { open: open, close: close };
        return modalEl._beaconModal;
    }

    var confirmModalEl = null;
    var confirmApi = null;
    var confirmResolve = null;

    function ensureConfirmModal() {
        if (confirmModalEl) return;
        confirmModalEl = document.createElement('div');
        confirmModalEl.className = 'beacon-modal beacon-modal--confirm';
        confirmModalEl.id = 'beaconConfirmModal';
        confirmModalEl.hidden = true;
        confirmModalEl.setAttribute('role', 'alertdialog');
        confirmModalEl.setAttribute('aria-modal', 'true');
        confirmModalEl.setAttribute('aria-labelledby', 'beaconConfirmTitle');
        confirmModalEl.setAttribute('aria-describedby', 'beaconConfirmMessage');
        confirmModalEl.innerHTML =
            '<button type="button" class="beacon-modal__backdrop" data-beacon-confirm-cancel aria-label="Close dialog"></button>' +
            '<div class="beacon-modal__dialog" tabindex="-1">' +
                '<header class="beacon-modal__header">' +
                    '<h2 class="beacon-modal__title" id="beaconConfirmTitle">Confirm</h2>' +
                    '<button type="button" class="beacon-modal__close" data-beacon-confirm-cancel aria-label="Close">' +
                        '<i class="bi bi-x-lg" aria-hidden="true"></i>' +
                    '</button>' +
                '</header>' +
                '<div class="beacon-modal__scroll">' +
                '<div class="beacon-modal__body">' +
                    '<p class="beacon-modal__message" id="beaconConfirmMessage"></p>' +
                '</div>' +
                '</div>' +
                '<footer class="beacon-modal__footer">' +
                    '<button type="button" class="btn btn-outline-secondary" data-beacon-confirm-cancel>Cancel</button>' +
                    '<button type="button" class="btn btn-primary" data-beacon-confirm-ok>Confirm</button>' +
                '</footer>' +
            '</div>';
        document.body.appendChild(confirmModalEl);

        var confirmLastFocus = null;

        function confirmClose() {
            confirmModalEl.hidden = true;
            confirmModalEl.setAttribute('aria-hidden', 'true');
            unlockBodyScroll();
            if (confirmLastFocus && typeof confirmLastFocus.focus === 'function') {
                confirmLastFocus.focus();
            }
            confirmLastFocus = null;
        }

        confirmApi = {
            open: function () {
                confirmLastFocus = document.activeElement;
                confirmModalEl.hidden = false;
                confirmModalEl.setAttribute('aria-hidden', 'false');
                lockBodyScroll();
                var dialog = confirmModalEl.querySelector('.beacon-modal__dialog');
                if (dialog && typeof dialog.focus === 'function') {
                    dialog.focus();
                }
            },
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
            modal: {
                wire: wireModal,
                confirm: confirm,
                lockScroll: lockBodyScroll,
                unlockScroll: unlockBodyScroll,
            },
            wireActionMenus: wireActionMenus,
            closeAllActionMenus: closeAllActionMenus,
        };
    } else {
        window.Beacon.csrfToken = csrfToken;
        window.Beacon.apiFetch = apiFetch;
        window.Beacon.initCollapse = initCollapse;
        window.Beacon.applyAppearancePrefs = applyAppearancePrefs;
        window.Beacon.modal = window.Beacon.modal || {
            wire: wireModal,
            confirm: confirm,
            lockScroll: lockBodyScroll,
            unlockScroll: unlockBodyScroll,
        };
        if (window.Beacon.modal && !window.Beacon.modal.confirm) {
            window.Beacon.modal.confirm = confirm;
        }
        if (window.Beacon.modal && !window.Beacon.modal.lockScroll) {
            window.Beacon.modal.lockScroll = lockBodyScroll;
            window.Beacon.modal.unlockScroll = unlockBodyScroll;
        }
        window.Beacon.wireActionMenus = wireActionMenus;
        window.Beacon.closeAllActionMenus = closeAllActionMenus;
    }

    document.addEventListener('DOMContentLoaded', function () {
        initCollapse(document);
        wireActionMenus(document);
    });
})();
