// Shared UI helpers for Telegram / Discord notification receivers.
// Used by /settings (global config) and /monitors (per-monitor overrides).
(function () {
    'use strict';

    var MAX_RECEIVERS = 5;
    var builtinsCache = null;

    function el(html) {
        var tpl = document.createElement('template');
        tpl.innerHTML = html.trim();
        return tpl.content.firstChild;
    }

    function parsePolicy(raw) {
        if (!raw) return {};
        try {
            return JSON.parse(raw) || {};
        } catch (e) {
            return {};
        }
    }

    function readRowPolicy(row) {
        return parsePolicy(row.dataset.notifyPolicy);
    }

    function writeRowPolicy(row, policy) {
        var out = policy || {};
        var hasMode = !!(out.alert_mode && String(out.alert_mode).trim());
        var hasTpl =
            out.templates &&
            (String(out.templates.down || '').trim() || String(out.templates.recovered || '').trim());
        if (!hasMode && !hasTpl) {
            delete row.dataset.notifyPolicy;
        } else {
            row.dataset.notifyPolicy = JSON.stringify(out);
        }
    }

    function globalDefaults() {
        var root = window.Beacon && window.Beacon.notify;
        return (
            (root && root.globalDefaults) ||
            window.BeaconNotifyGlobalDefaults || {
                alert_mode: 'repeat',
                templates: {},
            }
        );
    }

    function loadBuiltins() {
        if (builtinsCache) return Promise.resolve(builtinsCache);
        var policy = (window.Beacon && window.Beacon.policy) || window.BeaconNotifyPolicy;
        if (policy && policy.fetchDefaults) {
            return policy.fetchDefaults().then(function (d) {
                builtinsCache = d;
                return d;
            });
        }
        return Promise.resolve({ alert_mode: 'repeat', templates: {} });
    }

    function mergeTemplates(def, globalTpl, rowTpl) {
        function pick(d, g, r) {
            if (r && String(r).trim()) return String(r).trim();
            if (g && String(g).trim()) return String(g).trim();
            return d || '';
        }
        var g = globalTpl || {};
        var r = rowTpl || {};
        return {
            down: pick((def.templates && def.templates.down) || '', g.down, r.down),
            recovered: pick((def.templates && def.templates.recovered) || '', g.recovered, r.recovered),
        };
    }

    function effectivePolicy(rowPolicy) {
        var row = rowPolicy || {};
        var global = globalDefaults();
        var mode = (row.alert_mode && String(row.alert_mode).trim()) || global.alert_mode || 'repeat';
        return loadBuiltins().then(function (def) {
            var templates = mergeTemplates(def, global.templates || {}, row.templates || {});
            var builtin = (def && def.templates) || {};
            var custom =
                templates.down !== (builtin.down || '') ||
                templates.recovered !== (builtin.recovered || '');
            return { mode: mode, custom: custom };
        });
    }

    function rowHasPolicy(row) {
        var p = readRowPolicy(row);
        var hasMode = !!(p.alert_mode && String(p.alert_mode).trim());
        var hasTpl =
            p.templates &&
            (String(p.templates.down || '').trim() || String(p.templates.recovered || '').trim());
        return hasMode || hasTpl;
    }

    function updateRowMeta(row) {
        var meta = row.querySelector('[data-notify-meta]');
        if (!meta) return;
        var inherited = !rowHasPolicy(row);
        effectivePolicy(readRowPolicy(row)).then(function (eff) {
            var modeLabel = eff.mode === 'once' ? 'Once' : 'Repeat';
            var tplLabel = eff.custom ? 'Custom' : 'Standard';
            var tplClass = eff.custom ? ' notify-row-meta__templates--custom' : '';
            meta.classList.toggle('notify-row-meta--inherited', inherited && !eff.custom);
            meta.innerHTML =
                '<span class="notify-row-meta__mode">' +
                modeLabel +
                '</span>' +
                '<span class="notify-row-meta__sep" aria-hidden="true">·</span>' +
                '<span class="notify-row-meta__templates' +
                tplClass +
                '">' +
                tplLabel +
                '</span>';
        });
    }

    function rowActionsHtml() {
        return (
            '<span class="notify-row-meta" data-notify-meta></span>' +
            '<div class="notify-row__actions">' +
            '<button type="button" class="btn notify-row__btn" data-notify-action="policy" title="Alert policy">' +
            '<i class="bi bi-gear"></i></button>' +
            '<button type="button" class="btn notify-row__btn notify-row__btn--danger" data-notify-action="remove" title="Remove receiver">' +
            '<i class="bi bi-x-lg"></i></button>' +
            '</div>'
        );
    }

    function applyRowValue(row, value) {
        value = value || {};
        if (value.policy) {
            writeRowPolicy(row, value.policy);
        }
        updateRowMeta(row);
    }

    function buildTelegramRow(value) {
        value = value || {};
        var row = el(
            '<div class="notify-row notify-row--telegram">' +
                '<div class="notify-row__fields">' +
                '<input type="text" class="form-control" data-notify-field="token" placeholder="Bot token" />' +
                '<input type="text" class="form-control" data-notify-field="chat_id" placeholder="Chat ID" />' +
                '</div>' +
                rowActionsHtml() +
                '</div>'
        );
        row.querySelector('[data-notify-field="token"]').value = value.token || '';
        row.querySelector('[data-notify-field="chat_id"]').value = value.chat_id || '';
        applyRowValue(row, value);
        return row;
    }

    function buildDiscordRow(value) {
        value = value || {};
        var row = el(
            '<div class="notify-row notify-row--discord">' +
                '<input type="text" class="form-control notify-row__field-main" data-notify-field="webhook" placeholder="Webhook URL" />' +
                rowActionsHtml() +
                '</div>'
        );
        row.querySelector('[data-notify-field="webhook"]').value = value.webhook || '';
        applyRowValue(row, value);
        return row;
    }

    function readRow(channel, row) {
        var data;
        if (channel === 'telegram') {
            data = {
                token: (row.querySelector('[data-notify-field="token"]').value || '').trim(),
                chat_id: (row.querySelector('[data-notify-field="chat_id"]').value || '').trim(),
            };
        } else {
            data = {
                webhook: (row.querySelector('[data-notify-field="webhook"]').value || '').trim(),
            };
        }
        var policy = readRowPolicy(row);
        if (policy.alert_mode || (policy.templates && (policy.templates.down || policy.templates.recovered))) {
            data.policy = policy;
        }
        return data;
    }

    function isRowFilled(channel, data) {
        if (channel === 'telegram') {
            return !!(data.token && data.chat_id);
        }
        return !!data.webhook;
    }

    function NotifyList(container, channel) {
        this.container = container;
        this.channel = channel;
        this.list = container.querySelector('[data-notify-list]');
        this.addBtn = container.querySelector('[data-notify-add]');
        this.helper = container.querySelector('[data-notify-helper]');
        this.bind();
    }

    NotifyList.prototype.rowBuilder = function (value) {
        return this.channel === 'telegram' ? buildTelegramRow(value) : buildDiscordRow(value);
    };

    NotifyList.prototype.bind = function () {
        var self = this;
        if (this.addBtn) {
            this.addBtn.addEventListener('click', function () {
                self.add();
            });
        }
        this.list.addEventListener('click', function (e) {
            var btn = e.target.closest('[data-notify-action]');
            if (!btn) return;
            var row = btn.closest('.notify-row');
            if (!row) return;
            if (btn.dataset.notifyAction === 'remove') {
                self.remove(row);
            } else if (btn.dataset.notifyAction === 'policy') {
                self.editPolicy(row);
            }
        });
    };

    NotifyList.prototype.editPolicy = function (row) {
        var modal = (window.Beacon && window.Beacon.policyModal) || window.BeaconReceiverPolicyModal;
        if (!modal) return;
        var data = readRow(this.channel, row);
        var delivery = { channel: this.channel };
        if (this.channel === 'telegram') {
            delivery.telegram = { token: data.token, chat_id: data.chat_id };
        } else {
            delivery.discord = { webhook: data.webhook };
        }
        modal.open(
            readRowPolicy(row),
            delivery,
            function (policy) {
                writeRowPolicy(row, policy);
                updateRowMeta(row);
            }
        );
    };

    NotifyList.prototype.rows = function () {
        return Array.from(this.list.querySelectorAll('.notify-row'));
    };

    NotifyList.prototype.setValues = function (values) {
        this.list.innerHTML = '';
        var list = Array.isArray(values) ? values : [];
        if (list.length === 0) {
            this.list.appendChild(this.rowBuilder());
        } else {
            for (var i = 0; i < list.length && i < MAX_RECEIVERS; i++) {
                this.list.appendChild(this.rowBuilder(list[i]));
            }
        }
        this.refresh();
    };

    NotifyList.prototype.values = function () {
        var out = [];
        var rows = this.rows();
        for (var i = 0; i < rows.length; i++) {
            var v = readRow(this.channel, rows[i]);
            if (isRowFilled(this.channel, v)) out.push(v);
        }
        return out;
    };

    NotifyList.prototype.add = function () {
        if (this.rows().length >= MAX_RECEIVERS) return;
        this.list.appendChild(this.rowBuilder());
        this.refresh();
    };

    NotifyList.prototype.remove = function (row) {
        row.remove();
        if (this.rows().length === 0) {
            this.list.appendChild(this.rowBuilder());
        }
        this.refresh();
    };

    NotifyList.prototype.refresh = function () {
        var rows = this.rows();
        var count = rows.length;
        var atMax = count >= MAX_RECEIVERS;
        if (this.addBtn) {
            this.addBtn.disabled = atMax;
        }
        if (this.helper) {
            this.helper.textContent = atMax
                ? 'Maximum of ' + MAX_RECEIVERS + ' receivers reached.'
                : 'Up to ' + MAX_RECEIVERS + ' receivers. Empty rows are ignored.';
        }
        rows.forEach(function (row, idx) {
            var removeBtn = row.querySelector('[data-notify-action="remove"]');
            if (removeBtn) {
                removeBtn.style.visibility = count === 1 ? 'hidden' : 'visible';
            }
            row.dataset.notifyIndex = String(idx);
            updateRowMeta(row);
        });
    };

    function init(container, channel, values) {
        var instance = new NotifyList(container, channel);
        instance.setValues(values || []);
        return instance;
    }

    function setGlobalDefaults(notifications) {
        var defs = notifications || { alert_mode: 'repeat', templates: {} };
        window.Beacon = window.Beacon || { notify: {} };
        window.Beacon.notify.globalDefaults = defs;
        window.BeaconNotifyGlobalDefaults = defs;
        document.querySelectorAll('.notify-row').forEach(updateRowMeta);
    }

    var notifyAPI = {
        MAX_RECEIVERS: MAX_RECEIVERS,
        init: init,
        setGlobalDefaults: setGlobalDefaults,
        updateRowMeta: updateRowMeta,
    };
    window.Beacon = window.Beacon || {};
    window.Beacon.notify = Object.assign(window.Beacon.notify || {}, notifyAPI);
    window.BeaconNotify = notifyAPI;
})();
