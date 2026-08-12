// Shared plumbing for the configuration forms.
//
// Configuration used to be one screen and one form, so one script could gather
// every field and PUT the lot. It is now three forms on three screens — server
// and access here, channels with notifications, peers with the peer list — and
// each must save its OWN part without disturbing the others.
//
// That works because the API takes a partial patch: a body that mentions only
// {"telegram": …} leaves everything else exactly as it was. What is shared is
// the small stuff every one of them needs — reading a field, showing a secret
// without leaking it, reporting the outcome — and nothing else.
(function () {
    'use strict';

    function field(form, name) {
        return form.querySelector('[name="' + name + '"]');
    }

    function value(form, name, fallback) {
        var el = field(form, name);
        if (!el) return fallback;
        var v = (el.value || '').trim();
        return v === '' && fallback !== undefined ? fallback : v;
    }

    function number(form, name, fallback) {
        var n = parseInt(value(form, name, ''), 10);
        return isNaN(n) ? fallback : n;
    }

    function checked(form, name) {
        var el = field(form, name);
        return !!(el && el.checked);
    }

    // A stored secret arrives masked. Showing it as-is would put it on screen
    // and in the DOM; leaving the field blank would read as "not set".
    function applySecret(input, val) {
        if (!input) return;
        val = (val || '').trim();
        if (!val) {
            input.value = '';
            return;
        }
        if (window.Beacon.notify && window.Beacon.notify.attachSecretField) {
            window.Beacon.notify.attachSecretField(input, val);
        } else {
            input.value = val;
        }
    }

    // Fields under a switch are disabled while it is off: a form that lets you
    // type into a channel nobody will use invites the question "did it save?".
    function bindToggle(form, name) {
        var box = field(form, name);
        if (!box) return;
        var group = box.closest('.inst-panel');
        var apply = function () {
            if (!group) return;
            group.querySelectorAll('input, select, textarea, button').forEach(function (el) {
                if (el === box) return;
                el.disabled = !box.checked;
            });
        };
        box.addEventListener('change', apply);
        apply();
    }

    function status(el, tone, text) {
        if (!el) return;
        el.textContent = text;
        el.className = tone ? 'inst-field-' + tone : 'inst-field-hint';
    }

    // save PUTs a patch and reports. The caller decides which sections the
    // patch carries; anything it leaves out is left alone by the server.
    function save(patch, msgEl) {
        status(msgEl, '', 'Saving…');
        return window.Beacon
            .apiFetch('/api/config', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(patch),
            })
            .then(function (res) {
                if (!res.ok) {
                    return res.text().then(function (t) {
                        status(msgEl, 'error', t || 'HTTP ' + res.status);
                        return null;
                    });
                }
                return res.json().then(function (saved) {
                    status(msgEl, 'hint', 'Saved');
                    return saved;
                });
            })
            .catch(function (err) {
                status(msgEl, 'error', String(err));
                return null;
            });
    }

    function load() {
        return window.Beacon.apiFetch('/api/config').then(function (r) {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.json();
        });
    }

    window.Beacon = window.Beacon || {};
    window.Beacon.form = {
        field: field,
        value: value,
        number: number,
        checked: checked,
        applySecret: applySecret,
        bindToggle: bindToggle,
        status: status,
        save: save,
        load: load,
    };
})();
