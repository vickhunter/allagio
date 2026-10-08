<?php

if (!defined('ABSPATH')) {
    define('ABSPATH', __DIR__);
}

$GLOBALS['options'] = isset($GLOBALS['options']) ? $GLOBALS['options'] : array();
$GLOBALS['actions'] = isset($GLOBALS['actions']) ? $GLOBALS['actions'] : array();
$GLOBALS['routes'] = isset($GLOBALS['routes']) ? $GLOBALS['routes'] : array();
$GLOBALS['scripts'] = isset($GLOBALS['scripts']) ? $GLOBALS['scripts'] : array();
$GLOBALS['styles'] = isset($GLOBALS['styles']) ? $GLOBALS['styles'] : array();
$GLOBALS['localized'] = isset($GLOBALS['localized']) ? $GLOBALS['localized'] : array();
$GLOBALS['http'] = isset($GLOBALS['http']) ? $GLOBALS['http'] : array();
$GLOBALS['filters'] = isset($GLOBALS['filters']) ? $GLOBALS['filters'] : array();
$GLOBALS['posts'] = isset($GLOBALS['posts']) ? $GLOBALS['posts'] : array();
$GLOBALS['users'] = isset($GLOBALS['users']) ? $GLOBALS['users'] : array();
$GLOBALS['comments'] = isset($GLOBALS['comments']) ? $GLOBALS['comments'] : array();
$GLOBALS['administrator'] = isset($GLOBALS['administrator']) ? $GLOBALS['administrator'] : true;
$GLOBALS['current_user'] = isset($GLOBALS['current_user']) ? $GLOBALS['current_user'] : (object) array(
    'ID' => 1,
    'user_login' => 'siteadmin',
    'user_email' => 'admin@example.com',
    'display_name' => 'Ada Admin',
);

if (!function_exists('check')) {
    function check($condition, $message) {
        if (!$condition) {
            fwrite(STDERR, $message . "\n");
            exit(1);
        }
    }
}

