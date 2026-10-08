<?php

define('ABSPATH', __DIR__);
$options = array();
$actions = array();
$cron = array();
$administrator = true;
$posts = array();

function add_action($name, $callback) { $GLOBALS['actions'][$name][] = $callback; }
function apply_filters($name, $value) { return $value; }
function register_activation_hook($path, $callback) { $GLOBALS['actions']['activation'][] = $callback; }
function plugin_dir_path($path) { return dirname($path) . '/'; }
function plugin_dir_url($path) { return 'https://blog.example/wp-content/plugins/affilio/'; }
function get_option($key, $default = false) { return $GLOBALS['options'][$key] ?? $default; }
function add_option($key, $value, $deprecated = '', $autoload = null) {
    if (array_key_exists($key, $GLOBALS['options'])) { return false; }
    $GLOBALS['options'][$key] = $value;
    return true;
}
function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; }
function delete_option($key) { unset($GLOBALS['options'][$key]); }
function current_user_can($capability, $id = null) { return $GLOBALS['administrator']; }
function home_url() { return 'https://blog.example'; }
function wp_parse_url($value, $component) { return parse_url($value, $component); }
function get_bloginfo($key) { return 'Example blog'; }
function admin_url($path) { return 'https://blog.example/wp-admin/' . $path; }
function rest_ensure_response($value) { return $value; }
function is_wp_error($value) { return $value instanceof WP_Error; }
function wp_next_scheduled($hook) { return $GLOBALS['cron'][$hook] ?? false; }
function wp_schedule_single_event($time, $hook) { $GLOBALS['cron'][$hook] = $time; }
function get_posts($args) {
    check($args['post_status'] === 'publish', 'Setup included unpublished content');
    check($args['posts_per_page'] === 10, 'Setup did not bound scan size');
    return array_slice($GLOBALS['posts'], $args['offset'], $args['posts_per_page']);
}
function get_permalink($post) { return 'https://blog.example/posts/' . $post->ID; }
class WP_Error {
    public $code;
    public $message;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; }
}
function check($condition, $message) { if (!$condition) { throw new RuntimeException($message); } }

require dirname(__DIR__) . '/affilio.php';
check(class_exists('Affilio_Editor_Proxy') && class_exists('Affilio_Onboarding'), 'Plugin required a missing class');
affilio_init();
$actions['activation'][0]();
check(get_option('affilio_setup_redirect') === true, 'Plugin activation did not lead to setup');

class SetupTestAPI {
    public $registrations = array();
    public $scans = array();
    public $lose_response = true;
    public $fail_scan = false;
    public $supports_setup = true;
    public function invalidate_status_cache() {}
    public function get_connection_status($force = false) {
        return array('connected' => get_option('affilio_api_key', '') !== '', 'message' => 'Connection status');
    }
    public function request($method, $path, $body = null) {
        if ($path === '/health') {
            return array('code' => 200, 'data' => array('site_installation_registration' => $this->supports_setup));
        }
        if ($path === '/sites/register') {
            $this->registrations[] = $body;
            if ($this->lose_response) {
                $this->lose_response = false;
                return new WP_Error('network', 'Response lost after registration');
            }
            return array('code' => 200, 'data' => array('site_id' => 'same-site', 'api_key' => 'server-issued-api-key'));
        }
        if ($path === '/scan/full') {
            $this->scans[] = $body;
            return array('code' => $this->fail_scan ? 503 : 202, 'data' => array('status' => 'queued'));
        }
        throw new RuntimeException('Unexpected setup API path');
    }
}

$api = new SetupTestAPI();
$setup = new Affilio_Onboarding($api);
check($setup->status()['phase'] === 'not_started' && count($api->registrations) === 0, 'Setup phoned home before Connect');
$administrator = false;
check(!$setup->allowed() && is_wp_error($setup->connect()), 'Non-admin could connect');
$administrator = true;
$api->supports_setup = false;
check(is_wp_error($setup->connect()) && count($api->registrations) === 0, 'Unsupported backend created an orphan registration');
$api->supports_setup = true;
check(is_wp_error($setup->connect()) && count($api->registrations) === 1, 'Lost response was marked connected');
check(get_option('affilio_api_key', '') === '', 'Lost response stored a key');
check($api->registrations[0] === array('site_url' => 'https://blog.example', 'plugin_version' => AFFILIO_VERSION), 'Registration sent more than the site URL and plugin version');
$state = $setup->connect();
check($state['phase'] === 'scanning' && $state['connection']['connected'], 'Retry did not connect');
check($api->registrations[0] === $api->registrations[1], 'Retry changed the registration payload');
check(get_option('affilio_api_key') === 'server-issued-api-key', 'Server API key was not stored');
check(strpos(json_encode($state), 'server-issued-api-key') === false, 'Setup leaked a credential to the browser');
$setup->connect();
check(count($api->registrations) === 2, 'Connected setup registered again');

for ($id = 1; $id <= 14; $id++) {
    $posts[] = (object) array('ID' => $id, 'post_title' => 'Post ' . $id, 'post_content' => 'Published content');
}
unset($cron['affilio_setup_scan']);
$setup->scan_batch();
check(get_option('affilio_setup_state')['scanned'] === 10 && count($api->scans[0]['articles']) === 10, 'First scan batch or progress incorrect');
$api->fail_scan = true;
unset($cron['affilio_setup_scan']);
$setup->scan_batch();
check(get_option('affilio_setup_state')['phase'] === 'scan_error' && get_option('affilio_setup_state')['offset'] === 10, 'Failed scan falsely advanced progress');
$api->fail_scan = false;
$setup->connect();
check(get_option('affilio_setup_state')['offset'] === 10, 'Retry lost scan progress');
$setup->scan_batch();
$state = $setup->status();
check($state['phase'] === 'complete' && $state['scanned'] === 14 && count($api->registrations) === 2, 'Resumed scan did not finish or registered another account');
check(get_option('affilio_setup_lock', false) === false, 'Setup did not release its lock');
echo "WordPress setup: activation, permission, credential recovery, batching and scan retries passed.\n";
