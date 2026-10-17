( function() {
    const { registerPlugin } = wp.plugins;
    const { PluginSidebar } = wp.editPost;
    const { createElement, useState } = wp.element;
    const { select } = wp.data;
    const locale = String((AffilioData && AffilioData.locale) || document.documentElement.lang || 'en').toLowerCase();
    const isItalian = locale.startsWith('it');
    const t = (en, it) => (isItalian ? it : en);

    const scoreColor = (score) => {
        if (score < 34) return '#d63638';
        if (score < 67) return '#dba617';
        return '#2a9d5b';
    };

    const Sidebar = () => {
        const connectionStatus = AffilioData.connectionStatus || null;
        const connected = !connectionStatus || !!connectionStatus.connected;
        const restBase = String((AffilioData && AffilioData.restBase) || '/wp-json/affilio/v1').replace(/\/$/, '');
        const [score, setScore] = useState(null);
        const [potential, setPotential] = useState(null);
        const [status, setStatus] = useState('');
        const [insertStatus, setInsertStatus] = useState('');
        const [detections, setDetections] = useState([]);
        const [recs, setRecs] = useState([]);

        const adminCall = async (path, payload) => {
            const res = await fetch(restBase + path, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-WP-Nonce': AffilioData.nonce,
                },
                body: JSON.stringify(payload || {}),
            });
            const data = await res.json().catch(() => ({}));
            if (!res.ok) {
                const msg = data && data.message ? data.message : t('Request failed', 'Richiesta non riuscita');
                throw new Error(msg);
            }
            return data;
        };

        const analyze = async () => {
            if (!connected) return;
            const post = select('core/editor').getCurrentPost();
            if (!post) return;
            setStatus(t('Analyzing...', 'Analisi in corso...'));
            try {
                const data = await adminCall('/editor/analyze', {
                    wp_post_id: post.id,
                    title: post.title,
                    content: post.content,
                    url: post.link || '',
                });
                setScore(data.monetization_score || 0);
                setPotential(data.potential || 'low');
                setDetections(data.detections || []);
                setRecs(data.recommendations || []);
                if ((data.recommendations || []).length === 0) {
                    setStatus(t('Analyzed. No recommendations yet.', 'Analisi completata. Nessuna raccomandazione al momento.'));
                    return;
                }
                setStatus('');
            } catch (e) {
                setStatus((e && e.message) ? e.message : t('Analysis failed', 'Analisi non riuscita'));
            }
        };

        const approveAndInsert = async (rec) => {
            if (!connected) return;
            const post = select('core/editor').getCurrentPost();
            if (!post) return;
            setInsertStatus(t('Inserting...', 'Inserimento in corso...'));
            try {
                await adminCall('/editor/approve-insert', {
                    post_id: post.id,
                    recommendation_id: rec.id,
                    insertion_type: rec.insertion_type || 'inline_mention',
                    article_context: post.content || '',
                });
                setInsertStatus(t('Recommendation inserted.', 'Raccomandazione inserita.'));
            } catch (e) {
                setInsertStatus((e && e.message) ? e.message : t('Insert failed', 'Inserimento non riuscito'));
            }
        };

        const sectionStyle = { border: '1px solid #dcdcde', borderRadius: '6px', background: '#fff', padding: '10px', marginTop: '10px' };
        const muted = { color: '#50575e', margin: '4px 0 0' };
        const heading = { margin: '0 0 6px' };
        const scoreStyle = score === null ? {} : { color: scoreColor(score), fontWeight: 'bold' };

        return createElement(
            PluginSidebar,
            { name: 'affilio-sidebar', title: 'Offer Rails' },
            createElement('div', { style: { padding: '16px' } },
                !connected
                    ? createElement('div', { style: { border: '1px solid #dba617', borderLeft: '4px solid #dba617', background: '#fff8e5', padding: '10px', marginBottom: '12px' } },
                        createElement('strong', null, t('Offer Rails not connected', 'Offer Rails non connesso')),
                        createElement('p', { style: muted }, connectionStatus && connectionStatus.message ? connectionStatus.message : t('Configure API key in settings.', 'Configura la chiave API nelle impostazioni.')),
                        createElement('p', { style: { margin: '8px 0 0' } },
                            createElement('a', { className: 'button button-secondary', href: AffilioData.settingsUrl || '/wp-admin/admin.php?page=affilio-settings' }, t('Open settings', 'Apri impostazioni'))
                        )
                    )
                    : null,
                createElement('div', { style: sectionStyle },
                    createElement('h3', { style: heading }, t('Monetization', 'Monetizzazione')),
                    score === null
                        ? createElement('p', { style: muted }, t('Not analyzed yet.', 'Non ancora analizzato.'))
                        : createElement('p', { style: scoreStyle }, `${t('Score', 'Punteggio')}: ${score}/100`),
                    createElement('p', { style: muted }, potential ? `${t('Potential', 'Potenziale')}: ${potential}` : `${t('Potential', 'Potenziale')}: n/a`),
                    createElement('p', { style: { margin: '10px 0 0' } },
                        createElement('button', { className: 'button button-primary', onClick: analyze, disabled: !connected }, t('Analyze', 'Analizza'))
                    )
                ),
                status ? createElement('p', null, status) : null,
                detections.length ? createElement('p', { style: muted }, `${t('Detected affiliate links', 'Link affiliati rilevati')}: ${detections.length}`) : null,
                insertStatus ? createElement('p', { style: { color: '#1e6f31', fontWeight: '600' } }, insertStatus) : null,
                recs.length ? createElement('div', { style: sectionStyle },
                    createElement('h3', { style: heading }, t('Recommendations', 'Raccomandazioni')),
                    recs.map(rec => createElement('div', { key: rec.id, style: { border: '1px solid #dcdcde', borderRadius: '4px', padding: '8px', marginTop: '8px' } },
                        createElement('strong', null, rec.program_name || rec.id),
                        createElement('p', { style: muted }, rec.reason || t('Recommendation', 'Raccomandazione')),
                        createElement('small', { style: { color: '#50575e' } }, rec.program_type ? `${t('Type', 'Tipo')}: ${rec.program_type}` : ''),
                        createElement('p', { style: { margin: '8px 0 0' } },
                            createElement('button', { className: 'button', onClick: () => approveAndInsert(rec), disabled: !connected }, t('Approve & Insert', 'Approva e inserisci'))
                        )
                    ))
                ) : null
            )
        );
    };

    registerPlugin('affilio-sidebar', { render: Sidebar });
} )();
