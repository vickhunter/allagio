<?php

require __DIR__ . '/d033-harness.php';
d033_log_file();
if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.19');
}
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$copy = 'Uno strumento utile.';
$disclosure = 'Affiliate disclosure: I may earn a commission if you buy through this link.';
$box = "<!-- wp:paragraph {\"className\":\"affilio-placement-" . $placement_id . "\"} -->\n<p class=\"affilio-placement-" . $placement_id . "\">" . $copy . " <a href=\"/go/" . $placement_id . "\" rel=\"sponsored nofollow\">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)</p>\n<!-- /wp:paragraph -->";
$post = (object) array('ID' => 2615, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Post', 'post_content' => $box);
$GLOBALS['post'] = $post;
$GLOBALS['posts'] = array($post);
$GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
$GLOBALS['options']['affilio_api_key'] = 'site-key-must-not-ack';
$GLOBALS['http'] = array();
$GLOBALS['revision_counts'] = array();

$distribution = new Affilio_Distribution(new stdClass());
$removed = $distribution->revoke_box(2615, $placement_id);
check($removed === true, 'revoke did not confirm removal');
check(strpos($post->post_content, 'affilio-placement-') === false, 'revoke left the box in the post');
check(strpos($post->post_content, '/go/' . $placement_id) === false, 'revoke left the go link');
$acks = d033_ack_bodies();
check(count($acks) === 1 && $acks[0]['body']['action'] === 'removed', 'revoke ack was not a single removed action');
check($acks[0]['body']['rel_sponsored'] === false, 'removed ack claimed rel_sponsored');
check(strpos(d033_log(), 'affilio plugin ack failed') === false, 'a successful revoke wrote a failure log');

$GLOBALS['http'] = array();
$restored = $distribution->restore_box(2615, $placement_id, array('copy' => $copy, 'disclosure' => $disclosure));
check($restored === true, 'restore did not confirm the box');
check(strpos($post->post_content, '(link sponsorizzato)') !== false, 'restore omitted the sponsor label');
check(Affilio_Distribution::box_in_content($post->post_content, $placement_id), 'restore did not put the box back');
$acks = d033_ack_bodies();
check(count($acks) === 1 && $acks[0]['body']['action'] === 'restored', 'restore ack action was not restored');
check($acks[0]['body']['rel_sponsored'] === true, 'restore ack did not read rel=sponsored from the saved HTML');

foreach ($GLOBALS['revision_counts'] as $kept) {
    check($kept === 0, 'revoke or restore created a revision');
}

$post->post_content = $box;
$GLOBALS['noop_saves'] = 1;
$GLOBALS['http'] = array();
$GLOBALS['updates'] = 0;
$retry = $distribution->revoke_box(2615, $placement_id);
check($retry === true, 'a no-op first save did not succeed on the retry');
check(strpos($post->post_content, 'affilio-placement-') === false, 'retry left the box');
$acks = d033_ack_bodies();
check(count($acks) === 1 && $acks[0]['body']['action'] === 'removed', 'retry sent more than one success ack');
check($GLOBALS['updates'] === 2, 'retry did not write twice');

$post->post_content = $box;
$GLOBALS['noop_saves'] = 5;
$GLOBALS['http'] = array();
$before_log = d033_log();
$failed = $distribution->revoke_box(2615, $placement_id);
check($failed === false, 'a double re-read failure was reported as success');
check(strpos($post->post_content, 'affilio-placement-') !== false, 'failed revoke changed the post despite no-op saves');
$acks = d033_ack_bodies();
check(count($acks) === 1 && $acks[0]['body']['action'] === 'failed', 'failure did not send a single failed ack');
check(isset($acks[0]['body']['reason']) && $acks[0]['body']['reason'] === 're-read failed', 'failed ack omitted the re-read reason');
$log = substr(d033_log(), strlen($before_log));
check(strpos($log, 'affilio plugin ack failed placement_id=' . $placement_id . ' post_id=2615 action=removed') !== false, 'failure log was: ' . $log);
ob_start();
$distribution->render_revert_notice();
$notice = ob_get_clean();
check(strpos($notice, 'Offer Rails could not confirm the box change.') !== false, 'admin notice was missing: ' . $notice);
check(strpos($notice, $placement_id) !== false && strpos($notice, '2615') !== false, 'admin notice omitted the placement or post');

echo "Plugin ack re-read, retry, and failure notice passed.\n";
