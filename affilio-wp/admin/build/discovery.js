( function() {
    const root = document.getElementById('affilio-discovery-root');
    if (!root) return;
    if (!AffilioData.adminKey) {
        const settings = AffilioData.settingsUrl || '/wp-admin/admin.php?page=affilio-settings';
        root.innerHTML = `<h2>Program Discovery Queue</h2><p>Admin API key is missing. Configure it in advanced settings: <a href="${settings}">Open settings</a></p>`;
        return;
    }
    root.innerHTML = '<h2>Program Discovery Queue</h2><p>Loading...</p>';

    const render = (items, pagination) => {
        let html = '<h2>Program Discovery Queue</h2>';
        html += '<input id="affilio-discovery-filter" type="text" placeholder="Filter by domain" style="margin-bottom:12px;" />';
        if (!items.length) {
            html += '<p>No items pending.</p>';
            root.innerHTML = html;
            return;
        }
        const rows = items.map(item => {
            const sample = item.sample_url ? `<a href="${item.sample_url}" target="_blank" rel="noopener noreferrer">Preview</a>` : '';
            const logo = item.logo_url ? `<img src="${item.logo_url}" alt="" style="height:20px;vertical-align:middle;" />` : '';
            const hint = item.suggested_name ? `Suggested: ${item.suggested_name}` : '';
            const reviews = (item.reviews || []).map(r => `<div style="font-size:12px;">${r.status} by ${r.actor || 'system'} (${r.reviewed_at})</div>`).join('');
            return `
                <tr data-domain="${item.domain}" data-id="${item.id}">
                    <td>${logo} ${item.domain}</td>
                    <td>${sample}</td>
                    <td>${hint}</td>
                    <td>${item.sites_count}</td>
                    <td>${item.status}</td>
                    <td>
                        <button class="button" data-id="${item.id}" data-status="reviewed">Mark Reviewed</button>
                        <button class="button" data-id="${item.id}" data-status="rejected">Reject</button>
                        <button class="button" data-id="${item.id}" data-action="promote">Promote</button>
                        <button class="button" data-toggle="history" data-id="${item.id}">History</button>
                    </td>
                </tr>
                <tr class="affilio-history" data-history="${item.id}" style="display:none;">
                    <td colspan="6">${reviews || 'No history yet.'}</td>
                </tr>
            `;
        }).join('');
        html += `
            <table class="widefat" id="affilio-discovery-table">
                <thead><tr><th>Domain</th><th>Sample URL</th><th>Hint</th><th>Sites</th><th>Status</th><th>Action</th></tr></thead>
                <tbody>${rows}</tbody>
            </table>
        `;
        if (pagination) {
            html += `<div style="margin-top:12px;">
                <button class="button" id="affilio-prev" ${pagination.prev_offset === null ? 'disabled' : ''}>Prev</button>
                <span style="margin:0 8px;">Page ${pagination.page}</span>
                <button class="button" id="affilio-next" ${pagination.next_offset === null ? 'disabled' : ''}>Next</button>
            </div>`;
        }
        root.innerHTML = html;

        const filter = document.getElementById('affilio-discovery-filter');
        if (filter) {
            filter.addEventListener('input', () => {
                const q = filter.value.toLowerCase();
                root.querySelectorAll('#affilio-discovery-table tbody tr[data-domain]').forEach(row => {
                    const domain = row.getAttribute('data-domain') || '';
                    row.style.display = domain.toLowerCase().includes(q) ? '' : 'none';
                });
            });
        }

        root.querySelectorAll('button[data-toggle="history"]').forEach(btn => {
            btn.addEventListener('click', () => {
                const id = btn.getAttribute('data-id');
                const row = root.querySelector(`tr[data-history="${id}"]`);
                if (row) {
                    row.style.display = row.style.display === 'none' ? '' : 'none';
                }
            });
        });

        root.querySelectorAll('button[data-id][data-status]').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.getAttribute('data-id');
                const status = btn.getAttribute('data-status');
                await fetch(`${AffilioData.apiUrl}/program-discovery/queue/${id}/review`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'X-Admin-Key': AffilioData.adminKey },
                    body: JSON.stringify({ status, actor: AffilioData.actor || 'admin' }),
                });
                location.reload();
            });
        });

        root.querySelectorAll('button[data-action="promote"]').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.getAttribute('data-id');
                const row = btn.closest('tr');
                const domain = row ? row.getAttribute('data-domain') : '';
                const suggested = row ? row.querySelector('td:nth-child(3)') : null;
                const suggestedName = suggested ? (suggested.textContent || '').replace('Suggested:', '').trim() : '';
                const name = window.prompt('Program name', suggestedName || '');
                if (name === null) return;
                const company = window.prompt('Company (optional)', name || '');
                if (company === null) return;
                const category = window.prompt('Primary category (e.g., marketing, hosting)', 'other');
                if (category === null) return;
                const website = window.prompt('Website URL (optional)', domain ? `https://${domain}` : '');
                if (website === null) return;
                const signup = window.prompt('Signup URL (optional)', '');
                if (signup === null) return;
                await fetch(`${AffilioData.apiUrl}/program-discovery/queue/${id}/promote`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'X-Admin-Key': AffilioData.adminKey },
                    body: JSON.stringify({
                        name,
                        company,
                        primary_category: category,
                        website_url: website,
                        signup_url: signup,
                        actor: AffilioData.actor || 'admin'
                    }),
                });
                location.reload();
            });
        });

        const prev = document.getElementById('affilio-prev');
        const next = document.getElementById('affilio-next');
        if (prev) {
            prev.addEventListener('click', () => {
                if (pagination.prev_offset !== null) loadQueue(pagination.prev_offset);
            });
        }
        if (next) {
            next.addEventListener('click', () => {
                if (pagination.next_offset !== null) loadQueue(pagination.next_offset);
            });
        }
    };

    const loadQueue = (offset = 0) => {
        fetch(AffilioData.apiUrl + '/program-discovery/queue?offset=' + offset + '&limit=50', {
            headers: { 'X-Admin-Key': AffilioData.adminKey }
        })
            .then(r => r.json())
            .then(data => render(data.queue || [], data.pagination))
            .catch(() => { root.innerHTML = '<h2>Program Discovery Queue</h2><p>Failed to load queue.</p>'; });
    };

    loadQueue(0);
} )();
