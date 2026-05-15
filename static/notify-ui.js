// Shared UI helpers for Telegram / Discord notification receivers.
// Used by /settings (global config) and /monitors (per-monitor overrides).
(function () {
    'use strict';

    var MAX_RECEIVERS = 5;

    function el(html) {
        var tpl = document.createElement('template');
        tpl.innerHTML = html.trim();
        return tpl.content.firstChild;
    }

    function statusClass(kind) {
        switch (kind) {
            case 'success':
                return 'small text-success';
            case 'error':
                return 'small text-danger';
            case 'warn':
                return 'small text-warning';
            default:
                return 'small text-muted';
        }
    }

    function setStatus(node, kind, text) {
        if (!node) return;
        node.className = statusClass(kind);
        node.textContent = text || '';
    }

    function buildTelegramRow(value) {
        value = value || {};
        var row = el(
            '<div class="notify-row row g-2 align-items-end mb-2">' +
                '<div class="col-md-5">' +
                // '<label class="form-label small">Bot token</label>' +
                '<input type="text" class="form-control" data-notify-field="token" placeholder="Bot token" />' +
                '</div>' +
                '<div class="col-md-4">' +
                // '<label class="form-label small">Chat ID</label>' +
                '<input type="text" class="form-control" data-notify-field="chat_id" placeholder="Chat ID" />' +
                '</div>' +
                '<div class="col-md-3 d-flex gap-1">' +
                '<button type="button" class="btn btn-outline-secondary" data-notify-action="test">' +
                'Test</button>' +
                '<button type="button" class="btn btn-outline-danger" data-notify-action="remove" title="Remove receiver">' +
                '<i class="bi bi-x-lg"></i></button>' +
                '</div>' +
                '<div class="col-12"><span class="notify-row-status small text-muted" data-notify-status></span></div>' +
                '</div>'
        );
        row.querySelector('[data-notify-field="token"]').value = value.token || '';
        row.querySelector('[data-notify-field="chat_id"]').value = value.chat_id || '';
        return row;
    }

    function buildDiscordRow(value) {
        value = value || {};
        var row = el(
            '<div class="notify-row row g-2 align-items-end mb-2">' +
                '<div class="col-md-9">' +
                // '<label class="form-label small">Webhook URL</label>' +
                '<input type="text" class="form-control" data-notify-field="webhook" placeholder="Webhook URL" />' +
                '</div>' +
                '<div class="col-md-3 d-flex gap-1">' +
                '<button type="button" class="btn btn-outline-secondary" data-notify-action="test">' +
                'Test</button>' +
                '<button type="button" class="btn btn-outline-danger" data-notify-action="remove" title="Remove receiver">' +
                '<i class="bi bi-x-lg"></i></button>' +
                '</div>' +
                '<div class="col-12"><span class="notify-row-status small text-muted" data-notify-status></span></div>' +
                '</div>'
        );
        row.querySelector('[data-notify-field="webhook"]').value = value.webhook || '';
        return row;
    }

    function readRow(channel, row) {
        if (channel === 'telegram') {
            return {
                token: (row.querySelector('[data-notify-field="token"]').value || '').trim(),
                chat_id: (row.querySelector('[data-notify-field="chat_id"]').value || '').trim(),
            };
        }
        return {
            webhook: (row.querySelector('[data-notify-field="webhook"]').value || '').trim(),
        };
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
            } else if (btn.dataset.notifyAction === 'test') {
                self.test(row, btn);
            }
        });
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
        });
    };

    NotifyList.prototype.test = function (row, btn) {
        var statusEl = row.querySelector('[data-notify-status]');
        var data = readRow(this.channel, row);
        if (!isRowFilled(this.channel, data)) {
            setStatus(statusEl, 'warn', 'Fill the fields first.');
            return;
        }
        var payload = { channel: this.channel };
        payload[this.channel] = data;
        var label = btn.innerHTML;
        btn.disabled = true;
        btn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>';
        setStatus(statusEl, 'muted', 'Sending test...');
        fetch('/api/notify/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        })
            .then(function (res) {
                return res
                    .json()
                    .catch(function () {
                        return { ok: res.ok, error: 'HTTP ' + res.status };
                    })
                    .then(function (body) {
                        return { status: res.status, body: body };
                    });
            })
            .then(function (result) {
                if (result.status === 200 && result.body.ok) {
                    setStatus(statusEl, 'success', 'Sent. Check the receiver.');
                } else if (result.status === 429) {
                    var wait = result.body.retry_after_sec || 0;
                    setStatus(statusEl, 'warn', 'Rate limited. Retry in ' + wait + 's.');
                } else {
                    setStatus(statusEl, 'error', result.body.error || 'Failed (HTTP ' + result.status + ').');
                }
            })
            .catch(function (err) {
                setStatus(statusEl, 'error', err && err.message ? err.message : 'Network error.');
            })
            .finally(function () {
                btn.disabled = false;
                btn.innerHTML = label;
            });
    };

    function init(container, channel, values) {
        var instance = new NotifyList(container, channel);
        instance.setValues(values || []);
        return instance;
    }

    window.BeaconNotify = {
        MAX_RECEIVERS: MAX_RECEIVERS,
        init: init,
    };
})();
