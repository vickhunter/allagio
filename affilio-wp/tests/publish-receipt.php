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
    return $values['ID'];
}
function get_option($key, $default = false) {
    return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
}
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$copy = 'Uno strumento utile per il team.';
$disclosure = 'Affiliate disclosure: I may earn a commission if you buy through this link.';
$tracked = '/v1/go/test?placement=' . $placement_id;

function affilio_receipt_code($published, $copy, $disclosure, $tracked) {
    $plain = strtolower(strip_tags((string) $published));
    $has_disclosure = strpos((string) $published, (string) $disclosure) !== false
        || preg_match('/\blink\b.{0,60}affiliat/iu', $plain)
        || strpos($plain, 'commissione') !== false;
    if (strpos((string) $published, (string) $copy) !== false && $has_disclosure && strpos((string) $published, (string) $tracked) !== false) {
        return 200;
    }
    return 409;
}

class ReceiptAPI {
    public function request($method, $path, $body = null) {
        global $placement_id, $copy, $disclosure, $tracked;
        $placement = array(
            'id' => $placement_id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => $copy,
            'disclosure' => $disclosure,
            'receipt' => array('tracked_link' => $tracked, 'publication_status' => 'awaiting_publication'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET' || strpos($path, '/commit') !== false) {
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            $GLOBALS['last_published'] = isset($body['published_text']) ? $body['published_text'] : '';
            $code = !empty($GLOBALS['force_receipt']) ? (int) $GLOBALS['force_receipt'] : affilio_receipt_code($GLOBALS['last_published'], $copy, $disclosure, $tracked);
            if ($code !== 200) {
                return array('code' => $code, 'data' => array('error' => 'published text must include approved copy, disclosure and exact tracked link'));
            }
            $GLOBALS['receipt_recorded'] = true;
            return array('code' => 200, 'data' => array('status' => 'publisher_reported'));
        }
        return array('code' => 404, 'data' => array());
    }
}

function check($condition, $message) {
    if (!$condition) {
        fwrite(STDERR, $message . "\n");
        exit(1);
    }
}

require __DIR__ . '/../includes/class-affilio-distribution.php';

$GLOBALS['allowed'] = true;
$GLOBALS['options'] = array();
$GLOBALS['force_receipt'] = 0;
$GLOBALS['receipt_recorded'] = false;
$fixture = 'Nota: alcuni link in questa pagina sono affiliati. Se acquisti tramite questi link, posso ricevere una commissione, senza costi extra per te.';
check(affilio_receipt_code($copy . "\n" . $tracked, $copy, $disclosure, $tracked) === 409, 'copy and tracked link with no disclosure were accepted');
check(affilio_receipt_code($fixture . "\n" . $copy, $copy, $disclosure, $tracked) === 409, 'a disclosure without the tracked link was accepted');
check(affilio_receipt_code('<p><em>' . $fixture . '</em></p>' . "\n" . $copy . "\n" . $tracked, $copy, $disclosure, $tracked) === 200, 'the fixture disclosure was rejected by the receipt contract');

$distribution = new Affilio_Distribution(new ReceiptAPI());
$request = new Request(array('post_id' => 7, 'placement_id' => $placement_id));

$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Strumenti', 'post_content' => '<p><em>' . $fixture . '</em></p>');
$result = $distribution->publish($request);
check(!is_wp_error($result) && !empty($result['published']), 'a post that already discloses was not published: ' . (is_wp_error($result) ? $result->get_error_message() . ' (' . $result->data['status'] . ')' : 'not published'));
check(!empty($GLOBALS['receipt_recorded']), 'the receipt was not recorded for a post that already discloses');
check(strpos($GLOBALS['last_published'], $fixture) !== false, 'the receipt omitted the post disclosure');
check(strpos($GLOBALS['post']->post_content, 'affiliate-disclosure') === false, 'the plugin added a second disclosure');

$GLOBALS['receipt_recorded'] = false;
$GLOBALS['options']['affilio_existing_disclosure'] = '1';
$GLOBALS['post']->post_content = '<p>Un articolo senza avvisi.</p>';
$result = $distribution->publish($request);
$decision = affilio_receipt_code($GLOBALS['last_published'], $copy, $disclosure, $tracked);
if ($decision === 409) {
    $status = is_wp_error($result) && isset($result->data['status']) ? (int) $result->data['status'] : 0;
    check($status === 409, 'a post with the setting and no disclosure failed with ' . $status . ' instead of 409');
    check(empty($GLOBALS['receipt_recorded']), 'a rejected receipt marked the placement published');
    fwrite(STDERR, "the setting skipped a disclosure that is not in the post\n");
    exit(1);
}
check(!is_wp_error($result) && !empty($result['published']), 'publish did not record a post that needed its own disclosure');
check(!empty($GLOBALS['receipt_recorded']), 'receipt was not recorded after the plugin added the missing disclosure');
check(strpos($GLOBALS['post']->post_content, 'affiliate-disclosure') !== false, 'the setting skipped the disclosure');

$GLOBALS['force_receipt'] = 409;
$GLOBALS['receipt_recorded'] = false;
$GLOBALS['options']['affilio_existing_disclosure'] = '';
$GLOBALS['post']->post_content = '<p>Un altro articolo senza avvisi.</p>';
$result = $distribution->publish($request);
$status = is_wp_error($result) && isset($result->data['status']) ? (int) $result->data['status'] : 0;
check(is_wp_error($result) && $status === 409, 'a rejected receipt did not fail publish with 409 (status ' . $status . ')');
check(empty($GLOBALS['receipt_recorded']), 'a rejected receipt marked the placement published');
check(empty($result['published']), 'publish reported the placement published after a 409');

echo "A post disclosure satisfies the receipt, and a setting cannot publish a post that has none.\n";
