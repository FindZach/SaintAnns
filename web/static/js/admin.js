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
        if (document.getElementById('asset-picker')) wireAssetPicker();
        initFileChoiceFields();
        initChangePassword();
        initPagesList();
        initPageEditor();
        initTableCRUD('mass-rows', '/api/v1/admin/mass-schedules', massTimeShape);
        initTableCRUD('confession-rows', '/api/v1/admin/confession-times', confessionShape);
        initHomilies();
        initBulletins();
        initEvents();
        initUsers();
    }

    // ========== Create / edit record forms ==========

    // Puts a "New X" modal form into create or edit mode. `fields` maps input
    // ids to values; checkboxes take booleans. Returns nothing; callers keep
    // their own `editingId` to decide between POST and PUT on submit.
    function setFormMode(form, { editing, title, fields }) {
        const modal = closest(form, '.parish-admin-modal');
        const heading = modal && modal.querySelector('.parish-admin-modal-head h2');
        if (heading) heading.textContent = title;
        const submit = form.querySelector('[type="submit"]');
        if (submit) submit.textContent = editing ? (submit.dataset.editLabel || 'Save') : (submit.dataset.createLabel || submit.textContent);
        form.reset();
        // reset() leaves hidden inputs alone; clear them so a file chosen while
        // editing one record doesn't carry over into the next.
        form.querySelectorAll('input[type="hidden"][id]').forEach((el) => { el.value = ''; });
        Object.entries(fields || {}).forEach(([id, val]) => {
            const el = document.getElementById(id);
            if (!el) return;
            if (el.type === 'checkbox') el.checked = !!val;
            else el.value = val == null ? '' : val;
        });
        form.querySelectorAll('input[type="hidden"][id]').forEach((el) => syncFileChoice(el.id));
        const err = form.querySelector('.parish-form-error');
        hide(err);
    }

    // ========== File-choice fields (PDF / image pickers in forms) ==========

    // A hidden input holds the value (a URL for PDFs, an asset id for images);
    // these helpers keep its visible name / thumbnail / Remove button in sync.
    function syncFileChoice(inputId, label, thumbURL) {
        const input = document.getElementById(inputId);
        if (!input) return;
        const nameEl = document.querySelector(`[data-file-name-for="${inputId}"]`);
        const thumb = document.querySelector(`[data-file-thumb-for="${inputId}"]`);
        const clearBtn = document.querySelector(`[data-clear-file="${inputId}"]`);
        if (!nameEl) return;
        const has = !!input.value;
        if (label !== undefined) input.dataset.label = label || '';
        if (thumbURL !== undefined) input.dataset.thumb = thumbURL || '';
        if (!has) { input.dataset.label = ''; input.dataset.thumb = ''; }
        const fallback = nameEl.dataset.emptyText || (nameEl.dataset.emptyText = nameEl.textContent);
        nameEl.textContent = has ? (input.dataset.label || fileNameFromURL(input.value) || 'Chosen') : fallback;
        if (clearBtn) clearBtn.hidden = !has;
        if (thumb) {
            thumb.hidden = !(has && input.dataset.thumb);
            if (has && input.dataset.thumb) thumb.src = input.dataset.thumb;
        }
    }

    function fileNameFromURL(u) {
        if (!u) return '';
        const last = decodeURIComponent(String(u).split('/').pop() || '');
        return last.replace(/^\d{10,}-/, '');
    }

    function initFileChoiceFields() {
        document.querySelectorAll('[data-pick-pdf]').forEach((btn) => {
            btn.addEventListener('click', () => {
                const id = btn.dataset.pickPdf;
                openAssetPicker('pdf', (a) => {
                    document.getElementById(id).value = a.url;
                    syncFileChoice(id, a.name || a.original_name || fileNameFromURL(a.url));
                });
            });
        });
        document.querySelectorAll('[data-pick-image]').forEach((btn) => {
            btn.addEventListener('click', () => {
                const id = btn.dataset.pickImage;
                openAssetPicker('image', (a) => {
                    document.getElementById(id).value = a.id;
                    syncFileChoice(id, a.name || a.original_name || fileNameFromURL(a.url), a.url);
                }, document.getElementById(id).value);
            });
        });
        document.querySelectorAll('[data-clear-file]').forEach((btn) => {
            btn.addEventListener('click', () => {
                const id = btn.dataset.clearFile;
                document.getElementById(id).value = '';
                syncFileChoice(id);
            });
        });
    }

    // ========== Change my password ==========

    function initChangePassword() {
        const btn = document.getElementById('change-password-btn');
        const form = document.getElementById('change-password-form');
        if (!btn || !form) return;
        const errBox = document.getElementById('change-password-error');
        btn.addEventListener('click', () => { form.reset(); hide(errBox); openModal('change-password-modal'); });
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            const next = document.getElementById('cp-new').value;
            if (next !== document.getElementById('cp-confirm').value) {
                showError(errBox, 'The new passwords don’t match.');
                return;
            }
            try {
                await apiFetch('/api/v1/admin/me/password', {
                    method: 'PUT',
                    body: { current_password: document.getElementById('cp-current').value, new_password: next },
                });
                closeModal(document.getElementById('change-password-modal'));
                toast('Password changed', 'ok');
            } catch (err) { showError(errBox, err.message); }
        });
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
        const titleIn = document.getElementById('new-page-title-input');
        const slugIn = document.getElementById('new-page-slug');
        const parentIn = document.getElementById('new-page-parent');
        const prefix = document.getElementById('new-page-slug-prefix');
        let slugTouched = false;
        if (slugIn) slugIn.addEventListener('input', () => { slugTouched = slugIn.value !== ''; });
        if (titleIn && slugIn) titleIn.addEventListener('input', () => { if (!slugTouched) slugIn.value = slugifyText(titleIn.value); });
        if (parentIn && prefix) parentIn.addEventListener('change', () => { prefix.textContent = '/' + (parentIn.value ? parentIn.value + '/' : ''); });
        if (form) {
            form.addEventListener('submit', async (e) => {
                e.preventDefault();
                hide(errBox);
                try {
                    const data = await apiFetch('/api/v1/admin/pages', {
                        method: 'POST',
                        body: {
                            slug: slugIn.value.trim(),
                            parent_slug: parentIn ? parentIn.value : '',
                            title: titleIn.value.trim(),
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

    let imageAssetsPromise = null;
    function imageAssetsOnce() {
        if (!imageAssetsPromise) {
            imageAssetsPromise = apiFetch('/api/v1/admin/assets?kind=image')
                .then((d) => d.assets || [])
                .catch(() => { imageAssetsPromise = null; return []; });
        }
        return imageAssetsPromise;
    }

    // ========== Rich text editor (Text blocks) ==========
    //
    // A contenteditable area styled like the public page, with a toolbar for
    // the handful of things a page needs. "Edit HTML" swaps in the raw source.
    // Formatting goes through execCommand: deprecated on paper, but it's what
    // every browser still ships, and it leaves existing HTML alone (unlike
    // editors that rebuild content into their own model and drop what they
    // don't know). The server sanitizes everything on render regardless.

    const RTE_BUTTONS = [
        ['bold', 'fa-bold', 'Bold (Ctrl+B)'],
        ['italic', 'fa-italic', 'Italic (Ctrl+I)'],
        ['underline', 'fa-underline', 'Underline (Ctrl+U)'],
        '|',
        ['insertUnorderedList', 'fa-list-ul', 'Bulleted list'],
        ['insertOrderedList', 'fa-list-ol', 'Numbered list'],
        '|',
        ['justifyLeft', 'fa-align-left', 'Align left'],
        ['justifyCenter', 'fa-align-center', 'Center'],
        '|',
        ['link', 'fa-link', 'Add a link'],
        ['unlink', 'fa-link-slash', 'Remove link'],
        ['image', 'fa-image', 'Insert a picture'],
        ['pdf', 'fa-file-pdf', 'Link to a PDF'],
        '|',
        ['removeFormat', 'fa-eraser', 'Clear formatting'],
    ];

    // Tags kept when pasting from Word, Google Docs or another website; the
    // rest are unwrapped to their text, and styles and classes are dropped.
    const RTE_PASTE_TAGS = new Set(['P', 'BR', 'B', 'STRONG', 'I', 'EM', 'U', 'A', 'UL', 'OL', 'LI', 'H2', 'H3', 'H4', 'BLOCKQUOTE']);

    function richEditorMarkup(html) {
        const buttons = RTE_BUTTONS.map((b) => b === '|'
            ? '<span class="parish-rte-sep" aria-hidden="true"></span>'
            : `<button type="button" class="parish-rte-btn" data-cmd="${b[0]}" title="${b[2]}" aria-label="${b[2]}"><i class="fas ${b[1]}"></i></button>`
        ).join('');
        return `<div class="parish-rte" data-rte>
            <div class="parish-rte-toolbar" role="toolbar" aria-label="Text formatting">
                <select class="parish-rte-style" data-rte-style title="Text style" aria-label="Text style">
                    <option value="p">Normal text</option>
                    <option value="h2">Large heading</option>
                    <option value="h3">Medium heading</option>
                    <option value="h4">Small heading</option>
                    <option value="blockquote">Quote</option>
                </select>
                <span class="parish-rte-sep" aria-hidden="true"></span>
                ${buttons}
                <button type="button" class="parish-rte-btn parish-rte-source-toggle" data-cmd="source" title="Edit the HTML directly" aria-pressed="false"><i class="fas fa-code"></i> HTML</button>
            </div>
            <div class="parish-rte-area parish-block-rich" contenteditable="true" role="textbox" aria-multiline="true" data-placeholder="Start typing…"></div>
            <textarea class="parish-rte-source" data-field="html" hidden spellcheck="false">${escText(html)}</textarea>
        </div>`;
    }

    // Parse HTML inertly (a <template> runs no scripts and loads no images)
    // and strip anything that could run when it's put into the live editor.
    function rteFragment(html) {
        const tpl = document.createElement('template');
        tpl.innerHTML = html;
        tpl.content.querySelectorAll('script, style, iframe, object, embed, form').forEach((n) => n.remove());
        tpl.content.querySelectorAll('*').forEach((n) => {
            [...n.attributes].forEach((a) => {
                const name = a.name.toLowerCase();
                if (name.startsWith('on')) n.removeAttribute(a.name);
                if ((name === 'href' || name === 'src') && /^\s*javascript:/i.test(a.value)) n.removeAttribute(a.name);
            });
        });
        return tpl.content;
    }

    function rteCleanPaste(html) {
        const frag = rteFragment(html);
        const walk = (node) => {
            [...node.childNodes].forEach((child) => {
                if (child.nodeType === Node.COMMENT_NODE) { child.remove(); return; }
                if (child.nodeType !== Node.ELEMENT_NODE) return;
                walk(child);
                if (!RTE_PASTE_TAGS.has(child.tagName)) {
                    // DIVs become paragraphs; everything else melts into its text.
                    if (child.tagName === 'DIV' && child.textContent.trim()) {
                        const p = document.createElement('p');
                        p.append(...child.childNodes);
                        child.replaceWith(p);
                    } else {
                        child.replaceWith(...child.childNodes);
                    }
                    return;
                }
                const href = child.tagName === 'A' ? child.getAttribute('href') : null;
                [...child.attributes].forEach((a) => child.removeAttribute(a.name));
                if (href && rteSafeHref(href)) child.setAttribute('href', href);
            });
        };
        walk(frag);
        const box = document.createElement('div');
        box.appendChild(frag);
        // A single pasted paragraph (a phrase copied from Word) joins the
        // current line instead of splitting it.
        const kids = [...box.childNodes].filter((n) => n.nodeType === Node.ELEMENT_NODE || n.textContent.trim());
        if (kids.length === 1 && kids[0].nodeName === 'P') return kids[0].innerHTML;
        return box.innerHTML;
    }

    // Accepts what staff type for a link; returns a usable address or ''.
    function rteSafeHref(raw) {
        let v = String(raw || '').trim();
        if (!v) return '';
        if (/^www\./i.test(v)) v = 'https://' + v;
        if (/^[^\s@/]+@[^\s@]+\.[a-z]{2,}$/i.test(v)) v = 'mailto:' + v;
        if (/^(https?:\/\/|mailto:|tel:|\/(?![\/\\])|#)/i.test(v)) return v;
        return '';
    }

    function initRichEditor(root, initialHTML) {
        if (!root) return null;
        const area = root.querySelector('.parish-rte-area');
        const source = root.querySelector('.parish-rte-source');
        const styleSel = root.querySelector('[data-rte-style]');
        const toggle = root.querySelector('[data-cmd="source"]');
        let dirty = false;
        let savedRange = null;

        area.appendChild(rteFragment(initialHTML));

        const fitSource = () => { source.style.height = 'auto'; source.style.height = (source.scrollHeight + 4) + 'px'; };
        const markDirty = () => { dirty = true; };
        area.addEventListener('input', markDirty);
        source.addEventListener('input', () => { markDirty(); fitSource(); });

        const saveSelection = () => {
            const sel = window.getSelection();
            if (sel.rangeCount && area.contains(sel.anchorNode)) savedRange = sel.getRangeAt(0).cloneRange();
        };
        const restoreSelection = () => {
            area.focus();
            if (!savedRange) return;
            const sel = window.getSelection();
            sel.removeAllRanges();
            sel.addRange(savedRange);
        };
        const exec = (cmd, value) => {
            document.execCommand('styleWithCSS', false, false);
            document.execCommand('defaultParagraphSeparator', false, 'p');
            document.execCommand(cmd, false, value);
            markDirty();
            refreshToolbar();
        };

        // Chrome's insertHTML command stamps inline styles onto what it inserts
        // to "preserve" the look; inserting through a Range keeps HTML clean.
        const insertClean = (html) => {
            restoreSelection();
            const sel = window.getSelection();
            if (!sel.rangeCount || !area.contains(sel.anchorNode)) {
                area.appendChild(rteFragment(html));
            } else {
                const range = sel.getRangeAt(0);
                range.deleteContents();
                const frag = rteFragment(html);
                const last = frag.lastChild;
                range.insertNode(frag);
                if (last) {
                    range.setStartAfter(last);
                    range.collapse(true);
                    sel.removeAllRanges();
                    sel.addRange(range);
                }
            }
            markDirty();
            saveSelection();
        };

        function refreshToolbar() {
            if (root.classList.contains('is-source')) return;
            const sel = window.getSelection();
            if (!sel.rangeCount || !area.contains(sel.anchorNode)) return;
            root.querySelectorAll('.parish-rte-btn[data-cmd]').forEach((b) => {
                const cmd = b.getAttribute('data-cmd');
                if (['bold', 'italic', 'underline', 'insertUnorderedList', 'insertOrderedList', 'justifyCenter'].includes(cmd)) {
                    let on = false;
                    try { on = document.queryCommandState(cmd); } catch (_) {}
                    b.classList.toggle('is-active', on);
                    b.setAttribute('aria-pressed', on ? 'true' : 'false');
                }
            });
            let block = '';
            try { block = String(document.queryCommandValue('formatBlock') || '').toLowerCase().replace(/[<>]/g, ''); } catch (_) {}
            styleSel.value = ['h2', 'h3', 'h4', 'blockquote'].includes(block) ? block : 'p';
        }

        document.addEventListener('selectionchange', () => {
            if (document.activeElement === area) { saveSelection(); refreshToolbar(); }
        });
        area.addEventListener('focus', () => document.execCommand('defaultParagraphSeparator', false, 'p'));

        area.addEventListener('paste', (e) => {
            const cd = e.clipboardData;
            if (!cd) return;
            e.preventDefault();
            const html = cd.getData('text/html');
            if (html) {
                saveSelection();
                insertClean(rteCleanPaste(html));
            } else {
                exec('insertText', cd.getData('text/plain'));
            }
        });

        // Keep the text selection when a toolbar button is clicked.
        root.querySelector('.parish-rte-toolbar').addEventListener('mousedown', (e) => {
            if (e.target.closest('button')) e.preventDefault();
        });

        styleSel.addEventListener('change', () => {
            restoreSelection();
            exec('formatBlock', '<' + styleSel.value + '>');
        });

        root.querySelectorAll('.parish-rte-btn[data-cmd]').forEach((btn) => {
            btn.addEventListener('click', () => {
                const cmd = btn.getAttribute('data-cmd');
                if (cmd === 'source') { toggleSource(); return; }
                saveSelection();
                switch (cmd) {
                    case 'link': {
                        const sel = window.getSelection();
                        const hasText = sel.rangeCount && !sel.isCollapsed && area.contains(sel.anchorNode);
                        const raw = prompt(hasText
                            ? 'Link address — a page on this site like /contact, or a full web address:'
                            : 'Select the words to turn into a link first.\n\nOr enter an address to insert it as a link:', 'https://');
                        if (raw === null) return;
                        const href = rteSafeHref(raw);
                        if (!href || href === 'https://') { alert('That doesn\'t look like a web address. Try something like /contact or https://example.com'); return; }
                        restoreSelection();
                        if (hasText) {
                            exec('createLink', href);
                        } else {
                            insertClean(`<a href="${escAttr(href)}">${escText(raw.trim())}</a>`);
                        }
                        return;
                    }
                    case 'image':
                        openAssetPicker('image', (asset) => {
                            insertClean(`<img src="${escAttr(asset.url)}" alt="${escAttr(asset.alt_text || '')}" loading="lazy">`);
                        });
                        return;
                    case 'pdf':
                        openAssetPicker('pdf', (asset) => {
                            restoreSelection();
                            const sel = window.getSelection();
                            if (sel.rangeCount && !sel.isCollapsed) {
                                exec('createLink', asset.url);
                            } else {
                                insertClean(`<a href="${escAttr(asset.url)}">${escText(asset.name || asset.original_name || 'Download PDF')}</a>`);
                            }
                        });
                        return;
                    default:
                        restoreSelection();
                        exec(cmd);
                }
            });
        });

        function toggleSource() {
            const toSource = !root.classList.contains('is-source');
            if (toSource) {
                source.value = currentHTML();
                root.classList.add('is-source');
                area.hidden = true;
                source.hidden = false;
                fitSource();
                source.focus();
            } else {
                area.replaceChildren(rteFragment(source.value));
                root.classList.remove('is-source');
                source.hidden = true;
                area.hidden = false;
                area.focus();
            }
            toggle.setAttribute('aria-pressed', toSource ? 'true' : 'false');
            root.querySelectorAll('.parish-rte-toolbar button:not([data-cmd="source"]), .parish-rte-toolbar select')
                .forEach((c) => { c.disabled = toSource; });
        }

        function currentHTML() {
            const box = area.cloneNode(true);
            // Chrome puts new lists and headings inside the current <p>, which
            // isn't valid HTML and leaves blank gaps on the public page.
            box.querySelectorAll('p').forEach((p) => {
                if (p.querySelector('ul, ol, h2, h3, h4, blockquote, p, div')) p.replaceWith(...p.childNodes);
            });
            box.querySelectorAll('p').forEach((p) => { if (!p.childNodes.length) p.remove(); });
            // Text typed straight into an empty box isn't in a paragraph yet.
            const BLOCKS = /^(P|H[1-6]|UL|OL|BLOCKQUOTE|DIV|FIGURE|TABLE|HR|SECTION|ARTICLE|HEADER|ADDRESS|PRE|IFRAME)$/;
            let para = null;
            [...box.childNodes].forEach((n) => {
                const inline = n.nodeType === Node.TEXT_NODE || (n.nodeType === Node.ELEMENT_NODE && !BLOCKS.test(n.tagName));
                if (!inline) { para = null; return; }
                if (n.nodeType === Node.TEXT_NODE && !n.textContent.trim() && !para) return;
                if (n.nodeName === 'BR' && !para) { n.remove(); return; }
                if (!para) { para = document.createElement('p'); n.before(para); }
                para.appendChild(n);
            });
            // Browsers leave a lone <br> in an emptied editor.
            const html = box.innerHTML.trim();
            return /^(<br\s*\/?>|<p>(<br\s*\/?>)?<\/p>)?$/i.test(html) ? '' : html;
        }

        return {
            getHTML() {
                if (!dirty) return initialHTML; // untouched blocks save byte-for-byte
                return root.classList.contains('is-source') ? source.value : currentHTML();
            },
        };
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
                return richEditorMarkup(d.html || '');
            },
            collect(el) { return { html: el._rte ? el._rte.getHTML() : '' }; },
            afterRender(el, d) {
                el._rte = initRichEditor(el.querySelector('[data-rte]'), (d && d.html) || '');
            },
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
                            <p class="parish-block-asset-name" data-asset-name hidden></p>
                            <button type="button" class="parish-admin-link" data-action="pick-image">${d.asset_id ? 'Change image' : 'Pick image'}</button>
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
                const nameEl = el.querySelector('[data-asset-name]');
                const pickBtn = el.querySelector('[data-action="pick-image"]');
                const showName = (asset) => {
                    const name = asset ? (asset.name || asset.original_name || '') : '';
                    nameEl.textContent = name;
                    nameEl.title = name;
                    nameEl.hidden = !name;
                };
                pickBtn.addEventListener('click', () => {
                    openAssetPicker('image', (asset) => {
                        previewEl.setAttribute('data-asset-id', asset.id);
                        previewEl.innerHTML = `<img data-asset-img src="${escAttr(asset.url)}" alt="${escAttr(asset.alt_text || '')}">`;
                        showName(asset);
                        pickBtn.textContent = 'Change image';
                    }, previewEl.getAttribute('data-asset-id'));
                });
                // Look up the chosen file for its preview and name.
                const id = previewEl.getAttribute('data-asset-id');
                const img = previewEl.querySelector('[data-asset-img]');
                if (id && id !== '0' && img) {
                    imageAssetsOnce().then((assets) => {
                        const match = assets.find((a) => String(a.id) === String(id));
                        if (match) {
                            img.src = match.url;
                            showName(match);
                        } else {
                            previewEl.innerHTML = '<span class="parish-block-asset-empty">Image missing — pick another</span>';
                        }
                    });
                }
            },
        },
        image_row: {
            label: 'Photo gallery', icon: 'fa-images',
            default: { asset_ids: [], caption: '' },
            render(d) {
                return `
                    <div class="parish-block-fields-stack">
                        <div class="parish-gallery-edit" data-gallery aria-label="Photos in this gallery"></div>
                        <div class="parish-gallery-edit-actions">
                            <button type="button" class="parish-admin-link" data-action="add-photos"><i class="fas fa-plus"></i> Add photos</button>
                            <span class="parish-admin-subtitle" data-gallery-count></span>
                        </div>
                        <label>Caption <input type="text" data-field="caption" value="${escAttr(d.caption || '')}"></label>
                    </div>`;
            },
            collect(el) {
                return {
                    asset_ids: Array.from(el.querySelectorAll('[data-gallery] [data-asset-id]'))
                        .map((t) => parseInt(t.getAttribute('data-asset-id'), 10))
                        .filter((n) => n > 0),
                    caption: el.querySelector('[data-field="caption"]').value,
                };
            },
            afterRender(el, d) {
                const list = el.querySelector('[data-gallery]');
                const countEl = el.querySelector('[data-gallery-count]');
                const updateCount = () => {
                    const n = list.children.length;
                    countEl.textContent = n ? `${n} photo${n === 1 ? '' : 's'} — drag to reorder` : 'No photos yet';
                };
                const addThumb = (id, url, name) => {
                    const t = document.createElement('div');
                    t.className = 'parish-gallery-thumb';
                    t.setAttribute('data-asset-id', id);
                    t.title = name || '';
                    t.innerHTML = `${url ? `<img src="${escAttr(url)}" alt="" loading="lazy">` : '<span class="parish-block-asset-empty">Missing</span>'}
                        <button type="button" class="parish-gallery-thumb-remove" aria-label="Remove photo"><i class="fas fa-times"></i></button>`;
                    t.querySelector('button').addEventListener('click', () => { t.remove(); updateCount(); });
                    list.appendChild(t);
                };
                const ids = (d && d.asset_ids) || [];
                imageAssetsOnce().then((assets) => {
                    const byId = new Map(assets.map((a) => [String(a.id), a]));
                    ids.forEach((id) => {
                        const a = byId.get(String(id));
                        addThumb(id, a ? a.url : '', a ? (a.name || a.original_name) : '');
                    });
                    updateCount();
                });
                updateCount();
                el.querySelector('[data-action="add-photos"]').addEventListener('click', () => {
                    openAssetPicker('image', (chosen) => {
                        chosen.forEach((a) => addThumb(a.id, a.url, a.name || a.original_name));
                        updateCount();
                    }, null, { multi: true });
                });
                if (window.Sortable) {
                    Sortable.create(list, { animation: 150, ghostClass: 'parish-editor-block-ghost' });
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
                        previewEl.innerHTML = `<span class="parish-block-asset-filename">${escText(asset.name || asset.original_name || 'PDF')}</span>`;
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

        const pubBtn = document.getElementById('publish-toggle-btn');
        if (pubBtn) pubBtn.addEventListener('click', () => togglePublish(editor, pubBtn));
    }

    async function togglePublish(editor, btn) {
        const published = editor.getAttribute('data-page-published') === '1';
        const next = !published;
        if (!next && !confirm('Unpublish this page? Visitors will get "not found" until you publish it again.')) return;
        btn.disabled = true;
        try {
            await apiFetch(`/api/v1/admin/pages/${editor.dataset.pageId}/publish`, { method: 'PUT', body: { publish: next } });
            editor.setAttribute('data-page-published', next ? '1' : '0');
            const state = document.getElementById('publish-state');
            if (state) {
                state.textContent = next ? 'Published' : 'Draft — not public';
                state.className = 'parish-admin-badge ' + (next ? 'parish-admin-badge-ok' : 'parish-admin-badge-draft');
            }
            btn.innerHTML = next ? '<i class="fas fa-eye-slash"></i> Unpublish' : '<i class="fas fa-globe"></i> Publish';
            toast(next ? 'Page published' : 'Page unpublished', 'ok');
        } catch (err) {
            toast(err.message, 'error');
        } finally {
            btn.disabled = false;
        }
    }

    // Block types the server renders but this editor has no form for. They're
    // shown read-only and saved back exactly as loaded.
    const PASSTHROUGH_LABELS = {
        child_pages: ['fa-sitemap', 'Sub-page cards (generated automatically)'],
        html_raw: ['fa-code', 'Embedded HTML'],
    };

    function makeBlockEl(type, data) {
        const def = BLOCK_TYPES[type];
        const el = document.createElement('article');
        el.className = 'parish-editor-block';
        el.setAttribute('data-block-type', type);
        el._originalData = Object.assign({}, data || {});
        el.innerHTML = `
            <header class="parish-editor-block-head">
                <span class="parish-editor-block-handle" aria-label="Drag to reorder"><i class="fas fa-grip-vertical"></i></span>
                <span class="parish-editor-block-type"><i class="fas ${def ? def.icon : (PASSTHROUGH_LABELS[type] || ['fa-cube'])[0]}"></i> ${def ? def.label : escText((PASSTHROUGH_LABELS[type] || [null, type])[1])}</span>
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
            body.innerHTML = `<p class="parish-admin-empty">This block can't be edited here yet. It will be kept as-is when you save — drag to move it, or remove it with ✕.</p>`;
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
        const parentEl = document.getElementById('page-parent');
        const metaEl = document.getElementById('page-meta');

        const blocks = [];
        editor.querySelectorAll('.parish-editor-block').forEach((el) => {
            const type = el.getAttribute('data-block-type');
            const def = BLOCK_TYPES[type];
            const original = el._originalData || {};
            if (!def) {
                blocks.push({ type, data: original });
                return;
            }
            // Start from what was loaded so fields this form doesn't expose survive.
            const collected = def.collect(el.querySelector('.parish-editor-block-body')) || {};
            blocks.push({ type, data: Object.assign({}, original, collected) });
        });

        const statusEl = document.getElementById('editor-status');
        statusEl.hidden = false;
        statusEl.textContent = 'Saving…';
        statusEl.className = 'parish-editor-status';

        try {
            const body = {
                title: titleEl ? titleEl.textContent.trim() : '',
                nav_label: navEl ? navEl.value.trim() : '',
                blocks,
            };
            if (parentEl) body.parent_slug = parentEl.value;
            if (metaEl) body.meta_description = metaEl.value;
            await apiFetch('/api/v1/admin/page-content/' + slug, { method: 'PUT', body });
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
    let pickerAssets = [];
    let pickerCurrentId = '';
    // Multi mode (a gallery's "Add photos"): clicking a card ticks it, and a
    // Done button hands the whole selection back in the order it was ticked.
    let pickerMulti = false;
    let pickerSelected = [];

    const PICKER_SORT_KEY = 'olv.assetPickerSort';
    const KIND_WORDS = { image: ['picture', 'pictures'], pdf: ['PDF', 'PDFs'], audio: ['audio file', 'audio files'], video: ['video', 'videos'] };

    function wireAssetPicker() {
        const uploader = document.getElementById('asset-upload-input');
        if (uploader) {
            uploader.addEventListener('change', uploadAsset);
        }
        const search = document.getElementById('asset-picker-search');
        const sort = document.getElementById('asset-picker-sort');
        const kind = document.getElementById('asset-picker-kind');
        if (search) search.addEventListener('input', renderAssetGrid);
        const done = document.getElementById('asset-picker-done');
        if (done) {
            done.addEventListener('click', () => {
                if (typeof pickerPendingCallback === 'function' && pickerSelected.length) pickerPendingCallback(pickerSelected.slice());
                closeModal(document.getElementById('asset-picker'));
            });
        }
        if (kind) kind.addEventListener('change', renderAssetGrid);
        if (sort) {
            try { sort.value = localStorage.getItem(PICKER_SORT_KEY) || 'newest'; } catch (_) {}
            if (!sort.value) sort.value = 'newest';
            sort.addEventListener('change', () => {
                try { localStorage.setItem(PICKER_SORT_KEY, sort.value); } catch (_) {}
                renderAssetGrid();
            });
        }
    }

    // `currentId` is the file already chosen, if any; it's shown first.
    async function openAssetPicker(kind, callback, currentId, opts) {
        pickerPendingKind = kind;
        pickerPendingCallback = callback;
        pickerCurrentId = currentId && currentId !== '0' ? String(currentId) : '';
        pickerMulti = !!(opts && opts.multi);
        pickerSelected = [];
        const title = document.getElementById('asset-picker-title');
        if (title) title.textContent = pickerMulti ? 'Choose photos' : 'Choose a file';
        const footer = document.getElementById('asset-picker-footer');
        if (footer) footer.hidden = !pickerMulti;
        updatePickerDone();
        const search = document.getElementById('asset-picker-search');
        if (search) search.value = '';
        // A picker opened for one kind (a flyer, a bulletin PDF) only shows
        // that kind; the type menu is for the open-ended case.
        const kindSel = document.getElementById('asset-picker-kind');
        if (kindSel) { kindSel.hidden = !!kind; kindSel.value = ''; }
        openModal('asset-picker');
        await loadAssetGrid(kind);
        if (search) search.focus();
    }

    async function loadAssetGrid(kind) {
        const grid = document.getElementById('asset-picker-grid');
        if (!grid) return;
        grid.innerHTML = '<p class="parish-admin-empty">Loading…</p>';
        try {
            const data = await apiFetch('/api/v1/admin/assets?kind=' + encodeURIComponent(kind || ''));
            pickerAssets = data.assets || [];
            renderAssetGrid();
        } catch (err) {
            grid.innerHTML = `<p class="parish-form-error">${escText(err.message)}</p>`;
        }
    }

    function assetDisplayName(a) {
        return a.name || a.original_name || String(a.url || '').split('/').pop() || '';
    }

    function renderAssetGrid() {
        const grid = document.getElementById('asset-picker-grid');
        if (!grid) return;
        const countEl = document.getElementById('asset-picker-count');
        const q = ((document.getElementById('asset-picker-search') || {}).value || '').trim().toLowerCase();
        const sort = ((document.getElementById('asset-picker-sort') || {}).value) || 'newest';
        const kindFilter = pickerPendingKind || ((document.getElementById('asset-picker-kind') || {}).value) || '';

        if (!pickerAssets.length) {
            grid.innerHTML = '<p class="parish-admin-empty">Nothing here yet. Use “Upload new” above to add a file.</p>';
            if (countEl) countEl.textContent = '';
            return;
        }

        const terms = q.split(/\s+/).filter(Boolean);
        const list = pickerAssets.filter((a) => {
            if (kindFilter && a.kind !== kindFilter) return false;
            const hay = (assetDisplayName(a) + ' ' + (a.alt_text || '')).toLowerCase();
            return terms.every((t) => hay.includes(t));
        });
        const time = (a) => Date.parse(a.created_at || '') || 0;
        const byName = (a, b) => assetDisplayName(a).localeCompare(assetDisplayName(b), undefined, { numeric: true, sensitivity: 'base' });
        list.sort({
            newest: (a, b) => time(b) - time(a) || b.id - a.id,
            oldest: (a, b) => time(a) - time(b) || a.id - b.id,
            az: byName,
            za: (a, b) => byName(b, a),
        }[sort] || (() => 0));

        if (countEl) {
            const words = KIND_WORDS[kindFilter] || ['file', 'files'];
            const total = pickerAssets.filter((a) => !kindFilter || a.kind === kindFilter).length;
            countEl.textContent = terms.length
                ? `${list.length} of ${total} ${total === 1 ? words[0] : words[1]}`
                : `${total} ${total === 1 ? words[0] : words[1]}`;
        }
        if (!list.length) {
            grid.innerHTML = '<p class="parish-admin-empty">No files match that search.</p>';
            return;
        }

        grid.innerHTML = '';
        const addHeading = (text) => {
            const h = document.createElement('h3');
            h.className = 'parish-asset-group';
            h.textContent = text;
            grid.appendChild(h);
        };
        // The file already in use goes first, so it's never lost at the bottom
        // of a long list.
        const current = pickerCurrentId && !terms.length ? list.find((a) => String(a.id) === pickerCurrentId) : null;
        if (current) {
            addHeading('Currently chosen');
            grid.appendChild(pickerCard(current, true));
        }
        // Date sorts get month headings so "the bulletin from August" is easy to find.
        const byDate = sort === 'newest' || sort === 'oldest';
        let lastGroup = null;
        list.forEach((a) => {
            if (a === current) return;
            const created = time(a) ? new Date(time(a)) : null;
            if (byDate) {
                const group = created ? created.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }) : 'Earlier';
                if (group !== lastGroup) {
                    addHeading(group);
                    lastGroup = group;
                }
            }
            grid.appendChild(pickerCard(a, false));
        });
    }

    function pickerCard(a, isCurrent) {
        const name = assetDisplayName(a);
        const created = Date.parse(a.created_at || '') ? new Date(a.created_at) : null;
        const card = document.createElement('button');
        card.type = 'button';
        card.className = 'parish-asset-card' + (isCurrent ? ' is-current' : '');
        card.title = name;
        const dateText = created ? created.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }) : '';
        const meta = `<span class="parish-asset-card-name">${escText(name)}</span>${dateText ? `<span class="parish-asset-card-date">${escText(dateText)}</span>` : ''}`;
        const icon = { pdf: 'fa-file-pdf', audio: 'fa-music', video: 'fa-video' }[a.kind] || 'fa-file';
        const iconHTML = `<span class="parish-asset-card-filetype"><i class="fas ${icon}"></i></span>`;
        if (a.kind === 'image') {
            card.innerHTML = `<img src="${escAttr(a.url)}" alt="" loading="lazy">${meta}`;
            // A file that isn't really a picture (one import was a Word file)
            // gets an icon instead of a broken-image box.
            card.querySelector('img').addEventListener('error', (e) => { e.target.outerHTML = iconHTML; }, { once: true });
        } else {
            card.innerHTML = iconHTML + meta;
        }
        if (pickerMulti) {
            const picked = pickerSelected.includes(a);
            card.classList.toggle('is-selected', picked);
            card.setAttribute('aria-pressed', picked ? 'true' : 'false');
            card.addEventListener('click', () => {
                const i = pickerSelected.indexOf(a);
                if (i >= 0) pickerSelected.splice(i, 1); else pickerSelected.push(a);
                card.classList.toggle('is-selected', i < 0);
                card.setAttribute('aria-pressed', i < 0 ? 'true' : 'false');
                updatePickerDone();
            });
            return card;
        }
        card.addEventListener('click', () => {
            if (typeof pickerPendingCallback === 'function') pickerPendingCallback(a);
            closeModal(document.getElementById('asset-picker'));
        });
        return card;
    }

    function updatePickerDone() {
        const done = document.getElementById('asset-picker-done');
        if (!done) return;
        const n = pickerSelected.length;
        done.disabled = !n;
        done.textContent = n ? `Add ${n} photo${n === 1 ? '' : 's'}` : 'Add photos';
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
        const form = document.getElementById('new-homily-form');
        const errBox = document.getElementById('new-homily-error');
        let editingId = null;

        newBtn.addEventListener('click', () => {
            editingId = null;
            setFormMode(form, { editing: false, title: 'New homily', fields: {} });
            openModal('new-homily-modal');
        });

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            const body = {
                preached_on: document.getElementById('homily-preached-on').value,
                title: document.getElementById('homily-title-input').value,
                author: document.getElementById('homily-author').value,
                body: document.getElementById('homily-body').value,
                audio_url: document.getElementById('homily-audio').value,
                video_url: document.getElementById('homily-video').value,
                document_url: document.getElementById('homily-document').value,
            };
            try {
                if (editingId) await apiFetch('/api/v1/admin/homilies/' + editingId, { method: 'PUT', body });
                else await apiFetch('/api/v1/admin/homilies', { method: 'POST', body });
                window.location.reload();
            } catch (err) { showError(errBox, err.message); }
        });

        document.getElementById('homily-list')?.addEventListener('click', async (e) => {
            const row = closest(e.target, '[data-row-id]');
            if (!row) return;
            if (e.target.closest('[data-action="edit-homily"]')) {
                const d = row.dataset;
                editingId = d.rowId;
                setFormMode(form, {
                    editing: true, title: 'Edit homily',
                    fields: {
                        'homily-preached-on': d.preachedOn, 'homily-title-input': d.title, 'homily-author': d.author,
                        'homily-body': d.body, 'homily-audio': d.audioUrl, 'homily-video': d.videoUrl, 'homily-document': d.documentUrl,
                    },
                });
                openModal('new-homily-modal');
                return;
            }
            if (e.target.closest('[data-action="delete-homily"]')) {
                if (!confirm('Delete this homily?')) return;
                try {
                    await apiFetch('/api/v1/admin/homilies/' + row.dataset.rowId, { method: 'DELETE' });
                    row.remove();
                } catch (err) { toast(err.message, 'error'); }
            }
        });
    }

    // ========== Bulletins ==========

    function initBulletins() {
        const newBtn = document.getElementById('new-bulletin-btn');
        if (!newBtn) return;
        const form = document.getElementById('new-bulletin-form');
        const errBox = document.getElementById('new-bulletin-error');
        const statusEl = document.getElementById('bulletin-upload-status');
        const pdfInput = document.getElementById('bulletin-pdf');
        const pdfLabel = document.getElementById('bulletin-pdf-label');
        const current = document.getElementById('bulletin-current-file');
        let editingId = null;

        function setMode(editing, fields, fileName) {
            setFormMode(form, { editing, title: editing ? 'Edit bulletin' : 'Upload bulletin', fields });
            pdfInput.required = !editing;
            if (pdfLabel) pdfLabel.textContent = editing ? 'Replace PDF (optional)' : 'PDF file';
            if (current) {
                current.hidden = !editing;
                current.textContent = editing ? 'Current file: ' + (fileName || 'PDF') : '';
            }
            if (statusEl) statusEl.textContent = '';
        }

        newBtn.addEventListener('click', () => {
            editingId = null;
            setMode(false, {});
            openModal('new-bulletin-modal');
        });

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            const file = pdfInput.files[0];
            if (!editingId && !file) { showError(errBox, 'Choose a PDF.'); return; }
            try {
                let assetId = 0;
                if (file) {
                    if (statusEl) statusEl.textContent = 'Uploading PDF…';
                    const fd = new FormData();
                    fd.append('file', file);
                    const uploadRes = await fetch('/api/v1/admin/assets', { method: 'POST', body: fd, credentials: 'same-origin' });
                    const uploadData = await uploadRes.json();
                    if (!uploadRes.ok) throw new Error(uploadData.error || 'Upload failed');
                    assetId = uploadData.id;
                }
                if (statusEl) statusEl.textContent = 'Saving…';
                const body = {
                    published_on: document.getElementById('bulletin-published-on').value,
                    label: document.getElementById('bulletin-label').value,
                    asset_id: assetId,
                };
                if (editingId) await apiFetch('/api/v1/admin/bulletins/' + editingId, { method: 'PUT', body });
                else await apiFetch('/api/v1/admin/bulletins', { method: 'POST', body });
                window.location.reload();
            } catch (err) {
                if (statusEl) statusEl.textContent = '';
                showError(errBox, err.message);
            }
        });

        document.getElementById('bulletin-list')?.addEventListener('click', async (e) => {
            const row = closest(e.target, '[data-row-id]');
            if (!row) return;
            if (e.target.closest('[data-action="edit-bulletin"]')) {
                editingId = row.dataset.rowId;
                setMode(true, { 'bulletin-published-on': row.dataset.publishedOn, 'bulletin-label': row.dataset.label }, row.dataset.fileName);
                openModal('new-bulletin-modal');
                return;
            }
            if (e.target.closest('[data-action="delete-bulletin"]')) {
                if (!confirm('Delete this bulletin?')) return;
                try {
                    await apiFetch('/api/v1/admin/bulletins/' + row.dataset.rowId, { method: 'DELETE' });
                    row.remove();
                } catch (err) { toast(err.message, 'error'); }
            }
        });
    }

    // ========== Events ==========

    function initEvents() {
        const newBtn = document.getElementById('new-event-btn');
        if (!newBtn) return;
        const form = document.getElementById('new-event-form');
        const errBox = document.getElementById('new-event-error');
        let editingId = null;

        newBtn.addEventListener('click', () => {
            editingId = null;
            setFormMode(form, { editing: false, title: 'New event', fields: {} });
            openModal('new-event-modal');
        });

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            const body = {
                slug: document.getElementById('event-slug').value,
                title: document.getElementById('event-title-input').value,
                location: document.getElementById('event-location').value,
                starts_at: document.getElementById('event-starts-at').value,
                ends_at: document.getElementById('event-ends-at').value,
                body: document.getElementById('event-body').value,
                featured: document.getElementById('event-featured').checked,
                tag: document.getElementById('event-tag').value,
                flyer_asset_id: parseInt(document.getElementById('event-flyer-id').value, 10) || 0,
            };
            try {
                if (editingId) await apiFetch('/api/v1/admin/events/' + editingId, { method: 'PUT', body });
                else await apiFetch('/api/v1/admin/events', { method: 'POST', body });
                window.location.reload();
            } catch (err) { showError(errBox, err.message); }
        });

        document.getElementById('event-list')?.addEventListener('click', async (e) => {
            const row = closest(e.target, '[data-row-id]');
            if (!row) return;
            if (e.target.closest('[data-action="edit-event"]')) {
                const d = row.dataset;
                editingId = d.rowId;
                setFormMode(form, {
                    editing: true, title: 'Edit event',
                    fields: {
                        'event-slug': d.slug, 'event-title-input': d.title, 'event-location': d.location,
                        'event-starts-at': d.startsAt, 'event-ends-at': d.endsAt, 'event-body': d.body,
                        'event-featured': d.featured === '1', 'event-tag': d.tag, 'event-flyer-id': d.flyerId,
                    },
                });
                if (d.flyerId) syncFileChoice('event-flyer-id', d.flyerName || fileNameFromURL(d.flyerUrl), d.flyerUrl);
                openModal('new-event-modal');
                return;
            }
            if (e.target.closest('[data-action="delete-event"]')) {
                if (!confirm('Delete this event?')) return;
                try {
                    await apiFetch('/api/v1/admin/events/' + row.dataset.rowId, { method: 'DELETE' });
                    row.remove();
                } catch (err) { toast(err.message, 'error'); }
            }
        });
    }

    // ========== Users (admins only) ==========

    function initUsers() {
        const table = document.getElementById('user-table');
        const newBtn = document.getElementById('new-user-btn');
        if (!table || !newBtn) return;
        const form = document.getElementById('new-user-form');
        const errBox = document.getElementById('new-user-error');

        newBtn.addEventListener('click', () => { form.reset(); hide(errBox); openModal('new-user-modal'); });
        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            hide(errBox);
            try {
                await apiFetch('/api/v1/admin/users', {
                    method: 'POST',
                    body: {
                        first_name: document.getElementById('nu-first').value.trim(),
                        last_name: document.getElementById('nu-last').value.trim(),
                        email: document.getElementById('nu-email').value.trim(),
                        role: document.getElementById('nu-role').value,
                        password: document.getElementById('nu-password').value,
                    },
                });
                window.location.reload();
            } catch (err) { showError(errBox, err.message); }
        });

        table.addEventListener('change', async (e) => {
            const sel = e.target.closest('[data-action="set-role"]');
            if (!sel) return;
            const row = closest(sel, '[data-user-id]');
            try {
                await apiFetch(`/api/v1/admin/users/${row.dataset.userId}/role`, { method: 'PUT', body: { role: sel.value } });
                toast('Role updated', 'ok');
            } catch (err) { toast(err.message, 'error'); }
        });

        table.addEventListener('click', async (e) => {
            const btn = e.target.closest('[data-action]');
            if (!btn || btn.tagName === 'SELECT') return;
            const row = closest(btn, '[data-user-id]');
            const id = row.dataset.userId;
            const name = row.dataset.userName.trim();
            try {
                if (btn.dataset.action === 'set-active') {
                    const active = btn.dataset.active === 'true';
                    if (!active && !confirm(`Deactivate ${name}? They will not be able to sign in.`)) return;
                    await apiFetch(`/api/v1/admin/users/${id}/status`, { method: 'PUT', body: { is_active: active } });
                    window.location.reload();
                } else if (btn.dataset.action === 'reset-password') {
                    const pw = prompt(`New temporary password for ${name} (at least 10 characters):`);
                    if (pw == null) return;
                    await apiFetch(`/api/v1/admin/users/${id}/password`, { method: 'PUT', body: { password: pw } });
                    toast('Password updated. Share it with them privately.', 'ok');
                }
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
    function slugifyText(s) {
        return String(s || '').normalize('NFKD').replace(/[̀-ͯ]/g, '')
            .toLowerCase().replace(/['’]/g, '').replace(/[^a-z0-9]+/g, '-')
            .replace(/^-+|-+$/g, '').slice(0, 80).replace(/-+$/, '');
    }

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
                if (q && !((a.name || a.original_name || '').toLowerCase().includes(q) ||
                           (a.url || '').toLowerCase().includes(q))) return false;
                return true;
            });
            if (filtered.length === 0) {
                grid.innerHTML = '<p class="parish-admin-empty">No matches. Try a different filter or search.</p>';
                return;
            }
            grid.innerHTML = filtered.map(a => {
                const name = escapeHtml(a.name || a.original_name || a.url.split('/').pop());
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
