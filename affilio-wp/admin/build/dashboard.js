( function() {
    const root = document.getElementById('affilio-dashboard-root');
    if (!root) return;
    const locale = String((AffilioData && AffilioData.locale) || document.documentElement.lang || 'en').toLowerCase();
    const isItalian = locale.startsWith('it');
    const t = (en, it) => (isItalian ? it : en);
    root.classList.add('affilio-dashboard-app');
    if (AffilioData.connectionStatus && !AffilioData.connectionStatus.connected) {
        const settings = AffilioData.settingsUrl || '/wp-admin/admin.php?page=affilio-settings';
        root.innerHTML = `
            <div class="affilio-panel affilio-panel-warning">
                <h2>Affilio Dashboard</h2>
                <p><strong>${t('Not connected.', 'Non connesso.')}</strong> ${AffilioData.connectionStatus.message || t('Configure your API key.', 'Configura la tua chiave API.')}</p>
                <p><a class="button button-primary" href="${settings}">${t('Open settings', 'Apri impostazioni')}</a></p>
            </div>
        `;
        return;
    }
    root.innerHTML = `<div class="affilio-panel"><h2>Affilio Dashboard</h2><p>${t('Loading data...', 'Caricamento dati...')}</p></div>`;

    const scoreColor = (score) => {
        if (score < 34) return '#d63638';
        if (score < 67) return '#dba617';
        return '#2a9d5b';
    };

    const potentialLabel = (score) => {
        if (score < 34) return t('Low', 'Basso');
        if (score < 67) return t('Medium', 'Medio');
        return t('High', 'Alto');
    };

    const computeOpportunityItems = (articles, recommendations) => {
        const recsByArticle = {};
        (recommendations || []).forEach((r) => {
            const articleId = r && r.article_id ? r.article_id : '';
            if (!articleId) return;
            if (!recsByArticle[articleId]) recsByArticle[articleId] = [];
            recsByArticle[articleId].push(r);
        });
        return (articles || []).map((a) => {
            const recs = recsByArticle[a.id] || [];
            const actionable = recs.filter((r) => {
                const status = String(r.status || 'pending').toLowerCase();
                return status === 'pending' || status === 'approved';
            });
            const pending = actionable.filter((r) => String(r.status || '').toLowerCase() === 'pending').length;
            const approved = actionable.filter((r) => String(r.status || '').toLowerCase() === 'approved').length;
            const bestRelevance = actionable.reduce((acc, r) => Math.max(acc, Number(r.relevance_score || 0)), 0);
            const score = Number(a.monetization_score || 0);
            const priority = score + (pending * 16) + (approved * 8) + Math.round(bestRelevance / 6);
            return {
                articleId: a.id,
                title: a.title || 'Untitled',
                postId: a.wp_post_id || '',
                score,
                pending,
                approved,
                actionableCount: actionable.length,
                bestRelevance,
                priority,
            };
        })
            .filter((item) => item.actionableCount > 0 || item.score >= 55)
            .sort((a, b) => b.priority - a.priority)
            .slice(0, 6);
    };

    const renderOpportunityPanel = (articles, recommendations) => {
        const items = computeOpportunityItems(articles, recommendations);
        if (!items.length) {
            return `
                <div class="affilio-panel">
                    <h3>${t('Quick Wins', 'Azioni rapide')}</h3>
                    <p>${t('No high-priority actions right now. Run a new scan after publishing more content.', 'Nessuna azione prioritaria al momento. Esegui una nuova scansione dopo aver pubblicato altri contenuti.')}</p>
                </div>
            `;
        }
        const cards = items.map((item) => {
            const pendingLabel = isItalian
                ? (item.pending === 1 ? 'raccomandazione in attesa' : 'raccomandazioni in attesa')
                : (item.pending === 1 ? 'pending recommendation' : 'pending recommendations');
            const approvedLabel = isItalian
                ? (item.approved === 1 ? 'raccomandazione approvata pronta da inserire' : 'raccomandazioni approvate pronte da inserire')
                : (item.approved === 1 ? 'approved recommendation ready to insert' : 'approved recommendations ready to insert');
            const subtitle = item.pending > 0
                ? `${item.pending} ${pendingLabel}`
                : `${item.approved} ${approvedLabel}`;
            return `
                <div class="affilio-opportunity-card">
                    <div class="affilio-opportunity-head">
                        <strong>${esc(item.title)}</strong>
                        <span class="affilio-opportunity-score">${t('Score', 'Punteggio')} ${item.score}</span>
                    </div>
                    <div class="affilio-opportunity-sub">${esc(subtitle)}</div>
                    <div class="affilio-opportunity-meta">Post ID: ${esc(item.postId)} · ${t('Relevance up to', 'Rilevanza fino a')} ${esc(item.bestRelevance)}</div>
                    <button class="button button-primary" data-jump-article="${esc(item.articleId)}">${t('Review now', 'Rivedi ora')}</button>
                </div>
            `;
        }).join('');
        return `
            <div class="affilio-panel">
                <h3>${t('Quick Wins', 'Azioni rapide')}</h3>
                <p>${t('Highest-impact opportunities based on score, recommendation quality, and actionability.', 'Opportunità a maggior impatto basate su punteggio, qualità raccomandazioni e facilità di azione.')}</p>
                <div class="affilio-opportunity-grid">${cards}</div>
            </div>
        `;
    };

    const renderToast = (message) => {
        const toast = document.createElement('div');
        toast.textContent = message;
        toast.style.position = 'fixed';
        toast.style.right = '16px';
        toast.style.bottom = '16px';
        toast.style.background = '#1d2327';
        toast.style.color = '#fff';
        toast.style.padding = '8px 12px';
        toast.style.borderRadius = '4px';
        toast.style.zIndex = 1000;
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 2000);
    };

    const esc = (value) => {
        return String(value || '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    };

    const textFromHTML = (value) => {
        const div = document.createElement('div');
        div.innerHTML = value || '';
        return (div.textContent || div.innerText || '').trim();
    };

    const positionLabel = (position) => {
        switch ((position || '').toLowerCase()) {
            case 'after_intro':
                return t('After intro', 'Dopo introduzione');
            case 'mid_article':
                return t('Mid article', 'Metà articolo');
            case 'before_conclusion':
                return t('Before conclusion', 'Prima della conclusione');
            case 'end_of_article':
                return t('End of article', 'Fine articolo');
            default:
                return t('Mid article', 'Metà articolo');
        }
    };

    const normalizePosition = (position) => {
        const p = String(position || '').toLowerCase();
        if (p === 'after_intro' || p === 'mid_article' || p === 'before_conclusion' || p === 'end_of_article') {
            return p;
        }
        return 'mid_article';
    };

    const statusLabel = (status) => {
        const s = String(status || '').toLowerCase();
        if (s === 'pending') return t('pending', 'in attesa');
        if (s === 'approved') return t('approved', 'approvata');
        if (s === 'dismissed') return t('dismissed', 'scartata');
        if (s === 'inserted') return t('inserted', 'inserita');
        return s || t('pending', 'in attesa');
    };

    const impactLabel = (impact) => {
        const i = String(impact || '').toLowerCase();
        if (i === 'high') return t('High click potential', 'Alto potenziale di click');
        if (i === 'medium') return t('Medium click potential', 'Medio potenziale di click');
        return t('Low click potential', 'Basso potenziale di click');
    };

    const safeHref = (value) => {
        const url = String(value || '').trim();
        if (url.startsWith('https://') || url.startsWith('http://')) {
            return url;
        }
        return '#';
    };

    const buildSnippetHTML = (programName, linkURL, generatedText) => {
        const name = esc(programName || t('Recommended tool', 'Strumento consigliato'));
        const href = esc(safeHref(linkURL));
        const text = esc(generatedText || t('A solid option to consider.', 'Un\'opzione solida da considerare.'));
        return `<p><a href="${href}" rel="nofollow sponsored">${name}</a> — ${text}</p>`;
    };

    const placementContextLine = (articleContext, position) => {
        const chunks = String(articleContext || '')
            .split(/\n\s*\n/)
            .map((s) => s.trim())
            .filter(Boolean);
        if (!chunks.length) {
            return t('No local article context available from WordPress API.', 'Contesto locale articolo non disponibile tramite API WordPress.');
        }
        if (position === 'after_intro') {
            return chunks[Math.min(1, chunks.length - 1)];
        }
        if (position === 'before_conclusion') {
            return chunks[Math.max(0, chunks.length - 2)];
        }
        if (position === 'end_of_article') {
            return chunks[chunks.length - 1];
        }
        return chunks[Math.floor(chunks.length / 2)];
    };

    const render = (articles, programs, recommendations) => {
        let html = `<div class="affilio-panel"><h2>Affilio Dashboard</h2><p>${t('Review article monetization and manage recommendations.', 'Controlla la monetizzazione degli articoli e gestisci le raccomandazioni.')}</p></div>`;
        if (!articles.length) {
            html += `<div class="affilio-panel"><p>${t('No articles scanned yet.', 'Nessun articolo ancora scansionato.')}</p></div>`;
        } else {
            html += renderOpportunityPanel(articles, recommendations);
            let rows = articles.map(a => {
                const score = a.monetization_score || 0;
                return `<tr data-article-id="${a.id}"><td>${a.title || ''}</td><td style="color:${scoreColor(score)};font-weight:bold;">${score}</td><td>${potentialLabel(score)}</td><td>${a.wp_post_id}</td></tr>`;
            }).join('');
            html += `
                <div class="affilio-panel">
                    <h3>${t('Articles', 'Articoli')}</h3>
                    <table class="widefat" id="affilio-articles-table">
                        <thead><tr><th>${t('Title', 'Titolo')}</th><th>${t('Score', 'Punteggio')}</th><th>${t('Potential', 'Potenziale')}</th><th>Post ID</th></tr></thead>
                        <tbody>${rows}</tbody>
                    </table>
                    <div id="affilio-article-detail" class="affilio-detail"></div>
                </div>
            `;
        }
        html += `
            <div class="affilio-panel">
                <h3>${t('Analytics', 'Analitiche')}</h3>
                <div id="affilio-analytics"></div>
            </div>
        `;
        html += `
            <div class="affilio-panel">
                <h3>${t('My Affiliate Links', 'I miei link affiliati')}</h3>
                <form id="affilio-add-program" class="affilio-inline-form">
                    <input type="text" name="name" placeholder="${t('Program name', 'Nome programma')}" required />
                    <input type="text" name="domain" placeholder="${t('Domain', 'Dominio')}" required />
                    <input type="text" name="link" placeholder="${t('Affiliate link', 'Link affiliato')}" />
                    <button class="button button-secondary">${t('Add', 'Aggiungi')}</button>
                </form>
        `;
        if (!programs.length) {
            html += `<p>${t('No affiliate links detected yet.', 'Nessun link affiliato rilevato al momento.')}</p>`;
        } else {
            let rows = programs.map(p => `<li>${p.name} (${p.domain || t('unknown', 'sconosciuto')})</li>`).join('');
            html += `<ul>${rows}</ul>`;
        }
        html += '</div>';
        root.innerHTML = html;
        setupAnalytics();

        const table = document.getElementById('affilio-articles-table');
        const detail = document.getElementById('affilio-article-detail');
        root.querySelectorAll('button[data-jump-article]').forEach((btn) => {
            btn.addEventListener('click', () => {
                if (!table) return;
                const articleId = btn.getAttribute('data-jump-article');
                const row = table.querySelector(`tr[data-article-id="${articleId}"]`);
                if (!row) return;
                row.scrollIntoView({ behavior: 'smooth', block: 'center' });
                row.classList.add('affilio-row-focus');
                setTimeout(() => row.classList.remove('affilio-row-focus'), 1500);
                row.click();
            });
        });
        if (table && detail) {
            table.querySelectorAll('tr[data-article-id]').forEach(row => {
                row.addEventListener('click', async () => {
                    const id = row.getAttribute('data-article-id');
                    detail.innerHTML = `<p>${t('Loading details...', 'Caricamento dettagli...')}</p>`;
                    const res = await fetch(AffilioData.apiUrl + '/articles/' + id, {
                        headers: { 'X-API-Key': AffilioData.apiKey }
                    });
                    const data = await res.json();
                    const recommendations = data.recommendations || [];
                    const recMap = {};
                    recommendations.forEach((r) => { recMap[r.id] = r; });
                    const generatedPlans = {};
                    let articleContextCache = null;

                    const recs = recommendations.map((r) => {
                        const status = (r.status || 'pending').toLowerCase();
                        const disabledInsert = status === 'inserted' ? 'disabled' : '';
                        const disabledDismiss = status === 'dismissed' ? 'disabled' : '';
                        const impact = String(r.expected_impact || '').toLowerCase();
                        return `<li class="affilio-rec-item" data-rec-item="${esc(r.id)}">
                            <div class="affilio-rec-head">
                                <strong>${esc(r.program_name || r.id)}</strong>
                                <span class="affilio-rec-status affilio-rec-status-${esc(status)}">${esc(statusLabel(status))}</span>
                            </div>
                            <p class="affilio-rec-reason">${esc(r.reason_summary || r.reason || t('Recommendation', 'Raccomandazione'))}</p>
                            <div class="affilio-rec-meta">
                                <span class="affilio-impact-badge affilio-impact-${esc(impact || 'low')}">${esc(impactLabel(impact))}</span>
                                <span><strong>${t('Why this fits', 'Perché è adatta')}:</strong> ${esc(r.reason || t('Relevant to this article context.', 'Rilevante per il contesto di questo articolo.'))}</span>
                            </div>
                            <div class="affilio-rec-meta">
                                <span><strong>${t('Placement logic', 'Logica di posizionamento')}:</strong> ${esc(r.placement_why || t('Placed near the most relevant context and click intent.', 'Posizionata vicino al contesto più rilevante e all’intento di click.'))}</span>
                            </div>
                            <div class="affilio-rec-actions">
                                <button class="button button-primary" data-rec="${esc(r.id)}" data-action="preview" ${disabledInsert}>${t('Preview Insert', 'Anteprima inserimento')}</button>
                                <button class="button" data-rec="${esc(r.id)}" data-action="dismiss" ${disabledDismiss}>${t('Dismiss', 'Scarta')}</button>
                            </div>
                            <div class="affilio-rec-preview" id="affilio-rec-preview-${esc(r.id)}"></div>
                        </li>`;
                    }).join('');
                    detail.innerHTML = `<h3>${t('Recommendations', 'Raccomandazioni')}</h3>${recs ? `<ul class="affilio-rec-list">${recs}</ul>` : `<p>${t('No recommendations yet.', 'Nessuna raccomandazione al momento.')}</p>`}`;

                    const loadArticleContext = async () => {
                        if (articleContextCache !== null) {
                            return articleContextCache;
                        }
                        try {
                            const wpRes = await fetch(`/wp-json/wp/v2/posts/${data.article.wp_post_id}?context=edit&_fields=content.rendered,excerpt.rendered,title.rendered`);
                            if (!wpRes.ok) {
                                articleContextCache = '';
                                return articleContextCache;
                            }
                            const wpPost = await wpRes.json();
                            const content = textFromHTML((wpPost && wpPost.content && wpPost.content.rendered) || '');
                            const excerpt = textFromHTML((wpPost && wpPost.excerpt && wpPost.excerpt.rendered) || '');
                            articleContextCache = (content || excerpt).slice(0, 1400);
                            return articleContextCache;
                        } catch (_) {
                            articleContextCache = '';
                            return articleContextCache;
                        }
                    };

                    detail.onclick = async (event) => {
                        const btn = event.target.closest('button[data-rec]');
                        if (!btn) {
                            return;
                        }
                        const recId = btn.getAttribute('data-rec');
                        const action = btn.getAttribute('data-action');
                        const rec = recMap[recId] || {};
                        const previewEl = detail.querySelector(`#affilio-rec-preview-${recId}`);

                        if (action === 'dismiss') {
                            await fetch(`${AffilioData.apiUrl}/recommendations/${recId}`, {
                                method: 'PATCH',
                                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                                body: JSON.stringify({ status: 'dismissed' }),
                            });
                            btn.innerText = 'Dismissed';
                            btn.disabled = true;
                            const statusEl = detail.querySelector(`[data-rec-item="${recId}"] .affilio-rec-status`);
                            if (statusEl) {
                                statusEl.textContent = statusLabel('dismissed');
                                statusEl.className = 'affilio-rec-status affilio-rec-status-dismissed';
                            }
                            renderToast(t('Recommendation dismissed', 'Raccomandazione scartata'));
                            return;
                        }

                        if (action === 'preview' || action === 'regenerate') {
                            btn.disabled = true;
                            btn.innerText = action === 'preview' ? t('Generating...', 'Generazione in corso...') : t('Regenerating...', 'Rigenerazione in corso...');
                            const articleContext = await loadArticleContext();
                            const gen = await fetch(AffilioData.apiUrl + '/generate/content', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                                body: JSON.stringify({
                                    recommendation_id: recId,
                                    insertion_type: rec.insertion_type || 'inline_mention',
                                    article_context: articleContext,
                                    preferred_tone: 'match article author voice',
                                    site_language: document.documentElement.lang || 'auto',
                                }),
                            });
                            btn.disabled = false;
                            btn.innerText = action === 'preview' ? t('Preview Insert', 'Anteprima inserimento') : t('Regenerate', 'Rigenera');
                            if (!gen.ok) {
                                renderToast(t('Failed to generate insert preview', 'Impossibile generare l\'anteprima inserimento'));
                                return;
                            }
                            const genData = await gen.json();
                            generatedPlans[recId] = genData;
                            if (previewEl) {
                                const suggestedPosition = normalizePosition(genData.suggested_position);
                                const defaultText = genData.generated_text || textFromHTML(genData.html || '');
                                const baseHTML = buildSnippetHTML(
                                    genData.program_name || rec.program_name || rec.id,
                                    genData.link_url || '#',
                                    defaultText
                                );
                                const around = placementContextLine(articleContext, suggestedPosition);
                                previewEl.innerHTML = `
                                    <div class="affilio-plan-card">
                                        <div class="affilio-plan-meta">
                                            <strong>${t('Planned position', 'Posizione prevista')}:</strong> ${esc(positionLabel(suggestedPosition))}
                                            <select class="affilio-position-select" data-rec="${esc(recId)}">
                                                <option value="after_intro" ${suggestedPosition === 'after_intro' ? 'selected' : ''}>${t('After intro', 'Dopo introduzione')}</option>
                                                <option value="mid_article" ${suggestedPosition === 'mid_article' ? 'selected' : ''}>${t('Mid article', 'Metà articolo')}</option>
                                                <option value="before_conclusion" ${suggestedPosition === 'before_conclusion' ? 'selected' : ''}>${t('Before conclusion', 'Prima della conclusione')}</option>
                                                <option value="end_of_article" ${suggestedPosition === 'end_of_article' ? 'selected' : ''}>${t('End of article', 'Fine articolo')}</option>
                                            </select>
                                        </div>
                                        <div class="affilio-plan-meta">
                                            <strong>${t('Why here', 'Perché qui')}:</strong> ${esc(genData.placement_reason || t('Chosen for relevance and click potential.', 'Scelta per rilevanza e potenziale di click.'))}
                                        </div>
                                        <div class="affilio-plan-meta" id="affilio-plan-context-${esc(recId)}">
                                            <strong>${t('Local context', 'Contesto locale')}:</strong> ${esc(around)}
                                        </div>
                                        <label class="affilio-plan-label" for="affilio-generated-text-${esc(recId)}">${t('Edit generated text', 'Modifica testo generato')}</label>
                                        <textarea class="affilio-plan-editor" id="affilio-generated-text-${esc(recId)}" data-rec-text="${esc(recId)}" rows="3">${esc(defaultText)}</textarea>
                                        <div class="affilio-plan-preview-html" id="affilio-plan-html-${esc(recId)}">${baseHTML}</div>
                                        <div class="affilio-plan-actions">
                                            <button class="button button-primary" data-rec="${esc(recId)}" data-action="confirm-insert">${t('Insert now', 'Inserisci ora')}</button>
                                            <button class="button" data-rec="${esc(recId)}" data-action="regenerate">${t('Regenerate', 'Rigenera')}</button>
                                        </div>
                                    </div>
                                `;
                                const editorEl = previewEl.querySelector(`textarea[data-rec-text="${recId}"]`);
                                const posEl = previewEl.querySelector(`select[data-rec="${recId}"]`);
                                const htmlEl = previewEl.querySelector(`#affilio-plan-html-${recId}`);
                                const contextEl = previewEl.querySelector(`#affilio-plan-context-${recId}`);
                                const refreshPreview = () => {
                                    if (!editorEl || !posEl || !htmlEl) {
                                        return;
                                    }
                                    const liveText = String(editorEl.value || '').trim();
                                    const livePosition = normalizePosition(posEl.value);
                                    htmlEl.innerHTML = buildSnippetHTML(
                                        genData.program_name || rec.program_name || rec.id,
                                        genData.link_url || '#',
                                        liveText || defaultText
                                    );
                                    if (contextEl) {
                                        contextEl.innerHTML = `<strong>${t('Local context', 'Contesto locale')}:</strong> ${esc(placementContextLine(articleContext, livePosition))}`;
                                    }
                                };
                                if (editorEl) {
                                    editorEl.addEventListener('input', refreshPreview);
                                }
                                if (posEl) {
                                    posEl.addEventListener('change', refreshPreview);
                                }
                            }
                            return;
                        }

                        if (action === 'confirm-insert') {
                            const plan = generatedPlans[recId];
                            if (!plan || !plan.html) {
                                renderToast(t('Generate preview first', 'Genera prima l\'anteprima'));
                                return;
                            }
                            const textArea = detail.querySelector(`textarea[data-rec-text="${recId}"]`);
                            const positionSelect = detail.querySelector(`select[data-rec="${recId}"]`);
                            const editedText = textArea ? String(textArea.value || '').trim() : '';
                            const chosenPosition = normalizePosition(positionSelect ? positionSelect.value : (plan.suggested_position || 'mid_article'));
                            const finalHTML = editedText !== ''
                                ? buildSnippetHTML(plan.program_name || rec.program_name || rec.id, plan.link_url || '#', editedText)
                                : (plan.html || '');
                            const approve = await fetch(`${AffilioData.apiUrl}/recommendations/${recId}`, {
                                method: 'PATCH',
                                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                                body: JSON.stringify({ status: 'approved' }),
                            });
                            if (!approve.ok && approve.status !== 409) {
                                renderToast(t('Failed to approve recommendation', 'Impossibile approvare la raccomandazione'));
                                return;
                            }
                            const insert = await fetch('/wp-json/affilio/v1/insert', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': AffilioData.nonce },
                                body: JSON.stringify({
                                    post_id: data.article.wp_post_id,
                                    html: finalHTML,
                                    recommendation_id: recId,
                                    suggested_position: chosenPosition,
                                }),
                            });
                            const insertData = await insert.json();
                            if (!insert.ok || insertData.status !== 'ok') {
                                renderToast((insertData && insertData.message) ? insertData.message : t('Insert failed', 'Inserimento non riuscito'));
                                return;
                            }
                            if (previewEl) {
                                previewEl.innerHTML = `<p class="affilio-rec-done">${t('Inserted successfully.', 'Inserito con successo.')}</p>`;
                            }
                            const statusEl = detail.querySelector(`[data-rec-item="${recId}"] .affilio-rec-status`);
                            if (statusEl) {
                                statusEl.textContent = statusLabel('inserted');
                                statusEl.className = 'affilio-rec-status affilio-rec-status-inserted';
                            }
                            detail.querySelectorAll(`button[data-rec="${recId}"]`).forEach((actionBtn) => {
                                if (actionBtn.getAttribute('data-action') !== 'dismiss') {
                                    actionBtn.disabled = true;
                                }
                            });
                            renderToast(t('Recommendation inserted', 'Raccomandazione inserita'));
                        }
                    };
                });
            });
        }

        const form = document.getElementById('affilio-add-program');
        if (form) {
            form.addEventListener('submit', async (e) => {
                e.preventDefault();
                const formData = new FormData(form);
                await fetch(AffilioData.apiUrl + '/user-programs', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-API-Key': AffilioData.apiKey,
                    },
                    body: JSON.stringify({
                        name: formData.get('name'),
                        domain: formData.get('domain'),
                        affiliate_link: formData.get('link'),
                    }),
                });
                location.reload();
            });
        }
    };

    const renderArticleClicks = (rows) => {
        if (!rows.length) {
            return `<p>${t('No click data yet.', 'Nessun click registrato al momento.')}</p>`;
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.title || 'Untitled'}</td>
                <td>${r.wp_post_id}</td>
                <td class="affilio-num">${r.clicks}</td>
            </tr>
        `).join('');
        return `
            <table class="widefat">
                <thead><tr><th>${t('Article', 'Articolo')}</th><th>Post ID</th><th>${t('Clicks', 'Click')}</th></tr></thead>
                <tbody>${tableRows}</tbody>
            </table>
        `;
    };

    const renderRecommendationClicks = (rows) => {
        if (!rows.length) {
            return `<p>${t('No click data yet.', 'Nessun click registrato al momento.')}</p>`;
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.program_name || r.recommendation_id}</td>
                <td>${r.program_type || ''}</td>
                <td class="affilio-num">${r.clicks}</td>
            </tr>
        `).join('');
        return `
            <table class="widefat">
                <thead><tr><th>${t('Recommendation', 'Raccomandazione')}</th><th>${t('Type', 'Tipo')}</th><th>${t('Clicks', 'Click')}</th></tr></thead>
                <tbody>${tableRows}</tbody>
            </table>
        `;
    };

    const renderSummary = (summary) => {
        const total = summary && typeof summary.total_clicks === 'number' ? summary.total_clicks : 0;
        const uniqueArticles = summary && typeof summary.unique_articles === 'number' ? summary.unique_articles : 0;
        const uniqueRecs = summary && typeof summary.unique_recommendations === 'number' ? summary.unique_recommendations : 0;
        return `
            <div class="affilio-metric-grid">
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Total Clicks', 'Click totali')}</div>
                    <div class="affilio-metric-value">${total}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Clicked Articles', 'Articoli con click')}</div>
                    <div class="affilio-metric-value">${uniqueArticles}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Clicked Recs', 'Raccomandazioni con click')}</div>
                    <div class="affilio-metric-value">${uniqueRecs}</div>
                </div>
            </div>
        `;
    };

    const renderTrends = (rows) => {
        if (!rows.length) {
            return `<p class="affilio-empty-inline">${t('No trend data in this range.', 'Nessun dato trend in questo intervallo.')}</p>`;
        }
        const max = rows.reduce((acc, r) => Math.max(acc, r.clicks || 0), 0) || 1;
        const points = rows.map(r => {
            const width = Math.max(2, Math.round(((r.clicks || 0) / max) * 100));
            return `
                <div class="affilio-trend-row">
                    <span class="affilio-trend-day">${r.day}</span>
                    <div class="affilio-trend-track">
                        <div class="affilio-trend-fill" style="width:${width}%;"></div>
                    </div>
                    <span class="affilio-trend-value">${r.clicks || 0}</span>
                </div>
            `;
        }).join('');
        return `<div class="affilio-trend-box">${points}</div>`;
    };

    const renderAttribution = (rows, meta) => {
        if (!rows.length) {
            return `<p class="affilio-empty-block">${t('No attribution data in this range.', 'Nessun dato attribuzione in questo intervallo.')}</p>`;
        }
        const totalRevenue = meta && typeof meta.estimated_total_revenue_usd === 'number' ? meta.estimated_total_revenue_usd : 0;
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.program_name || t('Unknown program', 'Programma sconosciuto')}</td>
                <td>${r.program_type || ''}</td>
                <td>${(r.attributed_clicks || 0).toFixed(2)}</td>
                <td>${(r.click_share_percent || 0).toFixed(2)}%</td>
                <td>$${(r.estimated_revenue_usd || 0).toFixed(2)}</td>
            </tr>
        `).join('');
        const modelLabel = meta && meta.model ? meta.model : '';
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Attribution (Estimated)', 'Attribuzione (stimata)')}</h4>
                <div class="affilio-meta-line">
                    ${t('Estimated total revenue', 'Ricavo totale stimato')}: <strong>$${totalRevenue.toFixed(2)}</strong>
                </div>
                <div class="affilio-meta-line">
                    ${t('Model', 'Modello')}: ${modelLabel}
                </div>
                <table class="widefat">
                    <thead><tr><th>${t('Program', 'Programma')}</th><th>${t('Type', 'Tipo')}</th><th>${t('Attributed Clicks', 'Click attribuiti')}</th><th>${t('Share', 'Quota')}</th><th>${t('Est. Revenue', 'Ricavo stimato')}</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderFunnel = (funnel) => {
        if (!funnel) {
            return `<p class="affilio-empty-block">${t('No funnel data in this range.', 'Nessun dato funnel in questo intervallo.')}</p>`;
        }
        const clicks = funnel.clicks || 0;
        const inserted = funnel.inserted || 0;
        const dismissed = funnel.dismissed || 0;
        const tracked = funnel.tracked_suggestions || 0;
        const clickRate = (funnel.insert_to_click_rate || 0).toFixed(2);
        const dismissRate = (funnel.dismiss_rate || 0).toFixed(2);
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Funnel', 'Funnel')}</h4>
                <div class="affilio-metric-grid">
                    <div class="affilio-metric-card">
                        <div class="affilio-metric-label">${t('Inserted', 'Inseriti')}</div>
                        <div class="affilio-metric-value affilio-metric-value-sm">${inserted}</div>
                    </div>
                    <div class="affilio-metric-card">
                        <div class="affilio-metric-label">${t('Clicks', 'Click')}</div>
                        <div class="affilio-metric-value affilio-metric-value-sm">${clicks}</div>
                    </div>
                    <div class="affilio-metric-card">
                        <div class="affilio-metric-label">${t('Dismissed', 'Scartati')}</div>
                        <div class="affilio-metric-value affilio-metric-value-sm">${dismissed}</div>
                    </div>
                    <div class="affilio-metric-card">
                        <div class="affilio-metric-label">${t('Tracked Recs', 'Raccomandazioni tracciate')}</div>
                        <div class="affilio-metric-value affilio-metric-value-sm">${tracked}</div>
                    </div>
                    <div class="affilio-metric-card">
                        <div class="affilio-metric-label">${t('Insert -> Click Rate', 'Tasso Inserimento -> Click')}</div>
                        <div class="affilio-metric-value affilio-metric-value-sm">${clickRate}%</div>
                    </div>
                    <div class="affilio-metric-card">
                        <div class="affilio-metric-label">${t('Dismiss Rate', 'Tasso di scarto')}</div>
                        <div class="affilio-metric-value affilio-metric-value-sm">${dismissRate}%</div>
                    </div>
                </div>
            </div>
        `;
    };

    const renderCohorts = (rows) => {
        if (!rows || !rows.length) {
            return `<p class="affilio-empty-block">${t('No cohort data in this range.', 'Nessun dato coorte in questo intervallo.')}</p>`;
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.cohort_date || ''}</td>
                <td>${r.articles || 0}</td>
                <td>${r.inserted || 0}</td>
                <td>${r.clicks || 0}</td>
                <td>${(r.clicks_per_article || 0).toFixed(2)}</td>
            </tr>
        `).join('');
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Cohorts (By Article Publish Date)', 'Coorti (per data pubblicazione articolo)')}</h4>
                <table class="widefat">
                    <thead><tr><th>${t('Cohort Date', 'Data coorte')}</th><th>${t('Articles', 'Articoli')}</th><th>${t('Inserted', 'Inseriti')}</th><th>${t('Clicks', 'Click')}</th><th>${t('Clicks/Article', 'Click/Articolo')}</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderRetention = (rows) => {
        if (!rows || !rows.length) {
            return `<p class="affilio-empty-block">${t('No retention data in this range.', 'Nessun dato retention in questo intervallo.')}</p>`;
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.cohort_week || ''}</td>
                <td>${r.articles || 0}</td>
                <td>${r.week0_active || 0}</td>
                <td>${(r.week1_rate || 0).toFixed(2)}%</td>
                <td>${(r.week2_rate || 0).toFixed(2)}%</td>
                <td>${(r.week3_rate || 0).toFixed(2)}%</td>
                <td>${(r.week3_churn || 0).toFixed(2)}%</td>
            </tr>
        `).join('');
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Retention/Churn (Weekly)', 'Retention/Churn (settimanale)')}</h4>
                <table class="widefat">
                    <thead><tr><th>${t('Cohort Week', 'Settimana coorte')}</th><th>${t('Articles', 'Articoli')}</th><th>Week0 ${t('Active', 'attivi')}</th><th>W1 ${t('Retention', 'Retention')}</th><th>W2 ${t('Retention', 'Retention')}</th><th>W3 ${t('Retention', 'Retention')}</th><th>W3 ${t('Churn', 'Churn')}</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderRecommendationQuality = (summary, rows) => {
        if (!summary && (!rows || !rows.length)) {
            return `<p class="affilio-empty-block">${t('No recommendation quality data in this range.', 'Nessun dato qualità raccomandazioni in questo intervallo.')}</p>`;
        }
        const s = summary || {};
        const totals = `
            <div class="affilio-metric-grid affilio-metric-grid-tight">
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Total Recs', 'Raccomandazioni totali')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${s.total || 0}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Approval Rate', 'Tasso approvazione')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${(s.approval_rate || 0).toFixed(2)}%</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Dismiss Rate', 'Tasso di scarto')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${(s.dismiss_rate || 0).toFixed(2)}%</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Insert Rate', 'Tasso inserimento')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${(s.insert_rate || 0).toFixed(2)}%</div>
                </div>
            </div>
        `;
        const tableRows = (rows || []).map(r => `
            <tr>
                <td>${r.program_name || t('Unknown program', 'Programma sconosciuto')}</td>
                <td>${r.program_type || ''}</td>
                <td>${r.total || 0}</td>
                <td>${(r.approval_rate || 0).toFixed(2)}%</td>
                <td>${(r.dismiss_rate || 0).toFixed(2)}%</td>
                <td>${(r.insert_rate || 0).toFixed(2)}%</td>
                <td>${(r.click_rate || 0).toFixed(2)}%</td>
            </tr>
        `).join('');
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Recommendation Quality', 'Qualità raccomandazioni')}</h4>
                ${totals}
                <table class="widefat">
                    <thead><tr><th>${t('Program', 'Programma')}</th><th>${t('Type', 'Tipo')}</th><th>${t('Total', 'Totale')}</th><th>${t('Approve %', 'Approva %')}</th><th>${t('Dismiss %', 'Scarta %')}</th><th>${t('Insert %', 'Inserisci %')}</th><th>${t('Click %', 'Click %')}</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderRecommendationTuning = (rows) => {
        if (!rows || !rows.length) {
            return `<p class="affilio-empty-block">${t('No tuning profile data yet (needs at least 3 outcomes per program).', 'Nessun profilo tuning disponibile (servono almeno 3 esiti per programma).')}</p>`;
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.program_name || r.program_id || ''}</td>
                <td class="affilio-muted">${r.program_id || ''}</td>
                <td>${r.is_user_program ? 'user' : 'curated'}</td>
                <td>${r.sample_size || 0}</td>
                <td>${(r.approval_rate || 0).toFixed(2)}%</td>
                <td>${(r.insert_rate || 0).toFixed(2)}%</td>
                <td>${(r.click_rate || 0).toFixed(2)}%</td>
                <td>${(r.multiplier || 0).toFixed(2)}x</td>
            </tr>
        `).join('');
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Recommendation Tuning Profiles', 'Profili tuning raccomandazioni')}</h4>
                <table class="widefat">
                    <thead><tr><th>${t('Program', 'Programma')}</th><th>Program ID</th><th>${t('Type', 'Tipo')}</th><th>${t('Sample', 'Campione')}</th><th>${t('Approve %', 'Approva %')}</th><th>${t('Insert %', 'Inserisci %')}</th><th>${t('Click %', 'Click %')}</th><th>${t('Multiplier', 'Moltiplicatore')}</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderTuningInsights = (insights) => {
        if (!insights || !insights.total_profiles) {
            return `
                <div class="affilio-analytics-section">
                    <h4>${t('Learning Insights', 'Insight di apprendimento')}</h4>
                    <p class="affilio-empty-block">${t('No learning insights yet. Keep approving/dismissing recommendations to train ranking.', 'Nessun insight disponibile. Continua ad approvare/scartare raccomandazioni per allenare il ranking.')}</p>
                </div>
            `;
        }
        const cards = `
            <div class="affilio-metric-grid affilio-metric-grid-tight">
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Profiles', 'Profili')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${insights.total_profiles || 0}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Boosted', 'Potenziati')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${insights.boosted_count || 0}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Penalized', 'Penalizzati')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${insights.penalized_count || 0}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('High confidence', 'Alta confidenza')}</div>
                    <div class="affilio-metric-value affilio-metric-value-sm">${insights.high_confidence_profiles || 0}</div>
                </div>
            </div>
        `;
        const boosted = (insights.top_boosted || []).map((item) => `
            <li>
                <strong>${esc(item.program_name || item.program_id || '')}</strong>
                <span class="affilio-muted">(${esc(item.program_type || '')})</span>
                <span> · ${(item.multiplier || 0).toFixed(2)}x</span>
                <div class="affilio-insight-reason">${esc(item.reason || '')}</div>
            </li>
        `).join('');
        const penalized = (insights.top_penalized || []).map((item) => `
            <li>
                <strong>${esc(item.program_name || item.program_id || '')}</strong>
                <span class="affilio-muted">(${esc(item.program_type || '')})</span>
                <span> · ${(item.multiplier || 0).toFixed(2)}x</span>
                <div class="affilio-insight-reason">${esc(item.reason || '')}</div>
            </li>
        `).join('');
        const notes = (insights.notes || []).map((n) => `<li>${esc(n)}</li>`).join('');
        return `
            <div class="affilio-analytics-section">
                <h4>${t('Learning Insights', 'Insight di apprendimento')}</h4>
                ${cards}
                <div class="affilio-insight-grid">
                    <div>
                        <h5>${t('Top boosted programs', 'Programmi più potenziati')}</h5>
                        ${boosted ? `<ul class="affilio-insight-list">${boosted}</ul>` : `<p class="affilio-empty-block">${t('No boosted programs yet.', 'Nessun programma potenziato al momento.')}</p>`}
                    </div>
                    <div>
                        <h5>${t('Top penalized programs', 'Programmi più penalizzati')}</h5>
                        ${penalized ? `<ul class="affilio-insight-list">${penalized}</ul>` : `<p class="affilio-empty-block">${t('No penalized programs yet.', 'Nessun programma penalizzato al momento.')}</p>`}
                    </div>
                </div>
                ${notes ? `<div class="affilio-insight-notes"><h5>${t('System notes', 'Note del sistema')}</h5><ul>${notes}</ul></div>` : ''}
            </div>
        `;
    };

    const setupAnalytics = () => {
        const container = document.getElementById('affilio-analytics');
        if (!container) return;
        let articleLimit = '10';
        let recLimit = '10';
        let articleSort = 'clicks';
        let recSort = 'clicks';
        let range = '30';
        let articleClicks = [];
        let recClicks = [];
        let summary = null;
        let trends = [];
        let attribution = [];
        let attributionMeta = null;
        let funnel = null;
        let cohorts = [];
        let retention = [];
        let qualitySummary = null;
        let qualityPrograms = [];
        let tuningProfiles = [];
        let tuningInsights = null;
        let attributionModel = 'last_click';

        const downloadExport = async (format) => {
            const query = new URLSearchParams();
            if (range !== '0') query.set('days', range);
            query.set('format', format);
            query.set('model', attributionModel);
            const res = await fetch(AffilioData.apiUrl + '/track/export?' + query.toString(), {
                headers: { 'X-API-Key': AffilioData.apiKey },
            });
            if (!res.ok) {
                throw new Error('export failed');
            }
            const blob = await res.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `affilio-analytics.${format}`;
            document.body.appendChild(a);
            a.click();
            a.remove();
            window.URL.revokeObjectURL(url);
        };

        const applySort = (rows, sortKey) => {
            const sorted = rows.slice();
            if (sortKey === 'title') {
                sorted.sort((a, b) => (a.title || '').localeCompare(b.title || ''));
            } else if (sortKey === 'name') {
                sorted.sort((a, b) => (a.program_name || '').localeCompare(b.program_name || ''));
            } else {
                sorted.sort((a, b) => (b.clicks || 0) - (a.clicks || 0));
            }
            return sorted;
        };

        const applyLimit = (rows, limit) => {
            if (limit === 'all') return rows;
            const n = parseInt(limit, 10);
            if (Number.isNaN(n) || n <= 0) return rows;
            return rows.slice(0, n);
        };

        const renderAnalytics = () => {
            const sortedArticles = applyLimit(applySort(articleClicks, articleSort), articleLimit);
            const sortedRecs = applyLimit(applySort(recClicks, recSort), recLimit);
            container.innerHTML = `
                <div class="affilio-controls-row">
                    <select id="affilio-attribution-model">
                        <option value="last_click">${t('Model: Last Click', 'Modello: ultimo click')}</option>
                        <option value="first_click">${t('Model: First Click', 'Modello: primo click')}</option>
                        <option value="linear_session">${t('Model: Linear Session', 'Modello: sessione lineare')}</option>
                    </select>
                    <button class="button" id="affilio-export-json">${t('Export JSON', 'Esporta JSON')}</button>
                    <button class="button" id="affilio-export-csv">${t('Export CSV', 'Esporta CSV')}</button>
                </div>
                ${renderSummary(summary)}
                ${renderTrends(trends)}
                ${renderFunnel(funnel)}
                ${renderCohorts(cohorts)}
                ${renderRetention(retention)}
                ${renderRecommendationQuality(qualitySummary, qualityPrograms)}
                ${renderTuningInsights(tuningInsights)}
                ${renderRecommendationTuning(tuningProfiles)}
                ${renderAttribution(attribution, attributionMeta)}
                <div class="affilio-split-grid">
                    <div class="affilio-subpanel">
                        <div class="affilio-subpanel-head">
                            <h4>${t('Top Articles', 'Articoli top')}</h4>
                            <div class="affilio-filter-group">
                                <select id="affilio-analytics-range">
                                    <option value="0">${t('All time', 'Sempre')}</option>
                                    <option value="7">${t('Last 7 days', 'Ultimi 7 giorni')}</option>
                                    <option value="30">${t('Last 30 days', 'Ultimi 30 giorni')}</option>
                                </select>
                                <select id="affilio-articles-sort">
                                    <option value="clicks">${t('Sort: Clicks', 'Ordina: click')}</option>
                                    <option value="title">${t('Sort: Title', 'Ordina: titolo')}</option>
                                </select>
                                <select id="affilio-articles-limit">
                                    <option value="10">Top 10</option>
                                    <option value="25">Top 25</option>
                                    <option value="all">${t('All', 'Tutti')}</option>
                                </select>
                            </div>
                        </div>
                        ${renderArticleClicks(sortedArticles)}
                    </div>
                    <div class="affilio-subpanel">
                        <div class="affilio-subpanel-head">
                            <h4>${t('Top Recommendations', 'Raccomandazioni top')}</h4>
                            <div class="affilio-filter-group">
                                <select id="affilio-recs-sort">
                                    <option value="clicks">${t('Sort: Clicks', 'Ordina: click')}</option>
                                    <option value="name">${t('Sort: Name', 'Ordina: nome')}</option>
                                </select>
                                <select id="affilio-recs-limit">
                                    <option value="10">Top 10</option>
                                    <option value="25">Top 25</option>
                                    <option value="all">${t('All', 'Tutti')}</option>
                                </select>
                            </div>
                        </div>
                        ${renderRecommendationClicks(sortedRecs)}
                    </div>
                </div>
            `;
            const rangeEl = document.getElementById('affilio-analytics-range');
            const articleSortEl = document.getElementById('affilio-articles-sort');
            const articleLimitEl = document.getElementById('affilio-articles-limit');
            const recSortEl = document.getElementById('affilio-recs-sort');
            const recLimitEl = document.getElementById('affilio-recs-limit');
            const attributionModelEl = document.getElementById('affilio-attribution-model');
            const exportJsonEl = document.getElementById('affilio-export-json');
            const exportCsvEl = document.getElementById('affilio-export-csv');
            if (rangeEl) rangeEl.value = range;
            if (articleSortEl) articleSortEl.value = articleSort;
            if (articleLimitEl) articleLimitEl.value = articleLimit;
            if (recSortEl) recSortEl.value = recSort;
            if (recLimitEl) recLimitEl.value = recLimit;
            if (attributionModelEl) attributionModelEl.value = attributionModel;
            if (rangeEl) rangeEl.addEventListener('change', (e) => {
                range = e.target.value;
                loadAnalytics();
            });
            if (articleSortEl) articleSortEl.addEventListener('change', (e) => { articleSort = e.target.value; renderAnalytics(); });
            if (articleLimitEl) articleLimitEl.addEventListener('change', (e) => { articleLimit = e.target.value; renderAnalytics(); });
            if (recSortEl) recSortEl.addEventListener('change', (e) => { recSort = e.target.value; renderAnalytics(); });
            if (recLimitEl) recLimitEl.addEventListener('change', (e) => { recLimit = e.target.value; renderAnalytics(); });
            if (attributionModelEl) attributionModelEl.addEventListener('change', (e) => {
                attributionModel = e.target.value;
                loadAnalytics();
            });
            if (exportJsonEl) exportJsonEl.addEventListener('click', async () => {
                try {
                    await downloadExport('json');
                } catch (e) {
                    renderToast(t('Failed to export JSON', 'Esportazione JSON non riuscita'));
                }
            });
            if (exportCsvEl) exportCsvEl.addEventListener('click', async () => {
                try {
                    await downloadExport('csv');
                } catch (e) {
                    renderToast(t('Failed to export CSV', 'Esportazione CSV non riuscita'));
                }
            });
        };

        const loadAnalytics = async () => {
            const rangeParam = range === '0' ? '' : `?days=${range}`;
            const attributionParam = rangeParam ? `${rangeParam}&model=${attributionModel}` : `?model=${attributionModel}`;
            const [articlesResp, recsResp, summaryResp, trendsResp, attributionResp, funnelResp, cohortsResp, retentionResp, qualityResp, tuningResp] = await Promise.all([
                fetch(AffilioData.apiUrl + '/track/articles' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/recommendations' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/summary' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/trends' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/attribution' + attributionParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/funnel' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/cohorts' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/retention' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/recommendation-quality' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
                fetch(AffilioData.apiUrl + '/track/recommendation-tuning' + rangeParam, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
            ]);
            articleClicks = articlesResp.articles || [];
            recClicks = recsResp.recommendations || [];
            summary = summaryResp.summary || null;
            trends = trendsResp.trends || [];
            attribution = attributionResp.programs || [];
            attributionMeta = attributionResp.meta || null;
            funnel = funnelResp.funnel || null;
            cohorts = cohortsResp.cohorts || [];
            retention = retentionResp.retention || [];
            qualitySummary = qualityResp.summary || null;
            qualityPrograms = qualityResp.programs || [];
            tuningProfiles = tuningResp.tuning || [];
            tuningInsights = tuningResp.insights || null;
            renderAnalytics();
        };

        loadAnalytics();
    };

    const fetchJSON = (path) => fetch(AffilioData.apiUrl + path, { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => {
        if (!r.ok) throw new Error(path + ' failed');
        return r.json();
    });

    Promise.all([
        fetchJSON('/articles').catch(() => ({ articles: [] })),
        fetchJSON('/user-programs').catch(() => ({ programs: [] })),
        fetchJSON('/recommendations').catch(() => ({ recommendations: [] })),
    ])
        .then(([articlesResp, programsResp, recommendationsResp]) => {
            render(
                articlesResp.articles || [],
                programsResp.programs || [],
                recommendationsResp.recommendations || []
            );
        })
        .catch(() => { root.innerHTML = `<div class="affilio-panel affilio-panel-warning"><h2>Affilio Dashboard</h2><p>${t('Failed to load data.', 'Caricamento dati non riuscito.')}</p></div>`; });
} )();
