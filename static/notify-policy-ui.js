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
        return null;
    }

    function placeholderChips(container, textarea) {
        if (!container || !textarea) return;
        container.innerHTML = '';
        var list = (defaultsCache && defaultsCache.placeholders) || [];
        list.forEach(function (p) {
            var btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'btn btn-sm btn-outline-secondary me-1 mb-1';
            btn.textContent = '{{' + p.key + '}}';
            btn.title = p.description || p.key;
            btn.addEventListener('click', function () {
                var tag = '{{' + p.key + '}}';
                var start = textarea.selectionStart;
                var end = textarea.selectionEnd;
                var val = textarea.value;
                textarea.value = val.slice(0, start) + tag + val.slice(end);
                textarea.focus();
                textarea.selectionStart = textarea.selectionEnd = start + tag.length;
            });
            container.appendChild(btn);
        });
    }

    function wireTemplateRow(row, defaults, opts) {
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
            if (opts.delivery) {
                testBtn.style.display = '';
            } else {
                testBtn.style.display = 'none';
            }
            testBtn.addEventListener('click', function () {
                var key = ta.getAttribute('data-policy-template');
                if (!key) return;
                var creds = deliveryCredentials(opts.delivery);
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
        if (modeSelect) {
            var mode = initial.alert_mode || (isGlobal ? def.alert_mode : '');
            modeSelect.value = mode || '';
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
        initial = initial || {};

        var modeSelect = root.querySelector('[data-policy-alert-mode]');
        var tplDown = root.querySelector('[data-policy-template="down"]');
        var tplRecovered = root.querySelector('[data-policy-template="recovered"]');
        var resetAll = root.querySelector('[data-policy-reset-all]');

        return fetchDefaults().then(function (def) {
            if (root.dataset.policyWired !== '1') {
                root.dataset.policyWired = '1';
                root.querySelectorAll('.policy-template-row').forEach(function (row) {
                    wireTemplateRow(row, def, opts);
                });
                if (resetAll) {
                    resetAll.addEventListener('click', function () {
                        if (modeSelect) modeSelect.value = def.alert_mode || 'repeat';
                        if (tplDown) tplDown.value = def.templates.down || '';
                        if (tplRecovered) tplRecovered.value = def.templates.recovered || '';
                        root.querySelectorAll('[data-policy-test-status]').forEach(function (el) {
                            setStatus(el, 'muted', '');
                        });
                    });
                }
            }
            applyPolicyValues(root, initial, opts, def);

            return {
                values: function () {
                    var out = {};
                    if (modeSelect) {
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

