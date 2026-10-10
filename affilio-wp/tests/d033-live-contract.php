<?php

// Live contract against a local API. Skips unless AFFILIO_E2E_FIXTURE points
// at the JSON written by the seed step. Not part of the release zip.

if (getenv('AFFILIO_E2E_FIXTURE') === false || getenv('AFFILIO_E2E_FIXTURE') === '') {
    echo "live contract skipped (AFFILIO_E2E_FIXTURE unset)\n";
    exit(0);
}

$fixture_path = getenv('AFFILIO_E2E_FIXTURE');
$fixture = json_decode((string) file_get_contents($fixture_path), true);
if (!is_array($fixture)) {
    fwrite(STDERR, "fixture is not JSON\n");
    exit(1);
}

require __DIR__ . '/../includes/class-affilio-api.php';
require __DIR__ . '/d033-harness.php';

if (!function_exists('wp_remote_retrieve_response_code')) {
    function wp_remote_retrieve_response_code($response) {
        return isset($response['response']['code']) ? $response['response']['code'] : 0;
    }
    function wp_remote_retrieve_body($response) {
        return isset($response['body']) ? $response['body'] : '';
    }
}

function d033_live_http($url, $args) {
    $method = isset($args['method']) ? strtoupper((string) $args['method']) : 'GET';
    $header_lines = array();
    $headers = isset($args['headers']) && is_array($args['headers']) ? $args['headers'] : array();
    foreach ($headers as $name => $value) {
        $header_lines[] = $name . ': ' . $value;
    }
    $content = isset($args['body']) ? (string) $args['body'] : '';
    $context = stream_context_create(array(
        'http' => array(
            'method' => $method,
            'header' => implode("\r\n", $header_lines),
            'content' => $content,
            'ignore_errors' => true,
            'follow_location' => 0,
            'timeout' => 30,
        ),
    ));
    $body = file_get_contents($url, false, $context);
    $code = 0;
    if (isset($http_response_header[0]) && preg_match('#HTTP/\S+\s+(\d+)#', $http_response_header[0], $match)) {
        $code = (int) $match[1];
    }
    $response = array(
        'response' => array('code' => $code),
        'body' => $body === false ? '' : $body,
        'headers' => array(),
    );
    $GLOBALS['live_calls'][] = array(
        'url' => $url,
        'args' => $args,
        'code' => $code,
        'body' => $response['body'],
    );
    return $response;
}

function d033_ack_result($placement_id) {
    $matched = null;
    foreach ($GLOBALS['live_calls'] as $call) {
        if (strpos($call['url'], '/placements/' . $placement_id . '/plugin-ack') !== false) {
            $matched = $call;
        }
    }
    return $matched;
}

$GLOBALS['live_calls'] = array();
$GLOBALS['http_handler'] = 'd033_live_http';
$GLOBALS['options']['affilio_api_url_override'] = $fixture['base'];
$GLOBALS['options']['affilio_api_key'] = $fixture['site_key'];
$GLOBALS['options']['affilio_plugin_key'] = $fixture['plugin_key'];
$GLOBALS['allowed'] = true;

if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.19');
}

require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$copy = $fixture['copy'];
$disclosure = $fixture['disclosure'];
$upgrade_id = $fixture['upgrade'];
$placed_id = $fixture['placed'];
$removed_id = $fixture['removed'];
$restored_id = $fixture['restored'];
$failed_id = $fixture['failed'];
$receipt_id = $fixture['receipt'];
$refused_id = $fixture['refused'];

function d033_box($id, $copy) {
    return "<!-- wp:paragraph {\"className\":\"affilio-placement-" . $id . "\"} -->\n"
        . "<p class=\"affilio-placement-" . $id . "\">" . $copy
        . " <a href=\"https://blog.example/go/" . $id . "\" rel=\"sponsored nofollow\">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)</p>\n"
        . "<!-- /wp:paragraph -->";
}

$upgrade = (object) array(
    'ID' => 2615,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Upgrade',
    'post_content' => d033_box($upgrade_id, $copy),
);
$GLOBALS['posts'] = array($upgrade);
$GLOBALS['post'] = $upgrade;
$before = $upgrade->post_content;
$updates_before = $GLOBALS['updates'];

$distribution = new Affilio_Distribution(new Affilio_API());
$distribution->maybe_upgrade();
$upgrade_call = d033_ack_result($upgrade_id);
check($upgrade_call !== null, 'upgrade did not send plugin-ack');
check($upgrade_call['code'] === 200, 'upgrade ack HTTP ' . $upgrade_call['code'] . ' ' . $upgrade_call['body']);
$upgrade_body = json_decode($upgrade_call['body'], true);
check(is_array($upgrade_body) && $upgrade_body['plugin_state'] === 'placed', 'upgrade ack state ' . $upgrade_call['body']);
$sent = json_decode($upgrade_call['args']['body'], true);
check($sent['action'] === 'placed' && $sent['rel_sponsored'] === true && !isset($sent['reason']), 'upgrade body ' . $upgrade_call['args']['body']);
check(!isset($upgrade_call['args']['headers']['X-API-Key']), 'upgrade ack sent X-API-Key');
check($upgrade_call['args']['headers']['X-Plugin-Key'] === $fixture['plugin_key'], 'upgrade ack did not send the plugin key');
check($upgrade->post_content === $before, 'upgrade ack rewrote the post');
check($GLOBALS['updates'] === $updates_before, 'upgrade ack saved the post');

