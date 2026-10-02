(function () {
    const root = document.getElementById('affilio-onboarding-root');
    if (!root) return;
    const it = String(AffilioSetup.locale || '').toLowerCase().startsWith('it');
    const t = (en, italian) => it ? italian : en;
    root.innerHTML = '<h2></h2><p data-description></p><p data-progress role="status" aria-live="polite"></p><p data-error role="alert"></p><button type="button" class="button button-primary" data-connect></button><p><a data-dashboard hidden></a></p>';
    root.querySelector('h2').textContent = t('Find earning opportunities in your posts', 'Trova opportunità di guadagno nei tuoi articoli');
    root.querySelector('[data-description]').textContent = t('Connect to create your site account and analyze published posts with Affilio. Your domain, site name and published article content are sent to ', 'Collega per creare il tuo account e analizzare gli articoli pubblicati con Affilio. Dominio, nome del sito e contenuto degli articoli pubblicati vengono inviati a ') + AffilioSetup.serviceUrl + '. ' + t('You review suggestions before anything is inserted.', 'Puoi rivedere i suggerimenti prima di ogni inserimento.');
    const button = root.querySelector('[data-connect]');
    const progress = root.querySelector('[data-progress]');
    const error = root.querySelector('[data-error]');
    const dashboard = root.querySelector('[data-dashboard]');
    const call = async (method) => {
        const response = await fetch(AffilioSetup.restBase.replace(/\/$/, '') + '/setup', { method, headers: { 'X-WP-Nonce': AffilioSetup.nonce } });
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || t('Setup failed. Try again.', 'Setup non riuscito. Riprova.'));
        return data;
    };
    const render = (data) => {
        const scanning = data.phase === 'scanning';
        const complete = data.phase === 'complete' && data.connection.connected;
        button.disabled = scanning || complete;
        button.textContent = t('Connect and find opportunities', 'Collega e trova opportunità');
        if (data.phase === 'scan_error') {
            button.textContent = t('Retry analysis', 'Riprova analisi');
            error.textContent = t('Connected, but analysis failed. Retry to continue.', 'Collegato, ma analisi non riuscita. Riprova per continuare.');
        }
        progress.textContent = complete
            ? `${data.scanned || 0} ` + t('published posts analyzed. Review your suggestions.', 'articoli pubblicati analizzati. Rivedi i suggerimenti.')
            : scanning ? `${data.scanned || 0} ` + t('posts analyzed. Scanning in the background…', 'articoli analizzati. Analisi in background…')
                : data.connection.connected ? t('Connected. Start your first analysis.', 'Collegato. Avvia la prima analisi.') : t('Ready to connect. No API key to copy.', 'Pronto per il collegamento. Nessuna chiave API da copiare.');
        dashboard.hidden = !complete;
        dashboard.href = data.dashboard_url;
        dashboard.textContent = t('Review opportunities', 'Rivedi opportunità');
        if (scanning) setTimeout(refresh, 3000);
    };
    const refresh = async () => {
        try { render(await call('GET')); }
        catch (failure) { error.textContent = failure.message; button.disabled = false; }
    };
    button.addEventListener('click', async () => {
        button.disabled = true;
        error.textContent = '';
        progress.textContent = t('Connecting…', 'Collegamento…');
        try { render(await call('POST')); }
        catch (failure) { error.textContent = failure.message; button.disabled = false; }
    });
    refresh();
})();
