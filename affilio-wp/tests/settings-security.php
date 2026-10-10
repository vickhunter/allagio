<?php

define('WP_DEBUG', true);

require __DIR__ . '/bootstrap.php';

$site_key = 'abcdSECRETKEY1234wxyz';
$admin_key = 'admnSECRETADMIN9999qrst';
$plugin_key = 'plugPLUGINKEY1234mnop';
$failures = array();

function fail_security($message) {
    $GLOBALS['failures'][] = $message;
}

function assert_key_hidden($html, $key, $label) {
    if (strpos($html, $key) !== false) {
        fail_security($label . ' printed the full stored key');
        return;
    }
    $last = substr($key, -4);
    if (strpos($html, $last) !== false) {
        fail_security($label . ' printed the last 4 characters of the stored key');
    }
    $length = strlen($key);
    for ($size = 5; $size <= $length; $size++) {
        for ($offset = 0; $offset + $size <= $length; $offset++) {
            $piece = substr($key, $offset, $size);
            if (strpos($html, $piece) !== false) {
                fail_security($label . ' printed a stored-key fragment longer than 4 characters');
                return;
            }
        }
    }
}

function run_scan($administrator, $nonce) {
    $bootstrap = var_export(__DIR__ . '/bootstrap.php', true);
    $plugin = var_export(dirname(__DIR__) . '/affilio.php', true);
    $flag = $administrator ? 'true' : 'false';
    $nonce_line = $nonce === null
        ? 'unset($_REQUEST["_wpnonce"]);'
        : '$_REQUEST["_wpnonce"] = ' . var_export($nonce, true) . ';';
    $code = <<<PHP
<?php
\$GLOBALS['administrator'] = {$flag};
{$nonce_line}
\$_SERVER['REQUEST_METHOD'] = 'POST';
require {$bootstrap};
require {$plugin};
class Affilio_ScanSpy {
    public function scan_published_batch(\$offset = 0) {
        fwrite(STDERR, "SCANNED\n");
        return array('scanned' => 0, 'skipped' => array(), 'fetched' => 0, 'done' => true, 'next_offset' => 0);
    }
}
class Affilio_QuietAPI {
    public function get_connection_status(\$force = false) {
        return array('connected' => true, 'message' => 'ok', 'state' => 'connected', 'label' => 'Connected');
    }
    public function invalidate_status_cache() {}
}
\$page = new Affilio_Admin(new Affilio_QuietAPI(), new Affilio_ScanSpy(), null);
try {
    \$page->handle_scan_all();
    echo "RETURNED\n";
} catch (RuntimeException \$e) {
    echo "REFUSED\n";
}
PHP;
    $file = tempnam(sys_get_temp_dir(), 'affilio-scan');
    file_put_contents($file, $code);
    $spec = array(0 => array('pipe', 'r'), 1 => array('pipe', 'w'), 2 => array('pipe', 'w'));
    $process = proc_open('php ' . escapeshellarg($file), $spec, $pipes);
    fclose($pipes[0]);
    $stdout = stream_get_contents($pipes[1]);
    $stderr = stream_get_contents($pipes[2]);
    fclose($pipes[1]);
    fclose($pipes[2]);
    proc_close($process);
    unlink($file);
    return array($stdout, $stderr);
}

$GLOBALS['options']['affilio_api_key'] = $site_key;
$GLOBALS['options']['affilio_admin_api_key'] = $admin_key;
$GLOBALS['options']['affilio_plugin_key'] = $plugin_key;
$GLOBALS['administrator'] = true;
$_SERVER['REQUEST_METHOD'] = 'GET';

require dirname(__DIR__) . '/affilio.php';
affilio_init();

$http_before_connect = count($GLOBALS['http']);
$connect = (new Affilio_Onboarding(new Affilio_API()))->connect();
if (!is_wp_error($connect) || $connect->get_error_message() !== 'Set the API URL in Settings before connecting.') {
    fail_security('connect with an empty API URL did not tell the owner to set the API URL first');
}
if (is_wp_error($connect) && stripos($connect->get_error_message(), 'not ready') !== false) {
    fail_security('connect with an empty API URL said the service is not ready');
}
if (count($GLOBALS['http']) !== $http_before_connect) {
    fail_security('connect with an empty API URL made an HTTP request');
}

