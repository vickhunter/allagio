<?php

require_once __DIR__ . '/wp-filters.php';

$GLOBALS['filters'] = isset($GLOBALS['filters']) ? $GLOBALS['filters'] : array();
$GLOBALS['affilio_filter_hooks'] = isset($GLOBALS['affilio_filter_hooks']) ? $GLOBALS['affilio_filter_hooks'] : array();
$GLOBALS['options'] = isset($GLOBALS['options']) ? $GLOBALS['options'] : array();
$GLOBALS['posts'] = isset($GLOBALS['posts']) ? $GLOBALS['posts'] : array();
$GLOBALS['http'] = isset($GLOBALS['http']) ? $GLOBALS['http'] : array();
$GLOBALS['revision_counts'] = isset($GLOBALS['revision_counts']) ? $GLOBALS['revision_counts'] : array();
$GLOBALS['updates'] = isset($GLOBALS['updates']) ? $GLOBALS['updates'] : 0;
$GLOBALS['rewrites'] = isset($GLOBALS['rewrites']) ? $GLOBALS['rewrites'] : array();
$GLOBALS['flushes'] = isset($GLOBALS['flushes']) ? $GLOBALS['flushes'] : 0;
$GLOBALS['allowed'] = true;

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
        public function get_error_code() { return (string) $this->code; }
        public function get_error_message() { return $this->message; }
        public function get_error_data($code = '') { return $this->data; }
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

if (!class_exists('Affilio_Walk_Wpdb')) {
    class Affilio_Walk_Wpdb {
        public $posts = 'wp_posts';
        public function esc_like($text) {
            return addcslashes((string) $text, '_%\\');
        }
        public function prepare($query) {
            $args = array_slice(func_get_args(), 1);
            $index = 0;
            return preg_replace_callback('/%[sdf]/', function ($match) use (&$index, $args) {
                $value = $args[$index++];
                if ($match[0] === '%d') {
                    return (string) (int) $value;
                }
                return "'" . str_replace("'", "''", (string) $value) . "'";
            }, $query);
        }
        public function get_results($sql) {
            if (!isset($GLOBALS['affilio_box_scan_sql']) || !is_array($GLOBALS['affilio_box_scan_sql'])) {
                $GLOBALS['affilio_box_scan_sql'] = array();
            }
            $GLOBALS['affilio_box_scan_sql'][] = (string) $sql;
            if (!preg_match("/post_status = '([^']+)'/", (string) $sql, $status)) {
                return array();
            }
            if (!preg_match("/post_type IN \\('([^']+)', '([^']+)'\\)/", (string) $sql, $types)) {
                return array();
            }
            if (!preg_match("/LIKE '([^']*)'/", (string) $sql, $like)) {
                return array();
            }
            if (!preg_match('/LIMIT (\\d+) OFFSET (\\d+)/', (string) $sql, $page)) {
                return array();
            }
            $needle = str_replace(array('\\%', '\\_', '\\\\'), array('%', '_', '\\'), $like[1]);
            $needle = trim($needle, '%');
            $allowed = array($types[1], $types[2]);
            $matched = array();
            foreach ($GLOBALS['posts'] as $post) {
                if (!is_object($post) || !isset($post->post_status) || (string) $post->post_status !== $status[1]) {
                    continue;
                }
                $type = isset($post->post_type) ? (string) $post->post_type : '';
                if (!in_array($type, $allowed, true)) {
                    continue;
                }
                $content = isset($post->post_content) ? (string) $post->post_content : '';
                if ($needle !== '' && strpos($content, $needle) === false) {
                    continue;
                }
                $matched[] = $post;
            }
            return array_slice($matched, (int) $page[2], (int) $page[1]);
        }
    }
}

