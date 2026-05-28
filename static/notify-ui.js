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
        return (
            window.BeaconNotifyGlobalDefaults || {
                alert_mode: 'repeat',
                templates: {},
            }
        );
    }

    function loadBuiltins() {
        if (builtinsCache) return Promise.resolve(builtinsCache);
        if (window.BeaconNotifyPolicy && window.BeaconNotifyPolicy.fetchDefaults) {
            return window.BeaconNotifyPolicy.fetchDefaults().then(function (d) {
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

    function updateRowBadges(row) {
        var badges = row.querySelector('[data-notify-badges]');
        if (!badges) return;
        effectivePolicy(readRowPolicy(row)).then(function (eff) {
            var modeLabel = eff.mode === 'once' ? 'Once' : 'Repeat';
            var tplLabel = eff.custom ? 'Custom' : 'Standard';
            badges.innerHTML =
                '<span class="badge rounded-pill text-bg-secondary me-1">' +
                modeLabel +
                '</span>' +
                '<span class="badge rounded-pill text-bg-secondary">' +
                tplLabel +
                '</span>';
        });
    }

    function rowActionsHtml() {
        return (
            '<div class="col-md-3 d-flex gap-1">' +
            '<button type="button" class="btn btn-outline-secondary" data-notify-action="policy" title="Alert policy">' +
            '<i class="bi bi-gear"></i></button>' +
            '<button type="button" class="btn btn-outline-danger" data-notify-action="remove" title="Remove receiver">' +
            '<i class="bi bi-x-lg"></i></button>' +
            '</div>' +
            '<div class="col-12"><span class="d-flex flex-wrap gap-1" data-notify-badges></span></div>'
        );
    }

    function applyRowValue(row, value) {
        value = value || {};
        if (value.policy) {
            writeRowPolicy(row, value.policy);
        }
        updateRowBadges(row);
    }

    function buildTelegramRow(value) {
        value = value || {};
        var row = el(
            '<div class="notify-row row g-2 align-items-end mb-2">' +
                '<div class="col-md-5">' +
                '<input type="text" class="form-control" data-notify-field="token" placeholder="Bot token" />' +
                '</div>' +
                '<div class="col-md-4">' +
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
            '<div class="notify-row row g-2 align-items-end mb-2">' +
                '<div class="col-md-9">' +
                '<input type="text" class="form-control" data-notify-field="webhook" placeholder="Webhook URL" />' +
                '</div>' +
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
        if (!window.BeaconReceiverPolicyModal) return;
        var data = readRow(this.channel, row);
        var delivery = { channel: this.channel };
        if (this.channel === 'telegram') {
            delivery.telegram = { token: data.token, chat_id: data.chat_id };
        } else {
            delivery.discord = { webhook: data.webhook };
        }
        window.BeaconReceiverPolicyModal.open(
            readRowPolicy(row),
            delivery,
            function (policy) {
                writeRowPolicy(row, policy);
                updateRowBadges(row);
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
            updateRowBadges(row);
        });
    };

    function init(container, channel, values) {
        var instance = new NotifyList(container, channel);
        instance.setValues(values || []);
        return instance;
    }

    function setGlobalDefaults(notifications) {
        window.BeaconNotifyGlobalDefaults = notifications || {
            alert_mode: 'repeat',
            templates: {},
        };
        document.querySelectorAll('.notify-row').forEach(updateRowBadges);
    }

    window.BeaconNotify = {
        MAX_RECEIVERS: MAX_RECEIVERS,
        init: init,
        setGlobalDefaults: setGlobalDefaults,
        updateRowBadges: updateRowBadges,
    };
})();