class Affilio_QuietAPI {
    public function get_connection_status($force = false) {
        return array('connected' => true, 'message' => 'Affilio is connected and ready.', 'state' => 'connected', 'label' => 'Connected');
    }
    public function invalidate_status_cache() {}
}
class Affilio_ScanSpy {
    public $calls = 0;
    public function scan_all_posts() { $this->calls++; }
}

$quiet = new Affilio_QuietAPI();
$page = new Affilio_Admin(new Affilio_API(), new Affilio_ScanSpy(), null);
ob_start();
$page->render_settings();
$settings = ob_get_clean();
assert_key_hidden($settings, $site_key, 'settings page');
assert_key_hidden($settings, $admin_key, 'advanced settings');
assert_key_hidden($settings, $plugin_key, 'plugin key field');
if (strpos($settings, 'placeholder="Key saved (' . substr($site_key, 0, 4) . '…)"') === false) {
    fail_security('settings API key field is missing the saved-key placeholder');
}
if (strpos($settings, 'name="affilio_api_key" value=""') === false) {
    fail_security('settings API key field did not render empty');
}
if (strpos($settings, 'placeholder="Key saved (' . substr($admin_key, 0, 4) . '…)"') === false) {
    fail_security('advanced admin key field is missing the saved-key placeholder');
}
if (strpos($settings, 'name="affilio_admin_api_key" value=""') === false) {
    fail_security('advanced admin key field did not render empty');
}
if (strpos($settings, 'placeholder="Key saved (' . substr($plugin_key, 0, 4) . '…)"') === false) {
    fail_security('plugin key field is missing the saved-key placeholder');
}
if (strpos($settings, 'name="affilio_plugin_key" value=""') === false) {
    fail_security('plugin key field did not render empty');
}
if (strpos($settings, 'X-Plugin-Key') === false) {
    fail_security('plugin key field did not say it is sent as X-Plugin-Key');
}
if (strpos($settings, 'class="affilio-key-hint"') === false || strpos($settings, 'Key saved (' . substr($site_key, 0, 4) . '…)') === false) {
    fail_security('status box did not show the first 4 characters and an ellipsis');
}

$GLOBALS['screen'] = (object) array('id' => 'post', 'base' => 'post');
ob_start();
$page->maybe_render_connection_notice();
$notice = ob_get_clean();
assert_key_hidden($notice, $site_key, 'connection notice');
assert_key_hidden($notice, $admin_key, 'connection notice');

$_SERVER['REQUEST_METHOD'] = 'POST';
$_POST = array(
    'affilio_api_key' => '',
    'affilio_admin_api_key' => '   ',
    'affilio_plugin_key' => '',
    '_wpnonce' => wp_create_nonce('affilio_settings'),
);
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
ob_end_clean();
if (get_option('affilio_api_key', '') !== $site_key) {
    fail_security('an empty API key field replaced the stored key');
}
if (get_option('affilio_admin_api_key', '') !== $admin_key) {
    fail_security('an empty admin key field replaced the stored key');
}
if (get_option('affilio_plugin_key', '') !== $plugin_key) {
    fail_security('an empty plugin key field replaced the stored key');
}

$replacement = 'zzzzNEWSECRETKEY5678abcd';
$plugin_replacement = 'plugNEWPLUGINKEY5678qrst';
$_POST['affilio_api_key'] = $replacement;
$_POST['affilio_admin_api_key'] = 'admnNEWADMINKEY5678wxyz';
$_POST['affilio_plugin_key'] = $plugin_replacement;
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
$replaced = ob_get_clean();
if (get_option('affilio_api_key', '') !== $replacement) {
    fail_security('a new API key did not replace the stored key');
}
if (get_option('affilio_plugin_key', '') !== $plugin_replacement) {
    fail_security('a new plugin key did not replace the stored key');
}
assert_key_hidden($replaced, $replacement, 'settings page after replace');
assert_key_hidden($replaced, $plugin_replacement, 'plugin key after replace');
$GLOBALS['options']['affilio_api_key'] = $site_key;
$GLOBALS['options']['affilio_admin_api_key'] = $admin_key;
$GLOBALS['options']['affilio_plugin_key'] = $plugin_key;

$dashboard = new Affilio_Admin($quiet, new Affilio_ScanSpy(), null);
ob_start();
$dashboard->render_dashboard();
$form = ob_get_clean();
assert_key_hidden($form, $plugin_key, 'dashboard');
$scan_nonce = null;
if (!preg_match('/name="_wpnonce" value="([^"]+)"/', $form, $nonce_match)) {
    fail_security('Scan All Posts form did not include a nonce');
} else {
    $scan_nonce = $nonce_match[1];
}

