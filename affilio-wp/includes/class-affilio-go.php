<?php

if (!defined('AFFILIO_GO_CIRCUIT_SECONDS')) {
    define('AFFILIO_GO_CIRCUIT_SECONDS', 60);
}
if (!defined('AFFILIO_GO_TIMEOUT')) {
    define('AFFILIO_GO_TIMEOUT', 1.5);
}
if (!defined('AFFILIO_GO_CIRCUIT_OPTION')) {
    define('AFFILIO_GO_CIRCUIT_OPTION', 'affilio_go_circuit_until');
}
if (!defined('AFFILIO_GO_FALLBACKS_OPTION')) {
    define('AFFILIO_GO_FALLBACKS_OPTION', 'affilio_go_fallbacks');
}

if (class_exists('Affilio_Go')) {
    return;
}

class Affilio_Go {
    public static function rewrite_api_ready() {
        global $wp_rewrite;
        if (is_object($wp_rewrite)) {
            return true;
        }
        // Real WordPress defines did_action before $wp_rewrite exists. Calling
        // add_rewrite_rule in that window fatals. Test stubs have no did_action.
        if (function_exists('did_action')) {
            return false;
        }
        if (!empty($GLOBALS['affilio_rewrite_requires_wp_rewrite'])) {
            return false;
        }
        return function_exists('add_rewrite_rule');
    }

    public static function register_rewrites() {
        if (!self::rewrite_api_ready()) {
            return;
        }
        if (function_exists('add_rewrite_rule')) {
            add_rewrite_rule('^go/([a-f0-9-]{36})/?$', 'index.php?affilio_go=$matches[1]', 'top');
        }
        if (function_exists('add_rewrite_tag')) {
            add_rewrite_tag('%affilio_go%', '([a-f0-9-]{36})');
        }
    }

    public static function query_vars($vars) {
        $vars[] = 'affilio_go';
        return $vars;
    }

    public static function privacy_headers() {
        return array(
            'Cache-Control' => 'no-store',
            'X-Robots-Tag' => 'noindex, nofollow',
        );
    }

    public static function send_headers() {
        if (!headers_sent()) {
            foreach (self::privacy_headers() as $name => $value) {
                header($name . ': ' . $value);
            }
        }
    }

    public static function mark_uncacheable() {
        if (!defined('DONOTCACHEPAGE')) {
            define('DONOTCACHEPAGE', true);
        }
    }

    public static function mark_go_request_uncacheable() {
        if (self::is_go_request()) {
            self::mark_uncacheable();
        }
    }

    public static function exclude_cache($exclude) {
        if (self::is_go_request()) {
            return true;
        }
        return $exclude;
    }

    public static function robots_txt($output, $public = true) {
        $line = 'Disallow: /go/';
        if (strpos((string) $output, $line) !== false) {
            return $output;
        }
        return rtrim((string) $output, "\n") . "\n" . $line . "\n";
    }

    public static function is_go_request() {
        return self::current_id() !== '';
    }

    public static function path_is_go() {
        return self::id_from_request() !== '';
    }

    public static function id_from_request() {
        if (isset($_GET['affilio_go'])) {
            $raw = (string) $_GET['affilio_go'];
            $candidate = function_exists('wp_unslash') ? (string) wp_unslash($raw) : $raw;
            if (preg_match('/^[a-f0-9-]{36}$/i', $candidate)) {
                return $candidate;
            }
        }
        $uri = isset($_SERVER['REQUEST_URI']) ? (string) $_SERVER['REQUEST_URI'] : '';
        if (preg_match('#/go/([a-f0-9-]{36})(?![a-f0-9-])#i', $uri, $match)) {
            return $match[1];
        }
        return '';
    }

    public static function current_id() {
        $from_request = self::id_from_request();
        if ($from_request !== '') {
            return $from_request;
        }
        global $wp_query;
        if (is_object($wp_query) && function_exists('get_query_var')) {
            $candidate = (string) get_query_var('affilio_go');
            if (preg_match('/^[a-f0-9-]{36}$/i', $candidate)) {
                return $candidate;
            }
        }
        return '';
    }

