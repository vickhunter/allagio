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
