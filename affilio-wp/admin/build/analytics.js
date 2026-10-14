(function () {
    const root = document.getElementById('affilio-analytics-root');
    if (!root || !window.AffilioData) return;
    const data = window.AffilioData;
    const locale = String(data.locale || document.documentElement.lang || 'en').toLowerCase();
    const t = (en, it) => (locale.startsWith('it') ? it : en);
    root.classList.add('affilio-analytics-app');

    if (data.connectionStatus && !data.connectionStatus.connected) {
        const settings = data.settingsUrl || '/wp-admin/admin.php?page=affilio-settings';
        root.innerHTML = '<div class="affilio-panel affilio-panel-warning"><h2>' + t('Analytics', 'Analitiche') + '</h2><p><strong>' + t('Not connected.', 'Non connesso.') + '</strong> ' + (data.connectionStatus.message || '') + '</p><p><a class="button button-primary" href="' + settings + '">' + t('Open settings', 'Apri impostazioni') + '</a></p></div>';
        return;
    }

    root.innerHTML = '<div class="affilio-panel"><h2>' + t('Analytics', 'Analitiche') + '</h2><p>' + t('Loading…', 'Caricamento…') + '</p></div>';
    const base = String(data.restBase || '').replace(/\/$/, '');
    fetch(base + '/proxy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': data.nonce },
        body: JSON.stringify({ method: 'GET', path: '/track/summary?days=30' })
    }).then(function (response) {
        if (!response.ok) throw new Error('summary');
        return response.json();
    }).then(function (body) {
        const summary = body.summary || {};
        root.innerHTML = '<div class="affilio-panel"><h2>' + t('Analytics', 'Analitiche') + '</h2><p>' + t('Clicks', 'Clic') + ': ' + Number(summary.total_clicks || 0) + '</p><p>' + t('Articles', 'Articoli') + ': ' + Number(summary.unique_articles || 0) + '</p></div>';
    }).catch(function () {
        root.innerHTML = '<div class="affilio-panel affilio-panel-warning"><h2>' + t('Analytics', 'Analitiche') + '</h2><p>' + t('Analytics could not be loaded.', 'Impossibile caricare le analitiche.') + '</p></div>';
    });
})();
