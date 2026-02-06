( function() {
    const { registerPlugin } = wp.plugins;
    const { PluginSidebar } = wp.editPost;
    const { createElement, useState } = wp.element;
    const { select } = wp.data;

    const scoreColor = (score) => {
        if (score < 34) return '#d63638';
        if (score < 67) return '#dba617';
        return '#2a9d5b';
    };

    const Sidebar = () => {
        const connectionStatus = AffilioData.connectionStatus || null;
        const connected = !connectionStatus || !!connectionStatus.connected;
        const [score, setScore] = useState(null);
        const [potential, setPotential] = useState(null);
        const [status, setStatus] = useState('');
        const [insertStatus, setInsertStatus] = useState('');
        const [detections, setDetections] = useState([]);
        const [recs, setRecs] = useState([]);

        const analyze = async () => {
            if (!connected) return;
            const post = select('core/editor').getCurrentPost();
            if (!post) return;
            setStatus('Analyzing...');
            try {
                const res = await fetch(AffilioData.apiUrl + '/scan/single', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-API-Key': AffilioData.apiKey,
                    },
                    body: JSON.stringify({
                        wp_post_id: post.id,
                        title: post.title,
                        content: post.content,
                        url: post.link || '',
                    }),
                });
                const data = await res.json();
                setScore(data.monetization_score || 0);
                setPotential(data.potential || 'low');
                setDetections(data.detections || []);
                setRecs(data.recommendations || []);
                if ((data.recommendations || []).length === 0) {
                    setStatus('Analyzed. No recommendations yet.');
                    return;
                }
                setStatus('');
            } catch (e) {
                setStatus('Failed to analyze');
            }
        };

        const approveAndInsert = async (rec) => {
            if (!connected) return;
            const post = select('core/editor').getCurrentPost();
            if (!post) return;
            await fetch(`${AffilioData.apiUrl}/recommendations/${rec.id}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                body: JSON.stringify({ status: 'approved' }),
            });
            const gen = await fetch(AffilioData.apiUrl + '/generate/content', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                body: JSON.stringify({ recommendation_id: rec.id, insertion_type: rec.insertion_type || 'inline_mention' }),
            });
            const genData = await gen.json();
            await fetch('/wp-json/affilio/v1/insert', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': AffilioData.nonce },
                body: JSON.stringify({
                    post_id: post.id,
                    html: genData.html || '',
                    recommendation_id: rec.id,
                }),
            });
            setInsertStatus('Recommendation inserted.');
        };

        const sectionStyle = { border: '1px solid #dcdcde', borderRadius: '6px', background: '#fff', padding: '10px', marginTop: '10px' };
        const muted = { color: '#50575e', margin: '4px 0 0' };
        const heading = { margin: '0 0 6px' };
        const scoreStyle = score === null ? {} : { color: scoreColor(score), fontWeight: 'bold' };

        return createElement(
            PluginSidebar,
            { name: 'affilio-sidebar', title: 'Affilio' },
            createElement('div', { style: { padding: '16px' } },
                !connected
                    ? createElement('div', { style: { border: '1px solid #dba617', borderLeft: '4px solid #dba617', background: '#fff8e5', padding: '10px', marginBottom: '12px' } },
                        createElement('strong', null, 'Affilio not connected'),
                        createElement('p', { style: muted }, connectionStatus && connectionStatus.message ? connectionStatus.message : 'Configure API key in settings.'),
                        createElement('p', { style: { margin: '8px 0 0' } },
                            createElement('a', { className: 'button button-secondary', href: AffilioData.settingsUrl || '/wp-admin/admin.php?page=affilio-settings' }, 'Open settings')
                        )
                    )
                    : null,
                createElement('div', { style: sectionStyle },
                    createElement('h3', { style: heading }, 'Monetization'),
                    score === null
                        ? createElement('p', { style: muted }, 'Not analyzed yet.')
                        : createElement('p', { style: scoreStyle }, `Score: ${score}/100`),
                    createElement('p', { style: muted }, potential ? `Potential: ${potential}` : 'Potential: n/a'),
                    createElement('p', { style: { margin: '10px 0 0' } },
                        createElement('button', { className: 'button button-primary', onClick: analyze, disabled: !connected }, 'Analyze')
                    )
                ),
                status ? createElement('p', null, status) : null,
                detections.length ? createElement('p', { style: muted }, `Detected affiliate links: ${detections.length}`) : null,
                insertStatus ? createElement('p', { style: { color: '#1e6f31', fontWeight: '600' } }, insertStatus) : null,
                recs.length ? createElement('div', { style: sectionStyle },
                    createElement('h3', { style: heading }, 'Recommendations'),
                    recs.map(rec => createElement('div', { key: rec.id, style: { border: '1px solid #dcdcde', borderRadius: '4px', padding: '8px', marginTop: '8px' } },
                        createElement('strong', null, rec.program_name || rec.id),
                        createElement('p', { style: muted }, rec.reason || 'Recommendation'),
                        createElement('small', { style: { color: '#50575e' } }, rec.program_type ? `Type: ${rec.program_type}` : ''),
                        createElement('p', { style: { margin: '8px 0 0' } },
                            createElement('button', { className: 'button', onClick: () => approveAndInsert(rec), disabled: !connected }, 'Approve & Insert')
                        )
                    ))
                ) : null
            )
        );
    };

    registerPlugin('affilio-sidebar', { render: Sidebar });
} )();
