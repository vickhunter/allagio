<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class DashboardSettingsA11yTest extends TestCase {
    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_disconnected_dashboard_card_is_server_fallback_only() {
        $admin = $this->admin(false);
        ob_start();
        $admin->render_dashboard();
        $html = ob_get_clean();

        $this->assertSame(1, substr_count($html, 'affilio-panel-warning'));
        $this->assertSame(1, substr_count($html, '<noscript>'));
        $this->assertMatchesRegularExpression('/<noscript>.*affilio-panel-warning.*<\/noscript>/s', $html);
        $outside = preg_replace('/<noscript>.*?<\/noscript>/s', '', $html);
        $this->assertStringNotContainsString('affilio-panel-warning', $outside);
        $this->assertStringContainsString('id="affilio-dashboard-root"', $html);

        $js = file_get_contents(dirname(__DIR__, 2) . '/admin/build/dashboard.js');
        $this->assertStringContainsString("getElementById('affilio-dashboard-root')", $js);
        $this->assertStringContainsString('affilio-panel-warning', $js);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_textdomain_loads_on_init() {
        $this->boot();
        affilio_init();
        $hooks = isset($GLOBALS['actions']['init']) ? $GLOBALS['actions']['init'] : array();
        $this->assertContains('affilio_load_textdomain', $hooks);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_settings_page_enqueues_admin_stylesheet() {
        $admin = $this->admin(false);
        $GLOBALS['styles'] = array();
        $admin->enqueue_dashboard_assets('affilio_page_affilio-settings');
        $this->assertArrayHasKey('affilio-admin', $GLOBALS['styles']);
        $this->assertStringContainsString('admin/css/affilio-admin.css', $GLOBALS['styles']['affilio-admin']);

        $GLOBALS['styles'] = array();
        $admin->enqueue_dashboard_assets('affilio_page_affilio-distribution');
        $this->assertArrayNotHasKey('affilio-admin', $GLOBALS['styles']);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_key_saved_stays_english_for_the_default_locale() {
        $GLOBALS['user_locale'] = 'en_US';
        $this->boot();
        $html = $this->settings_and_notice(false);
        $this->assertSame(3, substr_count($html, 'Key saved (abcd…)'));
        $this->assertStringContainsString('Key saved (admn…)', $html);
        $this->assertStringContainsString('Key saved (plug…)', $html);
        $this->assertStringNotContainsString('Chiave salvata', $html);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_key_saved_uses_the_italian_catalog() {
        $GLOBALS['user_locale'] = 'it_IT';
        $this->boot();
        $html = $this->settings_and_notice(false);
        $this->assertSame(3, substr_count($html, 'Chiave salvata (abcd…)'));
        $this->assertStringContainsString('Chiave salvata (admn…)', $html);
        $this->assertStringContainsString('Chiave salvata (plug…)', $html);
        $this->assertStringNotContainsString('Key saved', $html);
        $mo = file_get_contents(dirname(__DIR__, 2) . '/languages/affilio-it_IT.mo');
        $this->assertNotFalse($mo);
        $this->assertStringContainsString('Chiave salvata (%s)', $mo);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_connected_settings_replace_the_connect_prompt() {
        $GLOBALS['user_locale'] = 'en_US';
        $_SERVER['REQUEST_METHOD'] = 'GET';
        $admin = $this->admin(true);
        ob_start();
        $admin->render_settings();
        $html = ob_get_clean();

        $this->assertMatchesRegularExpression('/<div id="affilio-onboarding-root">.*<form method="post" action="https:\/\/blog\.example\/wp-admin\/admin-post\.php">.*name="_wpnonce" value="test-nonce".*name="action" value="affilio_scan_all".*<button type="submit" class="button button-primary button-large" data-connect>Start analysis<\/button>.*<\/div>/s', $html);
        $this->assertStringContainsString('Your site is connected.', $html);
        $this->assertStringContainsString('>Start analysis<', $html);
        $this->assertStringNotContainsString('Connected. Start your first analysis.', $html);
        $this->assertStringNotContainsString('Connect and find opportunities', $html);

        $admin->enqueue_dashboard_assets('affilio_page_affilio-settings');
        $copy = $GLOBALS['localized']['affilio-onboarding']['AffilioSetup']['copy'];
        $this->assertSame('Start analysis|Analysis in progress|Analysis complete', implode('|', array($copy['startAnalysis'], $copy['analysisInProgress'], $copy['analysisComplete'])));
        $this->assertSame('Your site is connected.', $copy['connectedStatus']);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_disconnected_settings_keep_the_connect_prompt() {
        $GLOBALS['user_locale'] = 'en_US';
        $_SERVER['REQUEST_METHOD'] = 'GET';
        $admin = $this->admin(false);
        ob_start();
        $admin->render_settings();
        $html = ob_get_clean();

        $this->assertMatchesRegularExpression('/<div id="affilio-onboarding-root">\s*<\/div>/', $html);
        $this->assertStringNotContainsString('Start analysis', $html);
        $this->assertStringNotContainsString('Your site is connected.', $html);

        $js = file_get_contents(dirname(__DIR__, 2) . '/admin/build/onboarding.js');
        $this->assertStringContainsString("t('Connect and find opportunities', 'Collega e trova opportunità')", $js);
        $this->assertStringContainsString("t('Ready to connect. No API key to copy.', 'Pronto per il collegamento. Nessuna chiave API da copiare.')", $js);
        $this->assertStringNotContainsString('Connected. Start your first analysis.', $js);
        $this->assertStringContainsString("catalog('startAnalysis', 'Start analysis')", $js);
        $this->assertStringContainsString("catalog('connectedStatus', 'Your site is connected.')", $js);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_italian_connected_settings_use_the_catalog() {
        $GLOBALS['user_locale'] = 'it_IT';
        $_SERVER['REQUEST_METHOD'] = 'GET';
        $admin = $this->admin(true);
        ob_start();
        $admin->render_settings();
        $html = ob_get_clean();

        $this->assertStringContainsString('Il sito è collegato.', $html);
        $this->assertStringContainsString('>Avvia analisi<', $html);
        $this->assertStringNotContainsString('Start analysis', $html);
        $this->assertStringNotContainsString('Your site is connected.', $html);
        $this->assertStringNotContainsString('Collegato. Avvia la prima analisi.', $html);

        $root = dirname(__DIR__, 2);
        $po = file_get_contents($root . '/languages/affilio-it_IT.po');
        $mo = file_get_contents($root . '/languages/affilio-it_IT.mo');
        $this->assertStringContainsString("msgid \"Start analysis\"\nmsgstr \"Avvia analisi\"\n\nmsgid \"Analysis in progress\"\nmsgstr \"Analisi in corso\"\n\nmsgid \"Analysis complete\"\nmsgstr \"Analisi completata\"", $po);
        $this->assertStringContainsString("msgid \"Your site is connected.\"\nmsgstr \"Il sito è collegato.\"", $po);
        $this->assertStringContainsString('Avvia analisi', $mo);
        $this->assertStringContainsString('Il sito è collegato.', $mo);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_connected_dashboard_reuses_the_connected_badge() {
        $GLOBALS['user_locale'] = 'en_US';
        $admin = $this->admin(true);
        ob_start();
        $admin->render_dashboard();
        $html = ob_get_clean();

        $this->assertStringContainsString('id="affilio-dashboard-connection"', $html);
        $this->assertStringContainsString('affilio-status-pill affilio-status-connected', $html);
        $this->assertStringContainsString('>Connected<', $html);
        $this->assertStringNotContainsString('affilio-next-step', $html);
        $this->assertStringNotContainsString('Connect your site in Settings.', $html);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_disconnected_dashboard_links_to_the_connect_step() {
        $GLOBALS['user_locale'] = 'en_US';
        $admin = $this->admin(false);
        ob_start();
        $admin->render_dashboard();
        $html = ob_get_clean();

        $this->assertStringContainsString('class="affilio-status-message"', $html);
        $this->assertStringContainsString('Not connected.', $html);
        $this->assertStringContainsString('class="affilio-next-step"', $html);
        $this->assertStringContainsString('Connect your site in Settings.', $html);
        $this->assertStringContainsString('href="https://blog.example/wp-admin/admin.php?page=affilio-settings"', $html);
        $this->assertStringNotContainsString('affilio-status-connected', $html);
        $this->assertSame(1, substr_count($html, 'affilio-panel-warning'));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_italian_dashboard_status_uses_the_catalog() {
        $GLOBALS['user_locale'] = 'it_IT';
        $disconnected = $this->admin(false);
        ob_start();
        $disconnected->render_dashboard();
        $html = ob_get_clean();
        $this->assertStringContainsString('Non connesso.', $html);
        $this->assertStringContainsString('Collega il sito in Impostazioni.', $html);
        $this->assertStringNotContainsString('Connect your site in Settings.', $html);

        $connected = $this->admin(true);
        ob_start();
        $connected->render_dashboard();
        $connected_html = ob_get_clean();
        $this->assertStringContainsString('>Connesso<', $connected_html);
        $this->assertStringContainsString('affilio-status-pill affilio-status-connected', $connected_html);

        $disconnected->enqueue_dashboard_assets('toplevel_page_affilio');
        $i18n = $GLOBALS['localized']['affilio-dashboard']['AffilioData']['i18n'];
        $this->assertSame('Connesso', $i18n['connected']);
        $this->assertSame('Non connesso.', $i18n['notConnected']);
        $this->assertSame('Collega il sito in Impostazioni.', $i18n['connectSite']);

        $mo = file_get_contents(dirname(__DIR__, 2) . '/languages/affilio-it_IT.mo');
        $this->assertStringContainsString('Connesso', $mo);
        $this->assertStringContainsString('Collega il sito in Impostazioni.', $mo);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_dashboard_status_escapes_a_broken_state() {
        $this->boot();
        $api = new class {
            public function get_connection_status($force = false) {
                return array(
                    'connected' => true,
                    'message' => '<img src=x onerror=alert(1)>',
                    'state' => 'connected"><img src=x>',
                    'label' => '<Connected>',
                );
            }
            public function invalidate_status_cache() {}
        };
        $_SERVER['REQUEST_METHOD'] = 'GET';
        $admin = new Affilio_Admin($api, null, null);
        ob_start();
        $admin->render_dashboard();
        $html = ob_get_clean();

        $this->assertStringContainsString('affilio-status-unknown_error', $html);
        $this->assertStringContainsString('>Connected<', $html);
        $this->assertStringNotContainsString('<img', $html);
        $this->assertStringNotContainsString('<Connected>', $html);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_dashboard_script_escapes_article_and_program_fields() {
        $js = file_get_contents(dirname(__DIR__, 2) . '/admin/build/dashboard.js');
        $this->assertStringContainsString("const title = esc(a.title || '');", $js);
        $this->assertTrue(
            str_contains($js, 'const articleId = esc(a.id);')
            && str_contains($js, "candidate.getAttribute('data-article-id') === articleId")
            && !str_contains($js, 'tr[data-article-id="${')
        );
        $this->assertStringContainsString('const postId = esc(a.wp_post_id);', $js);
        $this->assertStringContainsString('const name = esc(p.name);', $js);
        $this->assertStringContainsString('const domain = p.domain ? esc(p.domain)', $js);
        $this->assertStringNotContainsString('${a.title', $js);
        $this->assertStringNotContainsString('${p.name}', $js);
        $this->assertStringNotContainsString('${p.domain', $js);
    }

    private function settings_and_notice($connected) {
        if (!defined('WP_DEBUG')) {
            define('WP_DEBUG', true);
        }
        $GLOBALS['options']['affilio_api_key'] = 'abcdSECRETKEY1234wxyz';
        $GLOBALS['options']['affilio_admin_api_key'] = 'admnSECRETADMIN9999qrst';
        $GLOBALS['options']['affilio_plugin_key'] = 'plugPLUGINKEY1234mnop';
        $_SERVER['REQUEST_METHOD'] = 'GET';
        $admin = new Affilio_Admin($this->api($connected), null, null);
        ob_start();
        $admin->render_settings();
        $settings = ob_get_clean();
        $GLOBALS['screen'] = (object) array('id' => 'post', 'base' => 'post');
        ob_start();
        $admin->maybe_render_connection_notice();
        $notice = ob_get_clean();
        return $settings . $notice;
    }

    private function admin($connected) {
        $this->boot();
        return new Affilio_Admin($this->api($connected), null, null);
    }

    private function api($connected) {
        return new class($connected) {
            private $connected;
            public function __construct($connected) {
                $this->connected = $connected;
            }
            public function get_connection_status($force = false) {
                if ($this->connected) {
                    return array('connected' => true, 'message' => 'Affilio is connected and ready.', 'state' => 'connected', 'label' => 'Connected');
                }
                return array('connected' => false, 'message' => 'Configure your API key.', 'state' => 'missing', 'label' => 'Not connected');
            }
            public function invalidate_status_cache() {}
        };
    }

    private function boot() {
        require dirname(__DIR__) . '/bootstrap.php';
        require dirname(__DIR__, 2) . '/affilio.php';
    }
}
