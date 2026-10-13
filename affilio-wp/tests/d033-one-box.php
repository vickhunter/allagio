<?php

require __DIR__ . '/d033-harness.php';

if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.21');
}
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-go.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

if (!class_exists('OneBoxRequest')) {
    class OneBoxRequest {
        private $values;
        public function __construct($values) { $this->values = $values; }
        public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
    }
}

if (!class_exists('OneBoxAPI')) {
    class OneBoxAPI {
        public $commit_status = 200;
        public function request($method, $path, $body = null) {
            $GLOBALS['api_calls'][] = $method . ' ' . $path;
            $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
            $placement = array(
                'id' => $id,
                'surface_url' => 'https://blog.example/post',
                'status' => 'committed',
                'copy' => 'Uno strumento utile.',
                'disclosure' => 'Affiliate disclosure.',
                'fallback_url' => 'https://shop.example/plain',
                'receipt' => array('tracked_link' => '/v1/go/' . $id, 'publication_status' => 'awaiting_publication'),
            );
            if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
                return array('code' => 200, 'data' => array('connected' => true));
            }
            if ($method === 'GET') {
                return array('code' => 200, 'data' => array('placement' => $placement, 'surface_url' => $placement['surface_url']));
            }
            if (strpos($path, '/commit') !== false) {
                if ((int) $this->commit_status === 409) {
                    return array(
                        'code' => 409,
                        'data' => array('reason' => 'one_per_post', 'message' => 'Keep one box on this post.'),
                    );
                }
                return array('code' => 200, 'data' => array('placement' => $placement));
            }
            if (strpos($path, '/receipt') !== false) {
                return array('code' => 200, 'data' => array('ok' => true));
            }
            return array('code' => 404, 'data' => array());
        }
    }
}

function affilio_one_box_reset($content) {
    $post = (object) array(
        'ID' => 2615,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Articolo',
        'post_content' => $content,
    );
    $GLOBALS['post'] = $post;
    $GLOBALS['posts'] = array($post);
    $GLOBALS['options'] = array(
        'affilio_plugin_key' => 'plugin-only-key',
        'affilio_api_key' => 'site-key-must-not-ack',
    );
    $GLOBALS['http'] = array();
    $GLOBALS['api_calls'] = array();
    $GLOBALS['updates'] = 0;
    unset($GLOBALS['http_handler'], $GLOBALS['http_response']);
    return $post;
}

function affilio_one_box_ack($code, $body) {
    $GLOBALS['http_handler'] = function ($url, $args) use ($code, $body) {
        return array('response' => array('code' => $code), 'body' => $body, 'headers' => array());
    };
}

function affilio_one_box_warnings() {
    $GLOBALS['one_box_warnings'] = array();
    set_error_handler(function ($errno, $errstr) {
        $GLOBALS['one_box_warnings'][] = $errstr;
        return true;
    });
}

function affilio_one_box_warning_text() {
    restore_error_handler();
    return implode("\n", $GLOBALS['one_box_warnings']);
}

