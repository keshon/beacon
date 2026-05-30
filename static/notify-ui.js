// Notification receivers UI: settings lists + monitor override tri-state panels.
(function () {
    'use strict';

    var MAX_RECEIVERS = 5;
    var builtinsCache = null;
    var MODES = ['inherit', 'off', 'custom'];
    var MODE_LABELS = { inherit: 'Global', off: 'Off', custom: 'Custom' };

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
        return (root && root.globalDefaults) || { alert_mode: 'repeat', templates: {} };
    }

    function loadBuiltins() {
        if (builtinsCache) return Promise.resolve(builtinsCache);
        var policy = window.Beacon && window.Beacon.policy;
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

    function updateRowMeta(row, channel) {
        var meta = row.querySelector('[data-notify-meta]');
        if (!meta) return;
        var inherited = !rowHasPolicy(row);
        effectivePolicy(readRowPolicy(row)).then(function (eff) {
            var modeLabel = channel === 'email' ? 'Once' : eff.mode === 'once' ? 'Once' : 'Repeat';
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

    function applyRowValue(row, value, channel) {
        value = value || {};
        if (value.policy) {
            writeRowPolicy(row, value.policy);
        }
        updateRowMeta(row, channel);
    }

    var channels = {
        telegram: {
            buildRow: function (value) {
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
                applyRowValue(row, value, 'telegram');
                return row;
            },
            readRow: function (row) {
                var data = {
                    token: (row.querySelector('[data-notify-field="token"]').value || '').trim(),
                    chat_id: (row.querySelector('[data-notify-field="chat_id"]').value || '').trim(),
                };
                var policy = readRowPolicy(row);
                if (policy.alert_mode || (policy.templates && (policy.templates.down || policy.templates.recovered))) {
                    data.policy = policy;
                }
                return data;
            },
            isFilled: function (data) {
                return !!(data.token && data.chat_id);
            },
            delivery: function (data) {
                return { channel: 'telegram', telegram: { token: data.token, chat_id: data.chat_id } };
            },
        },
        discord: {
            buildRow: function (value) {
                value = value || {};
                var row = el(
                    '<div class="notify-row notify-row--discord">' +
                        '<div class="notify-row__fields">' +
                        '<input type="text" class="form-control" data-notify-field="webhook" placeholder="Webhook URL" />' +
                        '</div>' +
                        rowActionsHtml() +
                        '</div>'
                );
                row.querySelector('[data-notify-field="webhook"]').value = value.webhook || '';
                applyRowValue(row, value, 'discord');
                return row;
            },
            readRow: function (row) {
                var data = {
                    webhook: (row.querySelector('[data-notify-field="webhook"]').value || '').trim(),
                };
                var policy = readRowPolicy(row);
                if (policy.alert_mode || (policy.templates && (policy.templates.down || policy.templates.recovered))) {
                    data.policy = policy;
                }
                return data;
            },
            isFilled: function (data) {
                return !!data.webhook;
            },
            delivery: function (data) {
                return { channel: 'discord', discord: { webhook: data.webhook } };
            },
        },
        email: {
            buildRow: function (value) {
                value = value || {};
                var row = el(
                    '<div class="notify-row notify-row--email">' +
                        '<div class="notify-row__fields">' +
                        '<input type="email" class="form-control" data-notify-field="to" placeholder="recipient@example.com" />' +
                        '</div>' +
                        rowActionsHtml() +
                        '</div>'
                );
                row.querySelector('[data-notify-field="to"]').value = value.to || '';
                applyRowValue(row, value, 'email');
                return row;
            },
            readRow: function (row) {
                var data = { to: (row.querySelector('[data-notify-field="to"]').value || '').trim() };
                var policy = readRowPolicy(row);
                if (policy.alert_mode) delete policy.alert_mode;
                if (policy.templates && (policy.templates.down || policy.templates.recovered)) {
                    data.policy = policy;
                }
                return data;
            },
            isFilled: function (data) {
                return !!data.to;
            },
            delivery: function (data) {
                return { channel: 'email', email: { to: data.to } };
            },
        },
        webhook: {
            buildRow: function (value) {
                value = value || {};
                var row = el(
                    '<div class="notify-row notify-row--webhook">' +
                        '<div class="notify-row__fields">' +
                        '<input type="url" class="form-control" data-notify-field="url" placeholder="https://hooks.example.com/..." />' +
                        '</div>' +
                        rowActionsHtml() +
                        '</div>'
                );
                row.querySelector('[data-notify-field="url"]').value = value.url || '';
                applyRowValue(row, value, 'webhook');
                return row;
            },
            readRow: function (row) {
                var data = { url: (row.querySelector('[data-notify-field="url"]').value || '').trim() };
                var policy = readRowPolicy(row);
                if (policy.alert_mode || (policy.templates && (policy.templates.down || policy.templates.recovered))) {
                    data.policy = policy;
                }
                return data;
            },
            isFilled: function (data) {
                return !!data.url;
            },
            delivery: function (data) {
                return { channel: 'webhook', webhook: { url: data.url } };
            },
        },
    };

    function NotifyList(container, channel) {
        this.container = container;
        this.channel = channel;
        this.def = channels[channel];
        this.list = container.querySelector('[data-notify-list]');
        this.addBtn = container.querySelector('[data-notify-add]');
        this.helper = container.querySelector('[data-notify-helper]');
        this.bind();
    }

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
        var modal = window.Beacon && window.Beacon.policyModal;
        if (!modal) return;
        var list = this;
        var targetRow = row;
        modal.open(readRowPolicy(row), null, function (policy) {
            if (this.channel === 'email' && policy) {
                delete policy.alert_mode;
            }
            writeRowPolicy(row, policy);
            updateRowMeta(row, this.channel);
        }.bind(this), {
            channel: this.channel,
            getDelivery: function () {
                return list.def.delivery(list.def.readRow(targetRow));
            },
        });
    };

    NotifyList.prototype.rows = function () {
        return Array.from(this.list.querySelectorAll('.notify-row'));
    };

    NotifyList.prototype.setValues = function (values) {
        this.list.innerHTML = '';
        var list = Array.isArray(values) ? values : [];
        if (list.length === 0) {
            this.list.appendChild(this.def.buildRow());
        } else {
            for (var i = 0; i < list.length && i < MAX_RECEIVERS; i++) {
                this.list.appendChild(this.def.buildRow(list[i]));
            }
        }
        this.refresh();
    };

    NotifyList.prototype.values = function () {
        var out = [];
        this.rows().forEach(function (row) {
            var v = this.def.readRow(row);
            if (this.def.isFilled(v)) out.push(v);
        }, this);
        return out;
    };

    NotifyList.prototype.add = function () {
        if (this.rows().length >= MAX_RECEIVERS) return;
        this.list.appendChild(this.def.buildRow());
        this.refresh();
    };

    NotifyList.prototype.remove = function (row) {
        row.remove();
        if (this.rows().length === 0) {
            this.list.appendChild(this.def.buildRow());
        }
        this.refresh();
    };

    NotifyList.prototype.refresh = function () {
        var rows = this.rows();
        var count = rows.length;
        var atMax = count >= MAX_RECEIVERS;
        if (this.addBtn) this.addBtn.disabled = atMax;
        if (this.helper) {
            this.helper.textContent = atMax
                ? 'Maximum of ' + MAX_RECEIVERS + ' receivers reached.'
                : 'Up to ' + MAX_RECEIVERS + ' receivers. Empty rows are ignored.';
        }
        var ch = this.channel;
        rows.forEach(function (row, idx) {
            var removeBtn = row.querySelector('[data-notify-action="remove"]');
            if (removeBtn) removeBtn.style.visibility = count === 1 ? 'hidden' : 'visible';
            row.dataset.notifyIndex = String(idx);
            updateRowMeta(row, ch);
        });
    };

    function init(container, channel, values) {
        var instance = new NotifyList(container, channel);
        instance.setValues(values || []);
        return instance;
    }

    function normalizeChannelBlock(raw, legacyKey) {
        if (!raw) return { mode: 'inherit', targets: [] };
        if (Array.isArray(raw)) {
            return raw.length ? { mode: 'custom', targets: raw } : { mode: 'inherit', targets: [] };
        }
        if (raw.mode) {
            return { mode: raw.mode || 'inherit', targets: raw.targets || [] };
        }
        return { mode: 'inherit', targets: [] };
    }

    function setChannelMode(panel, mode) {
        mode = MODES.indexOf(mode) >= 0 ? mode : 'inherit';
        var seg = panel.querySelector('[data-notify-mode]');
        if (seg) {
            seg.querySelectorAll('[data-mode]').forEach(function (btn) {
                btn.classList.toggle('is-active', btn.dataset.mode === mode);
            });
        }
        panel.dataset.notifyCurrentMode = mode;
        var body = panel.querySelector('[data-notify-channel-body]');
        var hint = panel.querySelector('[data-notify-mode-hint]');
        if (body) body.classList.toggle('d-none', mode !== 'custom');
        if (hint) {
            if (mode === 'inherit') hint.textContent = 'Uses global settings for this channel.';
            else if (mode === 'off') hint.textContent = 'Disabled for this monitor.';
            else hint.textContent = 'Custom receiver list for this monitor only.';
        }
    }

    function wireModeSegment(panel) {
        var seg = panel.querySelector('[data-notify-mode]');
        if (!seg || seg._wired) return;
        seg._wired = true;
        seg.querySelectorAll('[data-mode]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                setChannelMode(panel, btn.dataset.mode);
            });
        });
    }

    function initOverridePanel(overridesEl, initial) {
        initial = initial || {};
        var lists = {};
        overridesEl.querySelectorAll('[data-notify-channel-panel]').forEach(function (panel) {
            var channel = panel.dataset.notifyChannelPanel;
            if (!channels[channel]) return;
            wireModeSegment(panel);
            var block = normalizeChannelBlock(initial[channel], channel);
            setChannelMode(panel, block.mode);
            var listRoot = panel.querySelector('[data-notify-channel-body]');
            lists[channel] = init(listRoot, channel, block.mode === 'custom' ? block.targets : []);
        });
        return lists;
    }

    function readListValues(listRoot, channel) {
        var def = channels[channel];
        if (!def || !listRoot) return [];
        var list = listRoot.querySelector('[data-notify-list]');
        if (!list) return [];
        var out = [];
        list.querySelectorAll('.notify-row').forEach(function (row) {
            var v = def.readRow(row);
            if (def.isFilled(v)) out.push(v);
        });
        return out;
    }

    function readOverrideFromPanel(overridesEl, lists) {
        lists = lists || {};
        var out = {};
        overridesEl.querySelectorAll('[data-notify-channel-panel]').forEach(function (panel) {
            var channel = panel.dataset.notifyChannelPanel;
            var mode = panel.dataset.notifyCurrentMode || 'inherit';
            if (mode === 'inherit') return;
            var block = { mode: mode };
            if (mode === 'custom') {
                if (lists[channel] && typeof lists[channel].values === 'function') {
                    block.targets = lists[channel].values();
                } else {
                    var listRoot = panel.querySelector('[data-notify-channel-body]');
                    block.targets = readListValues(listRoot, channel);
                }
            }
            out[channel] = block;
        });
        return Object.keys(out).length ? out : null;
    }

    function setGlobalDefaults(notifications) {
        var defs = notifications || { alert_mode: 'repeat', templates: {} };
        window.Beacon = window.Beacon || { notify: {} };
        window.Beacon.notify.globalDefaults = defs;
        document.querySelectorAll('.notify-row').forEach(function (row) {
            var ch = row.className.match(/notify-row--(\w+)/);
            updateRowMeta(row, ch ? ch[1] : 'telegram');
        });
    }

    window.Beacon = window.Beacon || {};
    window.Beacon.notify = Object.assign(window.Beacon.notify || {}, {
        MAX_RECEIVERS: MAX_RECEIVERS,
        channels: channels,
        init: init,
        initOverridePanel: initOverridePanel,
        readOverrideFromPanel: readOverrideFromPanel,
        setGlobalDefaults: setGlobalDefaults,
        updateRowMeta: updateRowMeta,
    });
})();
