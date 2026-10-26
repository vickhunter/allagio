var offerRailsShortTopicLimit = 400;

function offerRailsFindInputError(context) {
    const about = String(context || '').trim();
    if (about.length >= 10 && about.length <= 12000) return '';
    return 'Describe what this article is about (at least 10 characters). The article URL is optional.';
}

function offerRailsFindEmptyState(result, context, surfaceURL) {
    const reason = result && result.reason;
    const title = 'No relevant offers yet.';
    if (reason === 'no_eligible_offers') {
        return {
            title,
            detail: 'Only listed offers with verified payment configuration are eligible. Try another article or check back as builders join.',
            offerFetch: false,
        };
    }
    if (reason === 'below_relevance') {
        const score = Number(result.best_score);
        const threshold = Number(result.threshold);
        let detail = `The closest match is relevance ${score}/100, needs ${threshold}.`;
        const about = String(context || '').trim();
        const short = about.length < offerRailsShortTopicLimit;
        if (short) {
            detail += ' Paste a longer excerpt from the article, or fetch the text from the article URL.';
        }
        const url = String(surfaceURL || '').trim();
        return { title, detail, offerFetch: short && url !== '' };
    }
    return { title, detail: 'Try another article or check back as builders join.', offerFetch: false };
}

(() => {
    'use strict';
    const root = document.getElementById('workspace-app');
    const csrf = document.querySelector('meta[name="csrf-token"]')?.content || '';
    const escape = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
    const pill = value => `<span class="pill ${value === 'test' ? 'test' : value === 'live' ? 'live' : ''}">${escape(value)}</span>`;
    const placementStatusLabel = placement => placement.removed_at ? `Rimosso ${String(placement.removed_at).slice(0, 10)}` : placement.status;
    const money = (value, currency) => {
        try { return new Intl.NumberFormat(undefined, {style:'currency',currency}).format(value || 0); }
        catch { return `${Number(value || 0).toFixed(2)} ${currency}`; }
    };
    const apiOnce = async (path, method = 'GET', body) => {
        const response = await fetch('/owner/api' + path, {method, credentials:'same-origin', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, ...(body === undefined ? {} : {body:JSON.stringify(body)})});
        if (response.status === 401) { location.assign('/auth/login?return_to=' + encodeURIComponent(location.pathname)); throw new Error('Sign in to continue.'); }
        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
            const error = new Error(plainWords([data.error, data.hint].filter(Boolean).join(' ')) || 'The request could not be completed. Please retry.');
            error.status = response.status;
            error.data = data;
            throw error;
        }
        return data;
    };
    const plainWords = message => {
        const text = String(message || '');
        if (/readiness/i.test(text) && (/\/v1\//.test(text) || /\bPOST\s+\//.test(text))) {
            return 'This product has not passed readiness assessment. Fix the failed checks, then submit again.';
        }
        return text;
    };
    const isAsleep = error => error?.status === 502 || error?.status === 503 || error?.status === 504 || error?.name === 'AbortError' || (error instanceof TypeError);
    const api = async (path, method, body) => {
        try { return await apiOnce(path, method, body); }
        catch (error) {
            if (!isAsleep(error)) throw error;
            await ensureAwake();
            return apiOnce(path, method, body);
        }
    };
    const report = (message, bad = false, duplicate = false) => {
        const element = document.getElementById('workspace-feedback');
        if (!element) return;
        element.textContent = message;
        element.className = 'feedback ' + (bad ? 'error' : message ? 'success' : '');
        if (duplicate) element.setAttribute('aria-hidden', 'true');
        else element.removeAttribute('aria-hidden');
    };
    const WAKE_LIMIT = 60000;
    const WAKE_EVERY = 4000;
    const healthState = async () => {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 8000);
        try {
            const response = await fetch('/health', {cache:'no-store', signal:controller.signal});
            if (response.ok) return 'up';
            if (response.status === 502 || response.status === 503 || response.status === 504) return 'asleep';
            return 'other';
        } catch (error) {
            if (error?.name === 'AbortError' || error instanceof TypeError) return 'asleep';
            return 'other';
        } finally { clearTimeout(timer); }
    };
    const wakeMarkup = failed => failed
        ? '<section class="wake" role="alert"><h2>The server did not wake.</h2><p>It is still not answering.</p><button type="button" data-primary-action data-wake-retry>Try again</button></section>'
        : '<section class="wake" role="status"><h2>Waking the server, about 30 seconds</h2><p>This page will continue on its own.</p><div class="wake-track" aria-hidden="true"><span class="wake-bar" data-wake-bar></span></div></section>';
    const paintWake = failed => {
        if (!root) return;
        const html = wakeMarkup(failed);
        if (root.querySelector('form')) {
            let slot = document.getElementById('server-wake');
            if (!slot) {
                slot = document.createElement('div');
                slot.id = 'server-wake';
                root.prepend(slot);
            }
            slot.innerHTML = html;
        } else root.innerHTML = `<div id="server-wake">${html}</div>`;
        report('');
    };
    const setWakeBar = started => {
        const bar = document.querySelector('[data-wake-bar]');
        if (bar) bar.style.width = `${Math.min(100, Math.round(((Date.now() - started) / WAKE_LIMIT) * 100))}%`;
    };
    const pause = (ms, started) => new Promise(resolve => {
        const end = Date.now() + ms;
        const tick = () => {
            setWakeBar(started);
            const left = end - Date.now();
            if (left <= 0) resolve();
            else setTimeout(tick, Math.min(1000, left));
        };
        tick();
    });
    let wakePromise = null;
    const ensureAwake = () => {
        if (wakePromise) return wakePromise;
        wakePromise = (async () => {
            try {
                for (;;) {
                    const started = Date.now();
                    let shown = false;
                    while (Date.now() - started < WAKE_LIMIT) {
                        const state = await healthState();
                        if (state !== 'asleep') {
                            document.getElementById('server-wake')?.remove();
                            return;
                        }
                        if (!shown) { paintWake(false); shown = true; }
                        setWakeBar(started);
                        const left = WAKE_LIMIT - (Date.now() - started);
                        if (left <= 0) break;
                        await pause(Math.min(WAKE_EVERY, left), started);
                    }
                    paintWake(true);
                    await new Promise(resolve => document.querySelector('[data-wake-retry]')?.addEventListener('click', () => resolve(), {once:true}));
                }
            } finally { wakePromise = null; }
        })();
        return wakePromise;
    };
    const motion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';
    const scrollFocus = element => {
        if (!element) return;
        element.focus({preventScroll: true});
        element.scrollIntoView({block: 'center', inline: 'nearest', behavior: 'auto'});
    };
    // Inline error is the accessible one. #workspace-feedback keeps the same text for existing checks, hidden from assistive tech when it would only repeat.
    const showIssue = (anchor, id, message) => {
        if (!anchor) return;
        let slot = document.getElementById(id);
        if (!slot) {
            slot = document.createElement('p');
            slot.id = id;
            slot.className = 'field-error';
            slot.setAttribute('role', 'alert');
            slot.tabIndex = -1;
            anchor.insertAdjacentElement('afterend', slot);
        }
        slot.textContent = message;
        const described = (anchor.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean);
        if (!described.includes(id)) anchor.setAttribute('aria-describedby', described.concat(id).join(' '));
        if ('setAttribute' in anchor && anchor.matches?.('input, textarea, select')) anchor.setAttribute('aria-invalid', 'true');
        report(message, true, true);
        scrollFocus(slot);
    };
    const run = async (button, task) => {
        if (button) button.disabled = true;
        report('');
        try { await task(); } catch (error) { if (!error.displayed) report(error.message, true); }
        finally { if (button?.isConnected) button.disabled = false; }
    };
    const FIELD_HINTS = [['name', /name/i], ['website_url', /website/i], ['tagline', /tagline/i], ['docs_url', /docs_url/i], ['pricing_url', /pricing_url/i], ['keywords', /keyword/i], ['commission_value', /commission/i], ['landing_url', /landing_url/i], ['cookie_duration_days', /cookie/i], ['refund_window_days', /refund_window/i]];
    const showFormIssue = (form, error) => {
        error.displayed = true;
        let anchor = form.querySelector('button');
        for (const [name, pattern] of FIELD_HINTS) {
            if (pattern.test(error.message)) {
                const input = form.querySelector(`[name="${name}"]`);
                if (input) anchor = input;
                break;
            }
        }
        showIssue(anchor, 'form-error-' + (form.getAttribute('data-product') || form.getAttribute('data-new-offer') || 'field'), error.message);
    };
    // CheckoutStatus reads the Stripe confirmation page into this browser.
    // Owner JSON does not include connected, configuration_verified, or live_mode.
    const CheckoutStatus = (() => {
        const key = 'affilio.checkout.v1';
        const read = () => { try { return JSON.parse(sessionStorage.getItem(key) || '{}'); } catch { return {}; } };
        const write = value => { try { sessionStorage.setItem(key, JSON.stringify(value)); } catch { /* private mode */ } };
        return {
            remember() {
                const match = location.pathname.match(/^\/owner\/products\/([0-9a-f-]{36})\/connection$/i);
                if (!match) return;
                const text = document.body?.innerText || '';
                if (!/Stripe configuration verified/i.test(text)) return;
                const map = read();
                map[match[1]] = {connected:true, verified:true, live_mode:/Live mode:\s*true/i.test(text), source:'confirmation-page'};
                write(map);
            },
            forProduct(id) {
                const stored = read()[id];
                if (!stored) return {known:false, connected:false, verified:false, live_mode:false, source:'missing-api-field'};
                return {known:true, connected:!!stored.connected, verified:!!stored.verified, live_mode:!!stored.live_mode, source:stored.source || 'confirmation-page'};
            }
        };
    })();
    // OfferSeen remembers status in this tab. Owner JSON has no review_note, so a returned draft is only visible after this tab saw the offer submitted.
    const OfferSeen = (() => {
        const key = 'affilio.offer.seen.v1';
        const read = () => { try { return JSON.parse(sessionStorage.getItem(key) || '{}'); } catch { return {}; } };
        const write = value => { try { sessionStorage.setItem(key, JSON.stringify(value)); } catch { /* private mode */ } };
        return {
            remember(offers) {
                const map = read();
                const returned = [];
                for (const offer of offers) {
                    const prev = map[offer.id];
                    if (offer.status === 'draft' && (prev === 'submitted' || prev === 'returned')) {
                        if (prev === 'submitted') returned.push(offer.id);
                        map[offer.id] = 'returned';
                    } else if (offer.status !== 'draft') map[offer.id] = offer.status;
                    else if (!prev) map[offer.id] = 'draft';
                }
                write(map);
                return returned;
            },
            returned(id) { return read()[id] === 'returned'; }
        };
    })();
    const STATIONS = [
        {id:'product', label:'Product'},
        {id:'ready', label:'Ready'},
        {id:'stripe', label:'Checkout (optional)'},
        {id:'offer', label:'Offer'},
        {id:'submitted', label:'Submitted'},
        {id:'listed', label:'Listed'}
    ];
    const CHECK_LABELS = {site_reachable:'Site is reachable', checkout_present:'Checkout is present', ref_param_survives:'Referral parameter survives', pricing_page:'Pricing page', docs_present:'Documentation', privacy_policy:'Privacy policy', terms:'Terms', onboarding_flow:'Onboarding', payment_provider:'Payment provider'};
    const offerRank = status => ({listed:5, paused:4, submitted:3, draft:2})[status] || 0;
    const bestOffer = (offers, productId) => offers.filter(offer => offer.product_id === productId && offer.status !== 'retired').sort((a, b) => offerRank(b.status) - offerRank(a.status))[0] || null;
    const stationState = (product, offer, stripe) => {
        const done = {
            product: !!product,
            ready: product?.readiness_status === 'ready',
            stripe: !!(stripe?.known && stripe.connected && stripe.verified && stripe.live_mode === false),
            offer: !!offer && product?.readiness_status === 'ready',
            submitted: !!offer && offer.status !== 'draft',
            listed: !!offer && (offer.status === 'listed' || offer.status === 'paused')
        };
        const next = STATIONS.map(station => station.id).find(id => id !== 'stripe' && !done[id]) || null;
        return {done, next};
    };
    const describeProduct = (product, offers) => {
        const offer = product ? bestOffer(offers, product.id) : null;
        const stripe = product ? CheckoutStatus.forProduct(product.id) : {known:false, connected:false, verified:false, live_mode:false, source:'missing-api-field'};
        return {product, offer, stripe, state:stationState(product, offer, stripe)};
    };
    const pickFocus = (products, offers) => {
        if (!products.length) return describeProduct(null, offers);
        return products.map(product => describeProduct(product, offers)).sort((a, b) => {
            const rank = item => item.state.next ? STATIONS.findIndex(station => station.id === item.state.next) : STATIONS.length;
            return rank(a) - rank(b);
        })[0];
    };
    const primaryKind = (state, offer) => {
        if (!state.next && offer && (offer.status === 'listed' || offer.status === 'paused')) return 'listed-line';
        if (state.next === 'listed' && offer?.status === 'submitted') return 'waiting-line';
        return state.next;
    };
    const plainProgress = focus => {
        const next = focus.state.next;
        if (!focus.product) return 'No product yet. Next: register a product.';
        if (next === 'ready') return 'Your product is registered. Next: run the readiness check.';
        if (next === 'offer') return focus.state.done.stripe
            ? 'Stripe test mode is connected. Next: create a referral offer.'
            : 'Your product is ready. Next: create a referral offer.';
        if (next === 'submitted') return 'Your referral offer is saved. Next: submit it for review.';
        if (next === 'listed') return 'Your offer is submitted. Next: an operator lists it.';
        if (!next && focus.offer?.status === 'paused') return 'This offer is paused. Resume it when you want publishers to place it.';
        if (!next && focus.offer?.status === 'listed') return 'This offer is listed. Publishers can place it.';
        return 'Your offer is in progress.';
    };
    const humanSteps = focus => {
        const steps = [];
        if (!focus.state.done.stripe) steps.push('Checkout is optional. Connect a provider when you want one.');
        if (!focus.state.done.listed) steps.push('An operator approves the offer before it is listed.');
        return steps;
    };
    const stationDetail = (id, focus, returned) => {
        const {product, stripe} = focus;
        if (id === 'product') return 'Register the product this offer belongs to.';
        if (id === 'ready') return product?.readiness_status === 'failing' ? 'Readiness failed. Fix the checks below, then run it again.' : 'Run the readiness check against the live site.';
        if (id === 'stripe') return stripe?.known && stripe.live_mode ? 'Live mode is connected. This step needs Stripe test mode.' : 'Checkout is optional until a provider is connected.';
        if (id === 'offer') return 'Create the referral offer.';
        if (id === 'submitted') return returned ? 'The operator returned this offer. Update it and submit again.' : 'Submit the offer for operator review.';
        if (id === 'listed') return 'Waiting for the operator to list this offer.';
        return '';
    };
    const railMarkup = (focus, statusLine) => {
        const kind = primaryKind(focus.state, focus.offer);
        const returned = !!(focus.offer && OfferSeen.returned(focus.offer.id));
        const items = STATIONS.map(station => {
            const done = focus.state.done[station.id];
            const current = focus.state.next === station.id;
            const optional = station.id === 'stripe' && !done;
            const detail = optional
                ? (focus.stripe?.known && focus.stripe.live_mode ? 'no provider yet. Live mode is connected. This step needs Stripe test mode.' : 'no provider yet')
                : (current ? stationDetail(station.id, focus, returned) : '');
            const klass = done ? 'is-done' : optional ? 'is-optional' : current ? 'is-current' : 'is-todo';
            const flag = done ? 'Done' : current ? 'Now' : optional ? 'Optional' : 'Later';
            const optionalName = optional ? ` aria-label="${escape(station.label)}, optional step"` : '';
            const hidden = optional ? '<span class="visually-hidden">optional</span>' : '';
            return `<li class="${klass}" data-station="${station.id}"${optionalName} ${current ? 'aria-current="step"' : ''}>${hidden}<span class="station-flag">${flag}</span><span class="station-name">${station.label}</span>${detail ? `<span class="station-detail">${escape(detail)}</span>` : ''}</li>`;
        }).join('');
        const line = statusLine && kind === 'waiting-line'
            ? '<p class="offer-rail-status" data-primary-action tabindex="-1">Waiting for the operator to list this offer</p>'
            : statusLine && kind === 'listed-line'
                ? '<p class="offer-rail-status" data-primary-action tabindex="-1">Listed. Publishers can place this offer.</p>'
                : '';
        const who = focus.product ? escape(focus.product.name) : 'No product yet';
        return `<section class="offer-rail" aria-label="Offer progress"><p class="offer-rail-kicker">Offer rail</p><p class="offer-rail-who">${who}</p><ol>${items}</ol>${line}</section>`;
    };
    const checksMarkup = checks => {
        if (!checks?.length) return '';
        return `<ul class="check-list">${checks.map(check => {
            const label = CHECK_LABELS[check.key] || check.key;
            const gateFail = check.status === 'fail' && check.gate;
            const warn = check.status === 'warn' || (check.status === 'fail' && !check.gate);
            if (gateFail) {
                const id = 'check-' + check.key;
                return `<li class="check fail" id="${id}" aria-describedby="${id}-error"><span class="check-name">${escape(label)}</span><p class="field-error" id="${id}-error" role="alert" tabindex="-1">${escape(check.remediation || check.evidence || 'This check failed.')}</p></li>`;
            }
            if (warn) return `<li class="check warn"><span class="check-name">${escape(label)}</span><p class="check-note">${escape(check.remediation || check.evidence || '')}</p></li>`;
            return `<li class="check pass"><span class="check-name">${escape(label)}</span><p class="check-note">${escape(check.evidence || 'Passed.')}</p></li>`;
        }).join('')}</ul>`;
    };
    const field = (name, label, value = '', attributes = '', full = false) => `<label class="${full ? 'full' : ''}">${label}<input name="${name}" value="${escape(value)}" ${attributes}></label>`;
    const copyCommand = value => `<div class="copy-field setup-command"><div class="row"><span class="eyebrow">RUN IN YOUR PRODUCT REPOSITORY</span><button type="button" class="copy-action" data-copy>Copy command</button></div><pre>${escape(value)}</pre><span class="copy-status" role="status" aria-live="polite"></span></div>`;
    const empty = (title, text) => `<div class="empty"><h3>${title}</h3><p>${text}</p></div>`;
    const accountForm = (role) => `<section><h2>${role === 'maker' ? 'Connect your builder account' : 'Connect your publisher account'}</h2><form method="post" action="/owner/accounts"><input type="hidden" name="csrf" value="${escape(csrf)}"><input type="hidden" name="role" value="${role}">${role === 'publisher' ? field('domain','Your content domain','','placeholder="your-blog.com"') : ''}<details><summary>Connect an existing account</summary>${field('api_key','Existing owner key','','type="password" autocomplete="off"')}</details><button>Connect account</button></form></section>`;
    const commission = offer => offer.commission_type === 'percentage' ? `${offer.commission_value}% of the first purchase` : `${money(offer.commission_value,offer.currency || offer.commission_currency)} per first purchase`;
    let session;
    const overview = async () => {
        const makerHref = session.maker ? '/owner/products' : '/owner/products?setup=maker';
        const makerLabel = session.maker ? 'Open products' : 'Start as a builder';
        let rail = railMarkup(describeProduct(null, []), false);
        if (session.maker) {
            try {
                const [{products},{offers}] = await Promise.all([api('/products'),api('/offers')]);
                OfferSeen.remember(offers);
                rail = railMarkup(pickFocus(products, offers), false);
            } catch (error) {
                rail = `<section class="offer-rail" aria-label="Offer progress"><p class="offer-rail-kicker">Offer rail</p><p class="station-detail">${escape(error.message)} Open products to retry.</p></section>`;
            }
        }
        root.innerHTML = `${rail}<div class="grid"><section><span class="eyebrow">BUILDERS</span><h2>Give your product an offer.</h2><p>Connect your existing agent, prepare your checkout and choose the commission you want to offer.</p><a class="button" data-primary-action href="${makerHref}">${makerLabel} ↗</a></section><section><span class="eyebrow">PUBLISHERS</span><h2>Find a useful recommendation.</h2><p>Match an article to relevant products, approve the exact placement and publish a tracked link.</p><a class="button secondary" href="${session.publisher ? '/owner/placements' : '/owner/settings'}">${session.publisher ? 'Open placements' : 'Connect your content'} ↗</a></section></div><section><h2>One setup. Your own agent.</h2><p>Run this command in your product repository. Setup installs the integration skills and connects your agent. Checkout wiring, Stripe connection and verification are separate steps.</p>${copyCommand(session.install_command)}<p class="caption">The npm package offer-rails is not published yet. This command uses the installer archive on this host.</p><p class="caption">Node 20+ · Codex CLI handoff · Cursor and Claude configuration</p></section>`;
    };
    const productForm = (product, primaryRegister, registered) => `<section><h2>${product ? 'Edit product' : 'Register a product'}</h2><form data-product="${escape(product?.id || '')}"><div class="form-grid">${field('name','Product name',product?.name,'required maxlength="255"')}${field('website_url','Website',product?.website_url,'type="url" required maxlength="512"')}${field('tagline','What does it help people do?',product?.tagline,'required maxlength="512"',true)}${field('docs_url','Documentation URL',product?.docs_url,'type="url"')}${field('pricing_url','Pricing URL',product?.pricing_url,'type="url"')}${field('primary_category','Category',product?.primary_category,'placeholder="developer tools" autocomplete="off"')}${field('keywords','Topics, separated by commas',(product?.keywords || []).join(', '),'placeholder="postgres, monitoring, debugging" autocomplete="off"')}</div><p class="caption">Your product remains private until its offer is reviewed and listed.</p><button ${!product && primaryRegister ? 'data-primary-action' : 'class="secondary"'}>${product ? 'Save changes' : 'Register product'}</button>${product ? '<button type="button" class="secondary" data-cancel-edit>Cancel</button>' : ''}${registered ? '<p id="product-registered" class="success" role="status" tabindex="-1">Product registered. It is in your list.</p>' : ''}</form></section>`;
    const productsPage = async (options = {}) => {
        if (!session.maker) { root.innerHTML = '<section><h2>Start your builder account</h2><p>Your account can hold products and referral offers.</p><a class="button" data-primary-action href="/owner/products?setup=maker">Create builder account</a></section>'; return; }
        let products, offers;
        try {
            const loaded = await Promise.all([api('/products'), api('/offers')]);
            products = loaded[0].products || [];
            offers = loaded[1].offers || [];
        } catch (error) {
            root.innerHTML = `<section class="offer-rail"><p class="offer-rail-kicker">Offer rail</p><h2>Products could not be loaded.</h2><p class="field-error" role="alert" tabindex="-1">${escape(error.message)}</p><button type="button" data-primary-action data-retry-products>Retry</button></section>`;
            const alert = root.querySelector('[role="alert"]');
            scrollFocus(alert);
            root.querySelector('[data-retry-products]').addEventListener('click', () => run(root.querySelector('[data-retry-products]'), productsPage));
            return;
        }
        const justReturned = OfferSeen.remember(offers);
        const focus = pickFocus(products, offers);
        const kind = primaryKind(focus.state, focus.offer);
        let focusChecks = [];
        if (focus.product) {
            try {
                const result = await api(`/products/${focus.product.id}/assessments`);
                focusChecks = result.assessments?.[0]?.checks || [];
            } catch { focusChecks = []; }
        }
        const names = Object.fromEntries(products.map(product => [product.id, product.name]));
        const productSection = product => {
            const isFocus = product.id === focus.product?.id;
            const assessPrimary = isFocus && kind === 'ready';
            const stripePrimary = isFocus && kind === 'stripe';
            return `<section><div class="row"><h3>${escape(product.name)}</h3>${pill(product.readiness_status)}</div><p>${escape(product.tagline)}</p><p><a href="${escape(product.website_url)}" target="_blank" rel="noopener noreferrer">${escape(product.website_url)}</a></p><div class="actions"><button type="button" class="secondary" data-edit="${product.id}">Edit</button><button type="button" class="${assessPrimary ? '' : 'secondary'}" ${assessPrimary ? 'data-primary-action' : ''} data-assess="${product.id}">Check readiness</button><a class="button${stripePrimary ? '' : ' secondary'}" ${stripePrimary ? 'data-primary-action' : ''} href="/owner/products/${product.id}/connection">Connect Stripe</a><button type="button" class="secondary" data-checks="${product.id}">View checks</button></div><p class="assessment-status" data-assessment-live="${product.id}" aria-live="polite"></p><div data-assessment="${product.id}">${isFocus ? checksMarkup(focusChecks) : ''}</div></section>`;
        };
        const offerSection = offer => {
            const returned = offer.status === 'draft' && OfferSeen.returned(offer.id);
            const submitPrimary = kind === 'submitted' && offer.id === focus.offer?.id;
            const returnedNote = returned ? `<p class="field-error" role="alert" tabindex="-1" id="offer-returned-${offer.id}">The operator returned this offer to draft. Update it and submit again. The review note is not available from the API.</p>` : '';
            const action = offer.status === 'draft'
                ? `<button type="button" class="${submitPrimary ? '' : 'secondary'}" ${submitPrimary ? 'data-primary-action' : ''} data-submit-offer="${offer.id}">Submit for review</button>`
                : offer.status === 'listed' || offer.status === 'paused'
                    ? `<button type="button" class="secondary" data-offer="${offer.id}" data-status="${offer.status === 'listed' ? 'paused' : 'listed'}">${offer.status === 'listed' ? 'Pause offer' : 'Resume offer'}</button>`
                    : '<p class="caption">Awaiting operator review. Listing does not promise traffic or purchases.</p>';
            const tracked = typeof offer.tracked_link === 'string' && offer.tracked_link !== '' ? `<div class="copy-field"><div class="row"><span class="eyebrow">TRACKED LINK</span><button type="button" class="copy-action" data-copy data-copied="Copied. Paste the tracked link.">Copy link</button></div><pre>${escape(offer.tracked_link)}</pre><span class="copy-status" role="status" aria-live="polite"></span></div>` : '';
            return `<section><div class="row"><h3>${escape(names[offer.product_id] || 'Product')}</h3>${pill(offer.status)}</div><p class="offer-id">${escape(offer.id)}</p><p>${escape(commission(offer))} · ${offer.commission_currency} · ${offer.cookie_duration_days}-day attribution · ${offer.refund_window_days}-day refund hold</p><p>${escape(offer.terms)}</p>${tracked}${returnedNote}${action}</section>`;
        };
        const offerFormMarkup = () => `<form data-new-offer novalidate><div class="form-grid"><label class="full">Product<select name="product_id">${products.map(product => `<option value="${product.id}" ${product.id === focus.product?.id ? 'selected' : ''}>${escape(product.name)}</option>`).join('')}</select></label><label>Commission type<select name="commission_type"><option value="percentage">Percentage of first purchase</option><option value="flat">Fixed amount per first purchase</option></select></label>${field('commission_value','Commission value','','type="number" step="0.01" min="0.01" required')}<label>Currency<select name="commission_currency"><option>USD</option><option>EUR</option><option>GBP</option></select></label>${field('cookie_duration_days','Attribution window (days)','30','type="number" min="1" max="365" required')}${field('refund_window_days','Refund hold (days)','30','type="number" min="0" max="180" required')}${field('landing_url','Referral destination','','type="url" required',true)}<label class="full">Additional terms<textarea name="terms" maxlength="4000" placeholder="Explain any eligibility restrictions and how you will pay commissions."></textarea></label></div><p class="caption">The merchant pays commissions directly. Placement terms include a 10% service fee on commission; fee collection is not automated.</p><button class="${kind === 'offer' ? '' : 'secondary'}" ${kind === 'offer' ? 'data-primary-action' : ''}>Create offer</button></form>`;
        const offerCreator = () => {
            if (!products.length) return '';
            if (!offers.length || kind === 'offer') return `<section id="new-offer"><h2>Create a referral offer</h2>${offerFormMarkup()}</section>`;
            return `<section id="new-offer"><button type="button" class="secondary" data-expand-offer>New offer</button></section>`;
        };
        const ordered = focus.product ? [focus.product, ...products.filter(product => product.id !== focus.product.id)] : products;
        const showForm = !products.length || options.keepForm;
        const formBlock = showForm ? productForm(null, !products.length && kind === 'product', !!options.registered) : `<p><button type="button" class="secondary" data-add-product>Add product</button></p>`;
        root.innerHTML = `${railMarkup(focus, true)}<div id="product-form">${formBlock}</div>${products.length ? '' : empty('Your first product starts here.','Register its website and topics so relevant publishers can find it.')}${products.length ? '<h2>Your products</h2>' + ordered.map(productSection).join('') : ''}<h2>Your referral offers</h2>${offers.length ? offers.map(offerSection).join('') : empty('Choose what a referral is worth.','Create an offer after registering your product.')}${offerCreator()}<div id="maker-placements"></div>`;
        api('/maker-placements').then(data => {
            const slot = document.getElementById('maker-placements');
            if (!slot) return;
            const items = data.placements || [];
            slot.innerHTML = `<h2>Publisher placements</h2>${items.length ? items.map(placement => `<section><div class="row">${escape(placement.surface_url)} ${pill(placementStatusLabel(placement))}</div><p>${escape(placement.copy)}</p></section>`).join('') : empty('No publisher placements yet.','Placements of your offers show up here, including ones a publisher has removed.')}`;
        }).catch(() => {});
        root.querySelector('[data-product]')?.addEventListener('submit', saveProduct);
        root.querySelector('[data-add-product]')?.addEventListener('click', () => {
            document.getElementById('product-form').innerHTML = productForm(null, false, false);
            root.querySelector('[data-product]').addEventListener('submit', saveProduct);
            root.querySelector('[name="name"]').focus();
        });
        if (options.registered) scrollFocus(document.getElementById('product-registered'));
        root.querySelectorAll('[data-edit]').forEach(button => button.addEventListener('click', () => {
            const product = products.find(item => item.id === button.dataset.edit);
            document.getElementById('product-form').innerHTML = productForm(product, false);
            root.querySelector('[data-product]').addEventListener('submit', saveProduct);
            root.querySelector('[data-cancel-edit]').addEventListener('click', () => run(button, productsPage));
            root.querySelector('[name="name"]').focus();
        }));
        const showChecks = (id, checks) => {
            const slot = root.querySelector(`[data-assessment="${id}"]`);
            if (slot) slot.innerHTML = checks.length ? checksMarkup(checks) : '<p class="caption">No assessment recorded yet.</p>';
        };
        root.querySelectorAll('[data-assess]').forEach(button => button.addEventListener('click', () => {
            const live = button.closest('section')?.querySelector('[data-assessment-live]');
            if (live) live.textContent = 'Assessment running. Reading the live pages.';
            button.textContent = 'Checking live pages…';
            run(button, async () => {
                try {
                    const result = await api(`/products/${button.dataset.assess}/assess`, 'POST', {ref_param:'via'});
                    const failed = (result.checks || []).some(check => check.gate && check.status === 'fail');
                    await productsPage({focusCheck: failed});
                    if (failed) report(`Readiness: ${result.readiness_status}. Fix the checks below.`, true, true);
                    else report(`Readiness: ${result.readiness_status}. Review the checks before submitting an offer.`);
                } catch (error) {
                    error.displayed = true;
                    if (button.isConnected) showIssue(button, 'assess-error-' + button.dataset.assess, error.message);
                    throw error;
                }
            });
        }));
        root.querySelectorAll('[data-checks]').forEach(button => button.addEventListener('click', () => run(button, async () => {
            const result = await api(`/products/${button.dataset.checks}/assessments`);
            showChecks(button.dataset.checks, result.assessments?.[0]?.checks || []);
            const fail = root.querySelector(`[data-assessment="${button.dataset.checks}"] .check.fail .field-error`);
            if (fail) scrollFocus(fail);
        })));
        root.querySelectorAll('[data-submit-offer]').forEach(button => button.addEventListener('click', () => run(button, async () => {
            try { await api(`/offers/${button.dataset.submitOffer}/submit`, 'POST', {}); }
            catch (error) {
                error.displayed = true;
                showIssue(button, 'submit-error-' + button.dataset.submitOffer, error.message);
                throw error;
            }
            await productsPage();
            report('Offer submitted. Its current status is shown below.');
        })));
        root.querySelectorAll('[data-offer]').forEach(button => button.addEventListener('click', () => run(button, async () => { await api(`/offers/${button.dataset.offer}`, 'PATCH', {status:button.dataset.status}); await productsPage(); })));
        const bindNewOffer = form => {
            form.addEventListener('submit', event => {
                event.preventDefault();
                const missing = [...form.querySelectorAll('input[required], textarea[required], select[required]')].find(input => !String(input.value || '').trim());
                if (missing) {
                    const words = {commission_value:'Enter a commission value.', cookie_duration_days:'Enter the attribution window in days.', refund_window_days:'Enter the refund hold in days.', landing_url:'Enter the referral destination.'};
                    showIssue(missing, 'form-error-offer', words[missing.name] || 'Fill in this field.');
                    return;
                }
                const values = Object.fromEntries(new FormData(form));
                for (const key of ['commission_value','cookie_duration_days','refund_window_days']) values[key] = Number(values[key]);
                values.commission_unit = values.commission_type === 'percentage' ? 'percent' : 'currency'; values.ref_param = 'via';
                run(form.querySelector('button'), async () => {
                    try { await api('/offers', 'POST', values); }
                    catch (error) { showFormIssue(form, error); throw error; }
                    await productsPage();
                    report('Offer created. Check readiness and submit it for review. Checkout is optional.');
                });
            });
        };
        const openOffer = root.querySelector('[data-new-offer]');
        if (openOffer) bindNewOffer(openOffer);
        root.querySelector('[data-expand-offer]')?.addEventListener('click', () => {
            const section = document.getElementById('new-offer');
            section.innerHTML = `<h2>Create a referral offer</h2>${offerFormMarkup()}`;
            const form = section.querySelector('[data-new-offer]');
            bindNewOffer(form);
            form.querySelector('select, input, textarea')?.focus();
        });
        const firstFail = root.querySelector('.check.fail .field-error');
        if (firstFail && (options.focusCheck || focus.state.next === 'ready')) {
            if (options.focusCheck) scrollFocus(firstFail);
            else firstFail.scrollIntoView({block:'center', inline:'nearest', behavior:motion()});
        }
        const returned = justReturned[0] && document.getElementById('offer-returned-' + justReturned[0]);
        if (returned) scrollFocus(returned);
    };
    const topicValue = (form, name) => {
        const fields = [...form.querySelectorAll(`[name="${name}"]`)];
        for (const input of fields) {
            const live = String(input.value || '').trim();
            if (live) return live;
        }
        for (const input of fields) {
            const rendered = String(input.defaultValue || '').trim();
            if (rendered) return rendered;
        }
        return '';
    };
    const saveProduct = event => {
        event.preventDefault(); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
        const category = topicValue(form, 'primary_category');
        const keywords = topicValue(form, 'keywords').split(',').map(value => value.trim()).filter(Boolean);
        if (category) values.primary_category = category;
        else delete values.primary_category;
        if (keywords.length) values.keywords = keywords;
        else delete values.keywords;
        run(form.querySelector('button'), async () => {
            const creating = !form.dataset.product;
            try { await api('/products' + (creating ? '' : '/' + form.dataset.product), creating ? 'POST' : 'PATCH', values); }
            catch (error) { showFormIssue(form, error); throw error; }
            await productsPage(creating ? {registered: true, keepForm: true} : {});
            report(creating ? 'Product registered. It is in your list.' : 'Product saved.');
        });
    };
    const placementsPage = async () => {
        if (!session.publisher) { root.innerHTML = '<section><h2>Connect your content first.</h2><p>Install the WordPress plugin, connect your site, then use its publisher connection action. You can also create a publisher account for another publishing tool.</p><a class="button secondary" href="/downloads/offer-rails.zip">Download WordPress plugin</a></section>' + accountForm('publisher'); return; }
        const {placements} = await api('/placements');
        root.innerHTML = `<section><h2>Find a relevant offer</h2><form data-find><div class="form-grid">${field('surface_url','Published article URL','','type="url"',true)}<label class="full">What is this article about?<textarea name="context" minlength="10" maxlength="12000" required placeholder="Paste the article or describe its topic and audience."></textarea></label></div><button>Find opportunities</button></form><div id="opportunity-results"></div></section><h2>Placement inbox</h2>${placements.length ? placements.map(placement => `<section><div class="row"><a href="/owner/placements/${placement.id}">${escape(placement.surface_url)}</a>${pill(placementStatusLabel(placement))}</div><p>${escape(placement.copy)}</p><a href="/owner/placements/${placement.id}">Review placement ↗</a></section>`).join('') : empty('Your recommendations will appear here.','Start with an article where a product can help its readers.')}`;
        root.querySelector('[data-find]').addEventListener('submit',event => {
            event.preventDefault(); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
            const inputError = offerRailsFindInputError(values.context);
            if (inputError) { report(inputError, true); return; }
            run(form.querySelector('button'),async () => {
                const found = await api('/opportunities','POST',{context:values.context});
                const opportunities = found.opportunities || [];
                const results = document.getElementById('opportunity-results');
                if (opportunities.length) {
                    results.innerHTML = opportunities.map(offer => `<div class="card"><h3>${escape(offer.product_name)}</h3><p>${escape(offer.reason)}</p><p>${escape(commission(offer))} · ${escape(offer.currency)}</p><form data-prepare="${offer.offer_id}"><label>Recommendation copy<textarea name="copy" minlength="10" maxlength="4000" required>${escape('Consider ' + offer.product_name + ' if it fits the problem described in this article.')}</textarea></label><button>Prepare for owner review</button></form></div>`).join('');
                } else {
                    const state = offerRailsFindEmptyState(found, values.context, values.surface_url);
                    results.innerHTML = empty(state.title, state.detail) + (state.offerFetch ? '<p><button type="button" data-fetch-article>Fetch the article text</button></p>' : '');
                    const fetchButton = results.querySelector('[data-fetch-article]');
                    if (fetchButton) fetchButton.addEventListener('click', () => run(fetchButton, async () => {
                        const article = await api('/article-text','POST',{surface_url:values.surface_url});
                        const area = form.querySelector('textarea[name="context"]');
                        if (area && article.text) { area.value = article.text; form.requestSubmit(); }
                    }));
                }
                results.querySelectorAll('[data-prepare]').forEach(proposal => proposal.addEventListener('submit',event => {
                    event.preventDefault();
                    if (!String(values.surface_url || '').trim()) { report('Add the published article URL before preparing this placement.', true); return; }
                    run(proposal.querySelector('button'),async () => { const result = await api('/placements','POST',{offer_id:proposal.dataset.prepare,surface_url:values.surface_url,context:values.context,copy:new FormData(proposal).get('copy')}); location.assign('/owner/placements/' + result.placement.id); });
                }));
            });
        });
    };
    const table = (headers, rows) => `<div class="table-wrap"><table><thead><tr>${headers.map(header => `<th>${header}</th>`).join('')}</tr></thead><tbody>${rows.map(row => `<tr>${row.map(value => `<td>${value}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
    const conversionTable = (rows) => Array.isArray(rows) && rows.length ? `<section><h2>Sales and refunds</h2><p class="caption">A refund is applied only when its source is at least as trusted as the sale. A less trusted refund stays in review and does not change the commission.</p>${table(['Kind','Trust','Refund status','Amount','Reason'], rows.map(row => [escape(row.kind), escape(row.trust_level || ''), escape(row.refund_status || ''), escape(money(row.amount, row.currency)), escape(row.reason || '')]))}</section>` : '';
    const earningsPage = async () => {
        root.innerHTML = '<p>Commissions are paid directly by merchants. Test and live activity remain separate. A reported payment becomes received only after the publisher confirms it.</p>';
        if (session.maker) {
            const [data,payments] = await Promise.all([api('/earnings/maker'),api('/payments')]);
            root.insertAdjacentHTML('beforeend',`<section><h2>Your product referrals</h2>${data.offers.length ? table(['Product','Mode','Clicks','Purchases','Refunds','Revenue','Commission'],data.offers.map(offer => [escape(offer.product_name),pill(offer.mode),offer.clicks,offer.sales,offer.refunds,escape(money(offer.revenue,offer.currency)),escape(money(offer.commission,offer.currency))])) : empty('No attributed activity yet.','Listed offers and purchases will appear here as publishers refer readers.')}</section>${conversionTable(data.conversions)}`);
            root.insertAdjacentHTML('beforeend',`<section><h2>Pay your publishers</h2><p>Send payment using the publisher's instructions, then record its reference. Reporting a payment does not move money or confirm receipt.</p>${payments.payouts.length ? payments.payouts.map(payout => `<div class="card"><h3>${escape(payout.domain)}</h3><p>${escape(money(payout.amount,payout.currency))} · ${pill(payout.mode)} · ${pill(payout.status)}</p><p>${escape(payout.instructions || 'Contact the publisher through their site to arrange payment.')}</p>${payout.status === 'pending' && payout.mode === 'live' ? `<form data-report-payment="${payout.id}">${field('reference','Payment reference','','required maxlength="255"')}<label><input type="checkbox" name="sent" required> I have sent this payment outside Offer Rails.</label><button>Report payment sent</button></form>` : `<p class="caption">${payout.received ? 'Publisher confirmed receipt.' : 'Publisher receipt has not been confirmed.'}</p>`}</div>`).join('') : '<p>No payout batches due yet. Cleared commissions are batched by the operator.</p>'}</section>`);
            root.querySelectorAll('[data-report-payment]').forEach(form => form.addEventListener('submit',event => { event.preventDefault(); run(form.querySelector('button'),async () => { const data = new FormData(form); await api(`/payments/${form.dataset.reportPayment}/report`,'POST',{reference:data.get('reference'),sent:data.get('sent') === 'on'}); await earningsPage(); report('Payment reported. The publisher must confirm receipt separately.'); }); }));
        }
        if (session.publisher) {
            const [data,history] = await Promise.all([api('/earnings/publisher'),api('/history')]);
            root.insertAdjacentHTML('beforeend',`<section><h2>Your publisher commissions</h2>${data.balances.length ? table(['Currency','Mode','Pending','Cleared','Reserved','Payment reported','Received'],data.balances.map(balance => [escape(balance.currency),pill(balance.mode),escape(money(balance.pending,balance.currency)),escape(money(balance.cleared_unpaid,balance.currency)),escape(money(balance.reserved,balance.currency)),escape(money(balance.payment_reported,balance.currency)),escape(money(balance.paid,balance.currency))])) : empty('No commissions recorded yet.','Publish an approved placement and share it with readers who can benefit from the product.')}</section><section><h2>Commission history</h2><p class="caption">A refund is applied only when its source is at least as trusted as the sale. A less trusted refund stays in review and does not change the commission.</p>${history.ledger.length ? table(['Product','Mode','Kind','Amount','Status','Trust','Refund status'],history.ledger.map(entry => [escape(entry.product_name),pill(entry.mode),escape(entry.kind),escape(money(entry.amount,entry.currency)),escape(entry.status),escape(entry.trust_level || ''),escape(entry.refund_status || '')])) : '<p>No purchase or refund records yet.</p>'}</section>${conversionTable(history.conversions)}<section><h2>Merchant payment reports</h2>${history.payouts.length ? history.payouts.map(payout => `<div class="card">${escape(money(payout.amount,payout.currency))} · ${pill(payout.mode)} · ${pill(payout.status)}${payout.status === 'paid' && !payout.received_at && payout.mode === 'live' ? `<form method="post" action="/owner/payouts/${payout.id}/confirm-received"><input type="hidden" name="csrf" value="${escape(csrf)}"><button>Confirm funds arrived</button></form>` : payout.received_at ? '<p>You confirmed receipt.</p>' : ''}</div>`).join('') : '<p>No merchant payment reports yet.</p>'}</section>`);
        }
        if (!session.maker && !session.publisher) root.insertAdjacentHTML('beforeend',empty('Connect a product or publisher account.','Your activity will be recorded here once you start referring customers.'));
    };
    const settingsPage = async () => {
        const {grants} = await api('/agents');
        const skillURL = location.origin + '/SKILL.md';
        let skillNote = 'Coming soon. This skill file is not published yet.';
        try {
            const skill = await fetch(skillURL);
            if (skill.ok) skillNote = '';
        } catch { /* unpublished skill stays coming soon */ }
        const agentLine = `Read ${skillURL} and connect this site to Offer Rails`;
        let progress = 'Start a builder account to see offer progress.';
        let humans = ['Create a builder account before an agent can register a product.', 'An operator approves the offer before it is listed.'];
        if (session.maker) {
            try {
                const [{products},{offers}] = await Promise.all([api('/products'), api('/offers')]);
                OfferSeen.remember(offers);
                const focus = pickFocus(products, offers);
                progress = plainProgress(focus);
                humans = humanSteps(focus);
            } catch (error) {
                progress = error.message;
            }
        }
        const humanList = humans.length ? `<ul class="task-list">${humans.map(step => `<li>${escape(step)}</li>`).join('')}</ul>` : '<p>Nothing here needs a person right now.</p>';
        const skillStatus = `<p class="caption" data-skill-status>${skillNote ? escape(skillNote) : 'The skill file is available.'}</p>`;
        root.innerHTML = `<section><h2>Start an agent</h2><p>Give this line to an agent. It reads the skill file, then connects this site.</p><div class="copy-field agent-line"><div class="row"><span class="eyebrow">AGENT START</span><button type="button" class="copy-action${skillNote ? ' secondary' : ''}" data-copy data-copied="Copied. Paste it to your agent.">Copy line</button></div><pre>${escape(agentLine)}</pre><span class="copy-status" role="status" aria-live="polite"></span></div>${skillStatus}</section><section><h2>Where this offer stands</h2><p data-rail-plain>${escape(progress)}</p></section><section><h2>Still needs a person</h2>${humanList}</section><section><h2>Connect your existing agent</h2><p>Builders: run this in your product repository.</p>${copyCommand(session.install_command)}<p class="caption">The npm package offer-rails is not published yet. This command uses the installer archive on this host. The hosted MCP URL below is the working connection.</p><p>Hosted MCP URL</p><pre>${escape(session.mcp_url)}</pre><p class="caption">Choose maker or publisher permissions in the browser consent screen. Connections last up to seven days and can be revoked here.</p></section>${!session.publisher ? '<section><h2>WordPress publishers</h2><p>Upload and activate the plugin, connect the site, then connect its publisher account from the Distribution screen.</p><a class="button secondary" href="/downloads/offer-rails.zip">Download plugin ↓</a></section>' + accountForm('publisher') : ''}${!session.maker ? '<p><a href="/owner?setup=maker">Create a builder account ↗</a></p>' : ''}<section><h2>Connected agents</h2>${grants.length ? grants.map(grant => `<div class="card"><h3>${escape(grant.client_name)}</h3><p>${escape(grant.scope)} · expires ${escape(new Date(grant.expires_at).toLocaleString())}</p><form method="post" action="/owner/grants/${grant.id}/revoke"><input type="hidden" name="csrf" value="${escape(csrf)}"><button class="secondary">Disconnect agent</button></form></div>`).join('') : '<p>No agents connected yet.</p>'}</section><section><h2>Account</h2><p>${escape(session.email)}</p><p class="caption">Owner ID: ${escape(session.owner_id)}</p><form method="post" action="/owner/logout"><input type="hidden" name="csrf" value="${escape(csrf)}"><button class="secondary">Sign out</button></form></section>`;
        if (session.publisher) {
            const profile = await api('/payout-profile');
            root.insertAdjacentHTML('beforeend',`<section><h2>How merchants can arrange payment</h2><p>These instructions are shared only with merchants that have a payout due to you. Use a payment contact or contact-page URL.</p><form data-payout-profile><label>Payment contact instructions<textarea name="instructions" maxlength="2000">${escape(profile.instructions)}</textarea></label><button>Save instructions</button></form></section>`);
            root.querySelector('[data-payout-profile]').addEventListener('submit',event => { event.preventDefault(); const form = event.currentTarget; run(form.querySelector('button'),async () => { await api('/payout-profile','POST',{instructions:new FormData(form).get('instructions')}); report('Payment instructions saved.'); }); });
        }
    };
    const reviewPage = async () => {
        if (!session.operator) { root.innerHTML = empty('Operator access required.','Offer review is available only to the configured operator.'); return; }
        const {queue} = await api('/review/offers');
        root.innerHTML = queue.length ? queue.map(offer => `<section><h2>${escape(offer.product_name)}</h2><p>${escape(offer.product_tagline)}</p><p><a href="${escape(offer.website_url)}" target="_blank" rel="noopener noreferrer">${escape(offer.website_url)}</a></p><p>${escape(commission(offer))} · ${escape(offer.commission_currency)} · readiness ${escape(offer.readiness_status)}</p><p>${escape(offer.terms)}</p><div class="actions"><button data-review="${offer.id}" data-decision="approve">List offer</button><button class="secondary" data-review="${offer.id}" data-decision="reject">Return to draft</button></div></section>`).join('') : empty('No offers waiting for review.','New submitted offers will appear here.');
        root.insertAdjacentHTML('beforeend','<section><h2>Prepare commission payouts</h2><p>Batch cleared live commissions by merchant and publisher. This reserves the commissions and lets merchants report their external payments.</p><form data-batch><label>Minimum payout amount<input name="minimum" type="number" min="0.01" step="0.01" value="25" required></label><button>Prepare live payout batches</button></form></section>');
        root.querySelector('[data-batch]').addEventListener('submit',event => { event.preventDefault(); const form = event.currentTarget; run(form.querySelector('button'),async () => { const data = await api('/review/payouts/batch','POST',{min_payout:Number(new FormData(form).get('minimum')),include_test_mode:false}); report(`${data.payouts.length} payout batches prepared. No money was transferred.`); }); });
        root.querySelectorAll('[data-review]').forEach(button => button.addEventListener('click',() => run(button,async () => { await api(`/review/offers/${button.dataset.review}/${button.dataset.decision}`,'POST',{}); await reviewPage(); })));
    };
    document.querySelectorAll('form').forEach(form => {
        const key = form.querySelector('[name="stripe_key"]');
        if (!key) return;
        const secret = form.querySelector('[name="signing_secret"]');
        form.addEventListener('submit', event => {
            const value = key.value.trim().toLowerCase();
            if (value.startsWith('sk_live_') || value.startsWith('rk_live_') || value.startsWith('pk_live_')) {
                event.preventDefault();
                showIssue(form.querySelector('button') || key, 'stripe-live-key', 'Stripe test mode only. Live keys are not accepted.');
                return;
            }
            const signing = String(secret?.value || '').trim();
            if (signing.startsWith('whsec_') && signing.length >= 16 && signing.length <= 512) return;
            event.preventDefault();
            showIssue(form.querySelector('button') || secret || key, 'stripe-live-key', 'Use the signing secret from your Stripe webhook endpoint. It starts with whsec_.');
        });
    });
    document.querySelectorAll('nav a').forEach(link => { if (link.getAttribute('href') === location.pathname) link.setAttribute('aria-current','page'); });
    document.querySelectorAll('[data-commit-placement]').forEach(button => button.addEventListener('click',() => run(button,async () => { await api(`/placements/${button.dataset.commitPlacement}/commit`,'POST',{}); location.reload(); })));
    CheckoutStatus.remember();
    if (!root) return;
    root.insertAdjacentHTML('beforebegin','<div id="workspace-feedback" class="feedback" role="status" aria-live="polite"></div>');
    root.textContent = 'Loading your workspace…';
    const pages = {'/owner/products':['Products & offers',productsPage],'/owner/placements':['Placements',placementsPage],'/owner/earnings':['Earnings',earningsPage],'/owner/settings':['Setup & agents',settingsPage],'/owner/review':['Offer review',reviewPage]};
    const [title,render] = pages[location.pathname] || ['Your distribution workspace',overview];
    document.querySelector('h1').textContent = title;
    const boot = async () => {
        root.textContent = 'Loading your workspace…';
        try {
            if (await healthState() === 'asleep') await ensureAwake();
            session = await api('/session');
            await render();
        } catch (error) {
            if (isAsleep(error) && await healthState() === 'asleep') {
                await ensureAwake();
                return boot();
            }
            root.textContent = '';
            report(error.message, true);
        }
    };
    boot();
})();
