( function() {
    const root = document.getElementById('affilio-dashboard-root');
    if (!root) return;
    root.classList.add('affilio-dashboard-app');
    if (AffilioData.connectionStatus && !AffilioData.connectionStatus.connected) {
        const settings = AffilioData.settingsUrl || '/wp-admin/admin.php?page=affilio-settings';
        root.innerHTML = `
            <div class="affilio-panel affilio-panel-warning">
                <h2>Affilio Dashboard</h2>
                <p><strong>Not connected.</strong> ${AffilioData.connectionStatus.message || 'Configure your API key.'}</p>
                <p><a class="button button-primary" href="${settings}">Open settings</a></p>
            </div>
        `;
        return;
    }
    root.innerHTML = '<div class="affilio-panel"><h2>Affilio Dashboard</h2><p>Loading data...</p></div>';

    const scoreColor = (score) => {
        if (score < 34) return '#d63638';
        if (score < 67) return '#dba617';
        return '#2a9d5b';
    };

    const potentialLabel = (score) => {
        if (score < 34) return 'Low';
        if (score < 67) return 'Medium';
        return 'High';
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

    const render = (articles, programs) => {
        let html = '<div class="affilio-panel"><h2>Affilio Dashboard</h2><p>Review article monetization and manage recommendations.</p></div>';
        if (!articles.length) {
            html += '<div class="affilio-panel"><p>No articles scanned yet.</p></div>';
        } else {
            let rows = articles.map(a => {
                const score = a.monetization_score || 0;
                return `<tr data-article-id="${a.id}"><td>${a.title || ''}</td><td style="color:${scoreColor(score)};font-weight:bold;">${score}</td><td>${potentialLabel(score)}</td><td>${a.wp_post_id}</td></tr>`;
            }).join('');
            html += `
                <div class="affilio-panel">
                    <h3>Articles</h3>
                    <table class="widefat" id="affilio-articles-table">
                        <thead><tr><th>Title</th><th>Score</th><th>Potential</th><th>Post ID</th></tr></thead>
                        <tbody>${rows}</tbody>
                    </table>
                    <div id="affilio-article-detail" class="affilio-detail"></div>
                </div>
            `;
        }
        html += `
            <div class="affilio-panel">
                <h3>Analytics</h3>
                <div id="affilio-analytics"></div>
            </div>
        `;
        html += `
            <div class="affilio-panel">
                <h3>My Affiliate Links</h3>
                <form id="affilio-add-program" class="affilio-inline-form">
                    <input type="text" name="name" placeholder="Program name" required />
                    <input type="text" name="domain" placeholder="Domain" required />
                    <input type="text" name="link" placeholder="Affiliate link" />
                    <button class="button button-secondary">Add</button>
                </form>
        `;
        if (!programs.length) {
            html += '<p>No affiliate links detected yet.</p>';
        } else {
            let rows = programs.map(p => `<li>${p.name} (${p.domain || 'unknown'})</li>`).join('');
            html += `<ul>${rows}</ul>`;
        }
        html += '</div>';
        root.innerHTML = html;
        setupAnalytics();

        const table = document.getElementById('affilio-articles-table');
        const detail = document.getElementById('affilio-article-detail');
        if (table && detail) {
            table.querySelectorAll('tr[data-article-id]').forEach(row => {
                row.addEventListener('click', async () => {
                    const id = row.getAttribute('data-article-id');
                    detail.innerHTML = '<p>Loading details...</p>';
                    const res = await fetch(AffilioData.apiUrl + '/articles/' + id, {
                        headers: { 'X-API-Key': AffilioData.apiKey }
                    });
                    const data = await res.json();
                    const recs = (data.recommendations || []).map(r => {
                        return `<li>
                            <strong>${r.program_name || r.id}</strong> — ${r.reason || ''}
                            <button class="button" data-rec="${r.id}" data-action="approve">Approve & Insert</button>
                            <button class="button" data-rec="${r.id}" data-action="dismiss">Dismiss</button>
                        </li>`;
                    }).join('');
                    detail.innerHTML = `<h3>Recommendations</h3>${recs ? `<ul>${recs}</ul>` : '<p>No recommendations yet.</p>'}`;
                    detail.querySelectorAll('button[data-rec]').forEach(btn => {
                        btn.addEventListener('click', async () => {
                            const recId = btn.getAttribute('data-rec');
                            const action = btn.getAttribute('data-action');
                            if (action === 'dismiss') {
                                await fetch(`${AffilioData.apiUrl}/recommendations/${recId}`, {
                                    method: 'PATCH',
                                    headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                                    body: JSON.stringify({ status: 'dismissed' }),
                                });
                                btn.innerText = 'Dismissed';
                                btn.disabled = true;
                                renderToast('Recommendation dismissed');
                                return;
                            }
                            await fetch(`${AffilioData.apiUrl}/recommendations/${recId}`, {
                                method: 'PATCH',
                                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                                body: JSON.stringify({ status: 'approved' }),
                            });
                            const gen = await fetch(AffilioData.apiUrl + '/generate/content', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json', 'X-API-Key': AffilioData.apiKey },
                                body: JSON.stringify({ recommendation_id: recId, insertion_type: 'inline_mention' }),
                            });
                            const genData = await gen.json();
                            await fetch('/wp-json/affilio/v1/insert', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json', 'X-WP-Nonce': AffilioData.nonce },
                                body: JSON.stringify({
                                    post_id: data.article.wp_post_id,
                                    html: genData.html || '',
                                    recommendation_id: recId,
                                }),
                            });
                            btn.innerText = 'Inserted';
                            btn.disabled = true;
                            renderToast('Recommendation inserted');
                        });
                    });
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
            return '<p>No click data yet.</p>';
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.title || 'Untitled'}</td>
                <td>${r.wp_post_id}</td>
                <td style="font-weight:bold;">${r.clicks}</td>
            </tr>
        `).join('');
        return `
            <table class="widefat">
                <thead><tr><th>Article</th><th>Post ID</th><th>Clicks</th></tr></thead>
                <tbody>${tableRows}</tbody>
            </table>
        `;
    };

    const renderRecommendationClicks = (rows) => {
        if (!rows.length) {
            return '<p>No click data yet.</p>';
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.program_name || r.recommendation_id}</td>
                <td>${r.program_type || ''}</td>
                <td style="font-weight:bold;">${r.clicks}</td>
            </tr>
        `).join('');
        return `
            <table class="widefat">
                <thead><tr><th>Recommendation</th><th>Type</th><th>Clicks</th></tr></thead>
                <tbody>${tableRows}</tbody>
            </table>
        `;
    };

    const renderSummary = (summary) => {
        const total = summary && typeof summary.total_clicks === 'number' ? summary.total_clicks : 0;
        const uniqueArticles = summary && typeof summary.unique_articles === 'number' ? summary.unique_articles : 0;
        const uniqueRecs = summary && typeof summary.unique_recommendations === 'number' ? summary.unique_recommendations : 0;
        return `
            <div style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:10px;">
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:140px;">
                    <div style="font-size:12px;color:#50575e;">Total Clicks</div>
                    <div style="font-size:20px;font-weight:700;">${total}</div>
                </div>
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:140px;">
                    <div style="font-size:12px;color:#50575e;">Clicked Articles</div>
                    <div style="font-size:20px;font-weight:700;">${uniqueArticles}</div>
                </div>
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:140px;">
                    <div style="font-size:12px;color:#50575e;">Clicked Recs</div>
                    <div style="font-size:20px;font-weight:700;">${uniqueRecs}</div>
                </div>
            </div>
        `;
    };

    const renderTrends = (rows) => {
        if (!rows.length) {
            return '<p style="margin:0;">No trend data in this range.</p>';
        }
        const max = rows.reduce((acc, r) => Math.max(acc, r.clicks || 0), 0) || 1;
        const points = rows.map(r => {
            const width = Math.max(2, Math.round(((r.clicks || 0) / max) * 100));
            return `
                <div style="display:flex;align-items:center;gap:8px;margin:4px 0;">
                    <span style="display:inline-block;width:92px;font-size:12px;color:#50575e;">${r.day}</span>
                    <div style="background:#f0f0f1;flex:1;height:8px;position:relative;border-radius:10px;">
                        <div style="background:#2271b1;height:8px;border-radius:10px;width:${width}%;"></div>
                    </div>
                    <span style="display:inline-block;width:26px;text-align:right;font-size:12px;">${r.clicks || 0}</span>
                </div>
            `;
        }).join('');
        return `<div style="border:1px solid #dcdcde;border-radius:4px;padding:10px;margin-bottom:12px;">${points}</div>`;
    };

    const renderAttribution = (rows, meta) => {
        if (!rows.length) {
            return '<p style="margin:0 0 12px 0;">No attribution data in this range.</p>';
        }
        const totalRevenue = meta && typeof meta.estimated_total_revenue_usd === 'number' ? meta.estimated_total_revenue_usd : 0;
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.program_name || 'Unknown program'}</td>
                <td>${r.program_type || ''}</td>
                <td>${(r.attributed_clicks || 0).toFixed(2)}</td>
                <td>${(r.click_share_percent || 0).toFixed(2)}%</td>
                <td>$${(r.estimated_revenue_usd || 0).toFixed(2)}</td>
            </tr>
        `).join('');
        const modelLabel = meta && meta.model ? meta.model : '';
        return `
            <div style="margin-bottom:12px;">
                <h4 style="margin:0 0 6px 0;">Attribution (Estimated)</h4>
                <div style="font-size:12px;color:#50575e;margin-bottom:8px;">
                    Estimated total revenue: <strong>$${totalRevenue.toFixed(2)}</strong>
                </div>
                <div style="font-size:12px;color:#50575e;margin-bottom:8px;">
                    Model: ${modelLabel}
                </div>
                <table class="widefat">
                    <thead><tr><th>Program</th><th>Type</th><th>Attributed Clicks</th><th>Share</th><th>Est. Revenue</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderFunnel = (funnel) => {
        if (!funnel) {
            return '<p style="margin:0 0 12px 0;">No funnel data in this range.</p>';
        }
        const clicks = funnel.clicks || 0;
        const inserted = funnel.inserted || 0;
        const dismissed = funnel.dismissed || 0;
        const tracked = funnel.tracked_suggestions || 0;
        const clickRate = (funnel.insert_to_click_rate || 0).toFixed(2);
        const dismissRate = (funnel.dismiss_rate || 0).toFixed(2);
        return `
            <div style="margin-bottom:12px;">
                <h4 style="margin:0 0 6px 0;">Funnel</h4>
                <div style="display:flex;gap:8px;flex-wrap:wrap;">
                    <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                        <div style="font-size:12px;color:#50575e;">Inserted</div>
                        <div style="font-size:18px;font-weight:700;">${inserted}</div>
                    </div>
                    <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                        <div style="font-size:12px;color:#50575e;">Clicks</div>
                        <div style="font-size:18px;font-weight:700;">${clicks}</div>
                    </div>
                    <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                        <div style="font-size:12px;color:#50575e;">Dismissed</div>
                        <div style="font-size:18px;font-weight:700;">${dismissed}</div>
                    </div>
                    <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                        <div style="font-size:12px;color:#50575e;">Tracked Recs</div>
                        <div style="font-size:18px;font-weight:700;">${tracked}</div>
                    </div>
                    <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:160px;">
                        <div style="font-size:12px;color:#50575e;">Insert -> Click Rate</div>
                        <div style="font-size:18px;font-weight:700;">${clickRate}%</div>
                    </div>
                    <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                        <div style="font-size:12px;color:#50575e;">Dismiss Rate</div>
                        <div style="font-size:18px;font-weight:700;">${dismissRate}%</div>
                    </div>
                </div>
            </div>
        `;
    };

    const renderCohorts = (rows) => {
        if (!rows || !rows.length) {
            return '<p style="margin:0 0 12px 0;">No cohort data in this range.</p>';
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
            <div style="margin-bottom:12px;">
                <h4 style="margin:0 0 6px 0;">Cohorts (By Article Publish Date)</h4>
                <table class="widefat">
                    <thead><tr><th>Cohort Date</th><th>Articles</th><th>Inserted</th><th>Clicks</th><th>Clicks/Article</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderRetention = (rows) => {
        if (!rows || !rows.length) {
            return '<p style="margin:0 0 12px 0;">No retention data in this range.</p>';
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
            <div style="margin-bottom:12px;">
                <h4 style="margin:0 0 6px 0;">Retention/Churn (Weekly)</h4>
                <table class="widefat">
                    <thead><tr><th>Cohort Week</th><th>Articles</th><th>Week0 Active</th><th>W1 Retention</th><th>W2 Retention</th><th>W3 Retention</th><th>W3 Churn</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderRecommendationQuality = (summary, rows) => {
        if (!summary && (!rows || !rows.length)) {
            return '<p style="margin:0 0 12px 0;">No recommendation quality data in this range.</p>';
        }
        const s = summary || {};
        const totals = `
            <div style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:8px;">
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                    <div style="font-size:12px;color:#50575e;">Total Recs</div>
                    <div style="font-size:18px;font-weight:700;">${s.total || 0}</div>
                </div>
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                    <div style="font-size:12px;color:#50575e;">Approval Rate</div>
                    <div style="font-size:18px;font-weight:700;">${(s.approval_rate || 0).toFixed(2)}%</div>
                </div>
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                    <div style="font-size:12px;color:#50575e;">Dismiss Rate</div>
                    <div style="font-size:18px;font-weight:700;">${(s.dismiss_rate || 0).toFixed(2)}%</div>
                </div>
                <div style="background:#fff;border:1px solid #dcdcde;border-radius:4px;padding:10px;min-width:130px;">
                    <div style="font-size:12px;color:#50575e;">Insert Rate</div>
                    <div style="font-size:18px;font-weight:700;">${(s.insert_rate || 0).toFixed(2)}%</div>
                </div>
            </div>
        `;
        const tableRows = (rows || []).map(r => `
            <tr>
                <td>${r.program_name || 'Unknown program'}</td>
                <td>${r.program_type || ''}</td>
                <td>${r.total || 0}</td>
                <td>${(r.approval_rate || 0).toFixed(2)}%</td>
                <td>${(r.dismiss_rate || 0).toFixed(2)}%</td>
                <td>${(r.insert_rate || 0).toFixed(2)}%</td>
                <td>${(r.click_rate || 0).toFixed(2)}%</td>
            </tr>
        `).join('');
        return `
            <div style="margin-bottom:12px;">
                <h4 style="margin:0 0 6px 0;">Recommendation Quality</h4>
                ${totals}
                <table class="widefat">
                    <thead><tr><th>Program</th><th>Type</th><th>Total</th><th>Approve %</th><th>Dismiss %</th><th>Insert %</th><th>Click %</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
            </div>
        `;
    };

    const renderRecommendationTuning = (rows) => {
        if (!rows || !rows.length) {
            return '<p style="margin:0 0 12px 0;">No tuning profile data yet (needs at least 3 outcomes per program).</p>';
        }
        const tableRows = rows.map(r => `
            <tr>
                <td>${r.program_id || ''}</td>
                <td>${r.is_user_program ? 'user' : 'curated'}</td>
                <td>${r.sample_size || 0}</td>
                <td>${(r.approval_rate || 0).toFixed(2)}%</td>
                <td>${(r.insert_rate || 0).toFixed(2)}%</td>
                <td>${(r.click_rate || 0).toFixed(2)}%</td>
                <td>${(r.multiplier || 0).toFixed(2)}x</td>
            </tr>
        `).join('');
        return `
            <div style="margin-bottom:12px;">
                <h4 style="margin:0 0 6px 0;">Recommendation Tuning Profiles</h4>
                <table class="widefat">
                    <thead><tr><th>Program ID</th><th>Type</th><th>Sample</th><th>Approve %</th><th>Insert %</th><th>Click %</th><th>Multiplier</th></tr></thead>
                    <tbody>${tableRows}</tbody>
                </table>
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
                <div style="display:flex;justify-content:flex-end;gap:8px;margin-bottom:8px;">
                    <select id="affilio-attribution-model">
                        <option value="last_click">Model: Last Click</option>
                        <option value="first_click">Model: First Click</option>
                        <option value="linear_session">Model: Linear Session</option>
                    </select>
                    <button class="button" id="affilio-export-json">Export JSON</button>
                    <button class="button" id="affilio-export-csv">Export CSV</button>
                </div>
                ${renderSummary(summary)}
                ${renderTrends(trends)}
                ${renderFunnel(funnel)}
                ${renderCohorts(cohorts)}
                ${renderRetention(retention)}
                ${renderRecommendationQuality(qualitySummary, qualityPrograms)}
                ${renderRecommendationTuning(tuningProfiles)}
                ${renderAttribution(attribution, attributionMeta)}
                <div style="display:flex;flex-wrap:wrap;gap:16px;">
                    <div style="flex:1 1 320px;min-width:280px;">
                        <div style="display:flex;gap:8px;align-items:center;justify-content:space-between;flex-wrap:wrap;">
                            <h4 style="margin:0;">Top Articles</h4>
                            <div style="display:flex;gap:8px;">
                                <select id="affilio-analytics-range">
                                    <option value="0">All time</option>
                                    <option value="7">Last 7 days</option>
                                    <option value="30">Last 30 days</option>
                                </select>
                                <select id="affilio-articles-sort">
                                    <option value="clicks">Sort: Clicks</option>
                                    <option value="title">Sort: Title</option>
                                </select>
                                <select id="affilio-articles-limit">
                                    <option value="10">Top 10</option>
                                    <option value="25">Top 25</option>
                                    <option value="all">All</option>
                                </select>
                            </div>
                        </div>
                        ${renderArticleClicks(sortedArticles)}
                    </div>
                    <div style="flex:1 1 320px;min-width:280px;">
                        <div style="display:flex;gap:8px;align-items:center;justify-content:space-between;flex-wrap:wrap;">
                            <h4 style="margin:0;">Top Recommendations</h4>
                            <div style="display:flex;gap:8px;">
                                <select id="affilio-recs-sort">
                                    <option value="clicks">Sort: Clicks</option>
                                    <option value="name">Sort: Name</option>
                                </select>
                                <select id="affilio-recs-limit">
                                    <option value="10">Top 10</option>
                                    <option value="25">Top 25</option>
                                    <option value="all">All</option>
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
                    renderToast('Failed to export JSON');
                }
            });
            if (exportCsvEl) exportCsvEl.addEventListener('click', async () => {
                try {
                    await downloadExport('csv');
                } catch (e) {
                    renderToast('Failed to export CSV');
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
            renderAnalytics();
        };

        loadAnalytics();
    };

    Promise.all([
        fetch(AffilioData.apiUrl + '/articles', { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json()),
        fetch(AffilioData.apiUrl + '/user-programs', { headers: { 'X-API-Key': AffilioData.apiKey } }).then(r => r.json())
    ])
        .then(([articlesResp, programsResp]) => {
            render(
                articlesResp.articles || [],
                programsResp.programs || []
            );
        })
        .catch(() => { root.innerHTML = '<div class="affilio-panel affilio-panel-warning"><h2>Affilio Dashboard</h2><p>Failed to load data.</p></div>'; });
} )();