if (!function_exists('add_action')) {
    function add_action($name, $callback) { $GLOBALS['actions'][$name][] = $callback; }
    function add_filter($name, $callback) { $GLOBALS['filters'][$name][] = $callback; }
    function apply_filters($name, $value) {
        $args = func_get_args();
        if ($name === 'pre_http_request') {
            $GLOBALS['http'][] = array(
                'url' => isset($args[3]) ? $args[3] : '',
                'args' => isset($args[2]) ? $args[2] : array(),
                'hook' => 'pre_http_request',
            );
        }
        if (isset($GLOBALS['filters'][$name])) {
            $passed = array_slice($args, 1);
            foreach ($GLOBALS['filters'][$name] as $callback) {
                $passed[0] = call_user_func_array($callback, $passed);
            }
            return $passed[0];
        }
        return $value;
    }
    function register_activation_hook($path, $callback) { $GLOBALS['actions']['activation'][] = $callback; }
    function register_rest_route($namespace, $path, $args) { $GLOBALS['routes'][] = array($namespace, $path, $args); }
    function plugin_dir_path($path) { return dirname($path) . '/'; }
    function plugin_dir_url($path) { return 'https://blog.example/wp-content/plugins/affilio-wp/'; }
    function get_option($key, $default = false) { return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default; }
    function add_option($key, $value, $deprecated = '', $autoload = null) {
        if (array_key_exists($key, $GLOBALS['options'])) { return false; }
        $GLOBALS['options'][$key] = $value;
        return true;
    }
    function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; return true; }
    function delete_option($key) { unset($GLOBALS['options'][$key]); }
    function current_user_can($capability, $id = null) { return !empty($GLOBALS['administrator']); }
    function home_url($path = '') { return 'https://blog.example' . $path; }
    function site_url($path = '') { return home_url($path); }
    function admin_url($path = '') { return 'https://blog.example/wp-admin/' . $path; }
    function rest_url($path = '') { return 'https://blog.example/wp-json/' . ltrim($path, '/'); }
    function wp_parse_url($value, $component = -1) { return parse_url($value, $component); }
    function get_bloginfo($key) { return $key === 'name' ? 'Ada Admin Blog' : 'Example blog'; }
    function rest_ensure_response($value) { return $value; }
    function is_wp_error($value) { return $value instanceof WP_Error; }
    function wp_doing_ajax() { return false; }
    function is_network_admin() { return false; }
    function wp_safe_redirect($url) { $GLOBALS['redirect'] = $url; }
    function wp_next_scheduled($hook) { return false; }
    function wp_schedule_single_event($time, $hook) { $GLOBALS['cron'][$hook] = $time; }
    function wp_enqueue_script($handle, $src = '', $deps = array(), $ver = false, $in_footer = false) { $GLOBALS['scripts'][$handle] = $src; }
    function wp_enqueue_style($handle, $src = '', $deps = array(), $ver = false) { $GLOBALS['styles'][$handle] = $src; }
    function wp_localize_script($handle, $object, $data) { $GLOBALS['localized'][$handle][$object] = $data; }
    function wp_create_nonce($action) { return 'test-nonce'; }
    function get_locale() { return 'en_US'; }
    function get_user_locale() { return 'en_US'; }
    function wp_get_current_user() { return $GLOBALS['current_user']; }
    function get_permalink($post) { return 'https://blog.example/' . (is_object($post) ? $post->post_name : $post); }
    function get_post($id) {
        foreach ($GLOBALS['posts'] as $post) {
            if ((int) $post->ID === (int) $id) { return $post; }
        }
        return null;
    }
    function get_posts($args) {
        $status = isset($args['post_status']) ? $args['post_status'] : 'publish';
        $wanted = is_array($status) ? $status : array($status);
        $matched = array();
        foreach ($GLOBALS['posts'] as $post) {
            if (in_array('any', $wanted, true) || in_array($post->post_status, $wanted, true)) {
                $matched[] = $post;
            }
        }
        $offset = isset($args['offset']) ? (int) $args['offset'] : 0;
        $limit = isset($args['posts_per_page']) ? (int) $args['posts_per_page'] : count($matched);
        if ($limit < 0) { $limit = count($matched); }
        return array_slice($matched, $offset, $limit);
    }
    function get_comments($args = array()) { return $GLOBALS['comments']; }
    function wp_json_encode($value) { return json_encode($value); }
    function trailingslashit($value) { return rtrim($value, '/\\') . '/'; }
    function wp_remote_retrieve_response_code($response) { return isset($response['response']['code']) ? $response['response']['code'] : 0; }
    function wp_remote_retrieve_body($response) { return isset($response['body']) ? $response['body'] : ''; }
    function wp_remote_request($url, $args = array()) {
        $pre = apply_filters('pre_http_request', false, $args, $url);
        if ($pre !== false && $pre !== null) { return $pre; }
        return array('response' => array('code' => 200), 'body' => '{}');
    }
    function wp_remote_get($url, $args = array()) { return wp_remote_request($url, $args); }
    function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
    function esc_attr($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
    function esc_url($value) { return (string) $value; }
    function esc_url_raw($value) { return (string) $value; }
    function sanitize_text_field($value) { return trim((string) $value); }
    function sanitize_key($value) { return strtolower((string) $value); }
    function wp_die($message) { throw new RuntimeException(strip_tags((string) $message)); }
    function wp_nonce_field($action, $name = '_wpnonce') {
        echo '<input type="hidden" name="' . esc_attr($name) . '" value="' . esc_attr(wp_create_nonce($action)) . '" />';
    }
    function wp_verify_nonce($nonce, $action) {
        return hash_equals((string) wp_create_nonce($action), (string) $nonce);
    }
    function submit_button($text) { echo '<button>' . esc_html($text) . '</button>'; }
    function check_admin_referer($action) {
        $nonce = isset($_REQUEST['_wpnonce']) ? (string) $_REQUEST['_wpnonce'] : '';
        if (!wp_verify_nonce($nonce, $action)) {
            throw new RuntimeException('The link you followed has expired.');
        }
        return true;
    }
    function get_transient($key) { return isset($GLOBALS['transients'][$key]) ? $GLOBALS['transients'][$key] : false; }
    function set_transient($key, $value, $expiration) { $GLOBALS['transients'][$key] = $value; return true; }
    function delete_transient($key) { unset($GLOBALS['transients'][$key]); }
    function absint($value) { return abs((int) $value); }
    function wp_strip_all_tags($value) { return strip_tags((string) $value); }
    function wp_slash($value) { return addslashes((string) $value); }
    function wp_unslash($value) { return stripslashes((string) $value); }
    function wp_kses_post($value) { return (string) $value; }
    function wp_update_post($values, $error = false) { return $values['ID']; }
    function get_current_screen() { return isset($GLOBALS['screen']) ? $GLOBALS['screen'] : null; }
    function add_menu_page() { return 'affilio'; }
    function add_submenu_page() { return 'affilio'; }
}

if (!class_exists('WP_Error')) {
    class WP_Error {
        public $code;
        public $message;
        public $data;
        public function __construct($code, $message, $data = array()) {
            $this->code = $code;
            $this->message = $message;
            $this->data = $data;
        }
        public function get_error_message() { return $this->message; }
    }
}

if (!class_exists('WP_REST_Response')) {
    class WP_REST_Response {
        public $data;
        public $status;
        public function __construct($data, $status = 200) {
            $this->data = $data;
            $this->status = $status;
        }
    }
}

if (!class_exists('Affilio_Test_Request')) {
    class Affilio_Test_Request {
        private $params;
        private $json;
        public function __construct($params = array(), $json = null) {
            $this->params = $params;
            $this->json = $json === null ? $params : $json;
        }
        public function get_param($name) { return isset($this->params[$name]) ? $this->params[$name] : null; }
        public function get_json_params() { return $this->json; }
    }
}
