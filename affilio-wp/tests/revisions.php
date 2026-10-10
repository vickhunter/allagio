<?php

require_once __DIR__ . '/wp-filters.php';

$GLOBALS['filters'] = array();
$GLOBALS['affilio_filter_hooks'] = array();
$GLOBALS['revision_counts'] = array();
$GLOBALS['options'] = array();

function apply_filters($name, $value) {
    $passed = affilio_test_run_filters($name, array_slice(func_get_args(), 1));
    return $passed[0];
}

class WP_Error {
    public $code;
    public $message;
    public $data;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; $this->data = $data; }
    public function get_error_message() { return $this->message; }
}
function is_wp_error($value) { return $value instanceof WP_Error; }
function rest_ensure_response($value) { return $value; }
function current_user_can($capability, $id = null) { return true; }
function absint($value) { return abs((int) $value); }
function get_post($id) { return $id === 7 ? clone $GLOBALS['post'] : null; }
function get_permalink($post) { return 'https://blog.example/post'; }
function wp_strip_all_tags($value) { return strip_tags($value); }
function esc_html($value) { return htmlspecialchars($value, ENT_QUOTES, 'UTF-8'); }
function esc_url($value) { return $value; }
function wp_slash($value) { return addslashes($value); }
function get_option($key, $default = false) { return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default; }
function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; return true; }
function wp_update_post($values, $error = false) {
    $post = (object) array('ID' => $values['ID']);
    $kept = apply_filters('wp_revisions_to_keep', 12, $post);
    $GLOBALS['revision_counts'][] = $kept;
    $GLOBALS['post']->post_content = stripslashes($values['post_content']);
    return $values['ID'];
}
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
            'status' => $this->status,
            'copy' => 'A useful tool for R&D teams.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => 'awaiting_evidence'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET') { return array('code' => 200, 'data' => array('placement' => $placement)); }
        if (strpos($path, '/commit') !== false) {
            $this->status = 'committed';
            $placement['status'] = 'committed';
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            return array('code' => 200, 'data' => array('status' => 'publisher_reported'));
        }
        if (strpos($path, '/removed') !== false) {
            return array('code' => 200, 'data' => array('status' => 'removed'));
        }
        return array('code' => 404, 'data' => array());
    }
}
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }

require __DIR__ . '/../includes/class-affilio-distribution.php';

$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Developer tools', 'post_content' => '<p>Existing article.</p>');
$distribution = new Affilio_Distribution(new DistributionAPI());
$request = new Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));
$result = $distribution->publish($request);
check(!is_wp_error($result) && !empty($result['published']), 'Publish failed.');
$reverted = $distribution->revert_inserted(7, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa');
check(!empty($reverted['matched']), 'Remove did not match the inserted block.');
check(count($GLOBALS['revision_counts']) >= 2, 'Insert and remove did not both write the post.');
foreach ($GLOBALS['revision_counts'] as $kept) {
    check($kept === 0, 'Plugin write kept revisions: ' . var_export($kept, true));
}
check(apply_filters('wp_revisions_to_keep', 12, $GLOBALS['post']) === 12, 'Revision filter was left in place.');
$added = array();
$removed = array();
foreach ($GLOBALS['affilio_filter_hooks'] as $hook) {
    if ($hook['name'] !== 'wp_revisions_to_keep') {
        continue;
    }
    if ($hook['op'] === 'add') {
        $added[] = $hook['priority'];
    }
    if ($hook['op'] === 'remove') {
        $removed[] = $hook['priority'];
    }
}
check(count($added) >= 2 && count($removed) === count($added), 'Insert and remove did not both hook wp_revisions_to_keep.');
foreach ($added as $priority) {
    check($priority === PHP_INT_MAX, 'wp_revisions_to_keep was hooked at ' . var_export($priority, true) . ' instead of PHP_INT_MAX.');
}
foreach ($removed as $priority) {
    check($priority === PHP_INT_MAX, 'wp_revisions_to_keep was removed at ' . var_export($priority, true) . ' instead of PHP_INT_MAX.');
}
$release = file_get_contents(__DIR__ . '/../includes/class-affilio-distribution.php');
check(strpos($release, 'function add_filter') === false && strpos($release, 'function remove_filter') === false, 'Filter stubs are still shipped in the plugin.');
echo "Plugin block insert and remove do not create post revisions.\n";
