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
function wp_update_post($values, $error = false) { $GLOBALS['updates']++; $GLOBALS['post']->post_content = stripslashes($values['post_content']); return $values['ID']; }
function get_option($key, $default = false) { return $default; }
if (!function_exists('mb_substr')) {
    function mb_substr($string, $start, $length = null, $encoding = null) { return substr($string, $start, $length); }
}
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}
class CountingAPI {
    public $connected = false;
    public $connection_reads = 0;
    public $downstream = 0;
    public function request($method, $path, $body = null) {
        if (strpos($path, '/sites/owner-connection') !== false) {
            $this->connection_reads++;
            return array('code' => 200, 'data' => array('connected' => $this->connected));
        }
        $this->downstream++;
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        return array('code' => 200, 'data' => array(
            'status' => 'no_match',
            'opportunities' => array(),
            'placement' => array(
                'id' => $id,
                'surface_url' => 'https://blog.example/post',
                'status' => 'committed',
                'copy' => 'Uno strumento utile.',
                'disclosure' => 'Affiliate disclosure: I may earn a commission if you buy through this link.',
                'receipt' => array('tracked_link' => '/v1/go/test?placement=' . $id, 'publication_status' => 'publisher_reported'),
            ),
        ));
    }
}
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }
function refused($result) {
    return is_wp_error($result)
        && $result->get_error_message() === 'Collega prima il tuo account publisher.'
        && isset($result->data['status'])
        && (int) $result->data['status'] === 403;
}

require __DIR__ . '/../includes/class-affilio-distribution.php';

$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Strumenti', 'post_content' => '<p>Un articolo.</p>');
$GLOBALS['updates'] = 0;
$api = new CountingAPI();
$distribution = new Affilio_Distribution($api);
$article = new Request(array('post_id' => 7, 'offer_id' => 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'copy' => 'Valuta questo strumento per il problema dell articolo.'));
$publish = new Request(array('post_id' => 7, 'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));

check(refused($distribution->opportunities($article)), 'Opportunities did not refuse an unlinked publisher');
check(refused($distribution->prepare($article)), 'Prepare did not refuse an unlinked publisher');
check(refused($distribution->publish($publish)), 'Publish did not refuse an unlinked publisher');
check($api->downstream === 0 && $GLOBALS['updates'] === 0, 'An unlinked publisher still reached distribution or the article');
check($api->connection_reads === 1, 'The publisher connection was not cached for the request');

$api->connected = true;
$again = new Affilio_Distribution($api);
$result = $again->opportunities($article);
check(!is_wp_error($result) && isset($result['status']), 'A linked publisher was still refused');
check($again->opportunities($article) && $api->connection_reads === 2, 'A new request did not check the publisher connection again');

echo "Distribution refuses opportunities, prepare, and publish until the publisher account is connected.\n";