$placed = new Affilio_Plugin_Ack();
$placed_response = $placed->send($placed_id, AFFILIO_PLUGIN_ACK_PLACED, true, '');
check(wp_remote_retrieve_response_code($placed_response) === 200, 'placed ack HTTP ' . wp_remote_retrieve_body($placed_response));
$placed_body = json_decode(wp_remote_retrieve_body($placed_response), true);
check(is_array($placed_body) && $placed_body['plugin_state'] === 'placed', 'placed state ' . wp_remote_retrieve_body($placed_response));
$placed_call = d033_ack_result($placed_id);
check($placed_call !== null && !isset($placed_call['args']['headers']['X-API-Key']), 'placed ack sent the site key');

$removed = (object) array(
    'ID' => 2616,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Removed',
    'post_content' => d033_box($removed_id, $copy),
);
$GLOBALS['posts'][] = $removed;
check($distribution->revoke_box(2616, $removed_id) === true, 'revoke did not confirm removal');
$removed_call = d033_ack_result($removed_id);
check($removed_call !== null && $removed_call['code'] === 200, 'removed ack HTTP ' . ($removed_call ? $removed_call['body'] : 'missing'));
$removed_body = json_decode($removed_call['body'], true);
check($removed_body['plugin_state'] === 'removed', 'removed state ' . $removed_call['body']);
$removed_sent = json_decode($removed_call['args']['body'], true);
check($removed_sent['action'] === 'removed' && $removed_sent['rel_sponsored'] === false, 'removed body ' . $removed_call['args']['body']);

$restored = (object) array(
    'ID' => 2617,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Restored',
    'post_content' => 'Hello',
);
$GLOBALS['posts'][] = $restored;
check($distribution->restore_box(2617, $restored_id, array('copy' => $copy, 'disclosure' => $disclosure)) === true, 'restore did not confirm the box');
$restored_call = d033_ack_result($restored_id);
check($restored_call !== null && $restored_call['code'] === 200, 'restored ack HTTP ' . ($restored_call ? $restored_call['body'] : 'missing'));
$restored_body = json_decode($restored_call['body'], true);
check($restored_body['plugin_state'] === 'placed', 'restored state ' . $restored_call['body']);
$restored_sent = json_decode($restored_call['args']['body'], true);
check($restored_sent['action'] === 'restored' && $restored_sent['rel_sponsored'] === true, 'restored body ' . $restored_call['args']['body']);

$failed = (object) array(
    'ID' => 2618,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Failed',
    'post_content' => d033_box($failed_id, $copy),
);
$GLOBALS['posts'][] = $failed;
$GLOBALS['noop_saves'] = 5;
check($distribution->revoke_box(2618, $failed_id) === false, 'failed re-read was reported as success');
$failed_call = d033_ack_result($failed_id);
check($failed_call !== null && $failed_call['code'] === 200, 'failed ack HTTP ' . ($failed_call ? $failed_call['body'] : 'missing'));
$failed_body = json_decode($failed_call['body'], true);
check($failed_body['plugin_state'] === 'failed' && $failed_body['plugin_failed_reason'] === 're-read failed', 'failed state ' . $failed_call['body']);
$failed_sent = json_decode($failed_call['args']['body'], true);
check($failed_sent['action'] === 'failed' && $failed_sent['reason'] === 're-read failed', 'failed body ' . $failed_call['args']['body']);

$published = '<p class="affilio-placement-' . $receipt_id . '">' . $copy
    . ' <a href="https://blog.example/go/' . $receipt_id . '" rel="sponsored nofollow">Vedi il prodotto</a> (link sponsorizzato)</p>'
    . "\n" . $disclosure;
$receipt_method = new ReflectionMethod('Affilio_Distribution', 'request');
$receipt_method->setAccessible(true);
$receipt = $receipt_method->invoke($distribution, 'POST', '/placements/' . $receipt_id . '/receipt', array(
    'surface_url' => 'https://blog.example/post',
    'published_text' => $published,
    'evidence' => 'WordPress administrator inserted the approved placement in published post 7',
));
check(!is_wp_error($receipt), 'receipt failed: ' . (is_wp_error($receipt) ? $receipt->get_error_message() : ''));
check(isset($receipt['status']) && $receipt['status'] === 'publisher_reported', 'receipt status ' . json_encode($receipt));
$receipt_call = null;
foreach ($GLOBALS['live_calls'] as $call) {
    if (strpos($call['url'], '/placements/' . $receipt_id . '/receipt') !== false) {
        $receipt_call = $call;
    }
}
check($receipt_call !== null && isset($receipt_call['args']['headers']['X-API-Key']) && $receipt_call['args']['headers']['X-API-Key'] === $fixture['site_key'], 'receipt did not use the site key');
check(!isset($receipt_call['args']['headers']['X-Plugin-Key']), 'receipt sent X-Plugin-Key');
check(strpos($receipt_call['args']['body'], '/go/' . $receipt_id) !== false, 'receipt body omitted /go/');

$refused = d033_live_http(rtrim($fixture['base'], '/') . '/placements/' . $refused_id . '/plugin-ack', array(
    'method' => 'POST',
    'headers' => array(
        'Content-Type' => 'application/json',
        'X-API-Key' => $fixture['project_key'],
    ),
    'body' => json_encode(array('action' => 'placed', 'rel_sponsored' => true)),
));
check($refused['code'] === 403, 'project X-API-Key on plugin-ack returned ' . $refused['code'] . ' ' . $refused['body']);
$refused_body = json_decode($refused['body'], true);
check(is_array($refused_body) && isset($refused_body['error']) && $refused_body['error'] === 'plugin key required', 'project key error ' . $refused['body']);

echo "Live plugin ack and receipt contract passed.\n";
