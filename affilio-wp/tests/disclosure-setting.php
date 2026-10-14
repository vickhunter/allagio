<?php

require __DIR__ . '/bootstrap.php';

class Affilio_QuietAPI {
    public function get_connection_status($force = false) {
        return array('connected' => true, 'message' => 'Connected', 'state' => 'connected', 'label' => 'Connected');
    }
    public function invalidate_status_cache() {}
}

$GLOBALS['administrator'] = true;
$GLOBALS['user_locale'] = 'it_IT';
$_SERVER['REQUEST_METHOD'] = 'GET';
require dirname(__DIR__) . '/affilio.php';

$page = new Affilio_Admin(new Affilio_QuietAPI(), null, null);
ob_start();
$page->render_settings();
$settings = ob_get_clean();
check(strpos($settings, 'I miei articoli contengono già una disclosure') !== false, 'settings did not offer the existing-disclosure choice');
check(get_option('affilio_existing_disclosure', '') === '', 'existing disclosure setting defaulted on');

$_SERVER['REQUEST_METHOD'] = 'POST';
$_POST = array(
    'affilio_existing_disclosure' => '<script>alert(1)</script>',
    '_wpnonce' => wp_create_nonce('affilio_settings'),
);
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
ob_end_clean();
check(get_option('affilio_existing_disclosure', '') === '', 'unsanitized disclosure setting was stored');

$_POST['affilio_existing_disclosure'] = '1';
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
ob_end_clean();
check(get_option('affilio_existing_disclosure', '') === '1', 'checking the setting did not store 1');

unset($_POST['affilio_existing_disclosure']);
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
ob_end_clean();
check(get_option('affilio_existing_disclosure', '') === '', 'an unchecked setting stayed on');

echo "Existing-disclosure setting is optional, sanitized, and off by default.\n";
