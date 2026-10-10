<?php

require __DIR__ . '/d033-harness.php';
if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.19');
}
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$disclosure = '<p>This sponsored article mentions a sponsored offer in the disclosure text.</p>';
$plain = $disclosure . '<p><a href="/go/' . $placement_id . '" rel="nofollow">Vedi</a></p>';
$marked = $disclosure . '<p><a href="https://blog.example/go/' . $placement_id . '" rel="sponsored nofollow">Vedi</a></p>';
$post = (object) array('ID' => 2615, 'post_type' => 'post', 'post_status' => 'publish', 'post_content' => $plain);
$GLOBALS['post'] = $post;
$GLOBALS['posts'] = array($post);
$GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
$GLOBALS['http'] = array();

check(Affilio_Distribution::rel_sponsored_from_content($plain, $placement_id) === false, 'disclosure text was treated as rel=sponsored');
check(Affilio_Distribution::rel_sponsored_from_content($marked, $placement_id) === true, 'sponsored rel token was missed');

$distribution = new Affilio_Distribution(new stdClass());
$first = $distribution->restore_box(2615, $placement_id, array('copy' => 'Copy', 'disclosure' => 'Disclosure'));
check($first === true, 'restore of an existing nofollow link failed');
$acks = d033_ack_bodies();
check(count($acks) === 1 && $acks[0]['body']['rel_sponsored'] === false, 'ack inferred rel_sponsored from disclosure text');
check($acks[0]['body']['action'] === 'restored', 'rel check did not use the restore ack');

$post->post_content = $marked;
$GLOBALS['http'] = array();
$second = $distribution->restore_box(2615, $placement_id, array('copy' => 'Copy', 'disclosure' => 'Disclosure'));
check($second === true, 'restore of a sponsored rel failed');
$acks = d033_ack_bodies();
check(count($acks) === 1 && $acks[0]['body']['rel_sponsored'] === true, 'ack did not read rel=sponsored from the re-read anchor');
check($post->post_content === $marked, 'rel check rewrote the post');

echo "rel_sponsored is read from the re-read anchor, not the disclosure text.\n";
