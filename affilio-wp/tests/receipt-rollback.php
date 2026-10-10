<?php

require_once __DIR__ . '/wp-filters.php';

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
function get_post($id) { return (int) $id === 7 ? clone $GLOBALS['post'] : null; }
function get_permalink($post) { return 'https://blog.example/post'; }
function wp_strip_all_tags($value) { return strip_tags($value); }
function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
function esc_url($value) { return $value; }
function wp_slash($value) { return addslashes($value); }
function wp_update_post($values, $error = false) {
    $GLOBALS['updates']++;
    if ($GLOBALS['updates'] > 1 && $GLOBALS['noop_remaining'] > 0) {
        $GLOBALS['noop_remaining']--;
        return $values['ID'];
    }
    $GLOBALS['post']->post_content = stripslashes($values['post_content']);
    return $values['ID'];
}
function get_option($key, $default = false) {
    return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
}
function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; return true; }
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}
class DistributionAPI {
    public $reason = 'receipt refused';
    public function request($method, $path, $body = null) {
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $placement = array(
            'id' => $id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'approved',
            'copy' => 'A useful tool for R&D teams.',
            'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
            'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => 'awaiting_evidence'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET') { return array('code' => 200, 'data' => array('placement' => $placement)); }
        if (strpos($path, '/commit') !== false) {
            $placement['status'] = 'committed';
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            return array('code' => 503, 'data' => array('error' => $this->reason));
        }
        return array('code' => 404, 'data' => array());
    }
}
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }

require __DIR__ . '/../includes/class-affilio-distribution.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$original = '<p>Existing article.</p>';
$log_file = tempnam(sys_get_temp_dir(), 'affilio-rollback');
ini_set('log_errors', '1');
ini_set('error_log', $log_file);

function rollback_log() {
    global $log_file;
    return is_file($log_file) ? (string) file_get_contents($log_file) : '';
}

function notice_html($distribution) {
    ob_start();
    $distribution->render_revert_notice();
    return ob_get_clean();
}

function refused_publish($distribution, $noop_saves) {
    global $original;
    $GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Developer tools', 'post_content' => $original);
    $GLOBALS['updates'] = 0;
    $GLOBALS['noop_remaining'] = $noop_saves;
    $GLOBALS['options'] = array();
    return $distribution->publish(new Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa')));
}

function still_live($content) {
    global $placement_id;
    return strpos($content, 'affilio-placement-' . $placement_id) !== false || strpos($content, 'placement=' . $placement_id) !== false;
}

$api = new DistributionAPI();
$distribution = new Affilio_Distribution($api);

file_put_contents($log_file, '');
$result = refused_publish($distribution, 0);
check(is_wp_error($result), 'A refused receipt was treated as published.');
check($GLOBALS['updates'] === 2, 'A successful rollback did not stop after the first removal (updates ' . $GLOBALS['updates'] . ').');
check($GLOBALS['post']->post_content === $original && !still_live($GLOBALS['post']->post_content), 'A successful rollback left the placement block or go link.');
check(strpos(rollback_log(), 'receipt rollback failed') === false, 'A successful rollback wrote an error log entry.');
check(notice_html($distribution) === '', 'A successful rollback showed an admin notice.');
check(!array_key_exists('affilio_unreceipted_boxes', $GLOBALS['options']), 'A successful rollback stored an admin notice.');

file_put_contents($log_file, '');
$result = refused_publish($distribution, 1);
check(is_wp_error($result), 'A receipt that needed a rollback retry was treated as published.');
check($GLOBALS['updates'] === 3, 'A failed rollback did not retry the removal (updates ' . $GLOBALS['updates'] . ').');
check($GLOBALS['post']->post_content === $original && !still_live($GLOBALS['post']->post_content), 'The rollback retry left the placement block or go link.');
check(strpos(rollback_log(), 'receipt rollback failed') === false, 'A rollback that succeeded on retry wrote an error log entry.');
check(notice_html($distribution) === '', 'A rollback that succeeded on retry showed an admin notice.');

file_put_contents($log_file, '');
$result = refused_publish($distribution, 2);
check(is_wp_error($result), 'A receipt whose rollback failed twice was treated as published.');
check($GLOBALS['updates'] === 3, 'A double rollback failure wrote the post more than once after the retry (updates ' . $GLOBALS['updates'] . ').');
check(still_live($GLOBALS['post']->post_content), 'A double rollback failure removed the live box.');
$log = rollback_log();
check(strpos($log, 'placement_id=' . $placement_id) !== false, 'The error log omitted the placement id: ' . $log);
check(strpos($log, 'post_id=7') !== false, 'The error log omitted the post id: ' . $log);
check(strpos($log, 'reason=' . $api->reason) !== false, 'The error log omitted the refusal reason: ' . $log);
$html = notice_html($distribution);
check(strpos($html, 'The Affilio box is live without a receipt.') !== false, 'The admin notice did not say the box is live without a receipt: ' . $html);
check(strpos($html, $placement_id) !== false && strpos($html, 'post 7') !== false, 'The admin notice omitted the placement or post: ' . $html);
check(strpos($html, 'notice-error') !== false, 'The live-without-receipt warning was not an error notice.');

unlink($log_file);
echo "A refused receipt rolls the box back, retries once, and warns when the box stays live.\n";
