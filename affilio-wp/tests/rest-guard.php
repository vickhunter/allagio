<?php

require __DIR__ . '/bootstrap.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

function rest_outcome($callback) {
    $result = call_user_func($callback);
    if ($result instanceof WP_Error) {
        $status = 500;
        if (is_array($result->data) && isset($result->data['status'])) {
            $status = (int) $result->data['status'];
        }
        return array($status, $result->get_error_message());
    }
    if ($result === true) {
        return array(200, '');
    }
    return array(500, 'Non autorizzato');
}

$GLOBALS['options']['affilio_api_url_override'] = 'https://api.example.test/v1';
$GLOBALS['administrator'] = false;
require dirname(__DIR__) . '/affilio.php';
affilio_init();
if (!empty($GLOBALS['actions']['rest_api_init'])) {
    foreach ($GLOBALS['actions']['rest_api_init'] as $callback) {
        call_user_func($callback);
    }
}

$admin_paths = array('/proxy', '/setup', '/distribution/placements', '/distribution', '/distribution/connect');
$seen = array();
foreach ($GLOBALS['routes'] as $route) {
    if ($route[0] !== 'affilio/v1' || !in_array($route[1], $admin_paths, true)) {
        continue;
    }
    $spec = $route[2];
    $callbacks = isset($spec['permission_callback']) ? array($spec) : $spec;
    if (isset($spec[0]) && is_array($spec[0])) {
        $callbacks = $spec;
    }
    foreach ($callbacks as $entry) {
        if (!is_array($entry) || !isset($entry['permission_callback'])) {
            continue;
        }
        $seen[$route[1]] = true;
        list($status, $message) = rest_outcome($entry['permission_callback']);
        expect($status === 403, $route[1] . ' returned HTTP ' . $status . ' for a non-admin');
        expect($status !== 500, $route[1] . ' returned 500 ' . $message);
    }
}
foreach ($admin_paths as $path) {
    expect(!empty($seen[$path]), 'missing admin route ' . $path);
}

class DownAPI {
    public function request($method, $path, $body = null) {
        return new WP_Error('http_request_failed', 'cURL error 7: Failed to connect to api.internal.example port 8443 after 0 ms');
    }
    public function get_connection_status($force = false) {
        return array('connected' => false, 'message' => 'down');
    }
}
$GLOBALS['administrator'] = true;
$distribution = new Affilio_Distribution(new DownAPI());
$down = $distribution->placements();
expect($down instanceof WP_Error, 'placements did not fail closed when the API was down');
if ($down instanceof WP_Error) {
    $message = $down->get_error_message();
    expect(preg_match('/[àèéìòù]/u', $message) === 1, 'placements error was not a friendly Italian sentence');
    foreach (array('cURL', 'api.internal.example', '8443', 'port') as $leak) {
        expect(stripos($message, $leak) === false, 'placements error leaked ' . $leak);
    }
    $status = is_array($down->data) && isset($down->data['status']) ? (int) $down->data['status'] : 500;
    expect($status !== 500, 'placements transport failure became HTTP 500');
}

$GLOBALS['administrator'] = false;
$_SERVER['REQUEST_METHOD'] = 'POST';
$_REQUEST['_wpnonce'] = wp_create_nonce('affilio_scan_all');
$page = new Affilio_Admin(new DownAPI(), new Affilio_Scanner(new DownAPI()), null);
try {
    $page->handle_scan_all();
    expect(false, 'non-admin Scan All was not refused');
} catch (RuntimeException $e) {
    $status = isset($GLOBALS['wp_die']['status']) ? (int) $GLOBALS['wp_die']['status'] : 500;
    expect($status === 403, 'non-admin Scan All died with HTTP ' . $status);
}

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Admin routes refuse non-admins with 403 and hide transport details.\n";
