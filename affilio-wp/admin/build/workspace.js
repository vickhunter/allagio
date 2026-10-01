(() => {
    'use strict';
    const root = document.getElementById('workspace-app');
    const csrf = document.querySelector('meta[name="csrf-token"]')?.content || '';
    const escape = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
    const pill = value => `<span class="pill ${value === 'test' ? 'test' : value === 'live' ? 'live' : ''}">${escape(value)}</span>`;
    const money = (value, currency) => {
        try { return new Intl.NumberFormat(undefined, {style:'currency',currency}).format(value || 0); }
        catch { return `${Number(value || 0).toFixed(2)} ${currency}`; }
    };
    const api = async (path, method = 'GET', body) => {
        const response = await fetch('/owner/api' + path, {method, credentials:'same-origin', headers:{'Content-Type':'application/json','X-CSRF-Token':csrf}, ...(body === undefined ? {} : {body:JSON.stringify(body)})});
        if (response.status === 401) { location.assign('/auth/login?return_to=' + encodeURIComponent(location.pathname)); throw new Error('Sign in to continue.'); }
        const data = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(data.error || 'The request could not be completed. Please retry.');
        return data;
    };
    const report = (message, bad = false) => {
        const element = document.getElementById('workspace-feedback');
        if (element) { element.textContent = message; element.className = 'feedback ' + (bad ? 'error' : 'success'); }
    };
    const run = async (button, task) => {
        if (button) button.disabled = true;
        report('');
        try { await task(); } catch (error) { report(error.message, true); }
        finally { if (button?.isConnected) button.disabled = false; }
    };
    const field = (name, label, value = '', attributes = '', full = false) => `<label class="${full ? 'full' : ''}">${label}<input name="${name}" value="${escape(value)}" ${attributes}></label>`;
    const copyCommand = value => `<div class="copy-field setup-command"><div class="row"><span class="eyebrow">RUN IN YOUR PRODUCT REPOSITORY</span><button type="button" class="copy-action" data-copy>Copy command</button></div><pre>${escape(value)}</pre><span class="copy-status" role="status" aria-live="polite"></span></div>`;
    const empty = (title, text) => `<div class="empty"><h3>${title}</h3><p>${text}</p></div>`;
    const accountForm = (role) => `<section><h2>${role === 'maker' ? 'Connect your builder account' : 'Connect your publisher account'}</h2><form method="post" action="/owner/accounts"><input type="hidden" name="csrf" value="${escape(csrf)}"><input type="hidden" name="role" value="${role}">${role === 'publisher' ? field('domain','Your content domain','','placeholder="your-blog.com"') : ''}<details><summary>Connect an existing account</summary>${field('api_key','Existing owner key','','type="password" autocomplete="off"')}</details><button>Connect account</button></form></section>`;
    const commission = offer => offer.commission_type === 'percentage' ? `${offer.commission_value}% of the first purchase` : `${money(offer.commission_value,offer.currency || offer.commission_currency)} per first purchase`;
    let session;
    const overview = () => {
        root.innerHTML = `<div class="grid"><section><span class="eyebrow">BUILDERS</span><h2>Give your product an offer.</h2><p>Connect your existing agent, prepare your checkout and choose the commission you want to offer.</p><a class="button" href="${session.maker ? '/owner/products' : '/owner?setup=maker'}">${session.maker ? 'Open products' : 'Start as a builder'} ↗</a></section><section><span class="eyebrow">PUBLISHERS</span><h2>Find a useful recommendation.</h2><p>Match an article to relevant products, approve the exact placement and publish a tracked link.</p><a class="button secondary" href="${session.publisher ? '/owner/placements' : '/owner/settings'}">${session.publisher ? 'Open placements' : 'Connect your content'} ↗</a></section></div><section><h2>One setup. Your own agent.</h2><p>Run this command in your product repository. Setup installs the integration skills and connects your agent. Checkout wiring, Stripe connection and verification are separate steps.</p>${copyCommand(session.install_command)}<p class="caption">Node 20+ · Codex CLI handoff · Cursor and Claude configuration</p></section>`;
    };
    const productForm = product => `<section><h2>${product ? 'Edit product' : 'Register a product'}</h2><form data-product="${escape(product?.id || '')}"><div class="form-grid">${field('name','Product name',product?.name,'required maxlength="255"')}${field('website_url','Website',product?.website_url,'type="url" required maxlength="512"')}${field('tagline','What does it help people do?',product?.tagline,'required maxlength="512"',true)}${field('docs_url','Documentation URL',product?.docs_url,'type="url"')}${field('pricing_url','Pricing URL',product?.pricing_url,'type="url"')}${field('primary_category','Category',product?.primary_category,'placeholder="developer tools"')}${field('keywords','Topics, separated by commas',(product?.keywords || []).join(', '),'placeholder="postgres, monitoring, debugging"')}</div><p class="caption">This version supports Stripe one-time checkout. Your product remains private until its offer is reviewed and listed.</p><button>${product ? 'Save changes' : 'Register product'}</button>${product ? '<button type="button" class="secondary" data-cancel-edit>Cancel</button>' : ''}</form></section>`;
    const productsPage = async () => {
        if (!session.maker) { root.innerHTML = '<section><h2>Start your builder account</h2><p>Your account can hold products and referral offers.</p><a class="button" href="/owner?setup=maker">Create builder account</a></section>'; return; }
        const [{products},{offers}] = await Promise.all([api('/products'),api('/offers')]);
        const names = Object.fromEntries(products.map(product => [product.id,product.name]));
        root.innerHTML = `<div id="product-form">${productForm()}</div><h2>Your products</h2>${products.length ? products.map(product => `<section><div class="row"><h3>${escape(product.name)}</h3>${pill(product.readiness_status)}</div><p>${escape(product.tagline)}</p><p><a href="${escape(product.website_url)}" target="_blank" rel="noopener noreferrer">${escape(product.website_url)}</a></p><div class="actions"><button class="secondary" data-edit="${product.id}">Edit</button><button class="secondary" data-assess="${product.id}">Check readiness</button><a class="button secondary" href="/owner/products/${product.id}/connection">Connect Stripe</a><button class="secondary" data-checks="${product.id}">View checks</button></div><div data-assessment="${product.id}"></div></section>`).join('') : empty('Your first product starts here.','Register its website and topics so relevant publishers can find it.')}<h2>Your referral offers</h2>${offers.length ? offers.map(offer => `<section><div class="row"><h3>${escape(names[offer.product_id] || 'Product')}</h3>${pill(offer.status)}</div><p>${escape(commission(offer))} · ${offer.commission_currency} · ${offer.cookie_duration_days}-day attribution · ${offer.refund_window_days}-day refund hold</p><p>${escape(offer.terms)}</p>${offer.status === 'draft' ? `<button data-submit-offer="${offer.id}">Submit for review</button>` : offer.status === 'listed' || offer.status === 'paused' ? `<button class="secondary" data-offer="${offer.id}" data-status="${offer.status === 'listed' ? 'paused' : 'listed'}">${offer.status === 'listed' ? 'Pause offer' : 'Resume offer'}</button>` : '<p class="caption">Awaiting operator review. Listing does not promise traffic or purchases.</p>'}</section>`).join('') : empty('Choose what a referral is worth.','Create an offer after registering your product.')}${products.length ? `<section><h2>Create a referral offer</h2><form data-new-offer><div class="form-grid"><label class="full">Product<select name="product_id">${products.map(product => `<option value="${product.id}">${escape(product.name)}</option>`).join('')}</select></label><label>Commission type<select name="commission_type"><option value="percentage">Percentage of first purchase</option><option value="flat">Fixed amount per first purchase</option></select></label>${field('commission_value','Commission value','','type="number" step="0.01" min="0.01" required')}<label>Currency<select name="commission_currency"><option>USD</option><option>EUR</option><option>GBP</option></select></label>${field('cookie_duration_days','Attribution window (days)','30','type="number" min="1" max="365" required')}${field('refund_window_days','Refund hold (days)','30','type="number" min="0" max="180" required')}${field('landing_url','Referral destination','','type="url" required',true)}<label class="full">Additional terms<textarea name="terms" maxlength="4000" placeholder="Explain any eligibility restrictions and how you will pay commissions."></textarea></label></div><p class="caption">The merchant pays commissions directly. Placement terms include a 10% service fee on commission; fee collection is not automated.</p><button>Create offer</button></form></section>` : ''}`;
        root.querySelector('[data-product]').addEventListener('submit',saveProduct);
        root.querySelectorAll('[data-edit]').forEach(button => button.addEventListener('click',() => {
            const product = products.find(item => item.id === button.dataset.edit);
            document.getElementById('product-form').innerHTML = productForm(product);
            root.querySelector('[data-product]').addEventListener('submit',saveProduct);
            root.querySelector('[data-cancel-edit]').addEventListener('click',() => run(button,productsPage));
            root.querySelector('[name="name"]').focus();
        }));
        const showChecks = (id, checks) => { root.querySelector(`[data-assessment="${id}"]`).innerHTML = checks.length ? `<ul class="task-list">${checks.map(check => `<li>${escape(check.key)}: ${escape(check.evidence + (check.remediation ? " Next: " + check.remediation : ""))}</li>`).join('')}</ul>` : '<p class="caption">No assessment recorded yet.</p>'; };
        root.querySelectorAll('[data-assess]').forEach(button => button.addEventListener('click',() => run(button,async () => {
            report('Checking your deployed pages…');
            const result = await api(`/products/${button.dataset.assess}/assess`,'POST',{ref_param:'via'});
            showChecks(button.dataset.assess,result.checks || []);
            report(`Readiness: ${result.readiness_status}. Review the checks before submitting an offer.`);
        })));
        root.querySelectorAll('[data-checks]').forEach(button => button.addEventListener('click',() => run(button,async () => {
            const result = await api(`/products/${button.dataset.checks}/assessments`);
            showChecks(button.dataset.checks,result.assessments?.[0]?.checks || []);
        })));
        root.querySelectorAll('[data-submit-offer]').forEach(button => button.addEventListener('click',() => run(button,async () => { await api(`/offers/${button.dataset.submitOffer}/submit`,'POST',{}); await productsPage(); report('Offer submitted. Its current status is shown below.'); })));
        root.querySelectorAll('[data-offer]').forEach(button => button.addEventListener('click',() => run(button,async () => { await api(`/offers/${button.dataset.offer}`,'PATCH',{status:button.dataset.status}); await productsPage(); })));
        root.querySelector('[data-new-offer]')?.addEventListener('submit',event => {
            event.preventDefault(); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
            for (const key of ['commission_value','cookie_duration_days','refund_window_days']) values[key] = Number(values[key]);
            values.commission_unit = values.commission_type === 'percentage' ? 'percent' : 'currency'; values.ref_param = 'via';
            run(form.querySelector('button'),async () => { await api('/offers','POST',values); await productsPage(); report('Offer created. Check readiness, connect Stripe and submit it for review.'); });
        });
    };
    const saveProduct = event => {
        event.preventDefault(); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
        values.keywords = values.keywords.split(',').map(value => value.trim()).filter(Boolean); values.payment_provider = 'stripe';
        run(form.querySelector('button'),async () => { await api('/products' + (form.dataset.product ? '/' + form.dataset.product : ''),form.dataset.product ? 'PATCH' : 'POST',values); await productsPage(); report('Product saved.'); });
    };
    const placementsPage = async () => {
        if (!session.publisher) { root.innerHTML = '<section><h2>Connect your content first.</h2><p>Install the WordPress plugin, connect your site, then use its publisher connection action. You can also create a publisher account for another publishing tool.</p><a class="button secondary" href="/downloads/affilio.zip">Download WordPress plugin</a></section>' + accountForm('publisher'); return; }
        const {placements} = await api('/placements');
        root.innerHTML = `<section><h2>Find a relevant offer</h2><form data-find><div class="form-grid">${field('surface_url','Published article URL','','type="url" required',true)}<label class="full">What is this article about?<textarea name="context" minlength="10" maxlength="12000" required placeholder="Paste the article or describe its topic and audience."></textarea></label></div><button>Find opportunities</button></form><div id="opportunity-results"></div></section><h2>Placement inbox</h2>${placements.length ? placements.map(placement => `<section><div class="row"><a href="/owner/placements/${placement.id}">${escape(placement.surface_url)}</a>${pill(placement.status)}</div><p>${escape(placement.copy)}</p><a href="/owner/placements/${placement.id}">Review placement ↗</a></section>`).join('') : empty('Your recommendations will appear here.','Start with an article where a product can help its readers.')}`;
        root.querySelector('[data-find]').addEventListener('submit',event => {
            event.preventDefault(); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
            run(form.querySelector('button'),async () => {
                const {opportunities} = await api('/opportunities','POST',{context:values.context});
                const results = document.getElementById('opportunity-results');
                results.innerHTML = opportunities.length ? opportunities.map(offer => `<div class="card"><h3>${escape(offer.product_name)}</h3><p>${escape(offer.reason)}</p><p>${escape(commission(offer))} · ${escape(offer.currency)}</p><form data-prepare="${offer.offer_id}"><label>Recommendation copy<textarea name="copy" minlength="10" maxlength="4000" required>${escape('Consider ' + offer.product_name + ' if it fits the problem described in this article.')}</textarea></label><button>Prepare for owner review</button></form></div>`).join('') : empty('No relevant offers yet.','Only listed offers with verified payment configuration are eligible. Try another article or check back as builders join.');
                results.querySelectorAll('[data-prepare]').forEach(proposal => proposal.addEventListener('submit',event => {
                    event.preventDefault();
                    run(proposal.querySelector('button'),async () => { const result = await api('/placements','POST',{offer_id:proposal.dataset.prepare,surface_url:values.surface_url,context:values.context,copy:new FormData(proposal).get('copy')}); location.assign('/owner/placements/' + result.placement.id); });
                }));
            });
        });
    };
    const table = (headers, rows) => `<div class="table-wrap"><table><thead><tr>${headers.map(header => `<th>${header}</th>`).join('')}</tr></thead><tbody>${rows.map(row => `<tr>${row.map(value => `<td>${value}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
    const earningsPage = async () => {
        root.innerHTML = '<p>Commissions are paid directly by merchants. Test and live activity remain separate. A reported payment becomes received only after the publisher confirms it.</p>';
        if (session.maker) {
            const [data,payments] = await Promise.all([api('/earnings/maker'),api('/payments')]);
            root.insertAdjacentHTML('beforeend',`<section><h2>Your product referrals</h2>${data.offers.length ? table(['Product','Mode','Clicks','Purchases','Refunds','Revenue','Commission'],data.offers.map(offer => [escape(offer.product_name),pill(offer.mode),offer.clicks,offer.sales,offer.refunds,escape(money(offer.revenue,offer.currency)),escape(money(offer.commission,offer.currency))])) : empty('No attributed activity yet.','Listed offers and purchases will appear here as publishers refer readers.')}</section>`);
            root.insertAdjacentHTML('beforeend',`<section><h2>Pay your publishers</h2><p>Send payment using the publisher's instructions, then record its reference. Reporting a payment does not move money or confirm receipt.</p>${payments.payouts.length ? payments.payouts.map(payout => `<div class="card"><h3>${escape(payout.domain)}</h3><p>${escape(money(payout.amount,payout.currency))} · ${pill(payout.mode)} · ${pill(payout.status)}</p><p>${escape(payout.instructions || 'Contact the publisher through their site to arrange payment.')}</p>${payout.status === 'pending' && payout.mode === 'live' ? `<form data-report-payment="${payout.id}">${field('reference','Payment reference','','required maxlength="255"')}<label><input type="checkbox" name="sent" required> I have sent this payment outside Distributoor.</label><button>Report payment sent</button></form>` : `<p class="caption">${payout.received ? 'Publisher confirmed receipt.' : 'Publisher receipt has not been confirmed.'}</p>`}</div>`).join('') : '<p>No payout batches due yet. Cleared commissions are batched by the operator.</p>'}</section>`);
            root.querySelectorAll('[data-report-payment]').forEach(form => form.addEventListener('submit',event => { event.preventDefault(); run(form.querySelector('button'),async () => { const data = new FormData(form); await api(`/payments/${form.dataset.reportPayment}/report`,'POST',{reference:data.get('reference'),sent:data.get('sent') === 'on'}); await earningsPage(); report('Payment reported. The publisher must confirm receipt separately.'); }); }));
        }
        if (session.publisher) {
            const [data,history] = await Promise.all([api('/earnings/publisher'),api('/history')]);
            root.insertAdjacentHTML('beforeend',`<section><h2>Your publisher commissions</h2>${data.balances.length ? table(['Currency','Mode','Pending','Cleared','Reserved','Payment reported','Received'],data.balances.map(balance => [escape(balance.currency),pill(balance.mode),escape(money(balance.pending,balance.currency)),escape(money(balance.cleared_unpaid,balance.currency)),escape(money(balance.reserved,balance.currency)),escape(money(balance.payment_reported,balance.currency)),escape(money(balance.paid,balance.currency))])) : empty('No commissions recorded yet.','Publish an approved placement and share it with readers who can benefit from the product.')}</section><section><h2>Commission history</h2>${history.ledger.length ? table(['Product','Mode','Kind','Amount','Status'],history.ledger.map(entry => [escape(entry.product_name),pill(entry.mode),escape(entry.kind),escape(money(entry.amount,entry.currency)),escape(entry.status)])) : '<p>No purchase or refund records yet.</p>'}</section><section><h2>Merchant payment reports</h2>${history.payouts.length ? history.payouts.map(payout => `<div class="card">${escape(money(payout.amount,payout.currency))} · ${pill(payout.mode)} · ${pill(payout.status)}${payout.status === 'paid' && !payout.received_at && payout.mode === 'live' ? `<form method="post" action="/owner/payouts/${payout.id}/confirm-received"><input type="hidden" name="csrf" value="${escape(csrf)}"><button>Confirm funds arrived</button></form>` : payout.received_at ? '<p>You confirmed receipt.</p>' : ''}</div>`).join('') : '<p>No merchant payment reports yet.</p>'}</section>`);
        }
        if (!session.maker && !session.publisher) root.insertAdjacentHTML('beforeend',empty('Connect a product or publisher account.','Your activity will be recorded here once you start referring customers.'));
    };
    const settingsPage = async () => {
        const {grants} = await api('/agents');
        root.innerHTML = `<section><h2>Connect your existing agent</h2><p>Builders: run this in your product repository.</p>${copyCommand(session.install_command)}<p>Hosted MCP URL</p><pre>${escape(session.mcp_url)}</pre><p class="caption">Choose maker or publisher permissions in the browser consent screen. Connections last up to seven days and can be revoked here.</p></section>${!session.publisher ? '<section><h2>WordPress publishers</h2><p>Upload and activate the plugin, connect the site, then connect its publisher account from the Distribution screen.</p><a class="button secondary" href="/downloads/affilio.zip">Download plugin ↓</a></section>' + accountForm('publisher') : ''}${!session.maker ? '<p><a href="/owner?setup=maker">Create a builder account ↗</a></p>' : ''}<section><h2>Connected agents</h2>${grants.length ? grants.map(grant => `<div class="card"><h3>${escape(grant.client_name)}</h3><p>${escape(grant.scope)} · expires ${escape(new Date(grant.expires_at).toLocaleString())}</p><form method="post" action="/owner/grants/${grant.id}/revoke"><input type="hidden" name="csrf" value="${escape(csrf)}"><button class="secondary">Disconnect agent</button></form></div>`).join('') : '<p>No agents connected yet.</p>'}</section><section><h2>Account</h2><p>${escape(session.email)}</p><p class="caption">Owner ID: ${escape(session.owner_id)}</p><form method="post" action="/owner/logout"><input type="hidden" name="csrf" value="${escape(csrf)}"><button class="secondary">Sign out</button></form></section>`;
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
    document.querySelectorAll('nav a').forEach(link => { if (link.getAttribute('href') === location.pathname) link.setAttribute('aria-current','page'); });
    document.querySelectorAll('[data-commit-placement]').forEach(button => button.addEventListener('click',() => run(button,async () => { await api(`/placements/${button.dataset.commitPlacement}/commit`,'POST',{}); location.reload(); })));
    if (!root) return;
    root.insertAdjacentHTML('beforebegin','<div id="workspace-feedback" class="feedback" role="status" aria-live="polite"></div>');
    root.textContent = 'Loading your workspace…';
    const pages = {'/owner/products':['Products & offers',productsPage],'/owner/placements':['Placements',placementsPage],'/owner/earnings':['Earnings',earningsPage],'/owner/settings':['Setup & agents',settingsPage],'/owner/review':['Offer review',reviewPage]};
    const [title,render] = pages[location.pathname] || ['Your distribution workspace',overview];
    document.querySelector('h1').textContent = title;
    api('/session').then(value => { session = value; return render(); }).catch(error => { root.textContent = ''; report(error.message,true); });
})();
