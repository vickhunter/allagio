( function() {
    const root = document.getElementById('affilio-marketplace-root');
    if (!root) return;
    const locale = String((AffilioData && AffilioData.locale) || document.documentElement.lang || 'en').toLowerCase();
    const isItalian = locale.startsWith('it');
    const t = (en, it) => (isItalian ? it : en);
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
        root.innerHTML = `
            <div class="affilio-panel affilio-panel-warning">
                <h2>${t('Offer Marketplace', 'Marketplace Offerte')}</h2>
                <p><strong>${t('Not connected.', 'Non connesso.')}</strong> ${AffilioData.connectionStatus.message || t('Configure your API key.', 'Configura la tua chiave API.')}</p>
                <p><a class="button button-primary" data-primary-action href="${settings}">${t('Open settings', 'Apri impostazioni')}</a></p>
            </div>
        `;
        return;
    }
    root.innerHTML = `<div class="affilio-panel"><h2>${t('Offer Marketplace', 'Marketplace Offerte')}</h2><p>${t('Loading offers...', 'Caricamento offerte...')}</p></div>`;

    const esc = (value) => {
        const div = document.createElement('div');
        div.textContent = value === null || value === undefined ? '' : String(value);
        return div.innerHTML;
    };

    const money = (value, currency) => {
        const num = Number(value || 0);
        const code = String(currency || 'USD').toUpperCase();
        const symbol = code === 'USD' ? '$' : (code === 'EUR' ? '€' : code + ' ');
        return symbol + num.toFixed(num % 1 === 0 ? 0 : 2);
    };

    const commissionLabel = (offer) => {
        const type = String(offer.commission_type || '');
        const value = Number(offer.commission_value || 0);
        if (type === 'percentage') return `${value}% ${t('per sale', 'per vendita')}`;
        if (type === 'recurring') return `${value}% ${t('recurring', 'ricorrente')}`;
        return `${money(value, offer.commission_currency)} ${t('per sale', 'per vendita')}`;
    };

    const fitPill = (relevance) => {
        const score = Number(relevance || 0);
        const cls = score >= 50 ? 'affilio-opportunity-score' : 'affilio-opportunity-score affilio-fit-low';
        return `<span class="${cls}">${t('Fit', 'Affinità')} ${score}</span>`;
    };

    const offerCard = (offer, primary) => {
        const joined = !!offer.joined;
        const joinedPill = joined
            ? `<span class="affilio-rec-status affilio-rec-status-approved">${t('Joined', 'Attiva')}</span>`
            : '';
        const terms = offer.terms
            ? `<div class="affilio-rec-reason">${esc(offer.terms)}</div>`
            : `<div class="affilio-rec-reason">${esc(offer.product_tagline || '')}</div>`;
        const meta = `${commissionLabel(offer)} · ${esc(offer.cookie_duration_days)}${t('-day cookie', ' giorni di cookie')} · ${esc(offer.refund_window_days)}${t('-day payout hold', ' giorni di attesa pagamento')}`;
        let actions;
        if (joined) {
            const link = String(offer.tracked_link || '');
            actions = `
                <code class="affilio-code-chip" data-offer-link="${esc(offer.offer_id)}">${esc(link)}</code>
                <button class="button" data-copy-link="${esc(offer.offer_id)}" data-link="${esc(link)}">${t('Copy link', 'Copia link')}</button>
            `;
        } else {
            actions = `<button class="button${primary ? ' button-primary' : ''}" ${primary ? 'data-primary-action' : ''} data-join-offer="${esc(offer.offer_id)}">${t('Join offer', 'Attiva offerta')}</button>`;
        }
        return `
            <div class="affilio-rec-item" data-offer-card="${esc(offer.offer_id)}">
                <div class="affilio-rec-head">
                    <div class="affilio-rec-meta" style="margin: 0;">
                        <strong>${esc(offer.product_name)}</strong>
                        ${fitPill(offer.relevance)}
                        ${joinedPill}
                    </div>
                    <span class="affilio-muted">${meta}</span>
                </div>
                ${terms}
                <div class="affilio-rec-actions">${actions}</div>
            </div>
        `;
    };

    const render = (offers) => {
        let assigned = false;
        const cards = (offers || []).map(offer => {
            const primary = !offer.joined && !assigned;
            if (primary) assigned = true;
            return offerCard(offer, primary);
        }).join('');
        const empty = `<p class="affilio-empty-inline">${t('No listed offers yet. New offers appear here as makers join the network.', 'Nessuna offerta al momento. Le nuove offerte compariranno qui quando i maker si uniranno alla rete.')}</p>`;
        root.innerHTML = `
            <div class="affilio-panel">
                <div class="affilio-subpanel-head">
                    <h2 style="margin: 0;">${t('Offer Marketplace', 'Marketplace Offerte')}</h2>
                    <span class="affilio-muted">${t('Ranked by how well each offer fits your content.', 'Ordinate in base all\'affinità con i tuoi contenuti.')}</span>
                </div>
                <div class="affilio-rec-list">${cards || empty}</div>
            </div>
            <div class="affilio-panel affilio-panel-info">
                <p style="margin: 0;">${t('Joining an offer gives your posts a tracked link. Offer Rails adds the FTC disclosure once per post automatically, and commissions appear under Earnings.', 'Attivando un\'offerta i tuoi articoli ricevono un link tracciato. Offer Rails aggiunge automaticamente la divulgazione FTC una volta per articolo e le commissioni compaiono in Guadagni.')}</p>
            </div>
        `;
        bind(offers || []);
        if (!(offers || []).some(offer => !offer.joined)) {
            const copy = root.querySelector('[data-copy-link]');
            if (copy) {
                copy.setAttribute('data-primary-action', '');
                copy.classList.add('button-primary');
            } else {
                const note = root.querySelector('.affilio-empty-inline');
                if (note) note.setAttribute('data-primary-action', '');
            }
        }
    };

    const bind = (offers) => {
        root.querySelectorAll('[data-join-offer]').forEach((button) => {
            button.addEventListener('click', async () => {
                button.disabled = true;
                button.textContent = t('Joining...', 'Attivazione...');
                try {
                    const res = await affilioProxy('POST', '/marketplace/offers/' + button.getAttribute('data-join-offer') + '/join');
                    if (!res.ok) throw new Error('join failed');
                    await load();
                } catch (err) {
                    button.disabled = false;
                    button.textContent = t('Join offer', 'Attiva offerta');
                    const text = t('Could not join the offer. Try again.', 'Impossibile attivare l\'offerta. Riprova.');
                    let slot = button.parentElement.querySelector('.affilio-field-error');
                    if (!slot) {
                        slot = document.createElement('p');
                        slot.className = 'affilio-field-error';
                        slot.setAttribute('role', 'alert');
                        slot.tabIndex = -1;
                        button.insertAdjacentElement('afterend', slot);
                    }
                    slot.id = 'join-error-' + button.getAttribute('data-join-offer');
                    slot.textContent = text;
                    button.setAttribute('aria-describedby', slot.id);
                    slot.focus({preventScroll: true});
                    slot.scrollIntoView({block: 'center', inline: 'nearest'});
                }
            });
        });
        root.querySelectorAll('[data-copy-link]').forEach((button) => {
            button.addEventListener('click', async () => {
                try {
                    await navigator.clipboard.writeText(button.getAttribute('data-link') || '');
                    const original = button.textContent;
                    button.textContent = t('Copied', 'Copiato');
                    setTimeout(() => { button.textContent = original; }, 1500);
                } catch (err) {
                    window.prompt(t('Copy the tracked link:', 'Copia il link tracciato:'), button.getAttribute('data-link') || '');
                }
            });
        });
    };

    const load = async () => {
        try {
            const res = await affilioProxy('GET', '/marketplace/offers');
            if (!res.ok) throw new Error('load failed');
            const data = await res.json();
            const offers = (data.offers || []).map((offer) => {
                if (offer.tracked_link && offer.tracked_link.indexOf('/') === 0) {
                    // The API returns a path; make it absolute against the API host.
                    const apiOrigin = String(AffilioData.apiUrl || '').replace(/\/v1\/?$/, '');
                    return Object.assign({}, offer, { tracked_link: apiOrigin + offer.tracked_link });
                }
                return offer;
            });
            render(offers);
        } catch (err) {
            root.innerHTML = `
                <div class="affilio-panel affilio-panel-warning">
                    <h2>${t('Offer Marketplace', 'Marketplace Offerte')}</h2>
                    <p class="affilio-field-error" role="alert" tabindex="-1">${t('Could not load offers from the Offer Rails API.', 'Impossibile caricare le offerte dall\'API di Offer Rails.')}</p>
                    <p><button class="button button-primary" data-primary-action data-retry>${t('Retry', 'Riprova')}</button></p>
                </div>
            `;
            const alert = root.querySelector('[role="alert"]');
            if (alert) alert.focus();
            root.querySelector('[data-retry]').addEventListener('click', load);
        }
    };

    load();
} )();
