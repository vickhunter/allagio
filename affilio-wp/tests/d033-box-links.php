<?php

require __DIR__ . '/d033-harness.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$post = (object) array(
    'ID' => 7,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Strumenti',
    'post_content' => '<p>Articolo.</p>',
);
$GLOBALS['post'] = $post;
$GLOBALS['posts'] = array($post);

class D033BoxAPI {
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => 'Uno strumento.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'receipt' => array('tracked_link' => '/v1/go/' . $id, 'publication_status' => 'publisher_reported'),
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

class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}

check(Affilio_Distribution::tracked_link_ok('/v1/go/' . $placement_id, $placement_id), 'API /v1/go/<id> was rejected');
check(Affilio_Distribution::tracked_link_ok('/v1/go/test?placement=' . $placement_id, $placement_id), 'API placement query was rejected');
check(Affilio_Distribution::tracked_link_ok('https://blog.example/go/' . $placement_id, $placement_id), 'local /go/<id> was rejected');

$distribution = new Affilio_Distribution(new D033BoxAPI());
$result = $distribution->publish(new Request(array('post_id' => 7, 'placement_id' => $placement_id)));
check(!is_wp_error($result), 'publish failed');
check(strpos($post->post_content, 'href="https://blog.example/go/' . $placement_id . '"') !== false, 'box href was not the site-local /go/ path: ' . $post->post_content);
check(strpos($post->post_content, 'rel="sponsored nofollow"') !== false, 'box link is missing rel="sponsored nofollow"');
check(strpos($post->post_content, '/v1/go/') === false, 'box stored /v1/go/');

echo "Box links use /go/<placement_id> with rel=sponsored nofollow.\n";
