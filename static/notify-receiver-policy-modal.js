// Modal editor for per-receiver alert mode and templates.
(function () {
    'use strict';

    var modalEl = null;
    var formRoot = null;
    var onSaveCb = null;
    var currentDelivery = null;

    function ensureModal() {
        if (modalEl) return;
        modalEl = document.createElement('div');
        modalEl.className = 'modal fade';
        modalEl.id = 'receiverPolicyModal';
        modalEl.tabIndex = -1;
        modalEl.setAttribute('aria-hidden', 'true');
        modalEl.innerHTML =
            '<div class="modal-dialog modal-lg modal-dialog-scrollable">' +
            '<div class="modal-content">' +
            '<div class="modal-header">' +
            '<h5 class="modal-title">Receiver alert policy</h5>' +
            '<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>' +
            '</div>' +
            '<div class="modal-body" data-receiver-policy-form>' +
            '<p class="small text-muted mb-3">Empty fields inherit global defaults from Settings → Notifications. Use Test to preview the template on this receiver.</p>' +
            '<div class="row g-3">' +
            '<div class="col-md-6">' +
            '<label class="form-label small">Alert mode</label>' +
            '<select class="form-select" data-policy-alert-mode>' +
            '<option value="">Use global default</option>' +
            '<option value="repeat">Repeat while down</option>' +
            '<option value="once">Once on down + recovery</option>' +
            '</select>' +
            '</div>' +
            '<div class="col-md-6 d-flex align-items-end justify-content-md-end">' +
            '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-reset-all>Reset to built-in defaults</button>' +
            '</div>' +
            '<div class="col-12 policy-template-row">' +
            '<div class="d-flex justify-content-between align-items-center mb-1 flex-wrap gap-1">' +
            '<label class="form-label small mb-0">Down template</label>' +
            '<div class="d-flex align-items-center gap-1">' +
            '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-test>Test</button>' +
            '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-reset>Reset</button>' +
            '</div>' +
            '</div>' +
            '<textarea class="form-control font-monospace small" rows="4" data-policy-template="down" placeholder="Leave empty for global"></textarea>' +
            '<div class="mt-1"><span data-policy-test-status class="small text-muted"></span></div>' +
            '<div class="mt-1" data-policy-chips></div>' +
            '</div>' +
            '<div class="col-12 policy-template-row">' +
            '<div class="d-flex justify-content-between align-items-center mb-1 flex-wrap gap-1">' +
            '<label class="form-label small mb-0">Recovered template</label>' +
            '<div class="d-flex align-items-center gap-1">' +
            '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-test>Test</button>' +
            '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-reset>Reset</button>' +
            '</div>' +
            '</div>' +
            '<textarea class="form-control font-monospace small" rows="4" data-policy-template="recovered" placeholder="Leave empty for global"></textarea>' +
            '<div class="mt-1"><span data-policy-test-status class="small text-muted"></span></div>' +
            '<div class="mt-1" data-policy-chips></div>' +
            '</div>' +
            '</div>' +
            '</div>' +
            '<div class="modal-footer">' +
            '<button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">Cancel</button>' +
            '<button type="button" class="btn btn-primary" data-receiver-policy-save>Save</button>' +
            '</div>' +
            '</div></div>';
        document.body.appendChild(modalEl);
        formRoot = modalEl.querySelector('[data-receiver-policy-form]');
        modalEl.querySelector('[data-receiver-policy-save]').addEventListener('click', function () {
            if (!onSaveCb || !formRoot._policyForm) return;
            onSaveCb(formRoot._policyForm.values());
            if (window.bootstrap && bootstrap.Modal) {
                bootstrap.Modal.getInstance(modalEl).hide();
            }
        });
    }

    function open(initial, delivery, onSave) {
        ensureModal();
        onSaveCb = onSave;
        currentDelivery = delivery || null;
        return window.BeaconNotifyPolicy.init(formRoot, initial || {}, {
            globalMode: false,
            delivery: currentDelivery,
        }).then(function (pf) {
            formRoot._policyForm = pf;
            if (window.bootstrap && bootstrap.Modal) {
                bootstrap.Modal.getOrCreateInstance(modalEl).show();
            }
        });
    }

    window.BeaconReceiverPolicyModal = { open: open };
})();
