<?php

if (!defined('AFFILIO_PLUGIN_ACK_HEADER')) {
    define('AFFILIO_PLUGIN_ACK_HEADER', 'X-Plugin-Key');
}
if (!defined('AFFILIO_PLUGIN_ACK_OPTION')) {
    define('AFFILIO_PLUGIN_ACK_OPTION', 'affilio_plugin_key');
}
// Path is relative to the API base, which already ends in /v1.
// Full URL: <api>/v1/placements/<id>/plugin-ack
if (!defined('AFFILIO_PLUGIN_ACK_PATH')) {
    define('AFFILIO_PLUGIN_ACK_PATH', '/placements/%s/plugin-ack');
}
if (!defined('AFFILIO_PLUGIN_ACK_PLACED')) {
    define('AFFILIO_PLUGIN_ACK_PLACED', 'placed');
}
if (!defined('AFFILIO_PLUGIN_ACK_REMOVED')) {
    define('AFFILIO_PLUGIN_ACK_REMOVED', 'removed');
}
if (!defined('AFFILIO_PLUGIN_ACK_RESTORED')) {
    define('AFFILIO_PLUGIN_ACK_RESTORED', 'restored');
}
if (!defined('AFFILIO_PLUGIN_ACK_FAILED')) {
    define('AFFILIO_PLUGIN_ACK_FAILED', 'failed');
}

if (class_exists('Affilio_Plugin_Ack')) {
    return;
}

class Affilio_Plugin_Ack {
    public function send($placement_id, $action, $rel_sponsored, $reason = '') {
        $plugin_key = trim((string) get_option(AFFILIO_PLUGIN_ACK_OPTION, ''));
        $body = array(
            'action' => (string) $action,
            'rel_sponsored' => (bool) $rel_sponsored,
        );
        if ($action === AFFILIO_PLUGIN_ACK_FAILED) {
            $reason = trim((string) $reason);
            if ($reason === '') {
                $reason = 're-read failed';
            }
            $body['reason'] = $reason;
        }
        $base = class_exists('Affilio_API') ? Affilio_API::resolve_base_url() : '';
        $path = sprintf(AFFILIO_PLUGIN_ACK_PATH, rawurlencode((string) $placement_id));
        $url = $base === '' ? $path : trailingslashit($base) . ltrim($path, '/');
        $headers = array(
            'Content-Type' => 'application/json',
            AFFILIO_PLUGIN_ACK_HEADER => $plugin_key,
        );
        unset($headers['X-API-Key']);
        if (!function_exists('wp_remote_request')) {
            return new WP_Error('affilio_ack', 'ack transport unavailable');
        }
        return wp_remote_request($url, array(
            'method' => 'POST',
            'timeout' => 30,
            'redirection' => 0,
            'headers' => $headers,
            'body' => function_exists('wp_json_encode') ? wp_json_encode($body) : json_encode($body),
        ));
    }
}
