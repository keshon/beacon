// Beacon notification UI (policy, modal, receivers).
// Alert mode and message templates (global settings + receiver policy modal).
(function () {
    'use strict';

    var defaultsCache = null;

    function fetchDefaults() {
        if (defaultsCache) {
            return Promise.resolve(defaultsCache);
        }
        return window.Beacon.apiFetch('/api/notify/defaults')
            .then(function (r) {
                if (!r.ok) {
                    throw new Error('HTTP ' + r.status);
                }
                return r.json();
            })
            .then(function (data) {
                defaultsCache = data;
                return data;
            });
    }

    function globalDefaults() {
        var root = window.Beacon && window.Beacon.notify;
        return (root && root.globalDefaults) || { alert_mode: 'repeat', templates: {} };
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

    function postNotifyTest(payload) {
        return window.Beacon.apiFetch('/api/notify/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        }).then(function (res) {
            return res
                .json()
                .catch(function () {
                    return { ok: res.ok, error: 'HTTP ' + res.status };
                })
                .then(function (body) {
                    return { status: res.status, body: body };
                });
        });
    }

    function resolveTemplateText(key, fieldValue, defaults) {
        var v = (fieldValue || '').trim();
        if (v) return v;
        var global = globalDefaults().templates || {};
        if (global[key] && String(global[key]).trim()) {
            return String(global[key]).trim();
        }
        if (defaults && defaults.templates && defaults.templates[key]) {
            return String(defaults.templates[key]).trim();
        }
        return '';
    }

    function deliveryCredentials(delivery) {
        if (!delivery) return null;
        if (delivery.channel === 'telegram' && delivery.telegram) {
            var token = (delivery.telegram.token || '').trim();
            var chat = (delivery.telegram.chat_id || '').trim();
            if (!token || !chat) return null;
            return { channel: 'telegram', telegram: { token: token, chat_id: chat } };
        }
        if (delivery.channel === 'discord' && delivery.discord) {
            var webhook = (delivery.discord.webhook || '').trim();
            if (!webhook) return null;
            return { channel: 'discord', discord: { webhook: webhook } };
        }
        if (delivery.channel === 'email' && delivery.email) {
            var to = (delivery.email.to || '').trim();
            if (!to) return null;
            return { channel: 'email', email: { to: to } };
        }
        if (delivery.channel === 'webhook' && delivery.webhook) {
            var url = (delivery.webhook.url || '').trim();
            if (!url) return null;
            return { channel: 'webhook', webhook: { url: url } };
        }
        return null;
    }

    function resolveDelivery(opts) {
        if (!opts) return null;
        if (typeof opts.getDelivery === 'function') {
            return opts.getDelivery();
        }
        return opts.delivery || null;
    }

    function syncPolicyTestButtons(policyRoot) {
        if (!policyRoot) return;
        var opts = policyRoot._policyOpts || {};
        var hasCreds = !!deliveryCredentials(resolveDelivery(opts));
        policyRoot.querySelectorAll('[data-policy-test]').forEach(function (btn) {
            btn.style.display = hasCreds ? '' : 'none';
        });
    }

    // Inserts are a kit component: placing the value at the caret, returning
    // focus and emitting the event are kit.js's job. Only the key list is ours.
    function placeholderChips(container, textarea) {
        if (!container || !textarea) return;
        container.innerHTML = '';
        var list = (defaultsCache && defaultsCache.placeholders) || [];
        list.forEach(function (p) {
            var btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'inst-insert';
            btn.textContent = '{{' + p.key + '}}';
            btn.title = p.description || p.key;
            container.appendChild(btn);
        });
    }

    function wireTemplateRow(row, defaults, policyRoot) {
        var ta = row.querySelector('[data-policy-template]');
        var chips = row.querySelector('[data-policy-chips]');
        var resetBtn = row.querySelector('[data-policy-reset]');
        var testBtn = row.querySelector('[data-policy-test]');
        var testStatus = row.querySelector('[data-policy-test-status]');
        if (!ta) return;
        placeholderChips(chips, ta);
        if (resetBtn) {
            resetBtn.addEventListener('click', function () {
                var key = ta.getAttribute('data-policy-template');
                if (defaults && defaults.templates && key) {
                    ta.value = defaults.templates[key] || '';
                }
                setStatus(testStatus, 'muted', '');
            });
        }
        if (testBtn) {
            testBtn.addEventListener('click', function () {
                var key = ta.getAttribute('data-policy-template');
                if (!key) return;
                var opts = (policyRoot && policyRoot._policyOpts) || {};
                var creds = deliveryCredentials(resolveDelivery(opts));
                if (!creds) {
                    setStatus(testStatus, 'warn', 'Fill receiver fields first.');
                    return;
                }
                var template = resolveTemplateText(key, ta.value, defaults);
                if (!template) {
                    setStatus(testStatus, 'warn', 'No template to send. Enter text or use Reset.');
                    return;
                }
                var payload = {
                    channel: creds.channel,
                    status: key,
                    template: template,
                };
                if (creds.telegram) payload.telegram = creds.telegram;
                if (creds.discord) payload.discord = creds.discord;
                if (creds.email) payload.email = creds.email;
                if (creds.webhook) payload.webhook = creds.webhook;
                testBtn.disabled = true;
                setStatus(testStatus, 'muted', 'Sending…');
                postNotifyTest(payload)
                    .then(function (result) {
                        if (result.status === 200 && result.body.ok) {
                            setStatus(testStatus, 'success', 'Sent. Check the receiver.');
                        } else if (result.status === 429) {
                            var wait = result.body.retry_after_sec || 0;
                            setStatus(testStatus, 'warn', 'Rate limited. Retry in ' + wait + 's.');
                        } else {
                            setStatus(
                                testStatus,
                                'error',
                                result.body.error || 'Failed (HTTP ' + result.status + ').'
                            );
                        }
                    })
                    .catch(function (err) {
                        setStatus(testStatus, 'error', err && err.message ? err.message : 'Network error.');
                    })
                    .finally(function () {
                        testBtn.disabled = false;
                    });
            });
        }
    }

    function templateValue(initial, key, isGlobal, def) {
        var v = initial.templates && initial.templates[key];
        if (v && String(v).trim()) {
            return String(v).trim();
        }
        if (isGlobal && def && def.templates) {
            return def.templates[key] || '';
        }
        return '';
    }

    /**
     * @param {HTMLElement} root container with data-notify-policy
     * @param {object} initial { alert_mode, templates }
     * @param {object} opts { globalMode, delivery }
     */
    function applyPolicyValues(root, initial, opts, def) {
        var isGlobal = opts.globalMode !== false;
        initial = initial || {};
        var modeSelect = root.querySelector('[data-policy-alert-mode]');
        var tplDown = root.querySelector('[data-policy-template="down"]');
        var tplRecovered = root.querySelector('[data-policy-template="recovered"]');
        if (modeSelect && opts.channel !== 'email') {
            var mode = initial.alert_mode || (isGlobal ? def.alert_mode : '');
            modeSelect.value = mode || '';
        } else if (modeSelect && opts.channel === 'email') {
            modeSelect.value = '';
        }
        if (tplDown) tplDown.value = templateValue(initial, 'down', isGlobal, def);
        if (tplRecovered) tplRecovered.value = templateValue(initial, 'recovered', isGlobal, def);
        root.querySelectorAll('[data-policy-test-status]').forEach(function (el) {
            setStatus(el, 'muted', '');
        });
    }

    function initPolicyForm(root, initial, opts) {
        opts = opts || {};
        var isGlobal = opts.globalMode !== false;
        var hideAlertMode = opts.channel === 'email';
        initial = initial || {};

        var modeSelect = root.querySelector('[data-policy-alert-mode]');
        var modeRow =
            modeSelect &&
            (modeSelect.closest('[data-policy-alert-mode-row]') ||
                modeSelect.closest('[data-policy-alert-mode-row]'));
        if (modeRow) {
            modeRow.classList.toggle('invisible', hideAlertMode);
            modeRow.setAttribute('aria-hidden', hideAlertMode ? 'true' : 'false');
        }
        if (modeSelect) modeSelect.disabled = hideAlertMode;
        var tplDown = root.querySelector('[data-policy-template="down"]');
        var tplRecovered = root.querySelector('[data-policy-template="recovered"]');
        var resetAll = root.querySelector('[data-policy-reset-all]');

        return fetchDefaults().then(function (def) {
            root._policyOpts = opts;
            if (root.dataset.policyWired !== '1') {
                root.dataset.policyWired = '1';
                root.querySelectorAll('.policy-template-row').forEach(function (row) {
                    wireTemplateRow(row, def, root);
                });
                if (resetAll) {
                    resetAll.addEventListener('click', function () {
                        if (modeSelect && opts.channel !== 'email') {
                            modeSelect.value = def.alert_mode || 'repeat';
                        }
                        if (tplDown) tplDown.value = def.templates.down || '';
                        if (tplRecovered) tplRecovered.value = def.templates.recovered || '';
                        root.querySelectorAll('[data-policy-test-status]').forEach(function (el) {
                            setStatus(el, 'muted', '');
                        });
                    });
                }
            }
            applyPolicyValues(root, initial, opts, def);
            syncPolicyTestButtons(root);

            return {
                values: function () {
                    var out = {};
                    if (modeSelect && opts.channel !== 'email') {
                        var m = (modeSelect.value || '').trim();
                        if (m) out.alert_mode = m;
                    }
                    var templates = {};
                    if (tplDown && tplDown.value.trim()) templates.down = tplDown.value.trim();
                    if (tplRecovered && tplRecovered.value.trim())
                        templates.recovered = tplRecovered.value.trim();
                    if (Object.keys(templates).length) out.templates = templates;
                    return out;
                },
            };
        });
    }

    var policyAPI = {
        init: initPolicyForm,
        fetchDefaults: fetchDefaults,
        postNotifyTest: postNotifyTest,
    };
    window.Beacon = window.Beacon || {};
    window.Beacon.policy = policyAPI;
})();