if (!function_exists('is_wp_error')) {
    function is_wp_error($value) { return $value instanceof WP_Error; }
    function rest_ensure_response($value) { return $value; }
    function current_user_can($capability, $id = null) { return !empty($GLOBALS['allowed']); }
    function absint($value) { return abs((int) $value); }
    function get_permalink($post) { return 'https://blog.example/post'; }
    function wp_strip_all_tags($value) { return strip_tags((string) $value); }
    function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
    function esc_attr($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
    function esc_url($value) { return (string) $value; }
    function wp_slash($value) { return addslashes((string) $value); }
    function wp_unslash($value) { return stripslashes((string) $value); }
    function sanitize_text_field($value) { return trim(strip_tags((string) $value)); }
    function wp_json_encode($value) { return json_encode($value); }
    function trailingslashit($value) { return rtrim((string) $value, '/\\') . '/'; }
    function home_url($path = '') { return 'https://blog.example' . $path; }
    function admin_url($path = '') { return 'https://blog.example/wp-admin/' . $path; }
    function get_option($key, $default = false) {
        return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
    }
    function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; return true; }
    function delete_option($key) { unset($GLOBALS['options'][$key]); return true; }
    function get_post($id) {
        foreach ($GLOBALS['posts'] as $post) {
            if ((int) $post->ID === (int) $id) {
                return $post;
            }
        }
        if (isset($GLOBALS['post']) && (int) $GLOBALS['post']->ID === (int) $id) {
            return $GLOBALS['post'];
        }
        return null;
    }
    function get_posts($args = array()) {
        $status = isset($args['post_status']) ? $args['post_status'] : 'publish';
        $wanted = is_array($status) ? $status : array($status);
        $matched = array();
        foreach ($GLOBALS['posts'] as $post) {
            if (in_array('any', $wanted, true) || in_array($post->post_status, $wanted, true)) {
                $matched[] = $post;
            }
        }
        return $matched;
    }
    if (!isset($GLOBALS['wpdb'])) {
        $GLOBALS['wpdb'] = new Affilio_Walk_Wpdb();
    }
    function apply_filters($name, $value) {
        $passed = affilio_test_run_filters($name, array_slice(func_get_args(), 1));
        return $passed[0];
    }
    function wp_update_post($values, $error = false) {
        $kept = apply_filters('wp_revisions_to_keep', 12, (object) array('ID' => $values['ID']));
        $GLOBALS['revision_counts'][] = $kept;
        $GLOBALS['updates']++;
        if (!empty($GLOBALS['noop_saves'])) {
            $GLOBALS['noop_saves']--;
            return $values['ID'];
        }
        $content = stripslashes($values['post_content']);
        foreach ($GLOBALS['posts'] as $post) {
            if ((int) $post->ID === (int) $values['ID']) {
                $post->post_content = $content;
            }
        }
        if (isset($GLOBALS['post']) && (int) $GLOBALS['post']->ID === (int) $values['ID']) {
            $GLOBALS['post']->post_content = $content;
        }
        return $values['ID'];
    }
    function wp_remote_request($url, $args = array()) {
        $GLOBALS['http'][] = array('url' => $url, 'args' => $args);
        if (isset($GLOBALS['http_handler']) && is_callable($GLOBALS['http_handler'])) {
            return call_user_func($GLOBALS['http_handler'], $url, $args);
        }
        if (array_key_exists('http_response', $GLOBALS)) {
            return $GLOBALS['http_response'];
        }
        return array('response' => array('code' => 200), 'headers' => array(), 'body' => '{}');
    }
    function wp_remote_retrieve_header($response, $name) {
        if (!is_array($response) || !isset($response['headers']) || !is_array($response['headers'])) {
            return '';
        }
        foreach ($response['headers'] as $header => $value) {
            if (strtolower((string) $header) === strtolower((string) $name)) {
                return $value;
            }
        }
        return '';
    }
    function wp_redirect($location, $status = 302) {
        throw new RuntimeException('redirect ' . (int) $status . ' ' . $location);
    }
    function status_header($code) {
        throw new RuntimeException('status ' . (int) $code);
    }
    function add_rewrite_rule($pattern, $query, $priority = 'bottom') {
        global $wp_rewrite;
        if (function_exists('did_action') && !is_object($wp_rewrite)) {
            throw new Error('add_rewrite_rule called before WP_Rewrite exists');
        }
        $GLOBALS['rewrites'][] = array($pattern, $query, $priority);
    }
    function add_rewrite_tag($tag, $regex) {
        $GLOBALS['rewrite_tags'][$tag] = $regex;
    }
    function flush_rewrite_rules($hard = true) {
        $GLOBALS['flushes']++;
        if (isset($GLOBALS['flush_rewrite_rules_hook']) && is_callable($GLOBALS['flush_rewrite_rules_hook'])) {
            call_user_func($GLOBALS['flush_rewrite_rules_hook']);
        }
    }
    function wp_verify_nonce($nonce, $action) { return $nonce === 'nonce-for-' . $action; }
    function wp_nonce_field($action, $name, $referer = true, $echo = true) {
        return '<input type="hidden" name="' . $name . '" value="nonce-for-' . $action . '">';
    }
    function add_query_arg($args, $url) { return $url . '?' . http_build_query($args); }
    function add_action($name, $callback, $priority = 10, $accepted_args = 1) {
        $GLOBALS['actions'][$name][] = $callback;
        $GLOBALS['action_meta'][$name][] = array('priority' => $priority, 'accepted_args' => $accepted_args);
    }
    function register_activation_hook($path, $callback) { $GLOBALS['actions']['activation'][] = $callback; }
    function register_deactivation_hook($path, $callback) { $GLOBALS['actions']['deactivation'][] = $callback; }
}

if (!class_exists('Affilio_API')) {
    class Affilio_API {
        public static function resolve_base_url() { return 'https://api.example.test/v1'; }
        public function request($method, $path, $body = null) {
            if (isset($GLOBALS['affilio_api_request']) && is_callable($GLOBALS['affilio_api_request'])) {
                return call_user_func($GLOBALS['affilio_api_request'], $method, $path, $body);
            }
            return array('code' => 200, 'data' => array());
        }
    }
}

function d033_ack_bodies() {
    $bodies = array();
    foreach ($GLOBALS['http'] as $call) {
        if (strpos($call['url'], '/plugin-ack') === false) {
            continue;
        }
        $raw = isset($call['args']['body']) ? $call['args']['body'] : '';
        $decoded = json_decode($raw, true);
        $bodies[] = array(
            'url' => $call['url'],
            'headers' => isset($call['args']['headers']) ? $call['args']['headers'] : array(),
            'body' => is_array($decoded) ? $decoded : array(),
            'raw' => $raw,
        );
    }
    return $bodies;
}

function d033_log_file() {
    if (empty($GLOBALS['d033_log'])) {
        $GLOBALS['d033_log'] = tempnam(sys_get_temp_dir(), 'affilio-d033');
        ini_set('log_errors', '1');
        ini_set('error_log', $GLOBALS['d033_log']);
    }
    return $GLOBALS['d033_log'];
}

function d033_log() {
    $file = d033_log_file();
    return is_file($file) ? (string) file_get_contents($file) : '';
}
