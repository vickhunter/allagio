<?php

require_once __DIR__ . '/wp-filters.php';

if (!class_exists('WP_Error')) {
    class WP_Error {
        public $errors = array();
        public $error_data = array();

        public function __construct($code = '', $message = '', $data = '') {
            if ($code === '') {
                return;
            }
            $this->errors[$code][] = $message;
            if (!empty($data)) {
                $this->error_data[$code] = $data;
            }
        }

        public function get_error_code() {
            foreach ($this->errors as $code => $messages) {
                return (string) $code;
            }
            return '';
        }

        public function get_error_message($code = '') {
            if ($code === '') {
                $code = $this->get_error_code();
            }
            if (isset($this->errors[$code][0])) {
                return $this->errors[$code][0];
            }
            return '';
        }

        public function get_error_data($code = '') {
            if ($code === '') {
                $code = $this->get_error_code();
            }
            if (isset($this->error_data[$code])) {
                return $this->error_data[$code];
            }
            return null;
        }
    }
}

if (!function_exists('check')) {
    function check($condition, $message) {
        if (!$condition) {
            fwrite(STDERR, $message . "\n");
            exit(1);
        }
    }
}

if (!function_exists('is_wp_error')) {
    function is_wp_error($value) { return $value instanceof WP_Error; }
    function rest_ensure_response($value) { return $value; }
    function current_user_can($capability, $id = null) { return true; }
    function absint($value) { return abs((int) $value); }
    function get_permalink($post) { return 'https://blog.example/post'; }
    function wp_strip_all_tags($value) { return strip_tags((string) $value); }
    function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
    function esc_url($value) { return (string) $value; }
    function wp_slash($value) { return addslashes((string) $value); }
    function home_url() { return 'https://blog.example'; }
    function admin_url($path) { return 'https://blog.example/wp-admin/' . $path; }
    function add_action($name, $callback) { $GLOBALS['actions'][$name][] = $callback; }
    function wp_next_scheduled($hook) { return isset($GLOBALS['cron'][$hook]) ? $GLOBALS['cron'][$hook] : false; }
    function wp_schedule_single_event($when, $hook) { $GLOBALS['cron'][$hook] = $when; }
    function get_option($key, $default = false) {
        return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
    }
    function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; return true; }
    function add_option($key, $value, $deprecated = '', $autoload = null) {
        if (array_key_exists($key, $GLOBALS['options'])) {
            return false;
        }
        $GLOBALS['options'][$key] = $value;
        return true;
    }
    function delete_option($key) { unset($GLOBALS['options'][$key]); }
    function get_post($id) { return (int) $id === 7 ? $GLOBALS['post'] : null; }
    function wp_update_post($values, $error = false) {
        $GLOBALS['post']->post_content = stripslashes($values['post_content']);
        return $values['ID'];
    }
}

if (!class_exists('Affilio_API')) {
    class Affilio_API {
        public static function resolve_base_url() { return 'https://api.example.test/v1'; }
    }
}

if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.25');
}
if (!defined('AFFILIO_PLUGIN_ACK_OPTION')) {
    define('AFFILIO_PLUGIN_ACK_OPTION', 'affilio_plugin_key');
}

class N2ReceiptAPI {
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => 'Uno strumento utile.',
            'disclosure' => 'Affiliate disclosure.',
            'receipt' => array('tracked_link' => '/v1/go/' . $id, 'publication_status' => 'awaiting_publication'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET' || strpos($path, '/commit') !== false) {
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            return array(
                'code' => 409,
                'data' => array(
                    'reason' => 'receipt_mismatch',
                    'message' => 'The published text must include the approved copy, the disclosure, and the exact tracked link.',
                ),
            );
        }
        return array('code' => 404, 'data' => array());
    }
}

