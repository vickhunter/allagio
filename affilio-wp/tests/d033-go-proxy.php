<?php

require __DIR__ . '/d033-harness.php';
d033_log_file();
require __DIR__ . '/../includes/class-affilio-go.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$raw_ip = '203.0.113.60';
$_SERVER['REMOTE_ADDR'] = $raw_ip;
$_SERVER['HTTP_X_FORWARDED_FOR'] = '198.51.100.20';
$_SERVER['HTTP_USER_AGENT'] = 'Reader/1.0';
$_SERVER['HTTP_REFERER'] = 'https://blog.example/post-2615';
$_SERVER['REQUEST_URI'] = '/go/' . $placement_id;
$GLOBALS['options']['affilio_api_key'] = 'site-key-for-go';
$GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION] = array($placement_id => 'https://shop.example/fallback');
$GLOBALS['filters']['affilio_go_utc_date'] = array(array(
    'callback' => function ($date) { return '2026-04-10'; },
    'priority' => 10,
    'accepted_args' => 1,
));

check(Affilio_Go::truncate_ip('203.0.113.60') === '203.0.113.0', 'IPv4 /24 was ' . Affilio_Go::truncate_ip('203.0.113.60'));
check(Affilio_Go::truncate_ip('2001:db8:abcd:1234::1') === '2001:db8:abcd::', 'IPv6 /48 was ' . Affilio_Go::truncate_ip('2001:db8:abcd:1234::1'));
check(Affilio_Go::truncate_ip('::ffff:203.0.113.60') === '203.0.113.0', 'IPv4-mapped address was not truncated as IPv4');
$salt_a = Affilio_Go::daily_salt('site-key-for-go', '2026-04-10');
$salt_b = Affilio_Go::daily_salt('site-key-for-go', '2026-04-11');
check($salt_a !== $salt_b && $salt_a === hash('sha256', 'site-key-for-go|go-link|2026-04-10'), 'daily salt is not the documented SHA-256');
$expected_hash = Affilio_Go::client_hash('203.0.113.0', $salt_a);
check(Affilio_Go::visitor_hash() === $expected_hash, 'visitor hash did not use the truncated address and daily salt');
$_SERVER['REMOTE_ADDR'] = '198.51.100.8';
check(Affilio_Go::visitor_hash() !== $expected_hash, 'REMOTE_ADDR did not change the hash');
$_SERVER['REMOTE_ADDR'] = $raw_ip;
check(Affilio_Go::visitor_hash() === $expected_hash, 'X-Forwarded-For changed the hash');

$GLOBALS['http'] = array();
$GLOBALS['http_response'] = array(
    'response' => array('code' => 302),
    'headers' => array('Location' => 'https://shop.example/landed'),
);
$go = new Affilio_Go();
check($go->resolve($placement_id) === 'https://shop.example/landed', '302 Location was not passed through');
check(count($GLOBALS['http']) === 1, 'upstream was not called once');
$call = $GLOBALS['http'][0];
check($call['url'] === 'https://api.example.test/v1/go/' . $placement_id . '?via=plugin', 'upstream URL was ' . $call['url']);
check((float) $call['args']['timeout'] === 1.5, 'timeout was not 1.5');
check((int) $call['args']['redirection'] === 0, 'go proxy followed redirects');
$headers = $call['args']['headers'];
check($headers['X-API-Key'] === 'site-key-for-go', 'go proxy did not send the site key');
check($headers['X-OR-Client-Hash'] === $expected_hash, 'X-OR-Client-Hash did not match the HMAC');
check($headers['X-OR-User-Agent'] === 'Reader/1.0', 'user agent was not forwarded');
check($headers['X-OR-Referer'] === 'https://blog.example/post-2615', 'referer was not forwarded');
check(!isset($headers['X-OR-Client-IP']), 'raw IP header was sent');
$blob = json_encode($call) . json_encode($GLOBALS['options']);
check(strpos($blob, $raw_ip) === false, 'raw IP was sent or stored');

