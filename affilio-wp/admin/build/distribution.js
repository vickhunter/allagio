function affilioEmptyOfferMessage(result, copy) {
    const text = copy || {};
    const below = text.belowRelevance || 'Nessuna offerta adatta: la più vicina ha rilevanza %1$d/100, serve almeno %2$d.';
    const none = text.noEligibleOffers || 'Nessuna offerta idonea al momento.';
    if (result && result.reason === 'below_relevance') {
        return below.replace('%1$d', String(result.best_score)).replace('%2$d', String(result.threshold));
    }
    return none;
}

(() => {
    'use strict';
    const root = document.getElementById('affilio-distribution-root');
    if (!root) return;
    const escape = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
    const call = async (path = '', method = 'GET', body) => {
        const response = await fetch(AffilioDistribution.restBase.replace(/\/$/,'') + '/distribution' + path, {method,headers:{'X-WP-Nonce':AffilioDistribution.nonce,'Content-Type':'application/json'},...(body === undefined ? {} : {body:JSON.stringify(body)})});
        const data = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(data.message || 'Request failed. Please retry.');
        return data;
    };
    const message = (text, bad = false) => {
        const element = document.getElementById('distribution-message');
        if (!element) return;
        element.textContent = text;
        element.className = bad ? 'distribution-error' : '';
        if (bad) element.setAttribute('aria-hidden', 'true');
        else element.removeAttribute('aria-hidden');
    };
    const clearFieldError = () => root.querySelectorAll('.distribution-field-error').forEach(node => node.remove());
    const showFieldError = (anchor, text) => {
        message(text, true);
        if (!anchor) return;
        const slot = document.createElement('p');
        slot.className = 'distribution-field-error';
        slot.setAttribute('role', 'alert');
        slot.tabIndex = -1;
        slot.id = 'distribution-field-error';
        slot.textContent = text;
        anchor.insertAdjacentElement('afterend', slot);
        const described = (anchor.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean);
        if (!described.includes(slot.id)) anchor.setAttribute('aria-describedby', described.concat(slot.id).join(' '));
        slot.focus({preventScroll:true});
        slot.scrollIntoView({block:'center', inline:'nearest'});
    };
    const markNext = () => {
        root.querySelectorAll('[data-primary-action]').forEach(element => element.removeAttribute('data-primary-action'));
        root.querySelectorAll('.button-primary').forEach(element => element.classList.remove('button-primary'));
        const pick = root.querySelector('[data-connect-owner]')
            || root.querySelector('[data-publish]')
            || root.querySelector('[data-needs-review]')
            || root.querySelector('[data-find] button')
            || root.querySelector('a.button')
            || root.querySelector('[data-next-note]');
        if (!pick) return;
        pick.setAttribute('data-primary-action', '');
        pick.classList.add('button-primary');
    };
    const run = async (button, task) => {
        button.disabled = true; message(''); clearFieldError();
        try { await task(); } catch (error) { showFieldError(button, error.message); }
        finally { if (button.isConnected) button.disabled = false; }
    };
    let status;
    let articles;
    const refreshPlacements = async () => {
        const {placements} = await call('/placements');
        const inbox = document.getElementById('distribution-inbox');
        inbox.innerHTML = placements.length ? placements.map(placement => {
            const article = articles.find(item => item.url === placement.surface_url);
            const reported = placement.receipt?.publication_status === 'publisher_reported';
            const removed = placement.removed_at ? `Rimosso ${String(placement.removed_at).slice(0, 10)}` : '';
            const needsReview = !removed && !reported && placement.status !== 'approved' && placement.status !== 'committed';
            const revert = !removed && placement.revert_url ? `<a class="button" href="${escape(placement.revert_url)}">Rimuovi inserimento</a>` : '';
            return `<section><div class="distribution-row"><h3>${escape(article?.title || placement.surface_url)}</h3><span class="distribution-pill">${escape(removed || (reported ? 'published' : placement.status))}</span></div><p>${escape(placement.copy)}</p><p>${escape(placement.reason)}</p><div class="distribution-actions"><a class="button" ${needsReview ? 'data-needs-review' : ''} href="${escape(status.owner_url + '/placements/' + placement.id)}" target="_blank" rel="noopener noreferrer">Review copy & terms ↗</a>${article && (placement.status === 'approved' || placement.status === 'committed') && !reported && !removed ? `<button class="button" data-publish="${placement.id}" data-post="${article.id}">Insert approved recommendation</button>` : ''}${reported && !removed ? `<a href="${escape(placement.surface_url)}" target="_blank" rel="noopener noreferrer">View article ↗</a>` : ''}${revert}</div></section>`;
        }).join('') : '<section><h3>No placements yet.</h3><p>Choose an article to find relevant offers. You approve the recommendation before it is inserted.</p></section>';
        inbox.querySelectorAll('[data-publish]').forEach(button => button.addEventListener('click',() => run(button,async () => { await call('/publish','POST',{placement_id:button.dataset.publish,post_id:Number(button.dataset.post)}); await refreshPlacements(); message('Published. The approved recommendation and disclosure are in your article.'); })));
        markNext();
    };
    const render = async () => {
        status = await call();
        root.innerHTML = '<p id="distribution-message" role="status" aria-live="polite"></p>';
        if (!status.site_connected) { root.insertAdjacentHTML('beforeend',`<section><h2>Connect your site first.</h2><a class="button" href="${escape(status.settings_url)}">Open setup</a></section>`); markNext(); return; }
        if (!status.native_available) { root.insertAdjacentHTML('beforeend','<section><h2>Distribution is not enabled on this service.</h2><p>Your existing scan and affiliate tools remain available. Connect to the updated Allagio service to use native placements.</p></section>'); return; }
        if (!status.publisher_connected) {
            root.insertAdjacentHTML('beforeend','<section><h2>Connect your publisher account.</h2><p>Sign in once to review placements for this WordPress site and connect your agent. Your WordPress administrator session proves access to this installation.</p><button class="button" data-connect-owner>Connect publisher account</button><p data-owner-link></p><button class="button" data-refresh>Refresh connection</button></section>');
            root.querySelector('[data-connect-owner]').addEventListener('click',event => run(event.currentTarget,async () => {
                const data = await call('/connect','POST',{});
                const link = root.querySelector('[data-owner-link]');
                const anchor = document.createElement('a'); anchor.href = data.connect_url; anchor.target = '_blank'; anchor.rel = 'noopener noreferrer'; anchor.textContent = 'Sign in and confirm this site ↗'; anchor.className = 'button'; link.replaceChildren(anchor);
                message('Open the connection link, confirm this site, then refresh. The link expires in ten minutes.');
                markNext();
            }));
            root.querySelector('[data-refresh]').addEventListener('click',event => run(event.currentTarget,render));
            markNext();
            return;
        }
        ({articles} = await call('/articles'));
        root.insertAdjacentHTML('beforeend',`<section><div class="distribution-row"><h2>Your publisher account is connected.</h2><a href="${escape(status.owner_url + '/earnings')}" target="_blank" rel="noopener noreferrer">Earnings ↗</a></div><p>Search an article for offers with verified payment configuration. Approval and insertion remain separate actions.</p>${articles.length ? `<form data-find><label>Published article<select name="post_id">${articles.map(article => `<option value="${article.id}">${escape(article.title)}</option>`).join('')}</select></label><button class="button">Find relevant offers</button></form><div id="distribution-results"></div>` : '<p data-next-note>Publish your first article to find relevant offers.</p>'}</section><div class="distribution-row"><h2>Your placements</h2><button class="button" data-refresh-inbox>Refresh approvals</button></div><div id="distribution-inbox"></div>`);
        root.querySelector('[data-refresh-inbox]').addEventListener('click',event => run(event.currentTarget,refreshPlacements));
        root.querySelector('[data-find]')?.addEventListener('submit',event => {
            event.preventDefault(); const form = event.currentTarget; const postID = Number(new FormData(form).get('post_id'));
            run(form.querySelector('button'),async () => {
                const data = await call('/opportunities','POST',{post_id:postID});
                const opportunities = data.opportunities || [];
                const results = document.getElementById('distribution-results');
                results.innerHTML = opportunities.length ? opportunities.map(offer => `<section><h3>${escape(offer.product_name)}</h3><p>${escape(offer.reason)}</p><p>${escape(offer.commission_value)}${offer.commission_type === 'percentage' ? '%' : ' ' + escape(offer.currency)} per first paid checkout · ${offer.refund_window_days}-day refund hold</p><form data-offer="${offer.offer_id}"><label>Recommendation copy<textarea name="copy" minlength="10" maxlength="4000" required>${escape('Valuta ' + offer.product_name + ' se risolve il problema descritto in questo articolo.')}</textarea></label><button class="button">Prepare for review</button></form></section>`).join('') : `<p>${escape(affilioEmptyOfferMessage(data, AffilioDistribution))}</p>`;
                results.querySelectorAll('[data-offer]').forEach(proposal => proposal.addEventListener('submit',event => {
                    event.preventDefault();
                    run(proposal.querySelector('button'),async () => { await call('/prepare','POST',{post_id:postID,offer_id:proposal.dataset.offer,copy:new FormData(proposal).get('copy')}); await refreshPlacements(); message('Placement prepared. Open Review copy & terms to approve it, then refresh approvals here.'); });
                }));
            });
        });
        await refreshPlacements();
    };
    const showLoadFailure = error => {
        root.innerHTML = `<p id="distribution-message" class="distribution-error" aria-hidden="true"></p><p class="distribution-field-error" role="alert" tabindex="-1">${escape(error.message)}</p><button class="button button-primary" data-primary-action data-retry>Retry</button>`;
        const alert = root.querySelector('[role="alert"]');
        alert.focus();
        root.querySelector('[data-retry]').addEventListener('click', () => { root.textContent = 'Loading distribution…'; render().catch(showLoadFailure); });
    };
    root.textContent = 'Loading distribution…';
    render().catch(showLoadFailure);
})();
