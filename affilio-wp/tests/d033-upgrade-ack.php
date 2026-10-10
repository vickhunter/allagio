<?php

require __DIR__ . '/d033-harness.php';
if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.19');
}
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-go.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$post = (object) array(
    'ID' => 2615,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Existing',
    'post_content' => '<p class="affilio-placement-' . $placement_id . '">Box <a href="/go/' . $placement_id . '" rel="sponsored nofollow">Vedi</a></p>',
);
$GLOBALS['posts'] = array($post);
$GLOBALS['post'] = $post;
$GLOBALS['options'] = array(
    'affilio_seen_version' => '0.1.18',
    'affilio_api_key' => 'site-key-must-not-ack',
    'affilio_plugin_key' => 'plugin-only-key',
);
$GLOBALS['http'] = array();
$GLOBALS['updates'] = 0;
$GLOBALS['flushes'] = 0;

$distribution = new Affilio_Distribution(new stdClass());
$distribution->maybe_upgrade();
$acks = d033_ack_bodies();
check(count($acks) === 1, 'upgrade did not ack the existing box once');
check($acks[0]['body']['action'] === 'placed', 'upgrade ack action was not placed');
check($acks[0]['headers']['X-Plugin-Key'] === 'plugin-only-key', 'upgrade ack did not use the plugin key');
check(!isset($acks[0]['headers']['X-API-Key']), 'upgrade ack sent the site key header');
check(strpos($acks[0]['raw'], 'site-key-must-not-ack') === false, 'upgrade ack body contained the site key');
check(strpos($acks[0]['url'], 'backfill') === false, 'upgrade called a backfill URL');
foreach ($GLOBALS['http'] as $call) {
    check(strpos($call['url'], 'backfill') === false, 'upgrade requested ' . $call['url']);
}
check($GLOBALS['updates'] === 0, 'upgrade rewrote the post');
check($post->post_content === '<p class="affilio-placement-' . $placement_id . '">Box <a href="/go/' . $placement_id . '" rel="sponsored nofollow">Vedi</a></p>', 'upgrade changed saved HTML');
check(get_option('affilio_seen_version') === '0.1.19', 'seen version was not stored');
check($GLOBALS['flushes'] === 1, 'upgrade did not flush rewrites');

$GLOBALS['options'] = array();
$GLOBALS['posts'] = array();
$GLOBALS['http'] = array();
$distribution->maybe_upgrade();
check(d033_ack_bodies() === array(), 'a fresh install with no key and no boxes sent an ack');
check(get_option('affilio_seen_version') === '0.1.19', 'fresh install did not record the version');

echo "Upgrade acks existing boxes with the plugin key and does not rewrite them.\n";
