// Our Lady of the Valley Parish — client-side glue.
// Phase 1: mobile nav, contact form submit, admin login, logout, message
// list actions. The drag-and-drop editor arrives in Phase 4.

(function () {
    document.addEventListener('DOMContentLoaded', init);

    function init() {
        wireMobileMenu();
        wireContactForm();
        wireLoginForm();
        wireLogout();
        wireMessageList();
        wireRegistrationForm();
        wireRegistrationList();
        wireEnvelopeFieldToggle();
    }

    // ---------- Mobile menu ----------
    function wireMobileMenu() {
        const btn = document.getElementById('mobile-menu-btn');
        const menu = document.getElementById('mobile-menu');
        if (!btn || !menu) return;
        btn.addEventListener('click', () => {
            const open = !menu.hasAttribute('hidden');
            if (open) {
                menu.setAttribute('hidden', '');
                menu.classList.remove('is-open');
                btn.setAttribute('aria-expanded', 'false');
            } else {
                menu.removeAttribute('hidden');
                menu.classList.add('is-open');
                btn.setAttribute('aria-expanded', 'true');
            }
        });
    }

    // ---------- Contact form ----------
    function wireContactForm() {
        const form = document.getElementById('contact-form');
        if (!form) return;
        const errorBox = document.getElementById('contact-error');
        const successBox = document.getElementById('contact-success');

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errorBox);
            hide(successBox);

            const payload = {
                first_name: value('contact-first-name'),
                last_name: value('contact-last-name'),
                email: value('contact-email'),
                phone: value('contact-phone'),
                subject: value('contact-subject'),
                message: value('contact-message'),
                website: value('contact-website'),
            };

            try {
                const res = await fetch('/api/v1/contact', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
                const data = await safeJson(res);
                if (!res.ok) {
                    showError(errorBox, data && data.error ? data.error : 'Something went wrong. Please try again.');
                    return;
                }
                form.reset();
                show(successBox);
            } catch (err) {
                showError(errorBox, 'Network error. Please try again.');
            }
        });
    }

    // ---------- Admin login ----------
    function wireLoginForm() {
        const form = document.getElementById('login-form');
        if (!form) return;
        const errorBox = document.getElementById('login-error');

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errorBox);

            const payload = {
                email: value('login-email'),
                password: value('login-password'),
            };

            try {
                const res = await fetch('/api/v1/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
                const data = await safeJson(res);
                if (!res.ok) {
                    showError(errorBox, data && data.error ? data.error : 'Sign in failed.');
                    return;
                }
                // Cookie is set server-side; now hit /admin which the middleware
                // will accept (or bounce back to login with a flash if not admin).
                window.location.assign('/admin');
            } catch (err) {
                showError(errorBox, 'Network error. Please try again.');
            }
        });
    }

    // ---------- Logout ----------
    function wireLogout() {
        document.querySelectorAll('#nav-logout').forEach((btn) => {
            btn.addEventListener('click', async (e) => {
                e.preventDefault();
                try {
                    await fetch('/api/v1/auth/logout', { method: 'POST' });
                } catch (_) {
                    /* ignore */
                }
                window.location.assign('/');
            });
        });
    }

    // ---------- Admin messages list ----------
    function wireMessageList() {
        const list = document.querySelector('.parish-admin-message-list');
        if (!list) return;

        list.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action]');
            if (!btn) return;
            const li = btn.closest('[data-message-id]');
            if (!li) return;
            const id = li.getAttribute('data-message-id');
            const action = btn.getAttribute('data-action');

            if (action === 'toggle-read') {
                const read = btn.getAttribute('data-read') === 'true';
                try {
                    const res = await fetch(`/api/v1/admin/messages/${id}/read`, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ read }),
                    });
                    if (res.ok) window.location.reload();
                } catch (_) { /* ignore */ }
            } else if (action === 'delete') {
                if (!confirm('Delete this message? It can be recovered from the database.')) return;
                try {
                    const res = await fetch(`/api/v1/admin/messages/${id}`, { method: 'DELETE' });
                    if (res.ok) li.remove();
                } catch (_) { /* ignore */ }
            }
        });
    }

    // ---------- Parish Registration form ----------
    function wireRegistrationForm() {
        const form = document.getElementById('registration-form');
        if (!form) return;
        const errorBox = document.getElementById('reg-error');
        const successBox = document.getElementById('reg-success');

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errorBox);
            hide(successBox);

            const prefs = Array.from(form.querySelectorAll('input[name="contact_prefs"]:checked')).map(c => c.value);
            const payload = {
                intent: value('reg-intent'),
                head_first_name: value('reg-head-first'),
                head_last_name: value('reg-head-last'),
                head_email: value('reg-head-email'),
                head_phone: value('reg-head-phone'),
                spouse_first_name: value('reg-spouse-first'),
                spouse_last_name: value('reg-spouse-last'),
                spouse_email: value('reg-spouse-email'),
                spouse_phone: value('reg-spouse-phone'),
                address1: value('reg-address1'),
                address2: value('reg-address2'),
                city: value('reg-city'),
                state: value('reg-state'),
                postal_code: value('reg-zip'),
                household_members: value('reg-members'),
                primary_church: value('reg-primary-church'),
                wants_envelopes: document.getElementById('reg-envelopes')?.checked || false,
                envelope_number: value('reg-envelope-number'),
                contact_preferences: prefs,
                notes: value('reg-notes'),
                website: value('reg-website'),
            };

            try {
                const res = await fetch('/api/v1/parish-registration', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
                const data = await safeJson(res);
                if (!res.ok) {
                    showError(errorBox, data && data.error ? data.error : 'Something went wrong. Please try again.');
                    return;
                }
                form.reset();
                show(successBox);
                document.getElementById('envelope-number-field')?.setAttribute('hidden', '');
                window.scrollTo({ top: 0, behavior: 'smooth' });
            } catch (err) {
                showError(errorBox, 'Network error. Please try again.');
            }
        });
    }

    function wireEnvelopeFieldToggle() {
        const checkbox = document.getElementById('reg-envelopes');
        const field = document.getElementById('envelope-number-field');
        if (!checkbox || !field) return;
        const sync = () => {
            if (checkbox.checked) field.removeAttribute('hidden');
            else field.setAttribute('hidden', '');
        };
        checkbox.addEventListener('change', sync);
        sync();
    }

    // ---------- Admin registration list ----------
    function wireRegistrationList() {
        const list = document.getElementById('registration-list');
        if (!list) return;

        list.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action]');
            if (!btn) return;
            const li = btn.closest('[data-row-id]');
            if (!li) return;
            const id = li.getAttribute('data-row-id');
            const action = btn.getAttribute('data-action');

            if (action === 'toggle-read') {
                const read = btn.getAttribute('data-read') === 'true';
                try {
                    const res = await fetch(`/api/v1/admin/registrations/${id}/read`, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ read }),
                    });
                    if (res.ok) window.location.reload();
                } catch (_) { /* ignore */ }
            } else if (action === 'delete-registration') {
                if (!confirm('Delete this registration? It can be recovered from the database.')) return;
                try {
                    const res = await fetch(`/api/v1/admin/registrations/${id}`, { method: 'DELETE' });
                    if (res.ok) li.remove();
                } catch (_) { /* ignore */ }
            }
        });
    }

    // ---------- Helpers ----------
    function value(id) {
        const el = document.getElementById(id);
        return el ? el.value.trim() : '';
    }
    function hide(el) { if (el) el.setAttribute('hidden', ''); }
    function show(el) { if (el) el.removeAttribute('hidden'); }
    function showError(el, msg) { if (!el) return; el.textContent = msg; el.removeAttribute('hidden'); }
    async function safeJson(res) { try { return await res.json(); } catch (_) { return null; } }
})();
