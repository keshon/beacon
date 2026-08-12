// Channel configuration on the notifications screen.
//
// The four channels and the global policy used to live in Settings, three
// panels below the listen address. They belong here: the screen that shows
// whether alerts went out is the screen where you fix the channel that did not
// send them, and the trip through a second page in between was pure friction.
//
// The receiver editors themselves are unchanged — notify.js already owned
// them. What changed is where they are mounted and what the save sends: only
// the channel sections, so the server settings on the other screen are never
// in the patch and therefore never at risk.
(function () {
    'use strict';

    var form = document.getElementById('channelsForm');
    if (!form) return;

    var F = window.Beacon.form;
    var notify = window.Beacon.notify;
    var msg = document.getElementById('channelsMsg');

    var lists = {
        telegram: notify.init(document.getElementById('telegramPanel'), 'telegram', []),
        discord: notify.init(document.getElementById('discordPanel'), 'discord', []),
        email: notify.init(document.getElementById('emailPanel'), 'email', []),
        webhook: notify.init(document.getElementById('webhookPanel'), 'webhook', []),
    };
    var policyPanel = document.getElementById('notificationsPolicyPanel');

    // Other scripts on this page reach the editors through here — the monitor
    // dialog does the same with its own instances.
    window.Beacon.settings = {
        telegramList: lists.telegram,
        discordList: lists.discord,
        emailList: lists.email,
        webhookList: lists.webhook,
    };

    F.load()
        .then(function (cfg) {
            ['telegram', 'discord', 'email', 'webhook'].forEach(function (name) {
                var box = F.field(form, name + '_enabled');
                if (box) box.checked = !!(cfg[name] && cfg[name].enabled);
            });

            var smtp = (cfg.email && cfg.email.smtp) || {};
            F.field(form, 'email_smtp_host').value = smtp.host || '';
            F.field(form, 'email_smtp_port').value = smtp.port || '';
            F.field(form, 'email_smtp_username').value = smtp.username || '';
            F.field(form, 'email_smtp_from').value = smtp.from || '';
            F.field(form, 'email_smtp_tls').value = smtp.tls || 'starttls';
            F.applySecret(F.field(form, 'email_smtp_password'), smtp.password || '');
            if (!smtp.password) {
                F.field(form, 'email_smtp_password').placeholder =
                    cfg.secrets && cfg.secrets.email_smtp ? 'Leave blank to keep current' : 'Set password';
            }

            lists.telegram.setValues((cfg.telegram && cfg.telegram.targets) || []);
            lists.discord.setValues(
                ((cfg.discord && cfg.discord.webhooks) || []).map(function (w) {
                    return typeof w === 'string' ? { webhook: w } : w;
                })
            );
            lists.email.setValues((cfg.email && cfg.email.targets) || []);
            lists.webhook.setValues((cfg.webhook && cfg.webhook.webhooks) || []);

            var notifications = cfg.notifications || {};
            notify.setGlobalDefaults(notifications);
            window.Beacon.policy.init(policyPanel, notifications, { globalMode: true });

            ['telegram', 'discord', 'email', 'webhook'].forEach(function (name) {
                F.bindToggle(form, name + '_enabled');
            });
        })
        .catch(function (err) {
            F.status(msg, 'error', String(err));
        });

    form.addEventListener('submit', function (e) {
        e.preventDefault();

        var notifications = {
            alert_mode: (policyPanel.querySelector('[data-policy-alert-mode]').value || 'repeat').trim(),
            templates: {
                down: (policyPanel.querySelector('[data-policy-template="down"]').value || '').trim(),
                recovered: (policyPanel.querySelector('[data-policy-template="recovered"]').value || '').trim(),
            },
        };
        notify.setGlobalDefaults(notifications);

        F.save(
            {
                notifications: notifications,
                telegram: { enabled: F.checked(form, 'telegram_enabled'), targets: lists.telegram.values() },
                discord: { enabled: F.checked(form, 'discord_enabled'), webhooks: lists.discord.values() },
                email: {
                    enabled: F.checked(form, 'email_enabled'),
                    smtp: {
                        host: F.value(form, 'email_smtp_host', ''),
                        port: F.number(form, 'email_smtp_port', 587),
                        username: F.value(form, 'email_smtp_username', ''),
                        password: F.field(form, 'email_smtp_password').value,
                        from: F.value(form, 'email_smtp_from', ''),
                        tls: F.value(form, 'email_smtp_tls', 'starttls'),
                    },
                    targets: lists.email.values(),
                },
                webhook: { enabled: F.checked(form, 'webhook_enabled'), webhooks: lists.webhook.values() },
            },
            msg
        ).then(function (saved) {
            if (!saved) return;
            // Play back what was stored: the server sanitises and may have
            // dropped or trimmed a receiver, and the screen should show what is
            // actually saved rather than what was typed.
            lists.telegram.setValues((saved.telegram && saved.telegram.targets) || []);
            lists.discord.setValues(
                ((saved.discord && saved.discord.webhooks) || []).map(function (w) {
                    return typeof w === 'string' ? { webhook: w } : w;
                })
            );
            lists.email.setValues((saved.email && saved.email.targets) || []);
            lists.webhook.setValues((saved.webhook && saved.webhook.webhooks) || []);
        });
    });
})();
