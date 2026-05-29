// Modal editor for per-receiver alert mode and templates (Beacon shell, not Bootstrap).
(function () {
    'use strict';

    var modalEl = null;
    var formRoot = null;
    var onSaveCb = null;
    var currentDelivery = null;
    var lastActiveElement = null;

    function ensureModal() {
        if (modalEl) return;
        modalEl = document.createElement('div');
        modalEl.className = 'beacon-modal';
        modalEl.id = 'receiverPolicyModal';
        modalEl.hidden = true;
        modalEl.setAttribute('role', 'dialog');
        modalEl.setAttribute('aria-modal', 'true');
        modalEl.setAttribute('aria-labelledby', 'receiverPolicyModalTitle');
        modalEl.innerHTML =
            '<button type="button" class="beacon-modal__backdrop" data-beacon-modal-close aria-label="Close dialog"></button>' +
            '<div class="beacon-modal__dialog" tabindex="-1">' +
                '<header class="beacon-modal__header">' +
                    '<h2 class="beacon-modal__title" id="receiverPolicyModalTitle">Receiver alert policy</h2>' +
                    '<button type="button" class="beacon-modal__close" data-beacon-modal-close aria-label="Close">' +
                        '<i class="bi bi-x-lg" aria-hidden="true"></i>' +
                    '</button>' +
                '</header>' +

                '<div class="beacon-modal__body" data-receiver-policy-form>' +
                    '<p class="beacon-modal__intro">' +
                        'Empty fields inherit global defaults from Settings → Notifications. ' +
                        'Use Test to preview the template on this receiver.' +
                    '</p>' +

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
                            '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-reset-all>' +
                                'Reset to built-in defaults' +
                            '</button>' +
                        '</div>' +

                        '<div class="p-2"><div class="col-12 policy-template-row">' +
                            '<div class="d-flex justify-content-between align-items-center mb-1 flex-wrap gap-1">' +
                                '<label class="form-label small mb-0">Down template</label>' +

                                '<div class="d-flex align-items-center gap-1">' +
                                    '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-test>' +
                                        'Test' +
                                    '</button>' +

                                    '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-reset>' +
                                        'Reset' +
                                    '</button>' +
                                '</div>' +
                            '</div>' +

                            '<textarea ' +
                                'class="form-control font-monospace small" ' +
                                'rows="5" ' +
                                'data-policy-template="down" ' +
                                'placeholder="Leave empty for global">' +
                            '</textarea>' +

                            '<div class="mt-1">' +
                                '<span data-policy-test-status class="small text-muted"></span>' +
                            '</div>' +

                            '<div class="mt-1" data-policy-chips></div>' +
                        '</div></div>' +

                        '<div class="p-2"><div class="col-12 policy-template-row">' +
                            '<div class="d-flex justify-content-between align-items-center mb-1 flex-wrap gap-1">' +
                                '<label class="form-label small mb-0">Recovered template</label>' +

                                '<div class="d-flex align-items-center gap-1">' +
                                    '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-test>' +
                                        'Test' +
                                    '</button>' +

                                    '<button type="button" class="btn btn-sm btn-outline-secondary" data-policy-reset>' +
                                        'Reset' +
                                    '</button>' +
                                '</div>' +
                            '</div>' +

                            '<textarea ' +
                                'class="form-control font-monospace small" ' +
                                'rows="5" ' +
                                'data-policy-template="recovered" ' +
                                'placeholder="Leave empty for global">' +
                            '</textarea>' +

                            '<div class="mt-1">' +
                                '<span data-policy-test-status class="small text-muted"></span>' +
                            '</div>' +

                            '<div class="mt-1" data-policy-chips></div>' +
                        '</div></div>' +
                    '</div>' +
                '</div>' +

                '<footer class="beacon-modal__footer">' +
                    '<button type="button" class="btn btn-outline-secondary" data-beacon-modal-close>' +
                        'Cancel' +
                    '</button>' +

                    '<button type="button" class="btn btn-primary" data-receiver-policy-save>' +
                        'Save' +
                    '</button>' +
                '</footer>' +
            '</div>';

        document.body.appendChild(modalEl);

        formRoot = modalEl.querySelector('[data-receiver-policy-form]');
        var dialog = modalEl.querySelector('.beacon-modal__dialog');

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

    function closeModal() {
        if (!modalEl) return;
        modalEl.hidden = true;
        modalEl.setAttribute('aria-hidden', 'true');
        document.body.classList.remove('beacon-modal-open');
        if (lastActiveElement && typeof lastActiveElement.focus === 'function') {
            lastActiveElement.focus();
        }
        lastActiveElement = null;
    }

    function openModal() {
        lastActiveElement = document.activeElement;
        modalEl.hidden = false;
        modalEl.setAttribute('aria-hidden', 'false');
        document.body.classList.add('beacon-modal-open');
        var dialog = modalEl.querySelector('.beacon-modal__dialog');
        if (dialog) {
            dialog.focus();
        }
    }

    function open(initial, delivery, onSave) {
        ensureModal();
        onSaveCb = onSave;
        currentDelivery = delivery || null;
        var policy = (window.Beacon && window.Beacon.policy) || window.BeaconNotifyPolicy;
        return policy.init(formRoot, initial || {}, {
            globalMode: false,
            delivery: currentDelivery,
        }).then(function (pf) {
            formRoot._policyForm = pf;
            openModal();
        });
    }

    var modalAPI = { open: open, close: closeModal };
    window.Beacon = window.Beacon || {};
    window.Beacon.policyModal = modalAPI;
    window.BeaconReceiverPolicyModal = modalAPI;
})();
