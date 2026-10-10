<?php

require __DIR__ . '/d033-harness.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$post = (object) array(
    'ID' => 7,
    'post_type' => 'post',
    'post_status' => 'publish',
    'post_title' => 'Strumenti',
    'post_content' => '<p>Nota: alcuni link in questa pagina sono affiliati.</p>',
);
$GLOBALS['post'] = $post;
$GLOBALS['posts'] = array($post);
$GLOBALS['receipts'] = array();

class D033ReceiptAPI {
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => 'Uno strumento utile.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'fallback_url' => 'https://shop.example/offer',
            'receipt' => array('tracked_link' => '/v1/go/' . $id, 'publication_status' => 'awaiting_publication'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET' || strpos($path, '/commit') !== false) {
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            $GLOBALS['receipts'][] = $body;
            return array('code' => 200, 'data' => array('status' => 'publisher_reported'));
        }
        return array('code' => 404, 'data' => array());
    }
}

class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}

$distribution = new Affilio_Distribution(new D033ReceiptAPI());
$result = $distribution->publish(new Request(array('post_id' => 7, 'placement_id' => $placement_id)));
check(!is_wp_error($result) && !empty($result['published']), 'publish failed');
check(count($GLOBALS['receipts']) === 1, 'receipt was not posted');
$text = $GLOBALS['receipts'][0]['published_text'];
check(strpos($text, '/go/' . $placement_id) !== false, 'receipt published_text omitted /go/<id>');
check(strpos($text, '(link sponsorizzato)') !== false || strpos($text, 'link sponsorizzato') !== false, 'receipt omitted the sponsor label');
// The Go receipt parser at affilio-api/internal/handlers/placements.go still requires /v1/go/<id>.
// This plugin sends the site-local path and does not change that parser.
check(strpos($text, 'https://blog.example/go/' . $placement_id) !== false, 'receipt did not contain the absolute local go link');

echo "Receipt evidence uses /go/<placement_id>.\n";
