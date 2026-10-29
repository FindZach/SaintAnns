// Our Lady of the Valley Parish — admin console client-side glue.
// Phase 4: block editor (SortableJS), asset uploads, inline table CRUD,
// modal forms for homilies / bulletins / events.
//
// This file is loaded on every admin page via base.html. Each init function
// no-ops when its anchor element isn't on the page, so there's one script
// path for the whole console.

(function () {
    document.addEventListener('DOMContentLoaded', init);

    function init() {
        wireModals();
        wireLogout();
        initPagesList();
        initPageEditor();
        initTableCRUD('mass-rows', '/api/v1/admin/mass-schedules', massTimeShape);
        initTableCRUD('confession-rows', '/api/v1/admin/confession-times', confessionShape);
        initHomilies();
        initBulletins();
        initEvents();
    }

    // ========== Common helpers ==========

    async function apiFetch(url, options = {}) {
        const opts = Object.assign({ credentials: 'same-origin' }, options);
        if (opts.body && typeof opts.body !== 'string' && !(opts.body instanceof FormData)) {
            opts.body = JSON.stringify(opts.body);
            opts.headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers || {});
        }
        const res = await fetch(url, opts);
        let data = null;
        try { data = await res.json(); } catch (_) {}
        if (!res.ok) {
            const err = (data && data.error) || `${res.status} ${res.statusText}`;
            throw new Error(err);
        }
        return data || {};
    }

    function toast(message, kind) {
        const region = document.getElementById('toast-region');
        if (!region) return;
        const el = document.createElement('div');
        el.className = 'parish-toast parish-toast-' + (kind || 'info');
        el.textContent = message;
        region.appendChild(el);
        setTimeout(() => el.classList.add('parish-toast-leaving'), 2800);
        setTimeout(() => el.remove(), 3200);
    }

    function closest(el, sel) { return el && el.closest ? el.closest(sel) : null; }

    // ========== Modals ==========

    function wireModals() {
        document.addEventListener('click', (e) => {
            if (e.target.matches('[data-modal-close], [data-modal-close] *')) {
                const modal = closest(e.target, '.parish-admin-modal');
                if (modal) closeModal(modal);
            }
        });
        document.addEventListener('keydown', (e) => {
            if (e.key !== 'Escape') return;
            document.querySelectorAll('.parish-admin-modal:not([hidden])').forEach(closeModal);
        });
    }
    function openModal(id) {
        const modal = typeof id === 'string' ? document.getElementById(id) : id;
        if (!modal) return;
        modal.removeAttribute('hidden');
    }
    function closeModal(modal) {
        if (!modal) return;
        modal.setAttribute('hidden', '');
    }

    // ========== Logout ==========

    function wireLogout() {
        document.querySelectorAll('#nav-logout').forEach((btn) => {
            btn.addEventListener('click', async (e) => {
                e.preventDefault();
                try { await fetch('/api/v1/auth/logout', { method: 'POST' }); } catch (_) {}
                window.location.assign('/');
            });
        });
    }

    // ========== Pages list ==========

    function initPagesList() {
        const newBtn = document.getElementById('new-page-btn');
        if (!newBtn) return;
        newBtn.addEventListener('click', () => openModal('new-page-modal'));

        const form = document.getElementById('new-page-form');
        const errBox = document.getElementById('new-page-error');
        if (form) {
            form.addEventListener('submit', async (e) => {
                e.preventDefault();
                hide(errBox);
                try {
                    const data = await apiFetch('/api/v1/admin/pages', {
                        method: 'POST',
                        body: {
                            slug: document.getElementById('new-page-slug').value.trim(),
                            title: document.getElementById('new-page-title-input').value.trim(),
                            nav_label: document.getElementById('new-page-nav-label').value.trim(),
                        },
                    });
                    if (data.page && data.page.slug) {
                        window.location.assign('/admin/pages/' + data.page.slug);
                    } else {
                        window.location.reload();
                    }
                } catch (err) {
                    showError(errBox, err.message);
                }
            });
        }

        document.querySelectorAll('[data-action="delete-page"]').forEach((btn) => {
            btn.addEventListener('click', async () => {
                const row = closest(btn, '[data-page-id]');
                if (!row) return;
                if (!confirm(`Delete page "${row.dataset.pageSlug}"? (soft delete; can be recovered)`)) return;
                try {
                    await apiFetch('/api/v1/admin/pages/' + row.dataset.pageId, { method: 'DELETE' });
                    row.remove();
                } catch (err) { toast(err.message, 'error'); }
            });
        });
    }

    // ========== Block editor ==========

    const BLOCK_TYPES = {
        heading: {
            label: 'Heading', icon: 'fa-heading',
            default: { level: 2, text: '' },
            render(d, idx) {
                return `
                    <div class="parish-block-fields">
                        <label>Level
                            <select data-field="level">
                                <option value="2"${d.level===2?' selected':''}>H2</option>
                                <option value="3"${d.level===3?' selected':''}>H3</option>
                                <option value="4"${d.level===4?' selected':''}>H4</option>
                            </select>
                        </label>
                        <label class="parish-block-fields-grow">Text
                            <input type="text" data-field="text" value="${escAttr(d.text || '')}">
                        </label>
                    </div>`;
            },
            collect(el) {
                return {
                    level: parseInt(el.querySelector('[data-field="level"]').value, 10),
                    text: el.querySelector('[data-field="text"]').value,
                };
            },
        },
        rich_text: {
            label: 'Text', icon: 'fa-paragraph',
            default: { html: '' },
            render(d) {
                return `<div class="parish-block-fields">
                    <label>Body (HTML allowed)
                        <textarea data-field="html" rows="6">${escText(d.html || '')}</textarea>
                    </label>
                </div>`;
            },
            collect(el) { return { html: el.querySelector('[data-field="html"]').value }; },
        },
        image: {
            label: 'Image', icon: 'fa-image',
            default: { asset_id: 0, alt: '', caption: '', size: 'medium' },
            render(d) {
                return `
                    <div class="parish-block-fields">
                        <div class="parish-block-asset-preview" data-asset-id="${d.asset_id || ''}">
                            ${d.asset_id ? '<img data-asset-img>' : '<span class="parish-block-asset-empty">No image chosen</span>'}
                        </div>
                        <div class="parish-block-fields-stack">
                            <button type="button" class="parish-admin-link" data-action="pick-image">Pick image</button>
                            <label>Alt text <input type="text" data-field="alt" value="${escAttr(d.alt || '')}"></label>
                            <label>Caption <input type="text" data-field="caption" value="${escAttr(d.caption || '')}"></label>
                            <label>Size
                                <select data-field="size">
                                    <option value="small"${d.size==='small'?' selected':''}>Small</option>
                                    <option value="medium"${(d.size||'medium')==='medium'?' selected':''}>Medium</option>
                                    <option value="wide"${d.size==='wide'?' selected':''}>Wide</option>
                                </select>
                            </label>
                        </div>
                    </div>`;
            },
            collect(el) {
                return {
                    asset_id: parseInt(el.querySelector('[data-asset-id]').getAttribute('data-asset-id') || '0', 10),
                    alt: el.querySelector('[data-field="alt"]').value,
                    caption: el.querySelector('[data-field="caption"]').value,
                    size: el.querySelector('[data-field="size"]').value,
                };
            },
            afterRender(el) {
                const previewEl = el.querySelector('[data-asset-id]');
                const img = previewEl.querySelector('[data-asset-img]');
                if (img) img.src = ''; // populated after picker
                el.querySelector('[data-action="pick-image"]').addEventListener('click', () => {
                    openAssetPicker('image', (asset) => {
                        previewEl.setAttribute('data-asset-id', asset.id);
                        previewEl.innerHTML = `<img data-asset-img src="${escAttr(asset.url)}" alt="${escAttr(asset.alt_text || '')}">`;
                    });
                });
                // Load preview for existing asset_id
                const id = previewEl.getAttribute('data-asset-id');
                if (id && id !== '0' && img) {
                    img.src = `/static/uploads/_fallback_until_lookup_${id}`;
                    fetch('/api/v1/admin/assets?kind=image').then(r => r.json()).then(d => {
                        const match = (d.assets || []).find(a => a.id == id);
                        if (match && img) img.src = match.url;
                    }).catch(() => {});
                }
            },
        },
        pdf_link: {
            label: 'PDF link', icon: 'fa-file-pdf',
            default: { asset_id: 0, label: '' },
            render(d) {
                return `
                    <div class="parish-block-fields">
                        <div class="parish-block-asset-preview" data-asset-id="${d.asset_id || ''}">
                            ${d.asset_id ? '<span class="parish-block-asset-filename">PDF chosen</span>' : '<span class="parish-block-asset-empty">No PDF chosen</span>'}
                        </div>
                        <div class="parish-block-fields-stack">
                            <button type="button" class="parish-admin-link" data-action="pick-pdf">Pick PDF</button>
                            <label>Label <input type="text" data-field="label" value="${escAttr(d.label || '')}"></label>
                        </div>
                    </div>`;
            },
            collect(el) {
                return {
                    asset_id: parseInt(el.querySelector('[data-asset-id]').getAttribute('data-asset-id') || '0', 10),
                    label: el.querySelector('[data-field="label"]').value,
                };
            },
            afterRender(el) {
                const previewEl = el.querySelector('[data-asset-id]');
                el.querySelector('[data-action="pick-pdf"]').addEventListener('click', () => {
                    openAssetPicker('pdf', (asset) => {
                        previewEl.setAttribute('data-asset-id', asset.id);
                        previewEl.innerHTML = `<span class="parish-block-asset-filename">${escText(asset.original_name || 'PDF')}</span>`;
                    });
                });
            },
        },
        video_embed: {
            label: 'Video embed', icon: 'fa-video',
            default: { provider: 'youtube', url: '' },
            render(d) {
                return `<div class="parish-block-fields">
                    <label>Provider
                        <select data-field="provider">
                            <option value="youtube"${(d.provider||'youtube')==='youtube'?' selected':''}>YouTube</option>
                            <option value="vimeo"${d.provider==='vimeo'?' selected':''}>Vimeo</option>
                            <option value="facebook"${d.provider==='facebook'?' selected':''}>Facebook</option>
                        </select>
                    </label>
                    <label class="parish-block-fields-grow">URL
                        <input type="url" data-field="url" value="${escAttr(d.url || '')}" placeholder="https://…">
                    </label>
                </div>`;
            },
            collect(el) {
                return {
                    provider: el.querySelector('[data-field="provider"]').value,
                    url: el.querySelector('[data-field="url"]').value,
                };
            },
        },
        cta_button: {
            label: 'CTA button', icon: 'fa-hand-pointer',
            default: { href: '', label: '', style: 'primary' },
            render(d) {
                return `<div class="parish-block-fields">
                    <label class="parish-block-fields-grow">Label
                        <input type="text" data-field="label" value="${escAttr(d.label || '')}" placeholder="Reach out">
                    </label>
                    <label class="parish-block-fields-grow">Href
                        <input type="text" data-field="href" value="${escAttr(d.href || '')}" placeholder="/contact or https://…">
                    </label>
                    <label>Style
                        <select data-field="style">
                            <option value="primary"${(d.style||'primary')==='primary'?' selected':''}>Primary</option>
                            <option value="ghost"${d.style==='ghost'?' selected':''}>Ghost</option>
                        </select>
                    </label>
                </div>`;
            },
            collect(el) {
                return {
                    label: el.querySelector('[data-field="label"]').value,
                    href: el.querySelector('[data-field="href"]').value,
                    style: el.querySelector('[data-field="style"]').value,
                };
            },
        },
        divider: {
            label: 'Divider', icon: 'fa-minus',
            default: {},
            render() { return '<p class="parish-admin-subtitle">A horizontal rule.</p>'; },
            collect() { return {}; },
        },
        mass_times_card: {
            label: 'Mass times card', icon: 'fa-church',
            default: {},
            render() { return '<p class="parish-admin-subtitle">Pulls live from the Mass Times admin.</p>'; },
            collect() { return {}; },
        },
        confession_times_card: {
            label: 'Confession times card', icon: 'fa-dove',
            default: {},
            render() { return '<p class="parish-admin-subtitle">Pulls live from the Confession Times admin.</p>'; },
            collect() { return {}; },
        },
        event_list: {
            label: 'Event list', icon: 'fa-calendar',
            default: { tag: '', limit: 6 },
            render(d) {
                return `<div class="parish-block-fields">
                    <label>Tag filter <input type="text" data-field="tag" value="${escAttr(d.tag || '')}" placeholder="(all events)"></label>
                    <label>Limit <input type="number" data-field="limit" value="${d.limit || 6}" min="1" max="50"></label>
                </div>`;
            },
            collect(el) {
                return {
                    tag: el.querySelector('[data-field="tag"]').value,
                    limit: parseInt(el.querySelector('[data-field="limit"]').value, 10) || 6,
                };
            },
        },
        bulletin_list: {
            label: 'Bulletin list', icon: 'fa-file-pdf',
            default: { limit: 10 },
            render(d) {
                return `<div class="parish-block-fields">
                    <label>Limit <input type="number" data-field="limit" value="${d.limit || 10}" min="1" max="100"></label>
                </div>`;
            },
            collect(el) { return { limit: parseInt(el.querySelector('[data-field="limit"]').value, 10) || 10 }; },
        },
        staff_grid: {
            label: 'Staff grid', icon: 'fa-users',
            default: {},
            render() { return '<p class="parish-admin-subtitle">Shows every staff member in sort order.</p>'; },
            collect() { return {}; },
        },
    };

    function initPageEditor() {
        const editor = document.getElementById('block-editor');
        if (!editor) return;
        const loading = document.getElementById('editor-loading');
        let blocks = [];
        try { blocks = JSON.parse(editor.dataset.blocks || '[]') || []; } catch (_) { blocks = []; }
        if (loading) loading.remove();

        blocks.forEach((b) => editor.appendChild(makeBlockEl(b.type, b.data || {})));
        enableBlockSortable(editor);

        document.getElementById('add-block-btn').addEventListener('click', () => openPalette(editor));
        document.getElementById('save-page-btn').addEventListener('click', () => savePage(editor));
        renderPalette(editor);
        wireAssetPicker();
    }

    function makeBlockEl(type, data) {
        const def = BLOCK_TYPES[type];
        const el = document.createElement('article');
        el.className = 'parish-editor-block';
        el.setAttribute('data-block-type', type);
        el.innerHTML = `
            <header class="parish-editor-block-head">
                <span class="parish-editor-block-handle" aria-label="Drag to reorder"><i class="fas fa-grip-vertical"></i></span>
                <span class="parish-editor-block-type"><i class="fas ${def ? def.icon : 'fa-cube'}"></i> ${def ? def.label : type}</span>
                <button type="button" class="parish-editor-block-remove" data-action="remove-block" aria-label="Remove block">
                    <i class="fas fa-times"></i>
                </button>
            </header>
            <div class="parish-editor-block-body"></div>`;
        const body = el.querySelector('.parish-editor-block-body');
        if (def) {
            body.innerHTML = def.render(Object.assign({}, def.default, data || {}));
            if (typeof def.afterRender === 'function') def.afterRender(body, data || {});
        } else {
            body.innerHTML = `<p class="parish-admin-empty">Unknown block type: <code>${escText(type)}</code></p>`;
        }
        el.querySelector('[data-action="remove-block"]').addEventListener('click', () => {
            if (confirm('Remove this block?')) el.remove();
        });
        return el;
    }

    function enableBlockSortable(editor) {
        if (!window.Sortable) return;
        Sortable.create(editor, {
            handle: '.parish-editor-block-handle',
            animation: 150,
            ghostClass: 'parish-editor-block-ghost',
        });
    }

    function renderPalette(editor) {
        const list = document.getElementById('palette-list');
        if (!list) return;
        list.innerHTML = '';
        Object.keys(BLOCK_TYPES).forEach((type) => {
            const def = BLOCK_TYPES[type];
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'parish-palette-item';
            btn.innerHTML = `<i class="fas ${def.icon}"></i><span>${def.label}</span>`;
            btn.addEventListener('click', () => {
                editor.appendChild(makeBlockEl(type, Object.assign({}, def.default)));
                closeModal(document.getElementById('block-palette'));
            });
            list.appendChild(btn);
        });
    }

    function openPalette() { openModal('block-palette'); }

    async function savePage(editor) {
        const slug = editor.getAttribute('data-page-slug');
        const titleEl = document.getElementById('page-title');
        const navEl = document.getElementById('page-nav-label');
        const published = editor.getAttribute('data-page-published') === '1';

        const blocks = [];
        editor.querySelectorAll('.parish-editor-block').forEach((el) => {
            const type = el.getAttribute('data-block-type');
            const def = BLOCK_TYPES[type];
            if (!def) return;
            const data = def.collect(el.querySelector('.parish-editor-block-body')) || {};
            blocks.push({ type, data });
        });

        const statusEl = document.getElementById('editor-status');
        statusEl.hidden = false;
        statusEl.textContent = 'Saving…';
        statusEl.className = 'parish-editor-status';

        try {
            await apiFetch('/api/v1/admin/page-content/' + slug, {
                method: 'PUT',
                body: {
                    title: titleEl ? titleEl.textContent.trim() : '',
                    nav_label: navEl ? navEl.value.trim() : '',
                    blocks,
                    publish: !published, // publish on first save
                },
            });
            statusEl.textContent = 'Saved ✓';
            statusEl.className = 'parish-editor-status parish-editor-status-ok';
            setTimeout(() => { statusEl.hidden = true; }, 2000);
        } catch (err) {
            statusEl.textContent = 'Save failed: ' + err.message;
            statusEl.className = 'parish-editor-status parish-editor-status-error';
        }
    }

    // ========== Asset picker ==========

    let pickerPendingKind = null;
    let pickerPendingCallback = null;

    function wireAssetPicker() {
        const uploader = document.getElementById('asset-upload-input');
        if (uploader) {
            uploader.addEventListener('change', uploadAsset);
        }
    }

    async function openAssetPicker(kind, callback) {
        pickerPendingKind = kind;
        pickerPendingCallback = callback;
        openModal('asset-picker');
        await loadAssetGrid(kind);
    }

    async function loadAssetGrid(kind) {
        const grid = document.getElementById('asset-picker-grid');
        if (!grid) return;
        grid.innerHTML = '<p class="parish-admin-empty">Loading…</p>';
        try {
            const data = await apiFetch('/api/v1/admin/assets?kind=' + encodeURIComponent(kind || ''));
            const assets = data.assets || [];
            if (!assets.length) {
                grid.innerHTML = '<p class="parish-admin-empty">No assets yet. Upload one above.</p>';
                return;
            }
            grid.innerHTML = '';
            assets.forEach((a) => {
                const card = document.createElement('button');
                card.type = 'button';
                card.className = 'parish-asset-card';
                if (a.kind === 'image') {
                    card.innerHTML = `<img src="${escAttr(a.url)}" alt="${escAttr(a.alt_text || a.original_name || '')}"><span class="parish-asset-card-name">${escText(a.original_name || '')}</span>`;
                } else {
                    card.innerHTML = `<span class="parish-asset-card-filetype"><i class="fas fa-file-pdf"></i></span><span class="parish-asset-card-name">${escText(a.original_name || '')}</span>`;
                }
                card.addEventListener('click', () => {
                    if (typeof pickerPendingCallback === 'function') pickerPendingCallback(a);
                    closeModal(document.getElementById('asset-picker'));
                });
                grid.appendChild(card);
            });
        } catch (err) {
            grid.innerHTML = `<p class="parish-form-error">${escText(err.message)}</p>`;
        }
    }

    async function uploadAsset(e) {
        const file = e.target.files && e.target.files[0];
        if (!file) return;
        const status = document.getElementById('asset-upload-status');
        if (status) status.textContent = `Uploading ${file.name}…`;
        try {
            const fd = new FormData();
            fd.append('file', file);
            const res = await fetch('/api/v1/admin/assets', { method: 'POST', body: fd, credentials: 'same-origin' });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'Upload failed');
            if (status) status.textContent = `Uploaded ${data.original_name}`;
            await loadAssetGrid(pickerPendingKind);
        } catch (err) {
            if (status) status.textContent = 'Upload failed: ' + err.message;
        } finally {
            e.target.value = '';
        }
    }

    // ========== Table CRUD (Mass times, confessions) ==========

    function massTimeShape(row) {
        return {
            church: row.querySelector('[data-field="church"]').value,
            day_of_week: parseInt(row.querySelector('[data-field="day_of_week"]').value, 10),
            time_of_day: row.querySelector('[data-field="time_of_day"]').value,
            label: row.querySelector('[data-field="label"]').value,
            streamed_url: row.querySelector('[data-field="streamed_url"]').value,
        };
    }
    function confessionShape(row) {
        return {
            church: row.querySelector('[data-field="church"]').value,
            day_of_week: parseInt(row.querySelector('[data-field="day_of_week"]').value, 10),
            start_of_day: row.querySelector('[data-field="start_of_day"]').value,
            end_of_day: row.querySelector('[data-field="end_of_day"]').value,
            season: row.querySelector('[data-field="season"]').value,
            label: row.querySelector('[data-field="label"]').value,
        };
    }

    function initTableCRUD(tbodyId, baseURL, shape) {
        const tbody = document.getElementById(tbodyId);
        if (!tbody) return;

        tbody.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action]');
            if (!btn) return;
            const row = closest(btn, '[data-row-id]');
            if (!row) return;
            const id = row.dataset.rowId;
            try {
                if (btn.dataset.action === 'save-row') {
                    await apiFetch(`${baseURL}/${id}`, { method: 'PUT', body: shape(row) });
                    toast('Saved', 'ok');
                    row.classList.add('parish-admin-row-ok');
                    setTimeout(() => row.classList.remove('parish-admin-row-ok'), 1200);
                } else if (btn.dataset.action === 'delete-row') {
                    if (!confirm('Delete this row?')) return;
                    await apiFetch(`${baseURL}/${id}`, { method: 'DELETE' });
                    row.remove();
                }
            } catch (err) { toast(err.message, 'error'); }
        });

        // "Add row" button (per-table). Its data-create-url matches the API URL.
        const addBtn = document.querySelector(`[data-create-url="${baseURL}"]`);
        if (addBtn) {
            addBtn.addEventListener('click', async () => {
                // Build minimum payload from the first existing row's fields.
                const template = tbody.querySelector('tr');
                if (!template) {
                    // No template; nothing to clone — show a hint.
                    toast('Add a row by editing in SQL or save a row above first', 'info');
                    return;
                }
                const body = shape(template);
                // Clear text fields so the new row isn't a dupe
                if ('label' in body) body.label = '';
                try {
                    const res = await apiFetch(baseURL, { method: 'POST', body });
                    window.location.reload();
                } catch (err) { toast(err.message, 'error'); }
            });
        }

        // Optional drag-reorder.
        const reorderURL = tbody.getAttribute('data-reorder-url');
        if (reorderURL && window.Sortable) {
            Sortable.create(tbody, {
                handle: '.parish-admin-drag-handle',
                animation: 150,
                onEnd: async () => {
                    const ids = Array.from(tbody.querySelectorAll('tr')).map((r) => parseInt(r.dataset.rowId, 10));
                    try {
                        await apiFetch(reorderURL, { method: 'POST', body: { ids } });
                        toast('Reordered', 'ok');
                    } catch (err) { toast(err.message, 'error'); }
                },
            });
        }
    }

    // ========== Homilies ==========

    function initHomilies() {
        const newBtn = document.getElementById('new-homily-btn');
        if (!newBtn) return;
        newBtn.addEventListener('click', () => openModal('new-homily-modal'));

        const form = document.getElementById('new-homily-form');
        const errBox = document.getElementById('new-homily-error');
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            try {
                await apiFetch('/api/v1/admin/homilies', {
                    method: 'POST',
                    body: {
                        preached_on: document.getElementById('homily-preached-on').value,
                        title: document.getElementById('homily-title-input').value,
                        author: document.getElementById('homily-author').value,
                        body: document.getElementById('homily-body').value,
                        audio_url: document.getElementById('homily-audio').value,
                        video_url: document.getElementById('homily-video').value,
                        document_url: document.getElementById('homily-document').value,
                    },
                });
                window.location.reload();
            } catch (err) { showError(errBox, err.message); }
        });

        document.getElementById('homily-list')?.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action="delete-homily"]');
            if (!btn) return;
            const row = closest(btn, '[data-row-id]');
            if (!row || !confirm('Delete this homily?')) return;
            try {
                await apiFetch('/api/v1/admin/homilies/' + row.dataset.rowId, { method: 'DELETE' });
                row.remove();
            } catch (err) { toast(err.message, 'error'); }
        });
    }

    // ========== Bulletins ==========

    function initBulletins() {
        const newBtn = document.getElementById('new-bulletin-btn');
        if (!newBtn) return;
        newBtn.addEventListener('click', () => openModal('new-bulletin-modal'));

        const form = document.getElementById('new-bulletin-form');
        const errBox = document.getElementById('new-bulletin-error');
        const statusEl = document.getElementById('bulletin-upload-status');

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            const pdfInput = document.getElementById('bulletin-pdf');
            const file = pdfInput.files[0];
            if (!file) { showError(errBox, 'Choose a PDF.'); return; }

            try {
                if (statusEl) statusEl.textContent = 'Uploading PDF…';
                const fd = new FormData();
                fd.append('file', file);
                const uploadRes = await fetch('/api/v1/admin/assets', { method: 'POST', body: fd, credentials: 'same-origin' });
                const uploadData = await uploadRes.json();
                if (!uploadRes.ok) throw new Error(uploadData.error || 'Upload failed');

                if (statusEl) statusEl.textContent = 'Creating bulletin…';
                await apiFetch('/api/v1/admin/bulletins', {
                    method: 'POST',
                    body: {
                        published_on: document.getElementById('bulletin-published-on').value,
                        label: document.getElementById('bulletin-label').value,
                        asset_id: uploadData.id,
                    },
                });
                window.location.reload();
            } catch (err) {
                if (statusEl) statusEl.textContent = '';
                showError(errBox, err.message);
            }
        });

        document.getElementById('bulletin-list')?.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action="delete-bulletin"]');
            if (!btn) return;
            const row = closest(btn, '[data-row-id]');
            if (!row || !confirm('Delete this bulletin?')) return;
            try {
                await apiFetch('/api/v1/admin/bulletins/' + row.dataset.rowId, { method: 'DELETE' });
                row.remove();
            } catch (err) { toast(err.message, 'error'); }
        });
    }

    // ========== Events ==========

    function initEvents() {
        const newBtn = document.getElementById('new-event-btn');
        if (!newBtn) return;
        newBtn.addEventListener('click', () => openModal('new-event-modal'));

        const form = document.getElementById('new-event-form');
        const errBox = document.getElementById('new-event-error');
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            try {
                await apiFetch('/api/v1/admin/events', {
                    method: 'POST',
                    body: {
                        slug: document.getElementById('event-slug').value,
                        title: document.getElementById('event-title-input').value,
                        location: document.getElementById('event-location').value,
                        starts_at: document.getElementById('event-starts-at').value,
                        ends_at: document.getElementById('event-ends-at').value,
                        body: document.getElementById('event-body').value,
                        featured: document.getElementById('event-featured').checked,
                        tag: document.getElementById('event-tag').value,
                    },
                });
                window.location.reload();
            } catch (err) { showError(errBox, err.message); }
        });

        document.getElementById('event-list')?.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action="delete-event"]');
            if (!btn) return;
            const row = closest(btn, '[data-row-id]');
            if (!row || !confirm('Delete this event?')) return;
            try {
                await apiFetch('/api/v1/admin/events/' + row.dataset.rowId, { method: 'DELETE' });
                row.remove();
            } catch (err) { toast(err.message, 'error'); }
        });
    }

    // ========== Tiny helpers ==========

    function escText(s) {
        return String(s == null ? '' : s)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
    function escAttr(s) {
        return String(s == null ? '' : s)
            .replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
    function hide(el) { if (el) el.hidden = true; }
    function showError(el, msg) { if (!el) return; el.textContent = msg; el.hidden = false; }

    // ---------- Asset Library (/admin/assets) ----------
    (function initAssetLibrary() {
        const grid = document.getElementById('asset-grid');
        if (!grid) return;

        const uploadInput = document.getElementById('asset-upload-input');
        const uploadStatus = document.getElementById('asset-upload-status');
        const search = document.getElementById('asset-search');
        const filterBtns = document.querySelectorAll('.parish-asset-filter-btn');
        const toast = document.getElementById('asset-toast');
        const toastText = document.getElementById('asset-toast-text');

        let allAssets = [];
        let activeKind = '';
        let activeQuery = '';

        function assetIcon(kind) {
            if (kind === 'pdf') return 'fa-file-pdf';
            if (kind === 'audio') return 'fa-music';
            if (kind === 'video') return 'fa-video';
            return 'fa-image';
        }

        function render() {
            if (allAssets.length === 0) {
                grid.innerHTML = '<p class="parish-admin-empty">No assets uploaded yet. Click "Upload file" to get started.</p>';
                return;
            }
            const q = activeQuery.trim().toLowerCase();
            const filtered = allAssets.filter(a => {
                if (activeKind && a.kind !== activeKind) return false;
                if (q && !((a.original_name || '').toLowerCase().includes(q) ||
                           (a.url || '').toLowerCase().includes(q))) return false;
                return true;
            });
            if (filtered.length === 0) {
                grid.innerHTML = '<p class="parish-admin-empty">No matches. Try a different filter or search.</p>';
                return;
            }
            grid.innerHTML = filtered.map(a => {
                const name = escapeHtml(a.original_name || a.url.split('/').pop());
                const url = escapeHtml(a.url);
                const preview = a.kind === 'image'
                    ? `<img src="${url}" alt="${escapeHtml(a.alt_text || name)}" loading="lazy">`
                    : `<div class="parish-asset-tile-icon"><i class="fas ${assetIcon(a.kind)}"></i></div>`;
                const dims = (a.kind === 'image' && a.width && a.height)
                    ? `<span class="parish-asset-tile-dims">${a.width}×${a.height}</span>` : '';
                return `
                    <article class="parish-asset-tile" data-id="${a.id}" data-url="${url}" data-kind="${a.kind}">
                        <div class="parish-asset-tile-preview">${preview}</div>
                        <div class="parish-asset-tile-meta">
                            <p class="parish-asset-tile-name" title="${name}">${name}</p>
                            <p class="parish-asset-tile-sub">${a.kind}${dims ? ' · ' + dims : ''}</p>
                        </div>
                        <div class="parish-asset-tile-actions">
                            <button type="button" class="parish-admin-link" data-action="copy-url">
                                <i class="fas fa-link"></i> Copy URL
                            </button>
                            <a href="${url}" target="_blank" rel="noopener" class="parish-admin-link">
                                <i class="fas fa-arrow-up-right-from-square"></i> Open
                            </a>
                            <button type="button" class="parish-admin-link parish-admin-link-danger" data-action="delete-asset">
                                <i class="fas fa-trash"></i> Delete
                            </button>
                        </div>
                    </article>`;
            }).join('');
        }

        async function reload() {
            try {
                const res = await fetch('/api/v1/admin/assets', { credentials: 'same-origin' });
                if (!res.ok) throw new Error('Failed to load assets');
                const data = await res.json();
                allAssets = data.assets || [];
                render();
            } catch (err) {
                grid.innerHTML = `<p class="parish-admin-empty">Could not load assets: ${escapeHtml(err.message)}</p>`;
            }
        }

        function showToast(msg) {
            if (!toast) return;
            if (toastText) toastText.textContent = msg;
            toast.hidden = false;
            setTimeout(() => { toast.hidden = true; }, 1800);
        }

        async function uploadOne(file) {
            if (uploadStatus) uploadStatus.textContent = `Uploading ${file.name}…`;
            const fd = new FormData();
            fd.append('file', file);
            const res = await fetch('/api/v1/admin/assets', { method: 'POST', body: fd, credentials: 'same-origin' });
            if (!res.ok) {
                const err = await res.json().catch(() => ({}));
                throw new Error(err.error || `Upload failed (${res.status})`);
            }
            return res.json();
        }

        if (uploadInput) {
            uploadInput.addEventListener('change', async () => {
                const files = Array.from(uploadInput.files || []);
                if (files.length === 0) return;
                let okCount = 0;
                for (const file of files) {
                    try {
                        await uploadOne(file);
                        okCount++;
                    } catch (err) {
                        if (uploadStatus) uploadStatus.textContent = `${file.name}: ${err.message}`;
                    }
                }
                if (uploadStatus) uploadStatus.textContent = `Uploaded ${okCount} of ${files.length}.`;
                uploadInput.value = '';
                await reload();
                setTimeout(() => { if (uploadStatus) uploadStatus.textContent = ''; }, 3500);
            });
        }

        filterBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                filterBtns.forEach(b => b.classList.toggle('is-active', b === btn));
                activeKind = btn.getAttribute('data-filter') || '';
                render();
            });
        });

        if (search) {
            search.addEventListener('input', () => {
                activeQuery = search.value;
                render();
            });
        }

        grid.addEventListener('click', async (ev) => {
            const copyBtn = ev.target.closest('[data-action="copy-url"]');
            const delBtn = ev.target.closest('[data-action="delete-asset"]');
            const tile = ev.target.closest('.parish-asset-tile');
            if (!tile) return;
            const url = tile.getAttribute('data-url');
            const id = tile.getAttribute('data-id');
            if (copyBtn) {
                try {
                    await navigator.clipboard.writeText(url);
                    showToast('URL copied to clipboard');
                } catch (err) {
                    showToast('Could not copy — select the URL manually');
                }
                return;
            }
            if (delBtn) {
                if (!confirm('Delete this asset? It will stop appearing on any page that references it.')) return;
                const res = await fetch('/api/v1/admin/assets/' + id, { method: 'DELETE', credentials: 'same-origin' });
                if (res.ok) {
                    allAssets = allAssets.filter(a => String(a.id) !== String(id));
                    render();
                } else {
                    alert('Delete failed.');
                }
                return;
            }
        });

        function escapeHtml(s) {
            return String(s || '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
        }

        reload();
    })();

})();
