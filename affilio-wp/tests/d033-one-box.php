<?php

require __DIR__ . '/d033-harness.php';

if (!defined('AFFILIO_VERSION')) {
    define('AFFILIO_VERSION', '0.1.25');
}
require __DIR__ . '/../includes/class-affilio-plugin-ack.php';
require __DIR__ . '/../includes/class-affilio-go.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';

if (!function_exists('get_the_title')) {
    function get_the_title($post = 0) {
        if (isset($GLOBALS['affilio_encoded_title'])) {
            return $GLOBALS['affilio_encoded_title'];
        }
        if (is_object($post) && isset($post->post_title)) {
            return $post->post_title;
        }
        return '';
    }
}

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
        public $receipt_error = false;
        public $refuse_failures = 0;
        public $prepared_id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
        public $seen = array();
        public function request($method, $path, $body = null) {
            $GLOBALS['api_calls'][] = $method . ' ' . $path;
            $this->seen[] = array('method' => $method, 'path' => $path, 'body' => $body);
            $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
            if (preg_match('#/placements/([a-f0-9-]{36})#i', $path, $match)) {
                $id = $match[1];
            }
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
                if ($this->receipt_error) {
                    return new WP_Error('http_request_failed', 'network down');
                }
                return array('code' => 200, 'data' => array('ok' => true));
            }
            if (strpos($path, '/refuse') !== false) {
                if ((int) $this->refuse_failures > 0) {
                    $this->refuse_failures--;
                    return new WP_Error('http_request_failed', 'network down');
                }
                return array('code' => 200, 'data' => array('status' => 'pending'));
            }
            if ($method === 'POST' && rtrim($path, '/') === '/placements') {
                $created = $placement;
                $created['id'] = $this->prepared_id;
                $created['status'] = 'pending';
                return array('code' => 200, 'data' => array('id' => $this->prepared_id, 'placement' => $created));
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
    unset($GLOBALS['http_handler'], $GLOBALS['http_response'], $GLOBALS['affilio_encoded_title']);
    return $post;
}

function affilio_one_box_call($api, $method, $path) {
    foreach ($api->seen as $call) {
        if ($call['method'] === $method && $call['path'] === $path) {
            return $call;
        }
    }
    return null;
}

