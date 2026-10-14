"""Before/after design screenshots: python3 tests/design_shots.py <before|after> [repo_dir]
Same interactions on any version: digest, setup, empty digest, item panel (Enter on the first item),
command palette (Ctrl+K), approval-mode modal (first "Change"). A version without a feature just shows the page."""
import os, socket, subprocess, sys, time
from playwright.sync_api import sync_playwright
tag = sys.argv[1]; root = os.path.abspath(sys.argv[2] if len(sys.argv) > 2 else os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
OUT = '/workspace/builds/offer-rails/proof/d032-ui/v7-design'
s = socket.socket(); s.bind(('127.0.0.1', 0)); port = s.getsockname()[1]; s.close()
BASE = 'http://127.0.0.1:%d/' % port
srv = subprocess.Popen([sys.executable, '-m', 'http.server', str(port), '--bind', '127.0.0.1'], cwd=root, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(0.8); go = []
try:
    with sync_playwright() as p:
        b = p.chromium.launch(executable_path='/usr/bin/google-chrome', args=['--headless=new'])
        for w in (1440, 390):
            ctx = b.new_context(viewport={'width': w, 'height': 900}, ignore_https_errors=True, is_mobile=(w == 390), has_touch=(w == 390))
            ctx.route('**/v1/go/**', lambda r: (go.append(r.request.url), r.abort()))
            ctx.route('**/v1/**', lambda r: r.fulfill(status=200, content_type='application/json', body='{}'))
            pg = ctx.new_page()
            def shot(name, full=True):
                try: pg.wait_for_function("document.getAnimations().every(a => a.playState !== 'running' || !a.effect || a.effect.getTiming().iterations === Infinity || a.effect.getTiming().duration > 1000)", timeout=3000)
                except Exception: pass
                pg.screenshot(path=os.path.join(OUT, '%s-%s-%d.png' % (name, tag, w)), full_page=full)
            pg.goto(BASE + 'digest.html?fixture=normal'); pg.wait_for_selector('digest-item'); pg.wait_for_timeout(900); shot('digest')
            pg.goto(BASE + 'setup.html?fixture=maker'); pg.wait_for_selector('#continue'); pg.wait_for_timeout(900); shot('setup')
            pg.goto(BASE + 'digest.html?fixture=empty'); pg.wait_for_selector('[data-test=totals]'); pg.wait_for_timeout(900); shot('empty')
            pg.goto(BASE + 'digest.html?fixture=normal'); pg.wait_for_selector('digest-item'); pg.wait_for_timeout(900)
            pg.keyboard.press('j'); pg.keyboard.press('Enter'); pg.wait_for_timeout(500); shot('panel', full=False)
            pg.keyboard.press('Escape'); pg.wait_for_timeout(400)
            pg.keyboard.press('Control+k'); pg.wait_for_timeout(500); shot('palette', full=False)
            pg.keyboard.press('Escape'); pg.wait_for_timeout(400)
            pg.locator('[data-section] .mode a').first.click(); pg.wait_for_timeout(500); shot('modal', full=False)
            ctx.close()
        b.close()
finally:
    srv.terminate()
print('shots:', tag, 'go-link requests:', len(go))
