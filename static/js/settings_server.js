// Server and access — what is left of Settings once everything that belonged
// to another screen went to live there.
//
// It saves only its own sections. A patch that mentions listen, workers,
// default_interval and auth leaves channels, templates and peers untouched,
// which is what lets three screens edit one config without fighting.
(function () {
    'use strict';

    var form = document.getElementById('serverForm');
    if (!form) return;

    var F = window.Beacon.form;
    var msg = document.getElementById('serverMsg');

    F.load()
        .then(function (cfg) {
            F.field(form, 'listen').value = cfg.listen || ':8080';
            F.field(form, 'workers').value = cfg.workers || 10;
            F.field(form, 'default_interval').value = cfg.default_interval || '';
            F.field(form, 'username').value = (cfg.auth && cfg.auth.username) || '';

            var pw = F.field(form, 'password');
            pw.value = '';
            pw.placeholder = cfg.secrets && cfg.secrets.password
                ? 'Leave blank to keep current'
                : 'Set password';
        })
        .catch(function (err) {
            F.status(msg, 'error', String(err));
        });

    form.addEventListener('submit', function (e) {
        e.preventDefault();
        F.save(
            {
                listen: F.value(form, 'listen', ':8080'),
                workers: F.number(form, 'workers', 10),
                default_interval: F.number(form, 'default_interval', 0),
                auth: {
                    username: F.value(form, 'username', ''),
                    // Blank means "keep the current one": the server carries the
                    // stored password over rather than clearing it.
                    password: F.value(form, 'password', ''),
                },
            },
            msg
        ).then(function (saved) {
            if (!saved) return;
            F.field(form, 'password').value = '';
            var hint = document.getElementById('restartHint');
            if (hint && saved.requires_restart) {
                F.status(hint, 'error', 'Saved. Listen address or worker count changed — restart to apply.');
            }
        });
    });
})();
