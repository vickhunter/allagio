( function() {
    const root = document.getElementById('affilio-earnings-root');
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
                <h2>${t('Earnings', 'Guadagni')}</h2>
                <p><strong>${t('Not connected.', 'Non connesso.')}</strong> ${AffilioData.connectionStatus.message || t('Configure your API key.', 'Configura la tua chiave API.')}</p>
                <p><a class="button button-primary" data-primary-action href="${settings}">${t('Open settings', 'Apri impostazioni')}</a></p>
            </div>
        `;
        return;
    }
    root.innerHTML = `<div class="affilio-panel"><h2>${t('Earnings', 'Guadagni')}</h2><p>${t('Loading your balance...', 'Caricamento del saldo...')}</p></div>`;

    const esc = (value) => {
        const div = document.createElement('div');
        div.textContent = value === null || value === undefined ? '' : String(value);
        return div.innerHTML;
    };

    const money = (value, currency) => {
        const num = Number(value || 0);
        const code = String(currency || 'USD').toUpperCase();
        const symbol = code === 'USD' ? '$' : (code === 'EUR' ? '€' : code + ' ');
        const sign = num < 0 ? '−' : '';
        return sign + symbol + Math.abs(num).toFixed(2);
    };

    const shortDate = (value) => {
        if (!value) return '';
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) return '';
        return date.toLocaleDateString(isItalian ? 'it-IT' : 'en-US', { month: 'short', day: 'numeric' });
    };

    const statusPill = (entry) => {
        const status = String(entry.status || '').toLowerCase();
        if (status === 'cleared') {
            return `<span class="affilio-rec-status affilio-rec-status-approved">${t('Cleared', 'Maturata')}</span>`;
        }
        if (status === 'reversed') {
            return `<span class="affilio-rec-status affilio-rec-status-dismissed">${t('Reversed', 'Stornata')}</span>`;
        }
        const clears = entry.clears_at ? ` · ${shortDate(entry.clears_at)}` : '';
        return `<span class="affilio-rec-status affilio-rec-status-pending">${t('Pending', 'In attesa')}${clears}</span>`;
    };

    const modeLabel = (mode) => mode === 'test'
        ? t('Test · simulated earnings', 'Test · guadagni simulati')
        : (mode === 'live' ? t('Live', 'Reale') : t('Legacy · mode unverified', 'Storico · modalità non verificata'));

    const balanceCards = (balances) => {
        if (!balances.length) {
            balances = [{ currency: 'USD', pending: 0, cleared_unpaid: 0, paid: 0 }];
        }
        return balances.map((balance) => `
            <h3>${esc(balance.currency)} · ${esc(modeLabel(balance.mode))}</h3>
            <div class="affilio-metric-grid">
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Pending — inside refund windows', 'In attesa — entro le finestre di rimborso')}</div>
                    <div class="affilio-metric-value">${money(balance.pending, balance.currency)}</div>
                    <div class="affilio-balance-note">${t('held while the refund window or review remains open', 'trattenuto durante la finestra di rimborso o la revisione')}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Cleared — awaiting next payout', 'Maturato — in attesa del prossimo pagamento')}</div>
                    <div class="affilio-metric-value" style="color: #1e6f31;">${money(balance.cleared_unpaid, balance.currency)}</div>
                    <div class="affilio-balance-note">${t('available for a manual payout batch', 'disponibile per un pagamento manuale')}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Reserved for payout', 'Riservato al pagamento')}</div>
                    <div class="affilio-metric-value">${money(balance.reserved, balance.currency)}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Payment reported · receipt unconfirmed', 'Pagamento dichiarato · ricezione non confermata')}</div>
                    <div class="affilio-metric-value">${money(balance.payment_reported, balance.currency)}</div>
                </div>
                <div class="affilio-metric-card">
                    <div class="affilio-metric-label">${t('Receipt confirmed by owner', 'Ricezione confermata dal titolare')}</div>
                    <div class="affilio-metric-value">${money(balance.paid, balance.currency)}</div>
                </div>
            </div>
        `).join('');
    };

    const ledgerRows = (entries) => {
        if (!entries.length) {
            return `<tr><td colspan="5">${t('No commissions yet. Join offers in the Marketplace and they will appear here as readers convert.', 'Ancora nessuna commissione. Attiva le offerte nel Marketplace: appariranno qui quando i lettori convertono.')}</td></tr>`;
        }
        return entries.map((entry) => {
            const negative = Number(entry.amount || 0) < 0 || String(entry.kind) === 'clawback';
            return `
                <tr>
                    <td>${esc(shortDate(entry.created_at))}</td>
                    <td><strong>${esc(entry.product_name || t('Offer', 'Offerta'))}</strong></td>
                    <td>${esc(entry.kind)} · ${esc(modeLabel(entry.mode))}</td>
                    <td style="text-align: right;" class="${negative ? 'affilio-amount-negative' : ''}"><strong>${money(entry.amount, entry.currency)}</strong></td>
                    <td>${statusPill(entry)}</td>
                </tr>
            `;
        }).join('');
    };

    const payoutRows = (payouts) => {
        if (!payouts.length) {
            return `<p class="affilio-empty-inline">${t('No payouts yet — your first payout runs once your cleared balance passes the minimum.', 'Nessun pagamento ancora — il primo pagamento parte quando il saldo maturato supera il minimo.')}</p>`;
        }
        return payouts.map((payout) => {
            const reference = payout.provider_reference ? ` · ${esc(payout.provider_reference)}` : '';
            const paid = String(payout.status) === 'paid' && Boolean(payout.received_at);
            const pill = paid
                ? `<span class="affilio-rec-status affilio-rec-status-approved">${t('Receipt confirmed', 'Ricezione confermata')}</span>`
                : (String(payout.status) === 'cancelled'
                    ? `<span class="affilio-rec-status affilio-rec-status-dismissed">${t('Cancelled', 'Annullato')}</span>`
                    : `<span class="affilio-rec-status affilio-rec-status-pending">${String(payout.status) === 'paid' ? t('Payment reported', 'Pagamento dichiarato') : t('Reserved', 'Riservato')}</span>`);
            return `
                <div class="affilio-payout-row">
                    <div>
                        <strong>${money(payout.amount, payout.currency)} · ${esc(shortDate(payout.received_at || payout.paid_at || payout.created_at))}</strong>
                        <div class="affilio-muted">${esc(modeLabel(payout.mode))} · ${esc(payout.method || 'manual')}${reference}</div>
                    </div>
                    ${pill}
                </div>
            `;
        }).join('');
    };

    const load = async () => {
        try {
            const [balanceRes, historyRes] = await Promise.all([
                affilioProxy('GET', '/payouts/balance'),
                affilioProxy('GET', '/payouts/history'),
            ]);
            if (!balanceRes.ok || !historyRes.ok) throw new Error('load failed');
            const balance = await balanceRes.json();
            const history = await historyRes.json();
            root.innerHTML = `
                ${balanceCards(balance.balances || [])}
                <div class="affilio-panel">
                    <div class="affilio-subpanel-head">
                        <h4>${t('Commission ledger', 'Registro commissioni')}</h4>
                        <span class="affilio-muted">${t('Every entry traces back to a click on one of your posts.', 'Ogni voce risale a un click su uno dei tuoi articoli.')}</span>
                    </div>
                    <table class="widefat striped">
                        <thead>
                            <tr>
                                <th>${t('Date', 'Data')}</th>
                                <th>${t('Product', 'Prodotto')}</th>
                                <th>${t('Kind', 'Tipo')}</th>
                                <th style="text-align: right;">${t('Amount', 'Importo')}</th>
                                <th>${t('Status', 'Stato')}</th>
                            </tr>
                        </thead>
                        <tbody>${ledgerRows(history.ledger || [])}</tbody>
                    </table>
                </div>
                <div class="affilio-panel">
                    <h4>${t('Payouts', 'Pagamenti')}</h4>
                    ${payoutRows(history.payouts || [])}
                </div>
            `;
        } catch (err) {
            root.innerHTML = `
                <div class="affilio-panel affilio-panel-warning">
                    <h2>${t('Earnings', 'Guadagni')}</h2>
                    <p class="affilio-field-error" role="alert" tabindex="-1">${t('Could not load your balance from the Offer Rails API.', 'Impossibile caricare il saldo dall\'API di Offer Rails.')}</p>
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
