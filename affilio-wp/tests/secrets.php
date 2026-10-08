<?php

require __DIR__ . '/bootstrap.php';

$secret = 'abcdSECRETKEY1234wxyz';
$admin_secret = 'admnSECRETADMIN9999qrst';
$GLOBALS['options']['affilio_api_key'] = $secret;
$GLOBALS['options']['affilio_admin_api_key'] = $admin_secret;
$GLOBALS['options']['affilio_api_url_override'] = 'https://api.example.test/v1';
define('WP_DEBUG', true);

require dirname(__DIR__) . '/affilio.php';
affilio_init();

if (!empty($GLOBALS['actions']['rest_api_init'])) {
    foreach ($GLOBALS['actions']['rest_api_init'] as $callback) {
        call_user_func($callback);
    }
}
if (!empty($GLOBALS['actions']['admin_enqueue_scripts'])) {
    foreach ($GLOBALS['actions']['admin_enqueue_scripts'] as $callback) {
        call_user_func($callback, 'toplevel_page_affilio');
        call_user_func($callback, 'affilio_page_affilio-marketplace');
        call_user_func($callback, 'affilio_page_affilio-earnings');
        call_user_func($callback, 'affilio_page_affilio-analytics');
        call_user_func($callback, 'affilio_page_affilio-discovery');
    }
}

$masked = substr($secret, 0, 4) . '…' . substr($secret, -4);
$admin_masked = substr($admin_secret, 0, 4) . '…' . substr($admin_secret, -4);
$handles = array(
    'affilio-dashboard' => $masked,
    'affilio-marketplace' => $masked,
    'affilio-earnings' => $masked,
    'affilio-analytics' => $masked,
);
foreach ($handles as $handle => $expected) {
    $data = $GLOBALS['localized'][$handle]['AffilioData'];
    $encoded = json_encode($data);
    check(strpos($encoded, $secret) === false, $handle . ' localized the full API key');
    check(isset($data['apiKey']) && $data['apiKey'] === $expected, $handle . ' did not receive a masked API key');
    check(isset($data['restBase']) && $data['restBase'] !== '', $handle . ' is missing the WordPress proxy base');
}
$discovery = $GLOBALS['localized']['affilio-discovery']['AffilioData'];
$discovery_json = json_encode($discovery);
check(strpos($discovery_json, $admin_secret) === false, 'discovery localized the full admin key');
check($discovery['adminKey'] === $admin_masked, 'discovery admin key is not masked');
check(!isset($discovery['actor']) || ($discovery['actor'] !== 'siteadmin' && strpos($discovery_json, 'siteadmin') === false), 'discovery still receives the WordPress username');
check(strpos($discovery_json, 'admin@example.com') === false, 'discovery localized an email address');
check(strpos($discovery_json, 'Ada Admin') === false, 'discovery localized a display name');

$GLOBALS['options']['affilio_api_key'] = 'short';
$GLOBALS['localized'] = array();
foreach ($GLOBALS['actions']['admin_enqueue_scripts'] as $callback) {
    call_user_func($callback, 'toplevel_page_affilio');
}
$short = $GLOBALS['localized']['affilio-dashboard']['AffilioData']['apiKey'];
check($short === '••••••••', 'short API key was revealed to admin JavaScript');
check(strpos($short, 'short') === false, 'short API key leaked into admin JavaScript');
$GLOBALS['options']['affilio_api_key'] = $secret;

foreach (array('dashboard.js', 'marketplace.js', 'earnings.js', 'discovery.js', 'analytics.js') as $file) {
    $source = file_get_contents(dirname(__DIR__) . '/admin/build/' . $file);
    check(strpos($source, 'X-API-Key') === false, $file . ' still sends the API key from the browser');
    check(strpos($source, 'X-Admin-Key') === false, $file . ' still sends the admin key from the browser');
}
$discovery_source = file_get_contents(dirname(__DIR__) . '/admin/build/discovery.js');
check(strpos($discovery_source, 'actor') === false, 'discovery script still sends an actor');

$proxy = null;
foreach ($GLOBALS['routes'] as $route) {
    if ($route[0] === 'affilio/v1' && $route[1] === '/proxy') {
        $proxy = $route[2]['callback'];
    }
}
check(is_callable($proxy), 'server-side API proxy is not registered');

$before = count($GLOBALS['http']);
$response = call_user_func($proxy, new Affilio_Test_Request(array(
    'method' => 'GET',
    'path' => '/articles',
)));
check($response instanceof WP_REST_Response, 'proxy did not return the backend response');
check(count($GLOBALS['http']) === $before + 1, 'proxy did not call the backend');
$outbound = $GLOBALS['http'][count($GLOBALS['http']) - 1];
check($outbound['args']['headers']['X-API-Key'] === $secret, 'proxy did not attach the stored site key');
check(strpos(json_encode($response->data), $secret) === false, 'proxy response leaked the API key');

$denied = call_user_func($proxy, new Affilio_Test_Request(array(
    'method' => 'GET',
    'path' => '/articles-evil',
)));
check(is_wp_error($denied), 'proxy allowed a path outside the allowlist');

$traversal = call_user_func($proxy, new Affilio_Test_Request(array(
    'method' => 'GET',
    'path' => '/articles/../../wp/v2/users',
)));
check(is_wp_error($traversal), 'proxy allowed a traversal path');

$http_before_admin = count($GLOBALS['http']);
$review = call_user_func($proxy, new Affilio_Test_Request(array(
    'method' => 'POST',
    'path' => '/program-discovery/queue/1/review',
    'body' => array('status' => 'approved', 'actor' => 'siteadmin', 'email' => 'admin@example.com'),
)));
check($review instanceof WP_REST_Response, 'discovery proxy did not return a response');
$admin_call = $GLOBALS['http'][count($GLOBALS['http']) - 1];
check(count($GLOBALS['http']) === $http_before_admin + 1, 'discovery proxy made an unexpected request');
check($admin_call['args']['headers']['X-Admin-Key'] === $admin_secret, 'discovery proxy did not use the stored admin key');
check(!isset($admin_call['args']['headers']['X-API-Key']) || $admin_call['args']['headers']['X-API-Key'] === '', 'discovery proxy sent the site key');
$admin_body = isset($admin_call['args']['body']) ? $admin_call['args']['body'] : '';
check(strpos($admin_body, 'siteadmin') === false, 'discovery proxy forwarded the WordPress username');
check(strpos($admin_body, 'admin@example.com') === false, 'discovery proxy forwarded an email');
check(strpos($admin_body, 'approved') !== false, 'discovery proxy dropped the review status');

echo "Admin JavaScript receives a masked key and calls the API through WordPress.\n";