function affilio_one_box_calls($api, $method, $needle) {
    $found = array();
    foreach ($api->seen as $call) {
        if ($call['method'] === $method && strpos($call['path'], $needle) !== false) {
            $found[] = $call;
        }
    }
    return $found;
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

function affilio_review_prepare_and_receipt_send_decoded_title() {
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    $post = affilio_one_box_reset('<p>Articolo.</p>');
    $post->post_title = 'Stored &amp; title';
    $GLOBALS['affilio_encoded_title'] = 'Tom &amp; Jerry';
    $api = new OneBoxAPI();
    $distribution = new Affilio_Distribution($api);
    $prepared = $distribution->prepare(new OneBoxRequest(array(
        'post_id' => 2615,
        'offer_id' => 'offer-1',
        'copy' => 'Una copia.',
    )));
    check(!is_wp_error($prepared), 'prepare failed: ' . (is_wp_error($prepared) ? $prepared->get_error_message() : ''));
    $prepare_call = affilio_one_box_call($api, 'POST', '/placements');
    check(is_array($prepare_call), 'prepare did not post /placements');
    check(isset($prepare_call['body']['post_title']) && $prepare_call['body']['post_title'] === 'Tom & Jerry', 'prepare post_title was ' . json_encode(isset($prepare_call['body']['post_title']) ? $prepare_call['body']['post_title'] : null));
    check(isset($prepare_call['body']['post_id']) && $prepare_call['body']['post_id'] === 2615 && is_int($prepare_call['body']['post_id']), 'prepare post_id was ' . json_encode(isset($prepare_call['body']['post_id']) ? $prepare_call['body']['post_id'] : null));
    $published = $distribution->publish(new OneBoxRequest(array('post_id' => 2615, 'placement_id' => $id)));
    check(!is_wp_error($published), 'receipt publish failed: ' . (is_wp_error($published) ? $published->get_error_message() : ''));
    $receipt_call = affilio_one_box_call($api, 'POST', '/placements/' . $id . '/receipt');
    check(is_array($receipt_call), 'publish did not post a receipt');
    check(isset($receipt_call['body']['post_title']) && $receipt_call['body']['post_title'] === 'Tom & Jerry', 'receipt post_title was ' . json_encode(isset($receipt_call['body']['post_title']) ? $receipt_call['body']['post_title'] : null));
    check(isset($receipt_call['body']['post_id']) && $receipt_call['body']['post_id'] === 2615 && is_int($receipt_call['body']['post_id']), 'receipt post_id was ' . json_encode(isset($receipt_call['body']['post_id']) ? $receipt_call['body']['post_id'] : null));
    check(strpos($receipt_call['body']['post_title'], '&amp;') === false, 'receipt sent the encoded title');
    check($receipt_call['body']['post_title'] !== 'Stored & title', 'receipt used the stored post field instead of get_the_title');
}

function affilio_review_local_refuse_then_prepare_keeps_one_box() {
    $first = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    $second = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
    $post = affilio_one_box_reset('<p>Articolo senza riquadro.</p>');
    $api = new OneBoxAPI();
    $api->receipt_error = true;
    $api->refuse_failures = 2;
    $api->prepared_id = $second;
    $distribution = new Affilio_Distribution($api);
    $refused = $distribution->publish(new OneBoxRequest(array('post_id' => 2615, 'placement_id' => $first)));
    check(is_wp_error($refused), 'a failed receipt was treated as published');
    check(strpos($post->post_content, $first) === false, 'local rollback left the refused box in the post');
    check(preg_match_all('/affilio-placement-[a-f0-9-]{36}/i', $post->post_content) === 0, 'a refused receipt left a box');
    $refusals = affilio_one_box_calls($api, 'POST', '/refuse');
    check(count($refusals) === 1, 'local rollback did not call refuse once: ' . count($refusals));
    check($refusals[0]['path'] === '/placements/' . $first . '/refuse', 'refuse path was ' . $refusals[0]['path']);
    check($refusals[0]['body'] === array('reason' => 'local_rollback'), 'refuse body was ' . json_encode($refusals[0]['body']));
    $queue = get_option(Affilio_Distribution::refuse_queue_option(), array());
    check(isset($queue[$first]['reason']) && $queue[$first]['reason'] === 'local_rollback', 'a failed refuse was not queued');

    $blocked = $distribution->prepare(new OneBoxRequest(array(
        'post_id' => 2615,
        'offer_id' => 'offer-1',
        'copy' => 'Una copia.',
    )));
    check(is_wp_error($blocked), 'prepare created a placement while refuse was still failing');
    check(affilio_one_box_call($api, 'POST', '/placements') === null, 'a failed refuse retry still posted /placements');
    check(count(affilio_one_box_calls($api, 'POST', '/refuse')) === 2, 'prepare did not retry the queued refuse');
    check(strpos($post->post_content, 'affilio-placement-') === false, 'a blocked prepare inserted a box');

    $prepared = $distribution->prepare(new OneBoxRequest(array(
        'post_id' => 2615,
        'offer_id' => 'offer-1',
        'copy' => 'Una copia.',
    )));
    check(!is_wp_error($prepared), 'prepare after a successful refuse failed: ' . (is_wp_error($prepared) ? $prepared->get_error_message() : ''));
    $prepare_call = affilio_one_box_call($api, 'POST', '/placements');
    check(is_array($prepare_call), 'the retry did not prepare a placement');
    check(isset($prepare_call['body']['post_title']) && isset($prepare_call['body']['post_id']), 'retried prepare omitted the article title');
    $queue = get_option(Affilio_Distribution::refuse_queue_option(), array());
    check(!is_array($queue) || !isset($queue[$first]), 'a successful refuse stayed queued');
    check(count(affilio_one_box_calls($api, 'POST', '/refuse')) === 3, 'prepare did not send the queued refuse');

    $api->receipt_error = false;
    $published = $distribution->publish(new OneBoxRequest(array('post_id' => 2615, 'placement_id' => $second)));
    check(!is_wp_error($published), 'the new placement did not publish: ' . (is_wp_error($published) ? $published->get_error_message() : ''));
    $boxes = array();
    preg_match_all('/affilio-placement-([a-f0-9-]{36})/i', $post->post_content, $boxes);
    $ids = isset($boxes[1]) ? array_values(array_unique($boxes[1])) : array();
    check($ids === array($second), 'the post did not contain exactly the new box: ' . $post->post_content);
    check(strpos($post->post_content, $first) === false, 'the refused box came back');
}

function affilio_review_go_rejects_unsafe_urls() {
    d033_log_file();
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    affilio_one_box_reset('<p>Go.</p>');
    unset($GLOBALS['http_handler']);
    delete_option(AFFILIO_GO_CIRCUIT_OPTION);
    $GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION] = array($id => 'https://shop.example/plain');
    $go = new Affilio_Go();
    foreach (array('http://shop.example/landed', 'javascript:alert(1)') as $unsafe) {
        $GLOBALS['http_response'] = array(
            'response' => array('code' => 302),
            'headers' => array('Location' => $unsafe),
            'body' => '',
        );
        $location = $go->resolve($id);
        check($location !== $unsafe, 'unsafe Location was used: ' . $unsafe);
        check(stripos((string) $location, 'javascript:') === false, 'javascript Location was used');
        check(strpos((string) $location, 'http://') !== 0, 'http Location was used: ' . $location);
        check($location === 'https://shop.example/plain', 'unsafe Location did not fall back safely: ' . $location);
    }
    unset($GLOBALS['http_response']);
    $GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION] = array();
    $GLOBALS['http_handler'] = function () {
        return array(
            'response' => array('code' => 302),
            'headers' => array('Location' => 'javascript:alert(1)'),
            'body' => '',
        );
    };
    check($go->resolve($id) === 'https://blog.example/', 'an unsafe Location with no fallback was not sent home');
    $_SERVER['REQUEST_URI'] = '/go/' . $id;
    $GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION] = array($id => 'https://shop.example/plain');
    try {
        $go->handle();
        check(false, 'unsafe Location did not redirect');
    } catch (RuntimeException $redirect) {
        check(strpos($redirect->getMessage(), 'javascript:') === false, 'handle redirected to javascript: ' . $redirect->getMessage());
        check(strpos($redirect->getMessage(), 'http://') === false, 'handle redirected to http: ' . $redirect->getMessage());
        check(strpos($redirect->getMessage(), 'https://shop.example/plain') !== false, 'handle did not fall back safely: ' . $redirect->getMessage());
    }
    unset($GLOBALS['http_handler']);

    Affilio_Go::remember_fallback($id, 'http://shop.example/nope');
    Affilio_Go::remember_fallback($id, 'javascript:alert(1)');
    $stored = get_option(AFFILIO_GO_FALLBACKS_OPTION, array());
    check(isset($stored[$id]) && $stored[$id] === 'https://shop.example/plain', 'an unsafe fallback replaced the saved URL');
    Affilio_Go::remember_fallback($id, 'https://shop.example/kept');
    $stored = get_option(AFFILIO_GO_FALLBACKS_OPTION, array());
    check(isset($stored[$id]) && $stored[$id] === 'https://shop.example/kept', 'an https fallback was not stored');
    check(Affilio_Go::redirect_url_allowed(Affilio_Go::home_fallback()) === true, 'the site home URL was rejected');
    check(Affilio_Go::redirect_url_allowed('https://merchant.example/product') === true, 'an https Location was rejected');
    check(Affilio_Go::redirect_url_allowed('http://merchant.example/product') === false, 'an http Location was accepted');
    check(Affilio_Go::redirect_url_allowed('javascript:alert(1)') === false, 'a javascript Location was accepted');
}