function affilio_review_commit_one_per_post_does_not_insert() {
    $original = '<p>Articolo gia pubblicato.</p>';
    $post = affilio_one_box_reset($original);
    $api = new OneBoxAPI();
    $api->commit_status = 409;
    $distribution = new Affilio_Distribution($api);
    affilio_one_box_warnings();
    $result = $distribution->publish(new OneBoxRequest(array(
        'post_id' => 2615,
        'placement_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    )));
    $warnings = affilio_one_box_warning_text();
    check(strpos($warnings, 'Undefined') === false, 'commit refusal warned: ' . $warnings);
    check(is_wp_error($result), 'a second box commit was treated as published');
    $data = $result->get_error_data();
    check(is_array($data) && (int) $data['status'] === 409, 'one_per_post commit status was ' . json_encode($data));
    check(isset($data['reason']) && $data['reason'] === 'one_per_post', 'one_per_post commit dropped the reason');
    check($result->get_error_message() === 'Keep one box on this post.', 'commit refusal hid the server sentence: ' . $result->get_error_message());
    check($post->post_content === $original, 'a refused second box was inserted');
    check($GLOBALS['updates'] === 0, 'a refused second box wrote the post');
    foreach ($GLOBALS['api_calls'] as $call) {
        check(strpos($call, '/receipt') === false, 'a refused second box recorded a receipt');
    }
}

function affilio_review_ack_one_per_post_removes_the_new_box() {
    $original = '<p>Articolo senza riquadro.</p>';
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    $post = affilio_one_box_reset($original);
    affilio_one_box_ack(422, '{"check_id":"one_per_post","check_text":"Keep one box on this post."}');
    $distribution = new Affilio_Distribution(new OneBoxAPI());
    affilio_one_box_warnings();
    $result = $distribution->publish(new OneBoxRequest(array('post_id' => 2615, 'placement_id' => $id)));
    $warnings = affilio_one_box_warning_text();
    check(strpos($warnings, 'Undefined') === false && strpos($warnings, 'WP_Error::$data') === false, 'ack refusal warned: ' . $warnings);
    check(is_wp_error($result), 'a 422 one_per_post acknowledgement was treated as published');
    $data = $result->get_error_data();
    check(is_array($data) && (int) $data['status'] === 422 && $data['reason'] === 'one_per_post', 'ack refusal data was ' . json_encode($data));
    check($result->get_error_message() === 'Keep one box on this post.', 'ack refusal hid check_text: ' . $result->get_error_message());
    check(strpos($post->post_content, $id) === false, 'the new second box stayed in the post');
    check(strpos($post->post_content, 'Articolo senza riquadro') !== false, 'rollback removed the article');
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    check(!is_array($queue) || !isset($queue[$id . '|placed']), 'a 422 one_per_post acknowledgement was queued');
    $index = get_option('affilio_box_index', array());
    check(isset($index[$id]['phase']) && $index[$id]['phase'] === 'refused', 'a refused new box was marked placed');
    $notices = get_option('affilio_plugin_ack_notices', array());
    $notice_text = isset($notices[0]['message']) ? $notices[0]['message'] : '';
    check($notice_text === 'Keep one box on this post.', 'admin notice was: ' . $notice_text);

    $post->post_content = '<p>Ripristino.</p>';
    $GLOBALS['http'] = array();
    affilio_one_box_ack(422, '{"check_id":"one_per_post","check_text":"Keep one box on this post."}');
    $restored = $distribution->restore_box(2615, $id, array('copy' => 'Copy', 'disclosure' => 'Disclosure'));
    check($restored === false, 'a refused restore was treated as success');
    check(strpos($post->post_content, $id) === false, 'a refused restore left the new box in the post');
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    check(!is_array($queue) || !isset($queue[$id . '|restored']), 'a refused restore was queued');
}

function affilio_review_upgrade_one_per_post_keeps_the_existing_box() {
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    $original = '<p class="affilio-placement-' . $id . '">Box <a href="/go/' . $id . '" rel="sponsored nofollow">Vedi</a></p>';
    $post = affilio_one_box_reset($original);
    $GLOBALS['options']['affilio_seen_version'] = '0.1.19';
    $GLOBALS['flushes'] = 0;
    affilio_one_box_ack(422, '{"check_id":"one_per_post","check_text":"Keep one box on this post."}');
    $distribution = new Affilio_Distribution(new stdClass());
    $distribution->maybe_upgrade();
    check($post->post_content === $original, 'upgrade removed a box that was already in the post');
    check(get_option('affilio_seen_version') === AFFILIO_VERSION, 'upgrade from 0.1.19 did not save the version marker');
    $acks = d033_ack_bodies();
    check(count($acks) === 1 && $acks[0]['body']['action'] === 'placed', 'upgrade did not ack the existing box');
    check($acks[0]['headers']['X-Plugin-Key'] === 'plugin-only-key', 'upgrade ack did not use the plugin key');
    check(!isset($acks[0]['headers']['X-API-Key']), 'upgrade ack sent the site key');
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    check($queue === array() || $queue === false || $queue === null || (is_array($queue) && !isset($queue[$id . '|placed'])), 'upgrade queued a 422 acknowledgement');
    $index = get_option('affilio_box_index', array());
    check(isset($index[$id]['phase']) && $index[$id]['phase'] === 'refused', 'upgrade marked a refused box placed');
    $notices = get_option('affilio_plugin_ack_notices', array());
    $notice_text = isset($notices[0]['message']) ? $notices[0]['message'] : '';
    check($notice_text === 'Keep one box on this post.', 'upgrade notice was: ' . $notice_text);
    check(Affilio_Distribution::server_wants_restored(array('status' => 'committed', 'plugin_state' => 'pending'), 'refused') === false, 'a refused box was eligible for another restore');
}

function affilio_review_go_fallback_is_never_empty() {
    d033_log_file();
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    affilio_one_box_reset('<p>Go.</p>');
    $GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION] = array($id => 'https://shop.example/plain');
    $GLOBALS['http_response'] = array(
        'response' => array('code' => 302),
        'headers' => array('Location' => '   '),
        'body' => '',
    );
    $go = new Affilio_Go();
    check($go->resolve($id) === 'https://shop.example/plain', 'a blank Location was redirected');
    $GLOBALS['http_response'] = array(
        'response' => array('code' => 302),
        'headers' => array('Location' => "https://shop.example/landed\r\nSet-Cookie: a=b"),
        'body' => '',
    );
    check($go->resolve($id) === 'https://shop.example/plain', 'a broken Location was passed through');
    delete_option(AFFILIO_GO_CIRCUIT_OPTION);
    $GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION] = array($id => array('not-a-url'));
    unset($GLOBALS['http_response']);
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 500), 'headers' => array(), 'body' => '');
    };
    affilio_one_box_warnings();
    $location = $go->resolve($id);
    $warnings = affilio_one_box_warning_text();
    check(strpos($warnings, 'Array to string') === false, 'a bad cached fallback warned: ' . $warnings);
    check($location === 'https://blog.example/', 'missing fallback was: ' . $location);
    check($location !== '', 'go fallback was empty');
}

if (!function_exists('affilio_review_run_all')) {
    function affilio_review_run_all() {
        affilio_review_commit_one_per_post_does_not_insert();
        affilio_review_ack_one_per_post_removes_the_new_box();
        affilio_review_upgrade_one_per_post_keeps_the_existing_box();
        affilio_review_go_fallback_is_never_empty();
        echo "One box per post and go-link fallback passed.\n";
    }
}

$affilio_one_box_script = '';
if (isset($_SERVER['SCRIPT_FILENAME']) && $_SERVER['SCRIPT_FILENAME'] !== '') {
    $affilio_one_box_script = (string) realpath($_SERVER['SCRIPT_FILENAME']);
}
if ($affilio_one_box_script === '' && isset($argv[0])) {
    $affilio_one_box_script = (string) realpath($argv[0]);
}
if ($affilio_one_box_script === realpath(__FILE__)) {
    affilio_review_run_all();
}
