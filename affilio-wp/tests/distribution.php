<?php

class WP_Error {
    public $code;
    public $message;
    public $data;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; $this->data = $data; }
}
function is_wp_error($value) { return $value instanceof WP_Error; }
function rest_ensure_response($value) { return $value; }
function current_user_can($capability, $id = null) { return $GLOBALS['allowed'] && ($id === null || $id === 7); }
function absint($value) { return abs((int) $value); }
function get_post($id) { return $id === 7 ? clone $GLOBALS['post'] : null; }
function get_permalink($post) { return 'https://blog.example/post'; }
function wp_strip_all_tags($value) { return strip_tags($value); }
function esc_html($value) { return htmlspecialchars($value, ENT_QUOTES, 'UTF-8'); }
function esc_url($value) { return $value; }
function wp_slash($value) { return addslashes($value); }
function wp_update_post($values, $error = false) { $GLOBALS['post']->post_content = stripslashes($values['post_content']); $GLOBALS['updates']++; return $values['ID']; }
function get_option($key, $default = false) { return $default; }
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}
class DistributionAPI {
    public $status = 'pending';
    public $receipt_failed = false;
    public $recorded = false;
    public $receipt = null;
    public $commits = 0;
    public $surface = 'https://blog.example/post';
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array('id' => $id, 'surface_url' => $this->surface, 'status' => $this->status, 'copy' => 'A useful tool for R&D teams.', 'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.', 'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => $this->recorded ? 'publisher_reported' : 'awaiting_evidence'));
        if ($method === 'GET') { return array('code' => 200, 'data' => array('placement' => $placement)); }
        if (strpos($path, '/commit') !== false) {
            $this->commits++;
            if (!in_array($this->status, array('approved', 'committed'), true)) { return array('code' => 403, 'data' => array('error' => 'Not approved.')); }
            $this->status = 'committed';
            $placement['status'] = 'committed';
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            $this->receipt = $body;
            if ($this->receipt_failed) { return array('code' => 503, 'data' => array('error' => 'Retry.')); }
            $this->recorded = true;
            return array('code' => 200, 'data' => array('status' => 'publisher_reported'));
        }
        return array('code' => 404, 'data' => array());
    }
}
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }
require __DIR__ . '/../includes/class-affilio-distribution.php';
$GLOBALS['allowed'] = true;
$existing = str_repeat('Existing content. ', 2000) . '<p>Use C:\\tools and the "quoted" example.</p>';
$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Developer tools', 'post_content' => $existing);
$GLOBALS['updates'] = 0;
$api = new DistributionAPI();
$distribution = new Affilio_Distribution($api);
$request = new Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));
check(is_wp_error($distribution->publish($request)) && $GLOBALS['updates'] === 0, 'Pending placement was inserted.');
$api->status = 'approved';
$api->surface = 'https://blog.example/another-post';
check(is_wp_error($distribution->publish($request)) && $GLOBALS['updates'] === 0, 'Placement inserted in wrong article.');
$api->surface = 'https://blog.example/post';
$api->receipt_failed = true;
check(is_wp_error($distribution->publish($request)) && $GLOBALS['updates'] === 1, 'Receipt failure lost insertion state.');
$api->receipt_failed = false;
$result = $distribution->publish($request);
check(!is_wp_error($result) && $result['published'] && $GLOBALS['updates'] === 1, 'Retry inserted twice.');
check(substr_count($GLOBALS['post']->post_content, '<!-- distributoor-placement:') === 1, 'Duplicate placement marker.');
check(substr_count($GLOBALS['post']->post_content, 'Affiliate disclosure:') === 1, 'Duplicate disclosure.');
check(strpos($GLOBALS['post']->post_content, $existing) === 0, 'Existing article content was modified.');
check(strlen($api->receipt['published_text']) < 16000 && strpos($api->receipt['published_text'], 'R&D') !== false && strpos($api->receipt['published_text'], '/v1/go/test?placement=') !== false, 'Published evidence did not preserve exact approved text/link.');
$GLOBALS['allowed'] = false;
check(!$distribution->allowed() && is_wp_error($distribution->publish($request)), 'Unauthorized user published.');
$GLOBALS['allowed'] = true;
$GLOBALS['post']->post_status = 'draft';
check(is_wp_error($distribution->publish($request)), 'Draft article published.');
echo "WordPress distribution: ownership, approval, publication evidence and idempotent retries passed.\n";
