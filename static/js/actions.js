// The reverse channel, in the interface.
//
// Every action is attached to the place where the question is asked: check now
// sits on the row that says a monitor is down, mute and acknowledge sit on the
// outage. A control that answers a question two screens away from where it was
// raised is a control nobody finds in a hurry.
//
// Nothing here is optimistic. The button says what it is doing, then what
// happened; it never claims a result the server has not confirmed. On a screen
// whose whole job is telling the truth about a system, a hopeful spinner would
// be the one lie.
(function () {
    'use strict';

    function busy(btn, text) {
        btn.dataset.idle = btn.dataset.idle || btn.textContent;
        btn.disabled = true;
        btn.textContent = text;
    }

    function done(btn, text, reload) {
        btn.textContent = text;
        if (reload) {
            // The page is rendered from the store, so the honest way to show a
            // new fact is to fetch it, not to patch the DOM into agreeing.
            setTimeout(function () {
                window.location.reload();
            }, 600);
            return;
        }
        setTimeout(function () {
            btn.textContent = btn.dataset.idle;
            btn.disabled = false;
        }, 1600);
    }

    function post(url, body) {
        return window.Beacon.apiFetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body || {}),
        });
    }

    document.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-action]');
        if (!btn) return;
        var id = btn.getAttribute('data-monitor');
        if (!id) return;

        e.preventDefault();
        var action = btn.getAttribute('data-action');

        if (action === 'check') {
            busy(btn, 'Checking…');
            post('/api/monitors/' + id + '/check')
                .then(function (r) {
                    return r.ok ? r.json() : null;
                })
                .then(function (res) {
                    if (!res) return done(btn, 'Busy — try again');
                    // A check that is already running is not a failure, and the
                    // wording should not imply one.
                    done(btn, res.queued ? 'Queued' : 'Already running', res.queued);
                })
                .catch(function () {
                    done(btn, 'Failed');
                });
            return;
        }

        if (action === 'mute') {
            var minutes = parseInt(btn.getAttribute('data-minutes') || '60', 10);
            busy(btn, 'Muting…');
            post('/api/monitors/' + id + '/mute', { minutes: minutes, note: '' })
                .then(function (r) {
                    done(btn, r.ok ? 'Muted' : 'Failed', r.ok);
                })
                .catch(function () {
                    done(btn, 'Failed');
                });
            return;
        }

        if (action === 'unmute') {
            busy(btn, 'Unmuting…');
            post('/api/monitors/' + id + '/mute', { minutes: 0 })
                .then(function (r) {
                    done(btn, r.ok ? 'Unmuted' : 'Failed', r.ok);
                })
                .catch(function () {
                    done(btn, 'Failed');
                });
            return;
        }

        if (action === 'ack') {
            // The note is the point of acknowledging: "seen" without a reason
            // helps nobody, least of all the person who reads it at 4am.
            var note = window.prompt('What is known about this outage?', '');
            if (note === null) return;
            busy(btn, 'Saving…');
            post('/api/monitors/' + id + '/ack', { note: note })
                .then(function (r) {
                    done(btn, r.ok ? 'Acknowledged' : 'Failed', r.ok);
                })
                .catch(function () {
                    done(btn, 'Failed');
                });
        }
    });
})();