    public static function bypass_canonical($redirect_url, $requested_url = '') {
        if (self::path_is_go()) {
            return false;
        }
        return $redirect_url;
    }

    public static function rewrite_legacy_links($content) {
        if (!is_string($content) || strpos($content, '/v1/go/') === false) {
            return $content;
        }
        $rewritten = preg_replace_callback(
            '#(?:https?:)?(?://[^"\'\s>]*)?/v1/go/([A-Za-z0-9_-]+)(\?[^"\'\s>]*)?#',
            array('Affilio_Go', 'legacy_link_replacement'),
            $content
        );
        return is_string($rewritten) ? $rewritten : $content;
    }

    public static function legacy_link_replacement($match) {
        $code = $match[1];
        $query = isset($match[2]) ? $match[2] : '';
        $id = '';
        if (preg_match('/placement=([a-f0-9-]{36})/i', $query, $placement)) {
            $id = $placement[1];
        } elseif (preg_match('/^[a-f0-9-]{36}$/i', $code)) {
            $id = $code;
        }
        if ($id === '') {
            return $match[0];
        }
        return self::local_href($id);
    }

    public static function local_href($id) {
        $path = '/go/' . $id;
        if (function_exists('home_url')) {
            return home_url($path);
        }
        return $path;
    }

    public static function redirect_url_allowed($url) {
        $url = trim((string) $url);
        if ($url === '' || strpos($url, "\n") !== false || strpos($url, "\r") !== false) {
            return false;
        }
        if (self::same_redirect_target($url, self::home_fallback())) {
            return true;
        }
        $parts = parse_url($url);
        if (!is_array($parts) || empty($parts['scheme']) || empty($parts['host'])) {
            return false;
        }
        return strtolower((string) $parts['scheme']) === 'https';
    }

    private static function same_redirect_target($url, $home) {
        $normalize = function ($value) {
            return rtrim(strtolower(trim((string) $value)), '/');
        };
        $url = $normalize($url);
        $home = $normalize($home);
        return $url !== '' && $url === $home;
    }

    private static function safe_redirect_location($url) {
        if (self::redirect_url_allowed($url)) {
            return $url;
        }
        $home = self::home_fallback();
        if (self::redirect_url_allowed($home)) {
            return $home;
        }
        return '';
    }

    public static function remember_fallback($id, $url) {
        $url = trim((string) $url);
        if ($url === '' || !self::redirect_url_allowed($url) || !preg_match('/^[a-f0-9-]{36}$/i', (string) $id) || !function_exists('get_option') || !function_exists('update_option')) {
            return;
        }
        $all = get_option(AFFILIO_GO_FALLBACKS_OPTION, array());
        if (!is_array($all)) {
            $all = array();
        }
        $all[(string) $id] = $url;
        update_option(AFFILIO_GO_FALLBACKS_OPTION, $all, false);
    }

    public static function cached_fallback($id) {
        $all = function_exists('get_option') ? get_option(AFFILIO_GO_FALLBACKS_OPTION, array()) : array();
        if (is_array($all) && isset($all[$id]) && is_string($all[$id]) && $all[$id] !== '') {
            return $all[$id];
        }
        return '';
    }

    public static function home_fallback() {
        if (function_exists('home_url')) {
            $home = home_url('/');
            if (is_string($home) && $home !== '') {
                return $home;
            }
        }
        return '/';
    }

    public static function fallback_for($id) {
        $cached = self::cached_fallback($id);
        if ($cached !== '') {
            return $cached;
        }
        return self::home_fallback();
    }

