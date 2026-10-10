const stage = document.querySelector('.world-stage');
const toggle = document.querySelector('[data-motion]');
const reduced = matchMedia('(prefers-reduced-motion: reduce)');
const desktop = matchMedia('(min-width: 851px)');
let visible = false;
let loading = false;
let cleanup;
let paused = false;

const enabled = () => visible && desktop.matches && !reduced.matches && !navigator.connection?.saveData;
async function updateScene() {
    if (!enabled()) {
        cleanup?.();
        cleanup = undefined;
        return;
    }
    if (cleanup || loading) return;
    loading = true;
    try {
        const { mountScene } = await import('./landing-scene.bundle.js');
        if (enabled()) cleanup = mountScene(stage, toggle, { paused, onPause: value => { paused = value; } });
    } catch {
        stage.classList.remove('scene-ready');
    } finally {
        loading = false;
    }
}
if (stage && toggle) {
    const observer = new IntersectionObserver(entries => {
        visible = entries[0].isIntersecting;
        updateScene();
    }, { threshold: .05 });
    observer.observe(stage);
    reduced.addEventListener('change', updateScene);
    desktop.addEventListener('change', updateScene);
    window.addEventListener('pagehide', () => { cleanup?.(); cleanup = undefined; });
    window.addEventListener('pageshow', updateScene);
}

function offerRailsWithAttributionSource(href, source) {
    const allowed = ['chatgpt', 'claude', 'cursor', 'grok', 'x', 'community'];
    const value = String(source || '').trim().toLowerCase();
    if (!allowed.includes(value)) return href;
    try {
        const url = new URL(href, 'https://offer-rails.example');
        if (url.origin !== 'https://offer-rails.example') return href;
        url.searchParams.set('source', value);
        return url.pathname + url.search + url.hash;
    } catch {
        return href;
    }
}

function offerRailsCarryAttributionSource(doc, loc) {
    const params = new URLSearchParams(String((loc && loc.search) || ''));
    const source = params.get('source');
    doc.querySelectorAll('a[href]').forEach(link => {
        const href = link.getAttribute('href') || '';
        if (href.startsWith('/owner') || href.startsWith('/auth/login')) {
            link.setAttribute('href', offerRailsWithAttributionSource(href, source));
        }
    });
}

if (typeof document !== 'undefined' && typeof location !== 'undefined') {
    offerRailsCarryAttributionSource(document, location);
}
