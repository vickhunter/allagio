<?php

class WP_Error {
    public $code;
    public $message;
    public $data;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; $this->data = $data; }
    public function get_error_message() { return $this->message; }
}
function is_wp_error($value) { return $value instanceof WP_Error; }
function rest_ensure_response($value) { return $value; }
function current_user_can($capability, $id = null) { return !empty($GLOBALS['allowed']); }
function absint($value) { return abs((int) $value); }
function get_post($id) { return (int) $id === 7 ? $GLOBALS['post'] : null; }
function get_permalink($post) { return 'https://blog.example/post'; }
function wp_strip_all_tags($value) { return strip_tags((string) $value); }
function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
function esc_url($value) { return (string) $value; }
function wp_slash($value) { return addslashes((string) $value); }
function wp_update_post($values, $error = false) {
    $GLOBALS['post']->post_content = stripslashes($values['post_content']);
    $GLOBALS['updates']++;
    return $values['ID'];
}
function sanitize_key($value) { return strtolower((string) $value); }
function get_option($key, $default = false) {
    return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
}
function get_post_meta($id, $key, $single = false) {
    return isset($GLOBALS['meta'][$key]) ? $GLOBALS['meta'][$key] : '';
}
function update_post_meta($id, $key, $value) { $GLOBALS['meta'][$key] = $value; return true; }
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}
class DistributionAPI {
    public $status = 'approved';
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => 'Uno strumento utile per il team.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => 'publisher_reported'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET') { return array('code' => 200, 'data' => array('placement' => $placement)); }
        if (strpos($path, '/commit') !== false) { return array('code' => 200, 'data' => array('placement' => $placement)); }
        if (strpos($path, '/receipt') !== false) { return array('code' => 200, 'data' => array('status' => 'publisher_reported')); }
        return array('code' => 404, 'data' => array());
    }
}
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }
function added($before, $after, $needle) { return substr_count($after, $needle) - substr_count($before, $needle); }

require_once __DIR__ . '/wp-filters.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';
require __DIR__ . '/../includes/class-affilio-inserter.php';

$GLOBALS['allowed'] = true;
$GLOBALS['options'] = array();
$GLOBALS['meta'] = array();
$GLOBALS['updates'] = 0;
$italian = '<p>Questo articolo contiene link di affiliazione. Potrei ricevere una commissione.</p>';
$plain = '<p>Un articolo senza avvisi.</p>';
$api = new DistributionAPI();
$distribution = new Affilio_Distribution($api);
$request = new Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));
$inserter = new Affilio_Inserter($api);

$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Strumenti', 'post_content' => $italian);
$before = $GLOBALS['post']->post_content;
$distribution->publish($request);
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure-') === 0, 'Italian disclosure still received a second disclosure from distribution');
check(strpos($GLOBALS['post']->post_content, 'contiene link di affiliazione') !== false, 'Existing Italian disclosure was removed');

$GLOBALS['post']->post_content = $italian;
$GLOBALS['meta'] = array();
$before = $GLOBALS['post']->post_content;
$inserter->insert_inline_mention(7, '<p>Una menzione.</p>');
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure') === 0, 'Italian disclosure still received a second disclosure from the inserter');

$fixture = 'Nota: alcuni link in questa pagina sono affiliati. Se acquisti tramite questi link, posso ricevere una commissione, senza costi extra per te.';
$verbatim = '<p><em>' . $fixture . '</em></p>';
$reworded = '<p>Alcuni dei link qui sotto sono link affiliati.</p>';
$separated = '<p><em>Nota: alcuni link in questa pagina sono affiliati.</em></p>';

$GLOBALS['options']['affilio_existing_disclosure'] = '';
$GLOBALS['post']->post_content = $verbatim;
$before = $GLOBALS['post']->post_content;
$distribution->publish($request);
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure-') === 0, 'The blog disclosure fixture still received a second disclosure from distribution');
check(strpos($GLOBALS['post']->post_content, $fixture) !== false, 'The blog disclosure fixture was removed');

$GLOBALS['post']->post_content = $verbatim;
$GLOBALS['meta'] = array();
$before = $GLOBALS['post']->post_content;
$inserter->insert_inline_mention(7, '<p>Una menzione.</p>');
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure') === 0, 'The blog disclosure fixture still received a second disclosure from the inserter');

$GLOBALS['post']->post_content = $reworded;
$GLOBALS['meta'] = array();
$before = $GLOBALS['post']->post_content;
$distribution->publish($request);
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure-') === 0, 'A reworded affiliate-link sentence still received a second disclosure');

$GLOBALS['post']->post_content = $separated;
$GLOBALS['meta'] = array();
$before = $GLOBALS['post']->post_content;
$distribution->publish($request);
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure-') === 0, 'link and affiliat within 60 characters, inside tags, still received a second disclosure');

$GLOBALS['post']->post_content = $plain;
$GLOBALS['options']['affilio_existing_disclosure'] = '';
$before = $GLOBALS['post']->post_content;
$distribution->publish($request);
check(added($before, $GLOBALS['post']->post_content, '<!-- wp:paragraph {"className":"affilio-disclosure-') === 1, 'A post without a disclosure did not get exactly one');

$GLOBALS['post']->post_content = $plain;
$GLOBALS['meta'] = array();
$before = $GLOBALS['post']->post_content;
$inserter->insert_inline_mention(7, '<p>Una menzione.</p>');
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure') === 1, 'Inserter did not add exactly one disclosure');

$GLOBALS['post']->post_content = $plain;
$GLOBALS['options']['affilio_existing_disclosure'] = '1';
$before = $GLOBALS['post']->post_content;
$distribution->publish($request);
check(added($before, $GLOBALS['post']->post_content, '<!-- wp:paragraph {"className":"affilio-disclosure-') === 1, 'The existing-disclosure setting skipped a post that does not disclose');

$GLOBALS['post']->post_content = $plain;
$GLOBALS['meta'] = array();
$before = $GLOBALS['post']->post_content;
$inserter->insert_inline_mention(7, '<p>Una menzione.</p>');
check(added($before, $GLOBALS['post']->post_content, 'affilio-disclosure') === 1, 'The existing-disclosure setting skipped the inserter on a post that does not disclose');

echo "Disclosure is added once, skipped only when the post text discloses, and still added when the setting is on but the post does not.\n";
