<?php

require __DIR__ . '/d033-harness.php';
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';

$GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
$GLOBALS['options']['affilio_api_key'] = 'site-key-must-not-ack';
$GLOBALS['http'] = array();

$ack = new Affilio_Plugin_Ack();
$result = $ack->send('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', AFFILIO_PLUGIN_ACK_PLACED, true, '');
check(!is_wp_error($result), 'ack transport failed');
check(count($GLOBALS['http']) === 1, 'ack did not send exactly one request');
$call = $GLOBALS['http'][0];
check($call['url'] === 'https://api.example.test/v1/placements/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/plugin-ack', 'ack URL was ' . $call['url']);
check($call['args']['method'] === 'POST', 'ack was not POST');
check((int) $call['args']['redirection'] === 0, 'ack followed redirects');
$headers = $call['args']['headers'];
check(isset($headers['X-Plugin-Key']) && $headers['X-Plugin-Key'] === 'plugin-only-key', 'ack did not send the plugin-only key');
check(!isset($headers['X-API-Key']), 'ack sent X-API-Key');
$encoded = json_encode($headers) . (string) $call['args']['body'];
check(strpos($encoded, 'site-key-must-not-ack') === false, 'ack used the site key');
$body = json_decode($call['args']['body'], true);
check($body['action'] === 'placed' && $body['rel_sponsored'] === true && !isset($body['reason']), 'placed ack body was ' . $call['args']['body']);

$GLOBALS['options']['affilio_plugin_key'] = '';
$GLOBALS['http'] = array();
$ack->send('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', AFFILIO_PLUGIN_ACK_FAILED, false, '');
$empty = $GLOBALS['http'][0];
check($empty['args']['headers']['X-Plugin-Key'] === '', 'an empty plugin key fell back to another credential');
check(!isset($empty['args']['headers']['X-API-Key']), 'empty plugin key still sent the site key');
check(strpos((string) $empty['args']['body'], 'site-key-must-not-ack') === false, 'failed ack body contained the site key');

echo "Acks use only the plugin key.\n";
