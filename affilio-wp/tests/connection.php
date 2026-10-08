<?php

require __DIR__ . '/bootstrap.php';

$plugin = file_get_contents(dirname(__DIR__) . '/affilio.php');
$api = file_get_contents(dirname(__DIR__) . '/includes/class-affilio-api.php');
$admin = file_get_contents(dirname(__DIR__) . '/includes/class-affilio-admin.php');
foreach (array($plugin, $api, $admin) as $source) {
    check(strpos($source, 'railway.app') === false, 'plugin still defaults to a Railway URL');
    check(strpos($source, 'api.affilio.io') === false, 'plugin still falls back to api.affilio.io');
}

require dirname(__DIR__) . '/affilio.php';
affilio_init();

check(Affilio_API::resolve_base_url() === '', 'empty install still resolved an API base URL');
$http_before = count($GLOBALS['http']);
$client = new Affilio_API();
$status = $client->get_connection_status(true);
$result = $client->request('GET', '/articles');
check(count($GLOBALS['http']) === $http_before, 'empty API URL made an outbound HTTP call');
check(is_wp_error($result), 'request without an API URL did not fail closed');
check(strpos($status['message'], 'Not connected — set the API URL in Settings') !== false, 'missing URL did not explain how to connect');

ob_start();
$admin_page = new Affilio_Admin($client, new Affilio_Scanner($client), new Affilio_Inserter($client));
$GLOBALS['administrator'] = true;
$_SERVER['REQUEST_METHOD'] = 'GET';
$admin_page->render_settings();
$settings = ob_get_clean();
check(strpos($settings, 'Not connected — set the API URL in Settings') !== false, 'settings did not show the missing URL notice');

$GLOBALS['options']['affilio_api_url_override'] = 'https://override.example/v1';
check(Affilio_API::resolve_base_url() === 'https://override.example/v1', 'option override was ignored');
$GLOBALS['options']['affilio_api_url_override'] = '';
add_filter('affilio_api_base_url', function () { return 'https://filter.example/v1'; });
check(Affilio_API::resolve_base_url() === 'https://filter.example/v1', 'affilio_api_base_url filter was ignored');

echo "API URL comes only from configuration and stays quiet until it is set.\n";