    public static function truncate_ip($ip) {
        $ip = trim((string) $ip);
        if ($ip === '') {
            return '';
        }
        if ($ip[0] === '[') {
            $end = strpos($ip, ']');
            $ip = $end === false ? trim($ip, '[]') : substr($ip, 1, $end - 1);
        }
        $zone = strpos($ip, '%');
        if ($zone !== false) {
            $ip = substr($ip, 0, $zone);
        }
        if (substr_count($ip, ':') === 1 && strpos($ip, '.') !== false) {
            $ip = substr($ip, 0, strpos($ip, ':'));
        }
        if (stripos($ip, '::ffff:') === 0) {
            $mapped = substr($ip, 7);
            if (filter_var($mapped, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
                $ip = $mapped;
            }
        }
        if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
            $parts = explode('.', $ip);
            if (count($parts) !== 4) {
                return '';
            }
            return intval($parts[0]) . '.' . intval($parts[1]) . '.' . intval($parts[2]) . '.0';
        }
        if (!filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6)) {
            return '';
        }
        $bin = inet_pton($ip);
        if ($bin === false || strlen($bin) !== 16) {
            return '';
        }
        $v4prefix = str_repeat("\0", 10) . "\xff\xff";
        if (substr($bin, 0, 12) === $v4prefix) {
            $v4 = inet_ntop(substr($bin, 12, 4));
            return self::truncate_ip($v4);
        }
        $bin = substr($bin, 0, 6) . str_repeat("\0", 10);
        return self::ipv6_rfc5952($bin);
    }

    public static function ipv6_rfc5952($bin) {
        $groups = array();
        for ($i = 0; $i < 16; $i += 2) {
            $unpacked = unpack('n', substr($bin, $i, 2));
            $groups[] = $unpacked[1];
        }
        $best_start = -1;
        $best_len = 0;
        $index = 0;
        while ($index < 8) {
            if ($groups[$index] !== 0) {
                $index++;
                continue;
            }
            $end = $index;
            while ($end < 8 && $groups[$end] === 0) {
                $end++;
            }
            $len = $end - $index;
            if ($len > $best_len) {
                $best_len = $len;
                $best_start = $index;
            }
            $index = $end;
        }
        $parts = array();
        for ($index = 0; $index < 8;) {
            if ($best_len >= 2 && $index === $best_start) {
                $parts[] = '';
                $index += $best_len;
                if ($index >= 8) {
                    $parts[] = '';
                }
                continue;
            }
            $parts[] = dechex($groups[$index]);
            $index++;
        }
        $out = implode(':', $parts);
        return preg_replace('/:{3,}/', '::', $out);
    }

    public static function daily_salt($site_secret, $utc_date) {
        return hash('sha256', (string) $site_secret . '|go-link|' . (string) $utc_date);
    }

    public static function client_hash($truncated, $salt) {
        return hash_hmac('sha256', (string) $truncated, (string) $salt);
    }

    public static function utc_date() {
        $date = gmdate('Y-m-d');
        if (function_exists('apply_filters')) {
            $filtered = apply_filters('affilio_go_utc_date', $date);
            if (is_string($filtered) && preg_match('/^\d{4}-\d{2}-\d{2}$/', $filtered)) {
                return $filtered;
            }
        }
        return $date;
    }

    public static function visitor_hash() {
        $ip = isset($_SERVER['REMOTE_ADDR']) ? (string) $_SERVER['REMOTE_ADDR'] : '';
        $truncated = self::truncate_ip($ip);
        $secret = function_exists('get_option') ? trim((string) get_option('affilio_api_key', '')) : '';
        return self::client_hash($truncated, self::daily_salt($secret, self::utc_date()));
    }

    public function resolve($placement_id) {
        $cached = self::cached_fallback($placement_id);
        $fallback = $cached !== '' ? $cached : self::home_fallback();
        $until = function_exists('get_option') ? (int) get_option(AFFILIO_GO_CIRCUIT_OPTION, 0) : 0;
        if ($until > time()) {
            $this->log_fallback($placement_id, 'circuit_open');
            return self::safe_redirect_location($fallback);
        }
        $response = $this->call_upstream($placement_id);
        $location = trim(self::location_header($response));
        $code = self::response_code($response);
        if (!is_wp_error($response) && $code === 302 && self::redirect_url_allowed($location)) {
            return $location;
        }
        $transport_failed = is_wp_error($response);
        if (($transport_failed || $code >= 500) && function_exists('update_option')) {
            update_option(AFFILIO_GO_CIRCUIT_OPTION, time() + (int) AFFILIO_GO_CIRCUIT_SECONDS, false);
        }
        // An id the API does not know has no destination. Sending the reader
        // home would look like a successful click. A remembered fallback still wins.
        if (!$transport_failed && $code === 404 && $cached === '') {
            $this->log_fallback($placement_id, 'upstream_status_404');
            return '';
        }
        $reason = $transport_failed ? 'upstream_failed' : 'upstream_status_' . $code;
        $this->log_fallback($placement_id, $reason);
        return self::safe_redirect_location($fallback);
    }

    public function handle() {
        $id = self::current_id();
        if ($id === '') {
            return;
        }
        self::mark_uncacheable();
        self::send_headers();
        $location = $this->resolve($id);
        if ($location === '') {
            self::send_not_found();
            exit;
        }
        if (!self::redirect_url_allowed($location)) {
            $location = self::home_fallback();
        }
        if (!self::redirect_url_allowed($location)) {
            self::send_not_found();
            exit;
        }
        if (function_exists('wp_redirect')) {
            wp_redirect($location, 302);
            exit;
        }
        if (!headers_sent()) {
            header('Location: ' . $location, true, 302);
        }
        exit;
    }

    public static function send_not_found() {
        if (function_exists('status_header')) {
            status_header(404);
            return;
        }
        if (!headers_sent()) {
            header('HTTP/1.1 404 Not Found', true, 404);
        }
        if (function_exists('http_response_code')) {
            http_response_code(404);
        }
    }

    private function call_upstream($placement_id) {
        $base = class_exists('Affilio_API') ? Affilio_API::resolve_base_url() : '';
        $path = 'go/' . rawurlencode((string) $placement_id) . '?via=plugin';
        $url = $base === '' ? $path : trailingslashit($base) . $path;
        $site_key = function_exists('get_option') ? trim((string) get_option('affilio_api_key', '')) : '';
        $headers = array(
            'X-API-Key' => $site_key,
            'X-OR-Client-Hash' => self::visitor_hash(),
            'X-OR-User-Agent' => isset($_SERVER['HTTP_USER_AGENT']) ? (string) $_SERVER['HTTP_USER_AGENT'] : '',
            'X-OR-Referer' => isset($_SERVER['HTTP_REFERER']) ? (string) $_SERVER['HTTP_REFERER'] : '',
        );
        if (!function_exists('wp_remote_request')) {
            return new WP_Error('affilio_go', 'go transport unavailable');
        }
        return wp_remote_request($url, array(
            'method' => 'GET',
            'timeout' => (float) AFFILIO_GO_TIMEOUT,
            'redirection' => 0,
            'headers' => $headers,
        ));
    }

    private function log_fallback($placement_id, $reason) {
        error_log('affilio go fallback placement_id=' . $placement_id . ' reason=' . $reason);
    }

    public static function location_header($response) {
        if (is_wp_error($response) || !is_array($response)) {
            return '';
        }
        if (isset($response['headers']) && is_array($response['headers'])) {
            foreach ($response['headers'] as $name => $value) {
                if (strtolower((string) $name) === 'location') {
                    return (string) $value;
                }
            }
        }
        if (function_exists('wp_remote_retrieve_header')) {
            return (string) wp_remote_retrieve_header($response, 'location');
        }
        return '';
    }

    public static function response_code($response) {
        if (is_wp_error($response) || !is_array($response)) {
            return 0;
        }
        if (isset($response['response']['code'])) {
            return (int) $response['response']['code'];
        }
        if (function_exists('wp_remote_retrieve_response_code')) {
            return (int) wp_remote_retrieve_response_code($response);
        }
        return 0;
    }
}
