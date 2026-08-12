// Peer sync configuration on the peers screen.
//
// The panel used to be the last one in Settings, under four alert channels.
// It belongs next to the node list it configures: turning sync on and seeing
// who answered are one task, not two.
(function () {
    'use strict';

    var form = document.getElementById('networkForm');
    if (!form) return;

    var F = window.Beacon.form;
    var msg = document.getElementById('networkMsg');

    F.load()
        .then(function (cfg) {
            var n = cfg.network || {};
            F.field(form, 'network_enabled').checked = !!n.enabled;
            F.field(form, 'network_self_url').value = n.self_url || '';
            F.field(form, 'network_peers').value = (n.peers || []).join('\n');
            F.field(form, 'network_sync_interval').value = n.sync_interval || '';
            F.field(form, 'network_dead_timeout').value = n.dead_timeout || '';
            F.field(form, 'network_node_id').value = n.node_id || '';

            F.applySecret(F.field(form, 'network_sync_token'), n.sync_token || '');
            if (!n.sync_token) {
                F.field(form, 'network_sync_token').placeholder =
                    cfg.secrets && cfg.secrets.sync_token
                        ? 'Leave blank to keep current'
                        : 'Set shared sync token';
            }
            F.bindToggle(form, 'network_enabled');
        })
        .catch(function (err) {
            F.status(msg, 'error', String(err));
        });

    form.addEventListener('submit', function (e) {
        e.preventDefault();

        var peers = (F.field(form, 'network_peers').value || '')
            .split('\n')
            .map(function (s) {
                return s.trim();
            })
            .filter(Boolean);

        F.save(
            {
                network: {
                    enabled: F.checked(form, 'network_enabled'),
                    self_url: F.value(form, 'network_self_url', ''),
                    peers: peers,
                    sync_interval: F.number(form, 'network_sync_interval', 60),
                    dead_timeout: F.number(form, 'network_dead_timeout', 300),
                    node_id: F.value(form, 'network_node_id', ''),
                    sync_token: F.field(form, 'network_sync_token').value,
                },
            },
            msg
        ).then(function (saved) {
            if (!saved) return;
            // The node id is generated on first save; show what was assigned.
            if (saved.network && saved.network.node_id) {
                F.field(form, 'network_node_id').value = saved.network.node_id;
            }
        });
    });
})();