class N2RegisterAPI {
    public $plugin_key = 'issued-plugin-key';
    public function invalidate_status_cache() {}
    public function get_connection_status($force = false) {
        return array('connected' => get_option('affilio_api_key', '') !== '', 'message' => 'Connection status');
    }
    public function request($method, $path, $body = null) {
        if ($path === '/health') {
            return array('code' => 200, 'data' => array('site_installation_registration' => true));
        }
        if ($path === '/sites/register') {
            $data = array('site_id' => 'same-site', 'api_key' => 'server-issued-api-key');
            if ($this->plugin_key !== null) {
                $data['plugin_key'] = $this->plugin_key;
            }
            return array('code' => 200, 'data' => $data);
        }
        throw new RuntimeException('Unexpected setup API path ' . $path);
    }
}

function affilio_n2_receipt_error_keeps_server_status_and_reason() {
    if (!class_exists('Affilio_Distribution')) {
        require dirname(__DIR__) . '/includes/class-affilio-distribution.php';
    }
    $GLOBALS['options'] = array();
    $GLOBALS['post'] = (object) array(
        'ID' => 7,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Strumenti',
        'post_content' => '<p>Un articolo.</p>',
    );
    $distribution = new Affilio_Distribution(new N2ReceiptAPI());
    $request = new N2Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));
    $warnings = array();
    set_error_handler(function ($errno, $errstr) use (&$warnings) {
        $warnings[] = $errstr;
        return true;
    });
    $result = $distribution->publish($request);
    restore_error_handler();
    $warning_text = implode("\n", $warnings);
    check(strpos($warning_text, 'WP_Error::$data') === false, 'publish read WP_Error::$data: ' . $warning_text);
    check(is_wp_error($result), 'a 409 receipt was treated as published');
    $data = $result->get_error_data();
    check(is_array($data) && (int) $data['status'] === 409, 'receipt refusal status was ' . json_encode($data));
    check(isset($data['reason']) && $data['reason'] === 'receipt_mismatch', 'receipt refusal dropped the server reason');
    $message = $result->get_error_message();
    check(strpos($message, 'The published text must include the approved copy, the disclosure, and the exact tracked link.') !== false, 'receipt refusal hid the server message: ' . $message);
    check(strpos($message, 'Il servizio ha rifiutato la pubblicazione.') === false, 'receipt refusal replaced the server message with the generic refusal');
}

function affilio_register_response_stores_plugin_key() {
    if (!class_exists('Affilio_Onboarding')) {
        require dirname(__DIR__) . '/includes/class-affilio-onboarding.php';
    }
    $GLOBALS['options'] = array();
    $GLOBALS['cron'] = array();
    $api = new N2RegisterAPI();
    $setup = new Affilio_Onboarding($api);
    $state = $setup->connect();
    check(!is_wp_error($state), 'register connect failed');
    check(get_option('affilio_plugin_key') === 'issued-plugin-key', 'register did not store plugin_key');
    check(strpos(json_encode($state), 'issued-plugin-key') === false, 'connect returned the plugin key to the browser');
    delete_option('affilio_api_key');
    $api->plugin_key = null;
    $setup->connect();
    check(get_option('affilio_plugin_key') === 'issued-plugin-key', 'a register response without plugin_key cleared the saved key');
    delete_option('affilio_api_key');
    $api->plugin_key = '   ';
    $setup->connect();
    check(get_option('affilio_plugin_key') === 'issued-plugin-key', 'a blank plugin_key cleared the saved key');
}

if (!class_exists('N2Request')) {
    class N2Request {
        private $values;
        public function __construct($values) { $this->values = $values; }
        public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
    }
}

$n2_main = isset($_SERVER['SCRIPT_FILENAME']) ? realpath($_SERVER['SCRIPT_FILENAME']) : '';
if ($n2_main && $n2_main === realpath(__FILE__)) {
    affilio_n2_receipt_error_keeps_server_status_and_reason();
    affilio_register_response_stores_plugin_key();
    echo "N2 receipt error and plugin key registration passed.\n";
}
