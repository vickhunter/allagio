(function () {
    const root = document.getElementById('affilio-onboarding-root');
    if (!root) return;
    const it = String(AffilioSetup.locale || '').toLowerCase().startsWith('it');
    const t = (en, italian) => it ? italian : en;
    const copy = AffilioSetup.copy || {};
    const catalog = (key, fallback) => (copy[key] ? String(copy[key]) : fallback);
    root.innerHTML = '<h2></h2><p data-description></p><p data-progress role="status" aria-live="polite"></p><ul data-skipped hidden></ul><p data-error role="alert"></p><button type="button" class="button button-primary" data-connect></button><p><a data-dashboard hidden></a></p>';
    root.querySelector('h2').textContent = t('Find earning opportunities in your posts', 'Trova opportunità di guadagno nei tuoi articoli');
    root.querySelector('[data-description]').textContent = t('Connect to create your site account and analyze published posts with Affilio. Your domain, site name and published article content are sent to ', 'Collega per creare il tuo account e analizzare gli articoli pubblicati con Affilio. Dominio, nome del sito e contenuto degli articoli pubblicati vengono inviati a ') + AffilioSetup.serviceUrl + '. ' + t('You review suggestions before anything is inserted.', 'Puoi rivedere i suggerimenti prima di ogni inserimento.');
    const button = root.querySelector('[data-connect]');
    const progress = root.querySelector('[data-progress]');
    const error = root.querySelector('[data-error]');
    const dashboard = root.querySelector('[data-dashboard]');
    let siteConnected = false;
    const call = async (method) => {
        const response = await fetch(AffilioSetup.restBase.replace(/\/$/, '') + '/setup', { method, headers: { 'X-WP-Nonce': AffilioSetup.nonce } });
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || t('Setup failed. Try again.', 'Setup non riuscito. Riprova.'));
        return data;
    };
    const render = (data) => {
        const scanning = data.phase === 'scanning';
        const connected = !!(data.connection && data.connection.connected);
        const complete = data.phase === 'complete' && connected;
        siteConnected = connected;
        button.disabled = scanning || complete;
        button.textContent = t('Connect and find opportunities', 'Collega e trova opportunità');
        if (scanning) {
            button.textContent = catalog('analysisInProgress', t('Analysis in progress', 'Analisi in corso'));
        } else if (complete) {
            button.textContent = catalog('analysisComplete', t('Analysis complete', 'Analisi completata'));
        } else if (data.phase === 'scan_error') {
            button.textContent = t('Retry analysis', 'Riprova analisi');
            error.textContent = data.summary || t('Connected. Analysis will continue.', 'Collegato. L\'analisi continua.');
        } else if (connected) {
            button.textContent = catalog('startAnalysis', 'Start analysis');
        }
        const skipped = root.querySelector('[data-skipped]');
        skipped.textContent = '';
        (Array.isArray(data.skipped) ? data.skipped : []).forEach((row) => {
            const item = document.createElement('li');
            item.textContent = String(row.wp_post_id || '') + (row.reason ? ': ' + row.reason : '');
            skipped.appendChild(item);
        });
        skipped.hidden = skipped.childElementCount === 0;
        progress.textContent = complete
            ? (data.summary || (`${data.scanned || 0} ` + t('published posts analyzed. Review your suggestions.', 'articoli pubblicati analizzati. Rivedi i suggerimenti.')))
            : scanning ? `${data.scanned || 0} ` + t('posts analyzed. Scanning in the background…', 'articoli analizzati. Analisi in background…')
                : connected ? catalog('connectedStatus', 'Your site is connected.') : t('Ready to connect. No API key to copy.', 'Pronto per il collegamento. Nessuna chiave API da copiare.');
        dashboard.hidden = !complete;
        dashboard.href = data.dashboard_url;
        dashboard.textContent = t('Review opportunities', 'Rivedi opportunità');
        if (scanning) setTimeout(refresh, 3000);
    };
    const refresh = async () => {
        try { render(await call('GET')); }
        catch (failure) { error.textContent = failure.message; button.disabled = false; }
    };
    button.addEventListener('click', async (event) => {
        event.preventDefault();
        button.disabled = true;
        error.textContent = '';
        if (siteConnected) {
            button.textContent = catalog('analysisInProgress', t('Analysis in progress', 'Analisi in corso'));
        }
        progress.textContent = siteConnected
            ? (`0 ` + t('posts analyzed. Scanning in the background…', 'articoli analizzati. Analisi in background…'))
            : t('Connecting…', 'Collegamento…');
        try { render(await call('POST')); }
        catch (failure) { error.textContent = failure.message; button.disabled = false; }
    });
    refresh();
})();