function affilio_review_unknown_go_id_is_404() {
    d033_log_file();
    $id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
    affilio_one_box_reset('<p>Go.</p>');
    unset($GLOBALS['options'][AFFILIO_GO_FALLBACKS_OPTION]);
    delete_option(AFFILIO_GO_CIRCUIT_OPTION);
    unset($_GET['affilio_go']);
    $_SERVER['REQUEST_URI'] = '/go/' . $id;
    unset($GLOBALS['http_response']);
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 404), 'headers' => array(), 'body' => '');
    };
    $go = new Affilio_Go();
    check($go->resolve($id) === '', 'unknown go id resolved to a URL');
    check((int) get_option(AFFILIO_GO_CIRCUIT_OPTION, 0) === 0, 'an unknown go id opened the circuit');
    try {
        $go->handle();
        check(false, 'unknown go id redirected');
    } catch (RuntimeException $response) {
        check($response->getMessage() === 'status 404', 'unknown go id was ' . $response->getMessage());
        check(strpos($response->getMessage(), 'redirect 302') === false, 'unknown go id sent a 302');
        check(strpos($response->getMessage(), 'https://blog.example/') === false, 'unknown go id redirected home');
    }
}

if (!function_exists('affilio_review_run_all')) {
    function affilio_review_run_all() {
        affilio_review_commit_one_per_post_does_not_insert();
        affilio_review_ack_one_per_post_removes_the_new_box();
        affilio_review_upgrade_one_per_post_keeps_the_existing_box();
        affilio_review_go_fallback_is_never_empty();
        affilio_review_prepare_and_receipt_send_decoded_title();
        affilio_review_local_refuse_then_prepare_keeps_one_box();
        affilio_review_unknown_go_id_is_404();
        affilio_review_go_rejects_unsafe_urls();
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