$missing_nonce = run_scan(true, null);
if (strpos($missing_nonce[1], 'SCANNED') !== false || strpos($missing_nonce[0], 'REFUSED') === false) {
    fail_security('Scan All Posts started without a valid nonce');
}
$missing_cap = run_scan(false, $scan_nonce === null ? wp_create_nonce('affilio_scan_all') : $scan_nonce);
if (strpos($missing_cap[1], 'SCANNED') !== false || strpos($missing_cap[0], 'REFUSED') === false) {
    fail_security('Scan All Posts started without manage_options');
}
$allowed = run_scan(true, $scan_nonce === null ? wp_create_nonce('affilio_scan_all') : $scan_nonce);
if (strpos($allowed[1], 'SCANNED') === false) {
    fail_security('Scan All Posts with a valid nonce and manage_options did not scan');
}

$GLOBALS['probe_urls'] = array();
add_filter('pre_http_request', function ($pre, $args, $url) {
    $GLOBALS['probe_urls'][] = $url;
    if (strpos($url, '/health') !== false) {
        return array('response' => array('code' => 200), 'body' => '{}');
    }
    $sent = isset($args['headers']['X-API-Key']) ? $args['headers']['X-API-Key'] : '';
    if ($sent === 'abcdSECRETKEY1234wxyz') {
        return array('response' => array('code' => 200), 'body' => '[]');
    }
    return array('response' => array('code' => 401), 'body' => '{}');
});

$GLOBALS['options']['affilio_api_url_override'] = 'https://old.example/v1';
$GLOBALS['options']['affilio_api_key'] = $site_key;
$client = new Affilio_API();
$first = $client->get_connection_status();
if (empty($first['connected'])) {
    fail_security('connection status did not connect with the original key');
}
$GLOBALS['options']['affilio_api_key'] = 'zzzzOTHERKEY1234yyyy';
$second = $client->get_connection_status();
if (!empty($second['connected'])) {
    fail_security('changing the API key kept the cached connection status');
}
$GLOBALS['options']['affilio_api_url_override'] = 'https://new.example/v1';
$client->get_connection_status();
$saw_new_host = false;
foreach ($GLOBALS['probe_urls'] as $url) {
    if (strpos($url, 'new.example') !== false) {
        $saw_new_host = true;
    }
}
if (!$saw_new_host) {
    fail_security('changing the API URL kept the cached connection host');
}

$GLOBALS['options']['affilio_api_url_override'] = 'https://kept.example/v1';
$_SERVER['REQUEST_METHOD'] = 'POST';
$_POST = array(
    'affilio_api_key' => '',
    'affilio_admin_api_key' => '',
    'affilio_api_url_override' => 'http://evil.example/v1',
    '_wpnonce' => wp_create_nonce('affilio_settings'),
);
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
$rejected_url = ob_get_clean();
if (get_option('affilio_api_url_override', '') !== 'https://kept.example/v1') {
    fail_security('an http API URL override replaced the stored https URL');
}
if (strpos($rejected_url, 'deve usare https, salvo localhost.') === false) {
    fail_security('API URL override did not explain the https rule in Italian');
}
foreach (array('http://localhost/v1', 'http://127.0.0.1:8080/v1', 'https://ok.example/v1') as $allowed_url) {
    $_POST['affilio_api_url_override'] = $allowed_url;
    $_REQUEST = $_POST;
    ob_start();
    $page->render_settings();
    ob_end_clean();
    if (get_option('affilio_api_url_override', '') !== $allowed_url) {
        fail_security('allowed API URL was rejected: ' . $allowed_url);
    }
}
$_POST['affilio_api_url_override'] = 'http://localhost.evil.com/v1';
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
ob_end_clean();
if (get_option('affilio_api_url_override', '') !== 'https://ok.example/v1') {
    fail_security('a lookalike localhost host replaced the stored API URL');
}
$_POST['affilio_api_url_override'] = '';
$_REQUEST = $_POST;
ob_start();
$page->render_settings();
ob_end_clean();
if (get_option('affilio_api_url_override', '') !== '') {
    fail_security('an empty API URL override did not clear the stored URL');
}

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}

echo "Settings hide stored keys, scans require a nonce, and connection status follows the URL and key.\n";
