<?php

require __DIR__ . '/d033-harness.php';
if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.19');
}
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$copy = 'Uno strumento utile per il team.';
$disclosure = 'Affiliate disclosure: I may earn a commission if you buy through this link.';
$post = (object) array(
    'ID' => 2615,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Strumenti',
    'post_content' => '<p>Articolo da conservare.</p>',
);
$GLOBALS['post'] = $post;
$GLOBALS['posts'] = array($post);

class D033LabelAPI {
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => 'Uno strumento utile per il team.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => 'publisher_reported'),
            'fallback_url' => 'https://shop.example/product',
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET' || strpos($path, '/commit') !== false) {
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        return array('code' => 404, 'data' => array());
    }
}

$distribution = new Affilio_Distribution(new D033LabelAPI());
$result = $distribution->publish(new Request(array('post_id' => 2615, 'placement_id' => $placement_id)));
check(!is_wp_error($result), 'insert failed');
$content = $post->post_content;
check(strpos($content, '(link sponsorizzato)') !== false, 'inserted box is missing (link sponsorizzato)');
check(strpos($content, 'rel="sponsored nofollow"') !== false, 'inserted box is missing rel="sponsored nofollow"');
check(strpos($content, '/go/' . $placement_id) !== false, 'inserted box is missing the local go link');
check(strpos($content, '/v1/go/') === false, 'inserted box stored the legacy /v1/go/ link');

$restored = $distribution->restore_box(2615, $placement_id, array('copy' => $copy, 'disclosure' => $disclosure));
check($restored === true, 'restore did not confirm the labeled box');
check(strpos($post->post_content, '(link sponsorizzato)') !== false, 'the label disappeared before restore');

$reverted = Affilio_Distribution::revert_content($post->post_content, $placement_id);
check(!empty($reverted['matched']), 'classed box did not revert');
check(strpos($reverted['content'], '(link sponsorizzato)') === false, 'revert left the sponsor label');

$stripped = '<p>Prima.</p>' . "\n" . '<!-- wp:paragraph -->' . "\n" . '<p><a href="https://api.example.test/v1/go/test?placement=' . $placement_id . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)</p>' . "\n" . '<!-- /wp:paragraph -->';
$stripped_result = Affilio_Distribution::revert_content($stripped, $placement_id);
check(!empty($stripped_result['matched']) && strpos($stripped_result['content'], 'link sponsorizzato') === false, 'class-stripped /v1/go/?placement= box was not removed');
$local = '<p>Prima.</p>' . "\n" . '<!-- wp:paragraph -->' . "\n" . '<p><a href="/go/' . $placement_id . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)</p>' . "\n" . '<!-- /wp:paragraph -->';
$local_result = Affilio_Distribution::revert_content($local, $placement_id);
check(!empty($local_result['matched']) && $local_result['content'] === '<p>Prima.</p>', 'class-stripped /go/ box was not removed');

$post->post_content = '<p>Articolo da conservare.</p>';
$GLOBALS['revision_counts'] = array();
$again = $distribution->publish(new Request(array('post_id' => 2615, 'placement_id' => $placement_id)));
check(!is_wp_error($again), 'second insert failed');
$removed = $distribution->revert_inserted(2615, $placement_id, false, false);
check(!empty($removed['matched']), 'remove did not match');
$put_back = $distribution->restore_box(2615, $placement_id, array('copy' => $copy, 'disclosure' => $disclosure));
check($put_back === true, 'restore did not put the box back');
check(strpos($post->post_content, '(link sponsorizzato)') !== false, 'restore did not put the labeled box back');
check(count($GLOBALS['revision_counts']) >= 3, 'insert, remove, and restore did not each write');
foreach ($GLOBALS['revision_counts'] as $kept) {
    check($kept === 0, 'a box write kept revisions: ' . var_export($kept, true));
}
check(apply_filters('wp_revisions_to_keep', 12, $post) === 12, 'revision filter was left in place');
$release = file_get_contents(__DIR__ . '/../includes/class-affilio-distribution.php');
check(strpos($release, 'function add_filter') === false && strpos($release, 'function remove_filter') === false, 'filter stubs are shipped in the plugin class');
check(strpos(file_get_contents(__DIR__ . '/wp-filters.php'), 'function add_filter') !== false, 'test filter stubs are missing');

echo "Sponsor label, go-link revert, and revision skip passed.\n";

class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}