$before = d033_log();
$GLOBALS['http'] = array();
$GLOBALS['http_handler'] = function ($url, $args) {
    return new WP_Error('http_request_failed', 'down');
};
check($go->resolve($placement_id) === 'https://shop.example/fallback', 'failure did not use the cached fallback');
$until = (int) get_option(AFFILIO_GO_CIRCUIT_OPTION, 0);
check($until >= time() + 59 && $until <= time() + 61, 'circuit was not opened for 60 seconds');
$calls_after_open = count($GLOBALS['http']);
check($go->resolve($placement_id) === 'https://shop.example/fallback', 'open circuit did not stay on the fallback');
check(count($GLOBALS['http']) === $calls_after_open, 'open circuit still called the API');
$log = substr(d033_log(), strlen($before));
check(strpos($log, 'affilio go fallback placement_id=' . $placement_id . ' reason=upstream_failed') !== false, 'fallback log was: ' . $log);
check(strpos($log, 'reason=circuit_open') !== false, 'circuit log was missing');
check(strpos($log, $raw_ip) === false, 'fallback log contained the raw IP');

$headers = Affilio_Go::privacy_headers();
check($headers['Cache-Control'] === 'no-store', 'Cache-Control was not no-store');
check($headers['X-Robots-Tag'] === 'noindex, nofollow', 'X-Robots-Tag was not noindex, nofollow');
check(Affilio_Go::exclude_cache(false) === true, 'WP Fastest Cache exclusion did not skip /go/');
check(strpos(Affilio_Go::robots_txt("User-agent: *\n", true), "Disallow: /go/\n") !== false, 'robots.txt is missing Disallow: /go/');
Affilio_Go::mark_uncacheable();
check(defined('DONOTCACHEPAGE') && DONOTCACHEPAGE, 'DONOTCACHEPAGE was not set');

Affilio_Go::register_rewrites();
check($GLOBALS['rewrites'][0][0] === '^go/([a-f0-9-]{36})/?$', 'rewrite pattern was ' . $GLOBALS['rewrites'][0][0]);
check($GLOBALS['rewrites'][0][1] === 'index.php?affilio_go=$matches[1]', 'rewrite target was ' . $GLOBALS['rewrites'][0][1]);
$_GET = array();
$_SERVER['REQUEST_URI'] = '/blog/';
check(Affilio_Go::current_id() === '', 'a normal page looked like /go/');
$_GET['affilio_go'] = $placement_id;
check(Affilio_Go::current_id() === $placement_id, 'affilio_go query fallback failed');

if (!defined('ABSPATH')) {
    define('ABSPATH', __DIR__);
}
if (!function_exists('plugin_dir_path')) {
    function plugin_dir_path($path) { return dirname($path) . '/'; }
    function plugin_dir_url($path) { return 'https://blog.example/wp-content/plugins/affilio-wp/'; }
}
$GLOBALS['flushes'] = 0;
$GLOBALS['rewrites'] = array();
$GLOBALS['options']['affilio_seen_version'] = '0.1.19';
require __DIR__ . '/../affilio.php';
affilio_activate();
check($GLOBALS['flushes'] >= 1, 'activation did not flush rewrite rules');
$patterns = array();
foreach ($GLOBALS['rewrites'] as $rule) {
    $patterns[] = $rule[0];
}
check(in_array('^go/([a-f0-9-]{36})/?$', $patterns, true), 'activation did not register /go/');

try {
    $_SERVER['REQUEST_URI'] = '/go/' . $placement_id;
    unset($_GET['affilio_go']);
    (new Affilio_Go())->handle();
    check(false, 'handle did not redirect');
} catch (RuntimeException $redirect) {
    check(strpos($redirect->getMessage(), 'redirect 302 ') === 0, 'handle redirect was ' . $redirect->getMessage());
}

echo "Site-local /go/ proxy, circuit, and privacy headers passed.\n";
