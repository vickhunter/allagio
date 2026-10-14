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
$GLOBALS['options']['affilio_api_url_override'] = 'https://override.example/';
check(Affilio_API::resolve_base_url() === 'https://override.example/v1', 'a trailing slash was left on the API origin');
$GLOBALS['options']['affilio_api_url_override'] = 'https://api.example.test';
check(Affilio_API::resolve_base_url() === 'https://api.example.test/v1', 'a bare origin was used without /v1');

if (!class_exists('ConnectionRequest')) {
    class ConnectionRequest {
        private $values;
        public function __construct($values) { $this->values = $values; }
        public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
    }
}
$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$GLOBALS['posts'] = array((object) array(
    'ID' => 2615,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Articolo',
    'post_name' => 'post',
    'post_content' => '<p>Articolo.</p>',
));
$GLOBALS['options']['affilio_api_key'] = 'site-key';
$GLOBALS['http'] = array();
add_filter('pre_http_request', function ($pre, $args, $url) use ($placement_id) {
    if (strpos($url, '/receipt') !== false) {
        return new WP_Error('http_request_failed', 'network down');
    }
    $placement = array(
        'id' => $placement_id,
        'surface_url' => 'https://blog.example/post',
        'status' => 'committed',
        'copy' => 'Uno strumento utile.',
        'disclosure' => 'Affiliate disclosure.',
        'fallback_url' => 'https://shop.example/plain',
        'receipt' => array('tracked_link' => '/v1/go/' . $placement_id, 'publication_status' => 'awaiting_publication'),
    );
    $data = array('ok' => true);
    if (strpos($url, '/sites/owner-connection') !== false) {
        $data = array('connected' => true);
    } elseif (strpos($url, '/commit') !== false) {
        $data = array('placement' => $placement);
    } elseif (strpos($url, '/placements/') !== false) {
        $data = array('placement' => $placement, 'surface_url' => $placement['surface_url']);
    }
    return array('response' => array('code' => 200), 'body' => json_encode($data));
}, 10, 3);
$refused = (new Affilio_Distribution(new Affilio_API()))->publish(new ConnectionRequest(array(
    'post_id' => 2615,
    'placement_id' => $placement_id,
)));
check(is_wp_error($refused), 'a failed receipt on a bare origin was treated as published');
$refuse_urls = array();
foreach ($GLOBALS['http'] as $call) {
    if (strpos($call['url'], '/refuse') !== false) {
        $refuse_urls[] = $call['url'];
    }
}
check($refuse_urls === array('https://api.example.test/v1/placements/' . $placement_id . '/refuse'), 'bare-origin refuse URL was ' . json_encode($refuse_urls));

$GLOBALS['options']['affilio_api_url_override'] = '';
add_filter('affilio_api_base_url', function () { return 'https://filter.example/v1'; });
check(Affilio_API::resolve_base_url() === 'https://filter.example/v1', 'affilio_api_base_url filter was ignored');
add_filter('affilio_api_base_url', function () { return 'https://bare-filter.example/'; });
check(Affilio_API::resolve_base_url() === 'https://bare-filter.example/v1', 'a filtered bare origin was used without /v1');

echo "API URL comes only from configuration and stays quiet until it is set.\n";
