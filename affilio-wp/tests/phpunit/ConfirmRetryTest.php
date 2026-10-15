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
