<?php

class WP_Error {
    public $code;
    public $message;
    public $data;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; $this->data = $data; }
}
function is_wp_error($value) { return $value instanceof WP_Error; }
function rest_ensure_response($value) { return $value; }
function current_user_can($capability, $id = null) { return true; }
function absint($value) { return abs((int) $value); }
function get_post($id) { return (int) $id === 7 ? $GLOBALS['post'] : null; }
function get_permalink($post) { return 'https://blog.example/post'; }
function wp_strip_all_tags($value) { return strip_tags((string) $value); }
function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
function esc_url($value) { return (string) $value; }
function wp_slash($value) { return addslashes((string) $value); }
function wp_update_post($values, $error = false) {
    $GLOBALS['post']->post_content = stripslashes($values['post_content']);
    return $values['ID'];
}
function get_option($key, $default = false) { return $default; }
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}
class DistributionAPI {
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => 'Uno strumento <utile> & pronto.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => 'publisher_reported'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET') { return array('code' => 200, 'data' => array('placement' => $placement)); }
        if (strpos($path, '/commit') !== false) { return array('code' => 200, 'data' => array('placement' => $placement)); }
        return array('code' => 404, 'data' => array());
    }
}
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }

function parse_blocks($content) {
    $blocks = array();
    $pattern = '/<!--\s+(\/)?wp:([a-z0-9\/-]+)(?:\s+(\{.*?\}))?\s*(\/)?-->/s';
    $offset = 0;
    $length = strlen($content);
    while ($offset < $length && preg_match($pattern, $content, $match, PREG_OFFSET_CAPTURE, $offset)) {
        $at = $match[0][1];
        $before = substr($content, $offset, $at - $offset);
        if (trim($before) !== '') {
            $blocks[] = array('blockName' => null, 'innerHTML' => $before);
        }
        $closer = $match[1][0] === '/';
        $name = $match[2][0];
        $void = isset($match[4][0]) && $match[4][0] === '/';
        $end = $at + strlen($match[0][0]);
        if ($closer || $void) {
            $offset = $end;
            continue;
        }
        $close = '<!-- /wp:' . $name . ' -->';
        $close_at = strpos($content, $close, $end);
        if ($close_at === false) {
            $blocks[] = array('blockName' => null, 'innerHTML' => substr($content, $at));
            return $blocks;
        }
        $block_name = strpos($name, '/') === false ? 'core/' . $name : $name;
        $blocks[] = array('blockName' => $block_name, 'innerHTML' => substr($content, $end, $close_at - $end));
        $offset = $close_at + strlen($close);
    }
    $rest = substr($content, $offset);
    if (trim($rest) !== '') {
        $blocks[] = array('blockName' => null, 'innerHTML' => $rest);
    }
    return $blocks;
}

require __DIR__ . '/../includes/class-affilio-distribution.php';

$original = '<p>Articolo classico già pubblicato.</p>';
$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Strumenti', 'post_content' => $original);
$distribution = new Affilio_Distribution(new DistributionAPI());
$result = $distribution->publish(new Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa')));
check(!is_wp_error($result), 'Publish failed before the block could be checked');
$content = $GLOBALS['post']->post_content;
check(strpos($content, $original) === 0, 'The existing article was rewritten');
$insert = substr($content, strlen($original));
$blocks = parse_blocks($insert);
check(count($blocks) > 0, 'The insert produced no blocks');
foreach ($blocks as $block) {
    check($block['blockName'] === 'core/paragraph', 'The insert parsed as a freeform or classic block');
    check(strpos($block['innerHTML'], '<!-- wp:') === false, 'A block delimiter was left inside the paragraph');
}
$joined = implode('', array_map(function ($block) { return $block['innerHTML']; }, $blocks));
check(strpos($joined, '/v1/go/') !== false, 'The block is missing the tracked /v1/go/ link');
check(strpos($joined, 'rel="sponsored nofollow"') !== false, 'The tracked link is missing rel="sponsored nofollow"');
check(strpos($joined, 'Vedi il prodotto') !== false, 'The product link is not in Italian');
check(strpos($content, 'View product') === false, 'The English product link is still inserted');
check(strpos($joined, '<utile>') === false, 'Recommendation copy was not escaped');
check(strpos($joined, 'Uno strumento &lt;utile&gt; &amp; pronto.') !== false, 'Escaped recommendation copy is missing');

echo "Distribution insert is a Gutenberg paragraph block with an Italian tracked link.\n";