// Modal editor for per-receiver alert mode and templates (Beacon shell, not Bootstrap).
(function () {
    'use strict';

    var modalEl = null;
    var formRoot = null;
    var onSaveCb = null;
    var lastActiveElement = null;

    function ensureModal() {
        if (modalEl) return;
        // Native <dialog>: backdrop, Escape, an inert background and focus
        // return come from the platform.
        modalEl = document.createElement('dialog');
        modalEl.className = 'inst-dialog';
        modalEl.id = 'receiverPolicyModal';
        modalEl.setAttribute('aria-labelledby', 'receiverPolicyModalTitle');
        modalEl.innerHTML =
            '<div class="inst-dialog-head">' +
                '<h2 class="inst-dialog-title" id="receiverPolicyModalTitle">Receiver alert policy</h2>' +
                '<button type="button" class="inst-dialog-close inst-btn inst-btn--sm inst-btn--icon inst-btn--ghost"' +
                ' data-beacon-modal-close aria-label="Close">' +
                '<svg class="inst-icon" aria-hidden="true"><use href="#i-close"/></svg></button>' +
            '</div>' +

                '<div class="inst-dialog-body inst-form" data-receiver-policy-form>' +
                    '<p class="inst-u-dim">' +
                        'Empty fields inherit global defaults from Settings → Notifications. ' +
                        'Use Test to preview the template on this receiver.' +
                    '</p>' +

                    '<div class="row g-3">' +
                        '<div class="inst-field" data-policy-alert-mode-row>' +
                            '<label class="inst-label">Alert mode</label>' +
                            '<span class="inst-select-wrap">' +
                            '<select class="inst-select" data-policy-alert-mode>' +
                                '<option value="">Use global default</option>' +
                                '<option value="repeat">Repeat while down</option>' +
                                '<option value="once">Once on down + recovery</option>' +
                            '</select></span>' +
                        '</div>' +

                        '<div class="inst-form-actions inst-form-actions--end">' +
                            '<button type="button" class="inst-btn inst-btn--sm" data-policy-reset-all>' +
                                'Reset to built-in defaults' +
                            '</button>' +
                        '</div>' +

                        '<div class="inst-field policy-template-row">' +
                            '<div class="beacon-field-action">' +
                                '<label class="inst-label">Down template</label>' +
                                '<span class="inst-cluster inst-cluster--tight">' +
                                    '<button type="button" class="inst-btn inst-btn--sm" data-policy-test>Test</button>' +
                                    '<button type="button" class="inst-btn inst-btn--sm" data-policy-reset>Reset</button>' +
                                '</span>' +
                            '</div>' +
                            '<textarea class="inst-textarea inst-u-mono" rows="5"' +
                                ' data-policy-template="down" placeholder="Leave empty for global"></textarea>' +
                            '<span data-policy-test-status class="inst-field-hint"></span>' +
                            '<div class="inst-inserts" data-policy-chips></div>' +
                        '</div>' +

                        '<div class="inst-field policy-template-row">' +
                            '<div class="beacon-field-action">' +
                                '<label class="inst-label">Recovered template</label>' +
                                '<span class="inst-cluster inst-cluster--tight">' +
                                    '<button type="button" class="inst-btn inst-btn--sm" data-policy-test>Test</button>' +
                                    '<button type="button" class="inst-btn inst-btn--sm" data-policy-reset>Reset</button>' +
                                '</span>' +
                            '</div>' +
                            '<textarea class="inst-textarea inst-u-mono" rows="5"' +
                                ' data-policy-template="recovered" placeholder="Leave empty for global"></textarea>' +
                            '<span data-policy-test-status class="inst-field-hint"></span>' +
                            '<div class="inst-inserts" data-policy-chips></div>' +
                        '</div>' +
                    '</div>' +
                '</div>' +
                '</div>' +

                '<div class="inst-dialog-foot inst-dialog-foot--end">' +
                    '<button type="button" class="inst-btn" data-beacon-modal-close>' +
                        'Cancel' +
                    '</button>' +

                    '<button type="button" class="inst-btn inst-btn--primary" data-receiver-policy-save>' +
                        'Save' +
                    '</button>' +
                '</div>';

        document.body.appendChild(modalEl);

        formRoot = modalEl.querySelector('[data-receiver-policy-form]');

        modalEl.querySelector('[data-receiver-policy-save]').addEventListener('click', function () {
            if (!onSaveCb || !formRoot._policyForm) return;
            onSaveCb(formRoot._policyForm.values());
            closeModal();
        });

        modalEl.addEventListener('click', function (e) {
            if (e.target.closest('[data-beacon-modal-close]')) {
                closeModal();
            }
        });

        document.addEventListener('keydown', function (e) {
            if (!modalEl || modalEl.hidden) return;
            if (e.key === 'Escape') {
                e.preventDefault();
                closeModal();
            }
        });
    }

    // Focus return, the inert background and Escape come from the platform.
    function closeModal() {
        if (modalEl && modalEl.open) modalEl.close();
    }

    function openModal() {
        if (modalEl && !modalEl.open) modalEl.showModal();
    }

    function open(initial, delivery, onSave, opts) {
        ensureModal();
        onSaveCb = onSave;
        opts = opts || {};
        var channel = opts.channel || (delivery && delivery.channel) || '';
        initial = initial || {};
        if (channel === 'email') {
            initial = Object.assign({}, initial);
            delete initial.alert_mode;
        }
        var intro = formRoot && formRoot.querySelector('.inst-u-dim');
        if (intro) {
            if (channel === 'email') {
                intro.textContent =
                    'Email alerts are always sent once on down and once on recovery. ' +
                    'Customize templates below; empty fields inherit global defaults from Settings → Notifications.';
            } else {
                intro.textContent =
                    'Empty fields inherit global defaults from Settings → Notifications. ' +
                    'Use Test to preview the template on this receiver.';
            }
        }
        var policy = window.Beacon && window.Beacon.policy;
        var policyOpts = {
            globalMode: false,
            channel: channel,
        };
        if (typeof opts.getDelivery === 'function') {
            policyOpts.getDelivery = opts.getDelivery;
        } else if (delivery) {
            policyOpts.getDelivery = function () {
                return delivery;
            };
        }
        return policy.init(formRoot, initial, policyOpts).then(function (pf) {
            formRoot._policyForm = pf;
            openModal();
        });
    }

    var modalAPI = { open: open, close: closeModal };
    window.Beacon = window.Beacon || {};
    window.Beacon.policyModal = modalAPI;
})();

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
            // Kit icon buttons: an accessible name is required, the shape is square.
            '<button type="button" class="inst-btn inst-btn--sm inst-btn--icon inst-btn--ghost"' +
            ' data-notify-action="policy" aria-label="Alert policy" title="Alert policy">' +
            '<svg class="inst-icon" aria-hidden="true"><use href="#i-settings"/></svg></button>' +
            '<button type="button" class="inst-btn inst-btn--sm inst-btn--icon inst-btn--ghost"' +
            ' data-notify-action="remove" aria-label="Remove receiver" title="Remove receiver">' +
            '<svg class="inst-icon" aria-hidden="true"><use href="#i-close"/></svg></button>' +
            '</div>'
        );
    }

    function attachSecretField(input, displayValue) {
        if (!input) return;
        displayValue = (displayValue || '').trim();
        if (!displayValue) return;

        input.classList.add('notify-secret-field');
        input.type = 'password';
        input.autocomplete = 'off';
        input.value = displayValue;
        input.dataset.secretDisplay = displayValue;
        input.title = 'Saved — hover to preview, click to pin reveal';

        var pinned = false;

        function setRevealed(on) {
            input.type = on ? 'text' : 'password';
            input.classList.toggle('is-revealed', on);
        }

        input.addEventListener('mouseenter', function () {
            if (!pinned) setRevealed(true);
        });
        input.addEventListener('mouseleave', function () {
            if (!pinned) setRevealed(false);
        });
        input.addEventListener('click', function () {
            pinned = !pinned;
            setRevealed(pinned);
        });
        input.addEventListener('focus', function () {
            if (input.value === input.dataset.secretDisplay) {
                input.select();
            }
        });
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
                        '<input type="password" class="inst-input notify-secret-field" data-notify-field="token" placeholder="Bot token" autocomplete="off" />' +
                        '<input type="text" class="inst-input" data-notify-field="chat_id" placeholder="Chat ID" />' +
                        '</div>' +
                        rowActionsHtml() +
                        '</div>'
                );
                var tokenInput = row.querySelector('[data-notify-field="token"]');
                if (value.token) {
                    attachSecretField(tokenInput, value.token);
                } else {
                    tokenInput.value = '';
                }
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
                        '<input type="password" class="inst-input notify-secret-field" data-notify-field="webhook" placeholder="Webhook URL" autocomplete="off" />' +
                        '</div>' +
                        rowActionsHtml() +
                        '</div>'
                );
                var webhookInput = row.querySelector('[data-notify-field="webhook"]');
                if (value.webhook) {
                    attachSecretField(webhookInput, value.webhook);
                } else {
                    webhookInput.value = '';
                }
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
                        '<input type="email" class="inst-input" data-notify-field="to" placeholder="recipient@example.com" />' +
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
                        '<input type="url" class="inst-input" data-notify-field="url" placeholder="https://hooks.example.com/..." />' +
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
        attachSecretField: attachSecretField,
    });
})();
