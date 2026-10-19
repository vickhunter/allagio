<?php

$GLOBALS['options'] = array();
$GLOBALS['http'] = array();

function check($condition, $message) {
    if (!$condition) {
        fwrite(STDERR, $message . "\n");
        exit(1);
    }
}
function get_option($key, $default = false) {
    return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
}
function update_option($key, $value, $autoload = null) {
    $GLOBALS['options'][$key] = $value;
    return true;
}
function delete_option($key) {
    unset($GLOBALS['options'][$key]);
    return true;
}
function apply_filters($name, $value) { return $value; }
function trailingslashit($value) { return rtrim((string) $value, '/\\') . '/'; }
function wp_json_encode($value) { return json_encode($value); }
function is_wp_error($value) { return $value instanceof WP_Error; }
class WP_Error {
    public $message;
    public function __construct($code, $message, $data = array()) { $this->message = $message; }
}
function wp_remote_retrieve_response_code($response) {
    return isset($response['response']['code']) ? $response['response']['code'] : 0;
}
function wp_remote_retrieve_body($response) {
    return isset($response['body']) ? $response['body'] : '';
}
function wp_remote_request($url, $args = array()) {
    $GLOBALS['http'][] = array('url' => $url, 'args' => $args);
    if (isset($GLOBALS['http_handler'])) {
        return call_user_func($GLOBALS['http_handler'], $url, $args);
    }
    return array('response' => array('code' => 200), 'body' => '{}');
}

define('AFFILIO_API_URL', 'https://api.example.test');
require __DIR__ . '/../includes/class-affilio-go.php';
require __DIR__ . '/../includes/class-affilio-api.php';

$_SERVER['REQUEST_URI'] = '/.well-known/offer-rails.txt';
check(Affilio_Go::is_verification_request(), 'well-known path was not recognized');
check(Affilio_Go::verification_body() === '', 'a site with no key served a token');

$GLOBALS['options']['affilio_api_key'] = 'site-key';
check(Affilio_Go::verification_body() === '', 'a saved key with no token served a file');
$GLOBALS['options']['affilio_verification_token'] = 'server-token';
check(Affilio_Go::verification_body() === 'server-token', 'saved key did not serve the token');

$_SERVER['REQUEST_URI'] = '/.well-known/offer-rails.txt?cache=1';
check(Affilio_Go::verification_body() === 'server-token', 'a query string hid the well-known file');
$_SERVER['REQUEST_URI'] = '/blog/.well-known/offer-rails.txt/';
check(Affilio_Go::verification_body() === 'server-token', 'a subdirectory request hid the well-known file');
$_SERVER['REQUEST_URI'] = '/.well-known/offer-rails.txt.bak';
check(Affilio_Go::verification_body() === '', 'a lookalike path served the token');
$_SERVER['REQUEST_URI'] = '/go/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
check(Affilio_Go::verification_body() === '', 'a go link served the verification file');

$_SERVER['REQUEST_URI'] = '/.well-known/offer-rails.txt';
$GLOBALS['options']['affilio_verification_token'] = '';
ob_start();
Affilio_Go::maybe_serve_verification();
$empty = ob_get_clean();
check($empty === '', 'an empty token still wrote a body');

$api = new Affilio_API();
$kept = 'kept-token';
$GLOBALS['options']['affilio_verification_token'] = $kept;
$GLOBALS['http_handler'] = function ($url, $args) {
    return new WP_Error('http_request_failed', 'down');
};
$api->refresh_verification_token();
check(get_option('affilio_verification_token', '') === $kept, 'a failed refresh deleted the stored token');

$GLOBALS['http_handler'] = function ($url, $args) {
    check(isset($args['headers']['X-API-Key']) && $args['headers']['X-API-Key'] === 'site-key', 'token refresh did not send the site key');
    check(strpos($url, '/sites/current') !== false, 'token refresh did not read the current site');
    return array(
        'response' => array('code' => 200),
        'body' => '{"site_id":"site-1","verification_token":"fresh-token","verified":false,"verified_at":null}',
    );
};
$api->refresh_verification_token();
check(get_option('affilio_verification_token', '') === 'fresh-token', 'a current-site response did not store the token');

$GLOBALS['options']['affilio_api_key'] = '';
$api->refresh_verification_token();
check(!array_key_exists('affilio_verification_token', $GLOBALS['options']), 'clearing the site key left the verification token');

echo "domain verification file ok\n";
