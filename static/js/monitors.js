// Monitor CRUD on the unified dashboard (modal + action menus).
(function () {
    'use strict';

    var modalEl = null;
    var modalApi = null;
    var formEl = null;
    var titleEl = null;
    var saveBtn = null;
    var editId = null;

    function attachNotifyToForm(form) {
        if (!form || form._notifyAttached) return form._notify;
        var overrides = form.querySelector('[data-notify-overrides]');
        if (!overrides || !window.Beacon.notify.initOverridePanel) return null;
        if (form._notify) {
            try {
                form._notify.destroy && form._notify.destroy();
            } catch (e) {}
            form._notifyAttached = false;
            form._notify = null;
        }
        var initial = {};
        try {
            initial = JSON.parse(overrides.dataset.notifyOverrides || '{}') || {};
        } catch (e) {}
        form._notify = window.Beacon.notify.initOverridePanel(overrides, initial);
        form._notifyAttached = true;
        return form._notify;
    }

    function collectNotifyOverride(form) {
        var overrides = form.querySelector('[data-notify-overrides]');
        if (!overrides || !window.Beacon.notify.readOverrideFromPanel) return null;
        return window.Beacon.notify.readOverrideFromPanel(overrides, form._notify);
    }

    function collectHttpOptions(form) {
        var typeEl = form.querySelector('[data-monitor-type]');
        if (!typeEl || typeEl.value !== 'http') return undefined;
        var username = (form.querySelector('[data-http-username]')?.value || '').trim();
        var password = form.querySelector('[data-http-password]')?.value || '';
        var keyword = (form.querySelector('[data-http-keyword]')?.value || '').trim();
        var keywordInvert = !!(form.querySelector('[data-http-keyword-invert]')?.checked);
        if (!username && !password && !keyword && !keywordInvert) return null;
        var out = { keyword_invert: keywordInvert };
        if (username) out.username = username;
        if (password) out.password = password;
        if (keyword) out.keyword = keyword;
        return out;
    }

    function applyHttpOptions(form, raw) {
        var wrap = form.querySelector('[data-http-options]');
        if (!wrap) return;
        var opts = {};
        if (raw) {
            try {
                opts = typeof raw === 'string' ? JSON.parse(raw) : raw;
            } catch (e) {}
        } else if (wrap.dataset.httpJson) {
            try {
                opts = JSON.parse(wrap.dataset.httpJson || '{}') || {};
            } catch (e) {}
        }
        var userEl = form.querySelector('[data-http-username]');
        var passEl = form.querySelector('[data-http-password]');
        var kwEl = form.querySelector('[data-http-keyword]');
        var invEl = form.querySelector('[data-http-keyword-invert]');
        if (userEl) userEl.value = opts.username || '';
        if (passEl) passEl.value = '';
        if (kwEl) kwEl.value = opts.keyword || '';
        if (invEl) invEl.checked = !!opts.keyword_invert;
    }

    function syncHttpOptionsVisibility(form) {
        var typeEl = form.querySelector('[data-monitor-type]');
        var wrap = form.querySelector('[data-http-options]');
        if (!typeEl || !wrap) return;
        wrap.classList.toggle('d-none', typeEl.value !== 'http');
    }

    function wireHttpOptionsForm(form) {
        if (!form || form._httpOptionsWired) return;
        var typeEl = form.querySelector('[data-monitor-type]');
        if (!typeEl) return;
        form._httpOptionsWired = true;
        typeEl.addEventListener('change', function () {
            syncHttpOptionsVisibility(form);
        });
        syncHttpOptionsVisibility(form);
    }

    function monitorTargetPlaceholder(type) {
        return type === 'tcp' ? 'db.local:5432' : 'https://example.com';
    }

    function clientValidateMonitorTarget(type, target) {
        var t = String(target || '').trim();
        if (!t) return 'Target is required';
        if (type === 'tcp') {
            if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(t)) {
                return 'TCP target must be host:port, not a URL';
            }
            if (/^\[.+\]:\d+$/.test(t) || /^[^:]+:\d+$/.test(t)) {
                return null;
            }
            return 'TCP target must include a port, e.g. db.local:5432';
        }
        if (!/^https?:\/\//i.test(t)) {
            return 'HTTP target must start with http:// or https://';
        }
        return null;
    }

    function wireMonitorTargetForm(form) {
        if (!form || form._monitorTargetWired) return;
        var typeEl = form.querySelector('[data-monitor-type]');
        var targetEl = form.querySelector('[data-monitor-target]');
        var errEl = form.querySelector('[data-monitor-target-error]');
        if (!typeEl || !targetEl) return;
        form._monitorTargetWired = true;
        function showTargetError(msg) {
            if (!errEl) return;
            if (msg) {
                errEl.textContent = msg;
                errEl.classList.remove('d-none');
                targetEl.classList.add('is-invalid');
            } else {
                errEl.textContent = '';
                errEl.classList.add('d-none');
                targetEl.classList.remove('is-invalid');
            }
        }
        function syncPlaceholder() {
            targetEl.placeholder = monitorTargetPlaceholder(typeEl.value || 'http');
        }
        typeEl.addEventListener('change', syncPlaceholder);
        targetEl.addEventListener('input', function () {
            showTargetError(null);
        });
        syncPlaceholder();
        form._validateMonitorTarget = function () {
            var err = clientValidateMonitorTarget(typeEl.value || 'http', targetEl.value);
            showTargetError(err);
            return !err;
        };
    }

    function resetCollapsePanels(form) {
        form.querySelectorAll('.collapse.is-open').forEach(function (panel) {
            panel.classList.remove('is-open');
        });
        form.querySelectorAll('[data-beacon-collapse][aria-expanded="true"]').forEach(function (btn) {
            btn.setAttribute('aria-expanded', 'false');
        });
    }

    function resetFormForAdd() {
        editId = null;
        if (titleEl) titleEl.textContent = 'Add monitor';
        if (saveBtn) saveBtn.textContent = 'Add';
        formEl.reset();
        resetCollapsePanels(formEl);
        var overrides = formEl.querySelector('[data-notify-overrides]');
        if (overrides) overrides.dataset.notifyOverrides = '{}';
        var httpWrap = formEl.querySelector('[data-http-options]');
        if (httpWrap) delete httpWrap.dataset.httpJson;
        applyHttpOptions(formEl, '{}');
        wireMonitorTargetForm(formEl);
        wireHttpOptionsForm(formEl);
        attachNotifyToForm(formEl);
        syncHttpOptionsVisibility(formEl);
    }

    function openEditModal(config) {
        editId = config.id;
        if (titleEl) titleEl.textContent = 'Edit monitor';
        if (saveBtn) saveBtn.textContent = 'Save';
        formEl.querySelector('[name="name"]').value = config.name || '';
        formEl.querySelector('[name="type"]').value = config.type || 'http';
        formEl.querySelector('[name="target"]').value = config.target || '';
        formEl.querySelector('[name="interval"]').value =
            config.interval_sec != null && config.interval_sec > 0 ? String(config.interval_sec) : '';
        resetCollapsePanels(formEl);
        var overrides = formEl.querySelector('[data-notify-overrides]');
        if (overrides) overrides.dataset.notifyOverrides = config.notify_json || '{}';
        var httpWrap = formEl.querySelector('[data-http-options]');
        if (httpWrap) httpWrap.dataset.httpJson = config.http_json || '{}';
        wireMonitorTargetForm(formEl);
        wireHttpOptionsForm(formEl);
        applyHttpOptions(formEl, config.http_json || '{}');
        attachNotifyToForm(formEl);
        syncHttpOptionsVisibility(formEl);
        var typeEl = formEl.querySelector('[data-monitor-type]');
        var targetEl = formEl.querySelector('[data-monitor-target]');
        if (typeEl && targetEl) {
            targetEl.placeholder = monitorTargetPlaceholder(typeEl.value || 'http');
        }
        if (modalApi) modalApi.open();
    }

    function parseMonitorConfig(root) {
        var raw = root.getAttribute('data-monitor-config');
        if (!raw) return null;
        try {
            return JSON.parse(raw);
        } catch (e) {
            return null;
        }
    }

    function monitorRootFromAction(el) {
        return el.closest('[data-dashboard-monitor]');
    }

    async function submitForm(e) {
        e.preventDefault();
        if (formEl._validateMonitorTarget && !formEl._validateMonitorTarget()) return;
        var fd = new FormData(formEl);
        var payload = {
            name: fd.get('name'),
            type: fd.get('type'),
            target: fd.get('target'),
        };
        var iv = parseInt(fd.get('interval'), 10);
        if (!isNaN(iv) && iv > 0) payload.interval = iv;
        var no = collectNotifyOverride(formEl);
        var httpOpts = collectHttpOptions(formEl);

        var res;
        if (editId) {
            var patch = Object.assign({}, payload);
            if (!isNaN(iv)) patch.interval = iv > 0 ? iv : 0;
            patch.notify_override = no || {};
            if (httpOpts !== undefined) patch.http = httpOpts;
            res = await window.Beacon.apiFetch('/api/monitors/' + editId, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(patch),
            });
        } else {
            if (no) payload.notify_override = no;
            if (httpOpts) payload.http = httpOpts;
            res = await window.Beacon.apiFetch('/api/monitors', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
        }

        if (res.ok) {
            if (modalApi) modalApi.close();
            location.reload();
            return;
        }
        alert((await res.text()) || 'Could not save monitor');
    }

    function wireActionMenus() {
        document.addEventListener('click', async function (e) {
            var actionBtn = e.target.closest('[data-monitor-action]');
            if (!actionBtn) return;
            e.preventDefault();
            e.stopPropagation();
            if (window.Beacon.closeAllActionMenus) window.Beacon.closeAllActionMenus(null);

            var root = monitorRootFromAction(actionBtn);
            if (!root) return;
            var config = parseMonitorConfig(root);
            if (!config || !config.id) return;
            var action = actionBtn.getAttribute('data-monitor-action');

            if (action === 'edit') {
                openEditModal(config);
                return;
            }

            if (action === 'toggle') {
                var enabled = actionBtn.getAttribute('data-enabled') === 'true';
                var r = await window.Beacon.apiFetch('/api/monitors/' + config.id, {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ enabled: !enabled }),
                });
                if (r.ok) location.reload();
                return;
            }

            if (action === 'delete') {
                var ok = window.Beacon.modal && window.Beacon.modal.confirm
                    ? await window.Beacon.modal.confirm({
                          title: 'Delete monitor?',
                          message: 'This monitor will be removed permanently.',
                          confirmLabel: 'Delete',
                          cancelLabel: 'Cancel',
                          destructive: true,
                      })
                    : confirm('Delete this monitor?');
                if (!ok) return;
                var del = await window.Beacon.apiFetch('/api/monitors/' + config.id, { method: 'DELETE' });
                if (del.ok) location.reload();
            }
        });
    }

    function init() {
        modalEl = document.getElementById('monitorModal');
        formEl = document.getElementById('monitorModalForm');
        titleEl = document.getElementById('monitorModalTitle');
        saveBtn = document.getElementById('monitorModalSave');
        if (!modalEl || !formEl || !window.Beacon || !window.Beacon.modal) return;

        modalApi = window.Beacon.modal.wire(modalEl);
        wireMonitorTargetForm(formEl);
        wireHttpOptionsForm(formEl);
        attachNotifyToForm(formEl);

        formEl.addEventListener('submit', submitForm);

        document.querySelectorAll('[data-monitor-add]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                resetFormForAdd();
                if (modalApi) modalApi.open();
            });
        });

        wireActionMenus();
        if (window.Beacon.wireActionMenus) window.Beacon.wireActionMenus(document);
    }

    fetch('/api/config')
        .then(function (r) {
            return r.json();
        })
        .then(function (cfg) {
            if (window.Beacon && window.Beacon.notify) {
                window.Beacon.notify.setGlobalDefaults(cfg.notifications || {});
            }
        })
        .catch(function () {});

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
