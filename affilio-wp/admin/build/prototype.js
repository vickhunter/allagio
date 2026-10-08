(() => {
    'use strict';
    const root = document.getElementById('prototype-app');
    const title = document.getElementById('proto-title');
    const lead = document.getElementById('proto-lead');
    const toggle = document.getElementById('example-toggle');
    const storeKey = 'distributoor.prototype.v1';
    const surfaces = ['Blog', 'X', 'YouTube', 'TikTok', 'Pinterest', 'LinkedIn', 'Newsletter', 'App'];
    const buckets = ['<1k', '1-10k', '10-100k', '100k+'];
    const filledSurfaces = [
        { surface: 'Blog', bucket: '10-100k', topic: 'Databases', segment: '40–100 readers' },
        { surface: 'Newsletter', bucket: '1-10k', topic: 'Postgres', segment: '20–40 readers' },
        { surface: 'X', bucket: '10-100k', topic: 'Developer tools', segment: '100–400 readers' },
    ];
    const escape = value => String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const clone = value => JSON.parse(JSON.stringify(value));

    const fresh = filled => ({ filled, sharing: true, surfaces: filled ? clone(filledSurfaces) : [] });
    const load = () => {
        try {
            const saved = JSON.parse(sessionStorage.getItem(storeKey) || 'null');
            if (saved && typeof saved.filled === 'boolean' && Array.isArray(saved.surfaces)) return saved;
        } catch { /* a fresh example is enough */ }
        return fresh(true);
    };
    let state = load();
    const save = () => { try { sessionStorage.setItem(storeKey, JSON.stringify(state)); } catch { /* the page still works for this visit */ } };

    const screen = () => {
        const path = location.pathname.replace(/\/+$/, '');
        if (path.endsWith('/audience')) return 'audience';
        if (path.endsWith('/payments')) return 'payments';
        return 'share';
    };

    const leaving = () => state.sharing ? 'on' : 'off';
    const sendLine = () => state.sharing ? 'Being sent' : 'Not being sent';

    const offerFacts = () => state.filled
        ? [['Project', 'Rail Tool'], ['Category', 'Developer tools'], ['Commission terms', '20% of the first purchase'], ['Revenue range', '$1k–10k a month']]
        : [];

    const offerCard = () => {
        const facts = offerFacts();
        const body = facts.length
            ? `<dl class="proto-facts">${facts.map(([name, value]) => `<dt>${escape(name)}</dt><dd>${escape(value)}</dd>`).join('')}</dl>`
            : '<p>No offer yet. Your agent fills this card after a product exists.</p>';
        return `<section class="proto-card" aria-label="Offer card" data-leaving="${leaving()}"><p class="proto-kicker">Offer card</p><p class="send-state">${sendLine()}</p>${body}</section>`;
    };

    const audienceRows = () => state.surfaces.map(row => {
        const count = row.segment ? escape(row.segment) : 'Not counted yet';
        return `<dt>${escape(row.surface)}</dt><dd>${escape(row.topic || 'Topic not set')} · ${escape(row.bucket)} · ${count}</dd>`;
    }).join('');

    const audienceCard = () => {
        const body = state.surfaces.length
            ? `<dl class="proto-facts">${audienceRows()}</dl>`
            : '<p>No audience yet. Pick a surface, a size bucket, and a topic.</p>';
        return `<section class="proto-card" aria-label="Audience card" data-leaving="${leaving()}"><p class="proto-kicker">Audience card</p><p class="send-state">${sendLine()}</p>${body}<p class="proto-note">Groups smaller than 20 people are not shared.</p></section>`;
    };

    const neverShared = () => `<section class="proto-card" aria-label="Never shared"><h2>Never shared</h2><ul class="never-list"><li>Customer names</li><li>Emails</li><li>Handles</li><li>Customer lists</li><li>IPs</li></ul></section>`;

    const nextLink = (href, label) => `<p class="proto-next"><a class="button" data-primary-action href="${href}">${label}</a></p>`;

    const shareScreen = () => {
        title.textContent = 'What my agent shares';
        lead.textContent = 'Your agent prepares two cards. You see them here. Nothing that names a person leaves.';
        document.title = 'What my agent shares · Distributoor';
        root.innerHTML = `<div class="switch-row"><button type="button" class="switch" role="switch" aria-checked="${state.sharing ? 'true' : 'false'}" aria-labelledby="sharing-label" data-sharing><span></span></button><p id="sharing-label"><strong>Sharing on/off</strong><br>${state.sharing ? 'Sharing is on.' : 'Sharing is off. These cards stay here.'}</p></div><div class="proto-split">${offerCard()}${audienceCard()}</div>${neverShared()}${nextLink('/prototype/audience', 'See where the audience lives')}`;
    };

    const audienceScreen = () => {
        title.textContent = 'Where my audience lives';
        lead.textContent = 'Pick the surfaces you already use. Each one needs only a size bucket and a topic.';
        document.title = 'Where my audience lives · Distributoor';
        const chips = surfaces.map(name => {
            const on = state.surfaces.some(row => row.surface === name);
            return `<button type="button" class="chip" data-surface="${escape(name)}" aria-pressed="${on ? 'true' : 'false'}">${escape(name)}</button>`;
        }).join('');
        const editors = state.surfaces.map(row => `<div class="surface-edit"><h3>${escape(row.surface)}</h3><label>Size for ${escape(row.surface)}<select data-size="${escape(row.surface)}">${buckets.map(bucket => `<option${bucket === row.bucket ? ' selected' : ''}>${escape(bucket)}</option>`).join('')}</select></label><label>Topic for ${escape(row.surface)}<input data-topic="${escape(row.surface)}" value="${escape(row.topic)}" maxlength="80" autocomplete="off"></label></div>`).join('');
        root.innerHTML = `<div class="proto-split"><section class="proto-card"><h2>Surfaces</h2><div class="chips">${chips}</div>${editors || '<p class="proto-note">Nothing picked yet.</p>'}</section><div id="audience-preview">${audienceCard()}</div></div>${nextLink('/prototype/payments', 'See who pays whom')}`;
    };

    const paymentsScreen = () => {
        title.textContent = 'Who pays whom';
        lead.textContent = 'You see how a sale was recorded. The money moves between seller and publisher.';
        document.title = 'Who pays whom · Distributoor';
        const provider = state.filled
            ? '<p><strong>Stripe</strong></p><p>Also recognized, and kept on this site: Polar, Lemon Squeezy, Paddle, and others.</p>'
            : '<p>No provider named yet. Your agent can read one from the project and keep the name here.</p>';
        const sales = state.filled
            ? '<ul class="sale-list"><li class="sale"><span>This month · First purchase</span><span class="tag">Confirmed by provider</span></li><li class="sale"><span>This month · First purchase</span><span class="tag">Reported by seller</span></li></ul>'
            : '<p>No sales in this example.</p>';
        root.innerHTML = `<section class="proto-card" aria-label="Payment provider"><h2>Seller&rsquo;s payment provider</h2>${provider}</section><section class="proto-card" aria-label="Sales"><h2>Sales</h2>${sales}</section><p class="money-line">We never hold money. Payouts go from seller to publisher.</p>${nextLink('/prototype/share', 'Review what is shared')}`;
    };

    const paint = () => {
        document.querySelectorAll('nav[aria-label="Prototype"] a').forEach(link => {
            if (link.dataset.screen === screen()) link.setAttribute('aria-current', 'page');
            else link.removeAttribute('aria-current');
        });
        toggle.textContent = state.filled ? 'Show empty example' : 'Show filled example';
        toggle.setAttribute('aria-pressed', state.filled ? 'true' : 'false');
        const view = { share: shareScreen, audience: audienceScreen, payments: paymentsScreen }[screen()];
        view();
    };

    const rowFor = name => state.surfaces.find(row => row.surface === name);

    root.addEventListener('click', event => {
        const sharing = event.target.closest('[data-sharing]');
        if (sharing) {
            state.sharing = !state.sharing;
            save();
            paint();
            return;
        }
        const chip = event.target.closest('[data-surface]');
        if (!chip) return;
        const name = chip.dataset.surface;
        const existing = rowFor(name);
        if (existing) state.surfaces = state.surfaces.filter(row => row.surface !== name);
        else state.surfaces.push({ surface: name, bucket: '<1k', topic: '', segment: '' });
        save();
        paint();
    });

    root.addEventListener('input', event => {
        const topic = event.target.closest('[data-topic]');
        if (!topic) return;
        const row = rowFor(topic.dataset.topic);
        if (!row) return;
        row.topic = topic.value;
        row.segment = '';
        save();
        const preview = document.getElementById('audience-preview');
        if (preview) preview.innerHTML = audienceCard();
    });

    root.addEventListener('change', event => {
        const size = event.target.closest('[data-size]');
        if (!size) return;
        const row = rowFor(size.dataset.size);
        if (!row) return;
        row.bucket = size.value;
        row.segment = '';
        save();
        const preview = document.getElementById('audience-preview');
        if (preview) preview.innerHTML = audienceCard();
    });

    toggle.addEventListener('click', () => {
        const sharing = state.sharing;
        state = fresh(!state.filled);
        state.sharing = sharing;
        save();
        paint();
    });

    paint();
})();
