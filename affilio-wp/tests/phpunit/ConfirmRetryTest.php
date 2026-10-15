<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class ConfirmRetryTest extends TestCase {
    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_confirm_403_stays_pending_and_retries() {
        $this->assertConfirmStaysPending(403, '{"error":"plugin_key_required"}');
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_confirm_5xx_stays_pending_and_retries() {
        $this->assertConfirmStaysPending(503, '{"error":"unavailable"}');
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_prepare_retries_once_on_http_503() {
        $this->boot();
        affilio_one_box_reset('<p>Articolo pubblicato con abbastanza testo.</p>');
        $api = new ConfirmRetryPlacementAPI();
        $api->failures = array(array('code' => 503, 'data' => array('error' => 'unavailable')));
        $distribution = new Affilio_Distribution($api);
        $result = $distribution->prepare(new OneBoxRequest(array(
            'post_id' => 2615,
            'offer_id' => 'cccccccc-cccc-cccc-cccc-cccccccccccc',
            'copy' => 'Una frase abbastanza lunga per il riquadro.',
        )));
        $this->assertFalse(is_wp_error($result), is_wp_error($result) ? $result->get_error_message() : '');
        $this->assertSame(2, $api->placement_posts);
        $this->assertSame('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', $result['id'] ?? null);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_prepare_retries_once_on_transport_failure() {
        $this->boot();
        affilio_one_box_reset('<p>Articolo pubblicato con abbastanza testo.</p>');
        $api = new ConfirmRetryPlacementAPI();
        $api->failures = array(new WP_Error('http_request_failed', 'timeout'));
        $distribution = new Affilio_Distribution($api);
        $result = $distribution->prepare(new OneBoxRequest(array(
            'post_id' => 2615,
            'offer_id' => 'cccccccc-cccc-cccc-cccc-cccccccccccc',
            'copy' => 'Una frase abbastanza lunga per il riquadro.',
        )));
        $this->assertFalse(is_wp_error($result), is_wp_error($result) ? $result->get_error_message() : '');
        $this->assertSame(2, $api->placement_posts);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_prepare_does_not_retry_past_one_503() {
        $this->boot();
        affilio_one_box_reset('<p>Articolo pubblicato con abbastanza testo.</p>');
        $api = new ConfirmRetryPlacementAPI();
        $api->failures = array(
            array('code' => 503, 'data' => array('error' => 'unavailable')),
            array('code' => 503, 'data' => array('error' => 'unavailable')),
        );
        $distribution = new Affilio_Distribution($api);
        $result = $distribution->prepare(new OneBoxRequest(array(
            'post_id' => 2615,
            'offer_id' => 'cccccccc-cccc-cccc-cccc-cccccccccccc',
            'copy' => 'Una frase abbastanza lunga per il riquadro.',
        )));
        $this->assertTrue(is_wp_error($result));
        $this->assertSame(2, $api->placement_posts);
        $data = $result->get_error_data();
        $this->assertIsArray($data);
        $this->assertSame(503, (int) ($data['status'] ?? 0));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_prepare_does_not_retry_not_configured() {
        $this->boot();
        affilio_one_box_reset('<p>Articolo pubblicato con abbastanza testo.</p>');
        $api = new ConfirmRetryPlacementAPI();
        $api->failures = array(new WP_Error('affilio_not_configured', 'Not connected — set the API URL in Settings'));
        $distribution = new Affilio_Distribution($api);
        $result = $distribution->prepare(new OneBoxRequest(array(
            'post_id' => 2615,
            'offer_id' => 'cccccccc-cccc-cccc-cccc-cccccccccccc',
            'copy' => 'Una frase abbastanza lunga per il riquadro.',
        )));
        $this->assertTrue(is_wp_error($result));
        $this->assertSame(1, $api->placement_posts);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_confirm_400_stays_pending_without_a_retry() {
        $this->assertUnretryableConfirmStaysPending(400, '{"error":"bad_request"}');
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_confirm_422_stays_pending_without_a_retry() {
        $this->assertUnretryableConfirmStaysPending(422, '{"error":"unprocessable"}');
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_drain_marks_nothing_to_ack_placed() {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $post = affilio_one_box_reset('<p class="affilio-placement-' . $id . '">Box <a href="/go/' . $id . '" rel="sponsored nofollow">Vedi</a></p>');
        update_option('affilio_box_index', array(
            $id => array('post_id' => 2615, 'phase' => 'pending', 'copy' => 'Box', 'disclosure' => ''),
        ));
        $key = $id . '|placed';
        update_option(Affilio_Distribution::ack_queue_option(), array(
            $key => array(
                'placement_id' => $id,
                'action' => 'placed',
                'rel_sponsored' => false,
                'reason' => '',
                'attempts' => 1,
                'next_at' => time() - 1,
            ),
        ));
        affilio_one_box_ack(409, '{"reason":"nothing_to_ack"}');
        $distribution = new Affilio_Distribution(new OneBoxAPI());
        $distribution->drain_ack_queue();
        $index = get_option('affilio_box_index', array());
        $this->assertSame('placed', $index[$id]['phase'] ?? null);
        $this->assertArrayNotHasKey('ack_error', $index[$id]);
        $this->assertArrayNotHasKey($key, get_option(Affilio_Distribution::ack_queue_option(), array()));
        $this->assertStringContainsString('affilio-placement-' . $id, $post->post_content);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_admin_retry_respects_backoff_and_rereads_the_post() {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $post = $this->seedPendingAck($id, true);
        $key = $id . '|placed';
        $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
        $queue[$key]['next_at'] = time() + 600;
        $queue[$key]['rel_sponsored'] = false;
        update_option(Affilio_Distribution::ack_queue_option(), $queue);
        $GLOBALS['http'] = array();
        $distribution = new Affilio_Distribution(new OneBoxAPI());
        $distribution->retry_pending_acks();
        $this->assertCount(0, $GLOBALS['http']);
        $this->assertSame('pending', get_option('affilio_box_index', array())[$id]['phase'] ?? null);

        $queue[$key]['next_at'] = time() - 1;
        update_option(Affilio_Distribution::ack_queue_option(), $queue);
        affilio_one_box_ack(200, '{"plugin_state":"placed"}');
        $GLOBALS['http'] = array();
        $distribution->retry_pending_acks();
        $this->assertCount(1, $GLOBALS['http']);
        $body = json_decode($GLOBALS['http'][0]['args']['body'], true);
        $this->assertTrue($body['rel_sponsored'] ?? false);
        $this->assertSame('placed', get_option('affilio_box_index', array())[$id]['phase'] ?? null);

        $post->post_content = '<p>Il riquadro non c\'è più.</p>';
        $this->seedPendingAck($id, false);
        $GLOBALS['http'] = array();
        $distribution->retry_pending_acks();
        $this->assertCount(0, $GLOBALS['http']);
        $this->assertArrayNotHasKey($key, get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_admin_retry_skips_ajax() {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $distribution = new Affilio_Distribution(new OneBoxAPI());
        if (!defined('DOING_AJAX')) {
            define('DOING_AJAX', true);
        }
        $this->seedPendingAck($id, true);
        $GLOBALS['http'] = array();
        $distribution->retry_pending_acks();
        $this->assertCount(0, $GLOBALS['http']);
        $this->assertArrayHasKey($id . '|placed', get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_admin_retry_skips_heartbeat() {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $_REQUEST['action'] = 'heartbeat';
        $this->seedPendingAck($id, true);
        $GLOBALS['http'] = array();
        (new Affilio_Distribution(new OneBoxAPI()))->retry_pending_acks();
        $this->assertCount(0, $GLOBALS['http']);
        $this->assertArrayHasKey($id . '|placed', get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_admin_retry_skips_cron() {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        if (!defined('DOING_CRON')) {
            define('DOING_CRON', true);
        }
        $this->seedPendingAck($id, true);
        $GLOBALS['http'] = array();
        (new Affilio_Distribution(new OneBoxAPI()))->retry_pending_acks();
        $this->assertCount(0, $GLOBALS['http']);
        $this->assertArrayHasKey($id . '|placed', get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_admin_retry_requires_an_editor() {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $GLOBALS['allowed'] = false;
        $this->seedPendingAck($id, true);
        $GLOBALS['http'] = array();
        (new Affilio_Distribution(new OneBoxAPI()))->retry_pending_acks();
        $this->assertCount(0, $GLOBALS['http']);
        $this->assertSame('pending', get_option('affilio_box_index', array())[$id]['phase'] ?? null);
        $this->assertArrayHasKey($id . '|placed', get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_rewrite_legacy_links_escapes_home_url() {
        if (!function_exists('home_url')) {
            function home_url($path = '') {
                return 'https://blog.example' . $path;
            }
        }
        if (!function_exists('esc_url')) {
            function esc_url($value) {
                return 'safe:' . $value;
            }
        }
        require_once dirname(__DIR__, 2) . '/includes/class-affilio-go.php';
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $rendered = Affilio_Go::rewrite_legacy_links('<a href="https://api.example.test/v1/go/' . $id . '">qui</a>');
        $this->assertStringContainsString('safe:https://blog.example/go/' . $id, $rendered);
    }

    private function assertConfirmStaysPending($status, $body) {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $post = affilio_one_box_reset('<p>Articolo senza riquadro.</p>');
        affilio_one_box_ack($status, $body);
        $distribution = new Affilio_Distribution(new OneBoxAPI());
        $result = $distribution->publish(new OneBoxRequest(array(
            'post_id' => 2615,
            'placement_id' => $id,
        )));
        $this->assertFalse(is_wp_error($result) && isset($result->get_error_data()['reason']) && $result->get_error_data()['reason'] === 'one_per_post');
        $this->assertStringContainsString('affilio-placement-' . $id, $post->post_content);
        $index = get_option('affilio_box_index', array());
        $this->assertIsArray($index[$id] ?? null);
        $this->assertSame('pending', $index[$id]['phase'] ?? null);
        $error = $index[$id]['ack_error'] ?? null;
        $this->assertIsArray($error);
        $this->assertSame($status, (int) ($error['status'] ?? 0));
        $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
        $key = $id . '|placed';
        $this->assertArrayHasKey($key, $queue);

        $queue[$key]['next_at'] = time() - 1;
        update_option(Affilio_Distribution::ack_queue_option(), $queue);
        affilio_one_box_ack(200, '{"plugin_state":"placed"}');
        $distribution->drain_ack_queue();
        $index = get_option('affilio_box_index', array());
        $this->assertSame('placed', $index[$id]['phase'] ?? null);
        $this->assertArrayNotHasKey('ack_error', $index[$id]);
        $this->assertArrayNotHasKey($key, get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    private function assertUnretryableConfirmStaysPending($status, $body) {
        $this->boot();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $post = affilio_one_box_reset('<p>Articolo senza riquadro.</p>');
        affilio_one_box_ack($status, $body);
        $distribution = new Affilio_Distribution(new OneBoxAPI());
        $distribution->publish(new OneBoxRequest(array(
            'post_id' => 2615,
            'placement_id' => $id,
        )));
        $this->assertStringContainsString('affilio-placement-' . $id, $post->post_content);
        $index = get_option('affilio_box_index', array());
        $this->assertSame('pending', $index[$id]['phase'] ?? null);
        $this->assertSame($status, (int) ($index[$id]['ack_error']['status'] ?? 0));
        $this->assertArrayNotHasKey($id . '|placed', get_option(Affilio_Distribution::ack_queue_option(), array()));
    }

    private function seedPendingAck($id, $with_box) {
        $content = $with_box
            ? '<p class="affilio-placement-' . $id . '">Box <a href="/go/' . $id . '" rel="sponsored nofollow">Vedi</a></p>'
            : '<p>Senza riquadro.</p>';
        $post = affilio_one_box_reset($content);
        update_option('affilio_box_index', array(
            $id => array('post_id' => 2615, 'phase' => 'pending', 'copy' => 'Box', 'disclosure' => ''),
        ));
        update_option(Affilio_Distribution::ack_queue_option(), array(
            $id . '|placed' => array(
                'placement_id' => $id,
                'action' => 'placed',
                'rel_sponsored' => false,
                'reason' => '',
                'attempts' => 1,
                'next_at' => time() - 1,
            ),
        ));
        affilio_one_box_ack(200, '{"plugin_state":"placed"}');
        return $post;
    }

    private function boot() {
        require_once dirname(__DIR__) . '/d033-one-box.php';
    }
}

if (!class_exists('ConfirmRetryPlacementAPI')) {
    class ConfirmRetryPlacementAPI {
        public $placement_posts = 0;
        public $failures = array();

        public function request($method, $path, $body = null) {
            $GLOBALS['api_calls'][] = $method . ' ' . $path;
            if ($method === 'POST' && rtrim((string) $path, '/') === '/placements') {
                $this->placement_posts++;
                if ($this->failures) {
                    $failure = array_shift($this->failures);
                    if ($failure !== null) {
                        return $failure;
                    }
                }
                return array('code' => 201, 'data' => array(
                    'id' => 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
                    'placement' => array('id' => 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'status' => 'pending'),
                ));
            }
            if ($method === 'GET' && strpos((string) $path, '/sites/owner-connection') !== false) {
                return array('code' => 200, 'data' => array('connected' => true));
            }
            return array('code' => 200, 'data' => array());
        }
    }
}
