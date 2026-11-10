( function() {
    const bindDashboardScan = () => {
        const form = document.getElementById('affilio-scan-all');
        if (!form) return;
        const button = form.querySelector('[data-scan]');
        const status = form.querySelector('[data-scan-status]');
        const fallback = form.querySelector('[data-scan-fallback]');
        if (!button) return;
        const data = window.AffilioData || {};
        const italian = String(data.locale || '').toLowerCase().startsWith('it');
        const i18n = data.i18n || {};
        const label = (key, en, it) => (i18n[key] ? String(i18n[key]) : (italian ? it : en));
        const actionLabel = button.textContent;
        const pollMs = Number(data.scanPollMs) > 0 ? Number(data.scanPollMs) : 1500;
        const timeoutMs = Number(data.scanTimeoutMs) > 0 ? Number(data.scanTimeoutMs) : 120000;
        const failedText = () => label('scanFailed', 'Analysis failed. Retry, or send the scan form.', 'Analisi non riuscita. Riprova, oppure invia il modulo di scansione.');
        const parseJson = (raw) => {
            const trimmed = String(raw || '').trim();
            if (!trimmed || (trimmed.charAt(0) !== '{' && trimmed.charAt(0) !== '[')) return null;
            try {
                return JSON.parse(trimmed);
            } catch (error) {
                return null;
            }
        };
        const call = async () => {
            const base = String(data.restBase || '').replace(/\/$/, '');
            const response = await fetch(base + '/setup/rescan', {
                method: 'POST',
                headers: { 'X-WP-Nonce': data.nonce || '' },
            });
            if (response.status === 409) return { phase: 'scanning' };
            const raw = await response.text();
            if (!response.ok) {
                const parsed = parseJson(raw);
                const message = parsed && typeof parsed.message === 'string' && parsed.message ? parsed.message : failedText();
                throw new Error(message);
            }
            const body = parseJson(raw);
            if (!body || typeof body !== 'object' || Array.isArray(body)) throw new Error(failedText());
            return body;
        };
        const setBusy = (busy) => {
            button.disabled = busy;
            button.setAttribute('aria-busy', busy ? 'true' : 'false');
            if (status) status.setAttribute('aria-busy', busy ? 'true' : 'false');
        };
        const showFallback = (visible) => {
            if (!fallback) return;
            fallback.hidden = !visible;
        };
        const finish = (message, errored) => {
            button.textContent = actionLabel;
            if (status) status.textContent = message;
            showFallback(errored);
            setBusy(false);
            button.focus();
        };
        let running = false;
        form.addEventListener('submit', async (event) => {
            event.preventDefault();
            if (running) return;
            running = true;
            button.textContent = actionLabel;
            showFallback(false);
            setBusy(true);
            const inProgress = label('analysisInProgress', 'Analysis in progress', 'Analisi in corso');
            const done = label('analysisComplete', 'Analysis complete', 'Analisi completata');
            const timedOut = label('scanTimedOut', 'Analysis is taking too long. Retry, or send the scan form.', "L'analisi sta impiegando troppo tempo. Riprova, oppure invia il modulo di scansione.");
            if (status) status.textContent = inProgress;
            const deadline = Date.now() + timeoutMs;
            try {
                let state = await call();
                while (state && state.phase === 'scanning') {
                    if (Date.now() >= deadline) {
                        finish(timedOut, true);
                        return;
                    }
                    if (status) status.textContent = inProgress;
                    await new Promise((resolve) => setTimeout(resolve, pollMs));
                    if (Date.now() >= deadline) {
                        finish(timedOut, true);
                        return;
                    }
                    state = await call();
                }
                if (state && state.phase === 'complete') {
                    finish(done, false);
                    return;
                }
                finish(failedText(), true);
            } catch (failure) {
                finish(failure && failure.message ? failure.message : failedText(), true);
            } finally {
                running = false;
            }
        });
        const fallbackButton = fallback ? (fallback.matches('button') ? fallback : fallback.querySelector('button')) : null;
        if (fallbackButton) {
            fallbackButton.addEventListener('click', () => form.submit());
        }
    };
    bindDashboardScan();
    const root = document.getElementById('affilio-dashboard-root');
    if (!root) return;
    const locale = String((AffilioData && AffilioData.locale) || document.documentElement.lang || 'en').toLowerCase();
    const isItalian = locale.startsWith('it');
    const t = (en, it) => (isItalian ? it : en);
    const esc = (value) => {
        return String(value || '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    };
    const safeHref = (value) => {
        const url = String(value || '').trim();
        if (url.startsWith('https://') || url.startsWith('http://')) {
            return url;
        }
        return '#';
    };
    const i18n = (AffilioData && AffilioData.i18n) || {};
    const phrase = (key, en, it) => {
        if (Object.prototype.hasOwnProperty.call(i18n, key) && i18n[key] !== '') {
            return String(i18n[key]);
        }
        return t(en, it);
    };
    const allowedState = (state) => {
        const known = ['connected', 'missing_key', 'missing_url', 'invalid_key', 'api_error', 'network_error', 'wrong_api_url', 'starting', 'unknown_error'];
        return known.indexOf(state) >= 0 ? state : 'unknown_error';
    };
    const connectionStatusHTML = () => {
        if (document.getElementById('affilio-dashboard-connection')) {
            return '';
        }
        const status = (AffilioData && AffilioData.connectionStatus) || {};
        if (status.connected) {
            const label = esc(phrase('connected', 'Connected', 'Connesso'));
            const state = allowedState(String(status.state || 'connected'));
            return `<div class="affilio-status-row affilio-dashboard-connection" data-connection-status><span class="affilio-status-pill affilio-status-${state}">${label}</span></div>`;
        }
        const settings = (AffilioData && AffilioData.settingsUrl) || '/wp-admin/admin.php?page=affilio-settings';
        const lead = esc(phrase('notConnected', 'Not connected.', 'Non connesso.'));
        const next = esc(phrase('connectSite', 'Connect your site in Settings.', 'Collega il sito in Impostazioni.'));
        return `<div class="affilio-status-row affilio-dashboard-connection" data-connection-status><p class="affilio-status-message">${lead} <a class="affilio-next-step" href="${esc(safeHref(settings))}">${next}</a></p></div>`;
    };
    const affilioProxy = (method, path, body) => {
        const base = String((AffilioData && AffilioData.restBase) || '').replace(/\/$/, '');
        const payload = { method: method, path: path };
        if (body !== undefined) payload.body = body;
        return fetch(base + '/proxy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': AffilioData.nonce },
            body: JSON.stringify(payload),
        });
    };
    root.classList.add('affilio-dashboard-app');
    if (AffilioData.connectionStatus && !AffilioData.connectionStatus.connected) {
        const settings = AffilioData.settingsUrl || '/wp-admin/admin.php?page=affilio-settings';
        const message = AffilioData.connectionStatus.message || t('Configure your API key.', 'Configura la tua chiave API.');
        root.innerHTML = connectionStatusHTML() + `
            <div class="affilio-panel affilio-panel-warning">
                <h2>Allagio Dashboard</h2>
                <p><strong>${t('Not connected.', 'Non connesso.')}</strong> ${esc(message)}</p>
                <p><a class="button button-primary" href="${esc(safeHref(settings))}">${t('Open settings', 'Apri impostazioni')}</a></p>
            </div>
        `;
        return;
    }
    root.innerHTML = connectionStatusHTML() + `<div class="affilio-panel"><h2>Allagio Dashboard</h2><p>${t('Loading data...', 'Caricamento dati...')}</p></div>`;

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
        let html = connectionStatusHTML() + `<div class="affilio-panel"><h2>Allagio Dashboard</h2><p>${t('Review article monetization and manage recommendations.', 'Controlla la monetizzazione degli articoli e gestisci le raccomandazioni.')}</p></div>`;
        if (!articles.length) {
            html += `<div class="affilio-panel"><p>${t('No articles scanned yet.', 'Nessun articolo ancora scansionato.')}</p></div>`;
        } else {
            html += renderOpportunityPanel(articles, recommendations);
            let rows = articles.map(a => {
                const rawScore = Number(a.monetization_score);
                const score = Number.isFinite(rawScore) ? rawScore : 0;
                const title = esc(a.title || '');
                const articleId = esc(a.id);
                const postId = esc(a.wp_post_id);
                return `<tr data-article-id="${articleId}"><td>${title}</td><td style="color:${scoreColor(score)};font-weight:bold;">${score}</td><td>${potentialLabel(score)}</td><td>${postId}</td></tr>`;
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
            let rows = programs.map(p => {
                const name = esc(p.name);
                const domain = p.domain ? esc(p.domain) : t('unknown', 'sconosciuto');
                return `<li>${name} (${domain})</li>`;
            }).join('');
            html += `<ul>${rows}</ul>`;
        }
        html += '</div>';
        root.innerHTML = html;

        const table = document.getElementById('affilio-articles-table');
        const detail = document.getElementById('affilio-article-detail');
        root.querySelectorAll('button[data-jump-article]').forEach((btn) => {
            btn.addEventListener('click', () => {
                if (!table) return;
                const articleId = btn.getAttribute('data-jump-article');
                const row = Array.prototype.find.call(
                    table.querySelectorAll('tr[data-article-id]'),
                    (candidate) => candidate.getAttribute('data-article-id') === articleId
                ) || null;
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
                    const res = await affilioProxy('GET', '/articles/' + id);
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

                    let articleRawHTML = '';
                    let articleHeadings = [];
                    let articleExistingLinks = [];
                    let articleWordCount = 0;

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
                            const rawHTML = (wpPost && wpPost.content && wpPost.content.rendered) || '';
                            articleRawHTML = rawHTML;
                            const content = textFromHTML(rawHTML);
                            const excerpt = textFromHTML((wpPost && wpPost.excerpt && wpPost.excerpt.rendered) || '');
                            articleContextCache = (content || excerpt).slice(0, 3000);
                            articleWordCount = content.split(/\s+/).filter(Boolean).length;

                            // Extract headings
                            const headingMatches = rawHTML.match(/<h[1-6][^>]*>([\s\S]*?)<\/h[1-6]>/gi) || [];
                            articleHeadings = headingMatches.map(h => textFromHTML(h)).filter(Boolean);

                            // Extract existing link domains
                            const linkMatches = rawHTML.match(/href="(https?:\/\/[^"]+)"/gi) || [];
                            const domains = new Set();
                            linkMatches.forEach(match => {
                                try {
                                    const url = match.replace(/^href="/i, '').replace(/"$/, '');
                                    const hostname = new URL(url).hostname.replace(/^www\./, '');
                                    domains.add(hostname);
                                } catch (_) {}
                            });
                            articleExistingLinks = Array.from(domains);

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
                            await affilioProxy('PATCH', '/recommendations/' + recId, { status: 'dismissed' });
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
                            const gen = await affilioProxy('POST', '/generate/content', {
                                    recommendation_id: recId,
                                    insertion_type: rec.insertion_type || 'inline_mention',
                                    article_context: articleContext,
                                    article_content: articleContext,
                                    article_headings: articleHeadings,
                                    existing_links: articleExistingLinks,
                                    article_word_count: articleWordCount,
                                    article_category: data.article.category || '',
                                    preferred_tone: 'match article author voice',
                                    site_language: (typeof AffilioData !== 'undefined' && AffilioData.locale) || document.documentElement.lang || 'en',
                                    regenerate: action === 'regenerate',
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
                                const defaultTransition = genData.transition_sentence || '';
                                const paragraphIndex = typeof genData.suggested_paragraph_index === 'number' ? genData.suggested_paragraph_index : -1;
                                const confidence = typeof genData.confidence_score === 'number' ? genData.confidence_score : 0;

                                const baseHTML = buildSnippetHTML(
                                    genData.program_name || rec.program_name || rec.id,
                                    genData.link_url || '#',
                                    defaultText
                                );

                                // Build contextual diff preview with surrounding paragraphs
                                const paragraphs = String(articleContext || '').split(/\n\s*\n/).map(s => s.trim()).filter(Boolean);
                                const totalParagraphs = paragraphs.length;
                                let insertIdx = paragraphIndex >= 0 ? Math.min(paragraphIndex, totalParagraphs) : Math.floor(totalParagraphs / 2);
                                const beforeParas = paragraphs.slice(Math.max(0, insertIdx - 2), insertIdx);
                                const afterParas = paragraphs.slice(insertIdx, Math.min(totalParagraphs, insertIdx + 2));

                                const confidenceLabel = confidence >= 0.7 ? t('High', 'Alta') : confidence >= 0.4 ? t('Medium', 'Media') : t('Low', 'Bassa');
                                const confidenceColor = confidence >= 0.7 ? 'high' : confidence >= 0.4 ? 'medium' : 'low';

                                const diffBefore = beforeParas.map(p => `<div class="affilio-preview-para">${esc(p)}</div>`).join('');
                                const diffAfter = afterParas.map(p => `<div class="affilio-preview-para">${esc(p)}</div>`).join('');
                                const insertLabel = totalParagraphs > 0
                                    ? `${t('Inserting after paragraph', 'Inserimento dopo il paragrafo')} ${insertIdx} ${t('of', 'di')} ${totalParagraphs}`
                                    : '';

                                previewEl.innerHTML = `
                                    <div class="affilio-plan-card">
                                        <div class="affilio-plan-meta">
                                            <strong>${t('Planned position', 'Posizione prevista')}:</strong> ${esc(positionLabel(suggestedPosition))}
                                            <select class="affilio-position-select" data-rec="${esc(recId)}" disabled>
                                                <option value="after_intro" ${suggestedPosition === 'after_intro' ? 'selected' : ''}>${t('After intro', 'Dopo introduzione')}</option>
                                                <option value="mid_article" ${suggestedPosition === 'mid_article' ? 'selected' : ''}>${t('Mid article', 'Met\u00e0 articolo')}</option>
                                                <option value="before_conclusion" ${suggestedPosition === 'before_conclusion' ? 'selected' : ''}>${t('Before conclusion', 'Prima della conclusione')}</option>
                                                <option value="end_of_article" ${suggestedPosition === 'end_of_article' ? 'selected' : ''}>${t('End of article', 'Fine articolo')}</option>
                                            </select>
                                            <span class="affilio-confidence-badge affilio-confidence-${confidenceColor}">${t('Confidence', 'Confidenza')}: ${confidenceLabel}</span>
                                        </div>
                                        <div class="affilio-plan-meta">
                                            <strong>${t('Why here', 'Perch\u00e9 qui')}:</strong> ${esc(genData.placement_reason || t('Chosen for relevance and click potential.', 'Scelta per rilevanza e potenziale di click.'))}
                                        </div>
                                        ${insertLabel ? `<div class="affilio-plan-meta"><strong>${esc(insertLabel)}</strong></div>` : ''}
                                        <div class="affilio-preview-diff" id="affilio-preview-diff-${esc(recId)}">
                                            ${diffBefore}
                                            <div class="affilio-preview-insertion" id="affilio-preview-insertion-${esc(recId)}">
                                                ${defaultTransition ? `<div class="affilio-preview-transition">${esc(defaultTransition)}</div>` : ''}
                                                ${baseHTML}
                                            </div>
                                            ${diffAfter}
                                        </div>
                                        <label class="affilio-plan-label" for="affilio-transition-text-${esc(recId)}">${t('Generated transition', 'Frase di transizione generata')}</label>
                                        <textarea class="affilio-plan-editor affilio-plan-editor-sm" id="affilio-transition-text-${esc(recId)}" data-rec-transition="${esc(recId)}" rows="2" readonly>${esc(defaultTransition)}</textarea>
                                        <label class="affilio-plan-label" for="affilio-generated-text-${esc(recId)}">${t('Generated recommendation', 'Raccomandazione generata')}</label>
                                        <textarea class="affilio-plan-editor" id="affilio-generated-text-${esc(recId)}" data-rec-text="${esc(recId)}" rows="3" readonly>${esc(defaultText)}</textarea>
                                        <div class="affilio-plan-actions">
                                            <button class="button button-primary" data-rec="${esc(recId)}" data-action="confirm-insert">${t('Insert now', 'Inserisci ora')}</button>
                                            <button class="button" data-rec="${esc(recId)}" data-action="regenerate">${t('Regenerate', 'Rigenera')}</button>
                                        </div>
                                    </div>
                                `;
                                const editorEl = previewEl.querySelector(`textarea[data-rec-text="${recId}"]`);
                                const transitionEl = previewEl.querySelector(`textarea[data-rec-transition="${recId}"]`);
                                const posEl = previewEl.querySelector(`select[data-rec="${recId}"]`);
                                const insertionEl = previewEl.querySelector(`#affilio-preview-insertion-${recId}`);
                                const refreshPreview = () => {
                                    if (!editorEl || !insertionEl) return;
                                    const liveText = String(editorEl.value || '').trim();
                                    const liveTransition = transitionEl ? String(transitionEl.value || '').trim() : '';
                                    const liveHTML = buildSnippetHTML(
                                        genData.program_name || rec.program_name || rec.id,
                                        genData.link_url || '#',
                                        liveText || defaultText
                                    );
                                    insertionEl.innerHTML = (liveTransition ? `<div class="affilio-preview-transition">${esc(liveTransition)}</div>` : '') + liveHTML;
                                };
                                if (editorEl) editorEl.addEventListener('input', refreshPreview);
                                if (transitionEl) transitionEl.addEventListener('input', refreshPreview);
                                if (posEl) posEl.addEventListener('change', refreshPreview);
                            }
                            return;
                        }

                        if (action === 'confirm-insert') {
                            const plan = generatedPlans[recId];
                            if (!plan || !plan.html) {
                                renderToast(t('Generate preview first', 'Genera prima l\'anteprima'));
                                return;
                            }

                            const articleTitle = esc(data.article.title || t('this article', 'questo articolo'));
                            if (previewEl) {
                                const confirmOverlay = document.createElement('div');
                                confirmOverlay.className = 'affilio-confirm-overlay';
                                confirmOverlay.innerHTML = `
                                    <div class="affilio-confirm-dialog">
                                        <h4>${t('Confirm insertion', 'Conferma inserimento')}</h4>
                                        <p>${t('Allagio will insert the generated recommendation into', 'Allagio inserirà la raccomandazione generata in')} <strong>${articleTitle}</strong>.</p>
                                        <div class="affilio-confirm-actions">
                                            <button class="button button-primary" data-confirm="yes">${t('Confirm & Insert', 'Conferma e Inserisci')}</button>
                                            <button class="button" data-confirm="no">${t('Cancel', 'Annulla')}</button>
                                        </div>
                                    </div>
                                `;
                                previewEl.appendChild(confirmOverlay);

                                const handleConfirm = async (e) => {
                                    const confirmBtn = e.target.closest('button[data-confirm]');
                                    if (!confirmBtn) return;
                                    if (confirmBtn.getAttribute('data-confirm') === 'no') {
                                        confirmOverlay.remove();
                                        return;
                                    }
                                    confirmBtn.disabled = true;
                                    confirmBtn.innerText = t('Inserting...', 'Inserimento in corso...');

                                    const restBase = String((AffilioData && AffilioData.restBase) || '/wp-json/affilio/v1').replace(/\/$/, '');
                                    const insert = await fetch(restBase + '/editor/approve-insert', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': AffilioData.nonce },
                                        body: JSON.stringify({
                                            post_id: data.article.wp_post_id,
                                            recommendation_id: recId,
                                        }),
                                    });
                                    const insertData = await insert.json().catch(() => ({}));
                                    confirmOverlay.remove();
                                    if (!insert.ok) {
                                        renderToast((insertData && (insertData.error || insertData.message)) ? (insertData.error || insertData.message) : t('Insert failed', 'Inserimento non riuscito'));
                                        return;
                                    }
                                    previewEl.innerHTML = `<p class="affilio-rec-done">${t('Inserted successfully.', 'Inserito con successo.')}</p>`;
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
                                };
                                confirmOverlay.addEventListener('click', handleConfirm);
                            }
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
                await affilioProxy('POST', '/user-programs', {
                    name: formData.get('name'),
                    domain: formData.get('domain'),
                    affiliate_link: formData.get('link'),
                });
                location.reload();
            });
        }
    };

    const fetchJSON = (path) => affilioProxy('GET', path).then(r => {
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
        .catch(() => { root.innerHTML = connectionStatusHTML() + `<div class="affilio-panel affilio-panel-warning"><h2>Allagio Dashboard</h2><p>${t('Failed to load data.', 'Caricamento dati non riuscito.')}</p></div>`; });
} )();
