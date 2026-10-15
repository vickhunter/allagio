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
