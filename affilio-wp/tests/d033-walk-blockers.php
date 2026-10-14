<?php

function affilio_walk_boot() {
    static $booted = false;
    if ($booted) {
        return;
    }
    $booted = true;
    require_once __DIR__ . '/d033-harness.php';
    affilio_walk_install_stubs();
    if (!defined('ABSPATH')) {
        define('ABSPATH', __DIR__);
    }
    $GLOBALS['affilio_rewrite_requires_wp_rewrite'] = true;
    $GLOBALS['get_query_var_calls'] = 0;
    require_once __DIR__ . '/../affilio.php';
}

function affilio_walk_install_stubs() {
    if (!function_exists('plugin_dir_path')) {
        function plugin_dir_path($path) { return dirname($path) . '/'; }
        function plugin_dir_url($path) { return 'https://blog.example/wp-content/plugins/affilio-wp/'; }
    }
    if (!function_exists('get_query_var')) {
        function get_query_var($var, $default = '') {
            $GLOBALS['get_query_var_calls'] = (isset($GLOBALS['get_query_var_calls']) ? (int) $GLOBALS['get_query_var_calls'] : 0) + 1;
            global $wp_query;
            if (!is_object($wp_query)) {
                throw new Error('get_query_var called before WP_Query exists');
            }
            if (method_exists($wp_query, 'get')) {
                return $wp_query->get($var, $default);
            }
            return $default;
        }
    }
    if (!function_exists('wp_next_scheduled')) {
        function wp_next_scheduled($hook) {
            return isset($GLOBALS['cron'][$hook]) ? $GLOBALS['cron'][$hook]['time'] : false;
        }
        function wp_schedule_event($timestamp, $schedule, $hook) {
            if (!isset($GLOBALS['cron_schedule_calls'][$hook])) {
                $GLOBALS['cron_schedule_calls'][$hook] = 0;
            }
            $GLOBALS['cron_schedule_calls'][$hook]++;
            $GLOBALS['cron'][$hook] = array('time' => $timestamp, 'schedule' => $schedule);
            return true;
        }
        function wp_clear_scheduled_hook($hook) {
            unset($GLOBALS['cron'][$hook]);
            return 0;
        }
    }
    if (!class_exists('Affilio_Walk_Request')) {
        class Affilio_Walk_Request {
            private $values;
            public function __construct($values) { $this->values = $values; }
            public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
        }
    }
    if (!class_exists('Affilio_Walk_API')) {
        class Affilio_Walk_API {
            public $get = array();
            public function request($method, $path, $body = null) {
                if (strpos((string) $path, '/sites/owner-connection') !== false) {
                    return array('code' => 200, 'data' => array('connected' => true));
                }
                if ($method === 'GET') {
                    return array('code' => 200, 'data' => $this->get);
                }
                if (strpos((string) $path, '/commit') !== false) {
                    $id = isset($GLOBALS['walk_placement_id']) ? $GLOBALS['walk_placement_id'] : '';
                    return array('code' => 200, 'data' => array('placement' => array(
                        'copy' => 'A useful tool',
                        'disclosure' => '(link sponsorizzato)',
                        'fallback_url' => 'https://shop.example/product',
                        'receipt' => array(
                            'tracked_link' => '/v1/go/test?placement=' . $id,
                            'publication_status' => 'publisher_reported',
                        ),
                    )));
                }
                return array('code' => 200, 'data' => array());
            }
        }
    }
}

function affilio_walk_do_action($name) {
    $callbacks = isset($GLOBALS['actions'][$name]) ? $GLOBALS['actions'][$name] : array();
    $meta = isset($GLOBALS['action_meta'][$name]) ? $GLOBALS['action_meta'][$name] : array();
    $indexed = array();
    foreach ($callbacks as $index => $callback) {
        $priority = isset($meta[$index]['priority']) ? (int) $meta[$index]['priority'] : 10;
        $indexed[] = array($priority, $index, $callback);
    }
    usort($indexed, function ($left, $right) {
        if ($left[0] === $right[0]) {
            return $left[1] <=> $right[1];
        }
        return $left[0] <=> $right[0];
    });
    foreach ($indexed as $row) {
        call_user_func($row[2]);
    }
}

function affilio_walk_distribution() {
    return new Affilio_Distribution(new Affilio_Walk_API());
}

function affilio_walk_box($id) {
    return "<!-- wp:paragraph {\"className\":\"affilio-placement-" . $id . "\"} -->\n"
        . "<p class=\"affilio-placement-" . $id . "\">Tool <a href=\"/go/" . $id . "\" rel=\"sponsored nofollow\">Vedi</a> (link sponsorizzato)</p>\n"
        . "<!-- /wp:paragraph -->";
}

function affilio_walk_b1_front_page_does_not_call_get_query_var() {
    affilio_walk_boot();
    $_GET = array();
    $_SERVER['REQUEST_URI'] = '/';
    $GLOBALS['wp_query'] = null;
    $GLOBALS['get_query_var_calls'] = 0;
    check(Affilio_Go::current_id() === '', 'front page looked like /go/');
    check((int) $GLOBALS['get_query_var_calls'] === 0, 'front page called get_query_var');
    affilio_mark_go_uncacheable();
    check((int) $GLOBALS['get_query_var_calls'] === 0, 'early hook called get_query_var');
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    $_SERVER['REQUEST_URI'] = '/go/' . $id;
    check(Affilio_Go::current_id() === $id, 'path /go/ was not detected before WP_Query');
    check((int) $GLOBALS['get_query_var_calls'] === 0, '/go/ called get_query_var before WP_Query');
    try {
        affilio_mark_go_uncacheable();
    } catch (Throwable $error) {
        check(false, 'early /go/ hook fataled: ' . $error->getMessage());
    }
    $_SERVER['REQUEST_URI'] = '/blog/';
    $_GET = array();
    $GLOBALS['wp_query'] = new Affilio_Walk_Query('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb');
    check(Affilio_Go::current_id() === 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'query var was ignored after WP_Query exists');
    $GLOBALS['wp_query'] = null;
}

function affilio_walk_b2_upgrade_saves_marker_after_partial_failure() {
    affilio_walk_boot();
    unset($GLOBALS['flush_rewrite_rules_hook']);
    $distribution = affilio_walk_distribution();
    $saved_rewrite = isset($GLOBALS['wp_rewrite']) ? $GLOBALS['wp_rewrite'] : null;
    $GLOBALS['wp_rewrite'] = null;
    update_option('affilio_seen_version', '0.1.19');
    delete_option('affilio_go_flush_pending');
    $GLOBALS['flushes'] = 0;
    $GLOBALS['rewrites'] = array();
    $distribution->maybe_upgrade();
    check(get_option('affilio_seen_version') === '0.1.19', 'upgrade during plugin load saved the version marker');
    check($GLOBALS['flushes'] === 0, 'upgrade during plugin load flushed rules');
    check($GLOBALS['rewrites'] === array(), 'add_rewrite_rule ran before WP_Rewrite existed');

    $GLOBALS['wp_rewrite'] = new stdClass();
    $GLOBALS['flushes'] = 0;
    $GLOBALS['flush_rewrite_rules_hook'] = function () use ($distribution) {
        $distribution->maybe_upgrade();
    };
    $distribution->maybe_upgrade();
    check($GLOBALS['flushes'] === 1, 'upgrade recursed into flush and looped');
    check(get_option('affilio_seen_version') === AFFILIO_VERSION, 're-entrant upgrade did not save the marker');

    update_option('affilio_seen_version', '0.1.19');
    delete_option('affilio_go_flush_pending');
    $GLOBALS['flushes'] = 0;
    $GLOBALS['flush_rewrite_rules_hook'] = function () {
        throw new RuntimeException('flush failed');
    };
    try {
        $distribution->maybe_upgrade();
    } catch (Throwable $error) {
        check(false, 'flush failure escaped the upgrade: ' . $error->getMessage());
    }
    check(get_option('affilio_seen_version') === AFFILIO_VERSION, 'version marker was not saved after a partial upgrade');
    check(get_option('affilio_go_flush_pending') === 'once', 'a failed flush was not limited to one retry');
    $after_first = $GLOBALS['flushes'];
    $distribution->maybe_upgrade();
    check($GLOBALS['flushes'] === $after_first + 1, 'the failed flush was not retried once');
    check(get_option('affilio_go_flush_pending', '') === '', 'flush retry did not stop');
    $stopped = $GLOBALS['flushes'];
    $distribution->maybe_upgrade();
    check($GLOBALS['flushes'] === $stopped, 'upgrade kept flushing after the retry');
    unset($GLOBALS['flush_rewrite_rules_hook']);
    $GLOBALS['wp_rewrite'] = $saved_rewrite;
}

function affilio_walk_b3_sync_cron_on_activation_upgrade_and_deactivation() {
    affilio_walk_boot();
    unset($GLOBALS['flush_rewrite_rules_hook']);
    if (empty($GLOBALS['actions']['affilio_sync_plugin_boxes'])) {
        affilio_init();
    }
    $bound = false;
    foreach ($GLOBALS['actions']['affilio_sync_plugin_boxes'] as $callback) {
        if (is_array($callback) && $callback[1] === 'sync_known_boxes') {
            $bound = true;
        }
    }
    check($bound, 'affilio_sync_plugin_boxes is not bound to sync_known_boxes');
    $schedules = apply_filters('cron_schedules', array());
    check(isset($schedules['affilio_five_minutes']['interval']) && (int) $schedules['affilio_five_minutes']['interval'] === 300, 'five minute cron schedule is missing');

    $GLOBALS['cron'] = array();
    $GLOBALS['wp_rewrite'] = new stdClass();
    affilio_activate();
    check(isset($GLOBALS['cron']['affilio_sync_plugin_boxes']), 'activation did not schedule box sync');
    check($GLOBALS['cron']['affilio_sync_plugin_boxes']['schedule'] === 'affilio_five_minutes', 'activation scheduled the wrong recurrence');

    $GLOBALS['cron'] = array();
    update_option('affilio_seen_version', '0.1.19');
    delete_option('affilio_go_flush_pending');
    affilio_walk_distribution()->maybe_upgrade();
    check(isset($GLOBALS['cron']['affilio_sync_plugin_boxes']), 'upgrade did not schedule box sync');

    foreach ($GLOBALS['actions']['deactivation'] as $callback) {
        call_user_func($callback);
    }
    check(!isset($GLOBALS['cron']['affilio_sync_plugin_boxes']), 'deactivation left affilio_sync_plugin_boxes scheduled');
}

function affilio_walk_b4_failed_acks_retry_with_backoff() {
    affilio_walk_boot();
    $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
    $post = (object) array(
        'ID' => 2615,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Ack',
        'post_content' => affilio_walk_box($id),
    );
    $GLOBALS['posts'] = array($post);
    $GLOBALS['post'] = $post;
    $GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
    $GLOBALS['http'] = array();
    update_option(Affilio_Distribution::ack_queue_option(), array());
    $GLOBALS['http_handler'] = function ($url, $args) {
        return new WP_Error('http_request_failed', 'timeout');
    };
    $distribution = affilio_walk_distribution();
    $distribution->revoke_box(2615, $id);
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    $key = $id . '|removed';
    check(isset($queue[$key]), 'a timed-out removed ack was not queued');
    check((int) $queue[$key]['attempts'] === 1, 'first queued ack did not start at attempt 1');
    $wait = (int) $queue[$key]['next_at'] - time();
    check($wait >= 55 && $wait <= 65, 'first backoff was ' . $wait . ' seconds');
    check(strpos($post->post_content, 'affilio-placement-' . $id) === false, 'the box stayed in the post while the ack was queued');

    $queue[$key]['next_at'] = time() - 1;
    update_option(Affilio_Distribution::ack_queue_option(), $queue);
    $GLOBALS['http'] = array();
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 503), 'body' => '{}');
    };
    $distribution->drain_ack_queue();
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    check(isset($queue[$key]) && (int) $queue[$key]['attempts'] === 2, 'a 503 did not back off the queued ack');
    $wait = (int) $queue[$key]['next_at'] - time();
    check($wait >= 295 && $wait <= 305, 'second backoff was ' . $wait . ' seconds');

    $queue[$key]['next_at'] = time() - 1;
    update_option(Affilio_Distribution::ack_queue_option(), $queue);
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 200), 'body' => '{"plugin_state":"removed"}');
    };
    $distribution->drain_ack_queue();
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    check(!isset($queue[$key]), 'a successful retry left the ack queued');

    $other = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
    $post->post_content = affilio_walk_box($other);
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 422), 'body' => '{"check_id":"rel_sponsored"}');
    };
    $distribution->revoke_box(2615, $other);
    $queue = get_option(Affilio_Distribution::ack_queue_option(), array());
    check(!isset($queue[$other . '|removed']), 'a 422 acknowledgement was queued for retry');
    unset($GLOBALS['http_handler']);
}

function affilio_walk_b5_leave_removed_sends_removed_ack() {
    affilio_walk_boot();
    $id = 'cccccccc-cccc-cccc-cccc-cccccccccccc';
    $post = (object) array(
        'ID' => 2615,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Leave removed',
        'post_content' => affilio_walk_box($id),
    );
    $GLOBALS['posts'] = array($post);
    $GLOBALS['post'] = $post;
    $GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
    update_option('affilio_box_index', array(
        $id => array('post_id' => 2615, 'phase' => 'placed', 'copy' => 'Tool', 'disclosure' => '(link sponsorizzato)'),
    ));
    update_option(Affilio_Distribution::ack_queue_option(), array());
    $api = new Affilio_Walk_API();
    $api->get = array(
        'id' => $id,
        'status' => 'revoked',
        'plugin_state' => 'pending',
        'failed_action' => '',
        'fallback_url' => 'https://shop.example/plain',
    );
    $GLOBALS['http'] = array();
    unset($GLOBALS['http_handler']);
    $distribution = new Affilio_Distribution($api);
    $distribution->sync_known_boxes();
    $acks = d033_ack_bodies();
    check(count($acks) === 1, 'leave removed did not send one ack, sent ' . count($acks));
    check($acks[0]['body']['action'] === 'removed', 'leave removed ack action was ' . (isset($acks[0]['body']['action']) ? $acks[0]['body']['action'] : ''));
    check(strpos($post->post_content, 'affilio-placement-' . $id) === false, 'leave removed left the box in the post');
    check(!array_key_exists('plugin_wait', $api->get), 'the fixture started returning plugin_wait');

    $post->post_content = '<p>Already gone.</p>';
    update_option('affilio_box_index', array(
        $id => array('post_id' => 2615, 'phase' => 'removed', 'copy' => 'Tool', 'disclosure' => '(link sponsorizzato)'),
    ));
    $api->get['plugin_state'] = 'offline';
    $GLOBALS['http'] = array();
    $distribution->sync_known_boxes();
    $acks = d033_ack_bodies();
    check(count($acks) === 1 && $acks[0]['body']['action'] === 'removed', 'an offline leave-removed placement was not acknowledged');

    $api->get['plugin_state'] = 'removed';
    $GLOBALS['http'] = array();
    $distribution->sync_known_boxes();
    check(d033_ack_bodies() === array(), 'a placement that is already removed was acknowledged again');
}

function affilio_walk_b6_go_without_slash_skips_canonical_redirect() {
    affilio_walk_boot();
    if (empty($GLOBALS['actions']['template_redirect'])) {
        affilio_init();
    }
    $id = 'dddddddd-dddd-dddd-dddd-dddddddddddd';
    $_GET = array();
    $_SERVER['REQUEST_URI'] = '/go/' . $id;
    $redirect = apply_filters('redirect_canonical', 'https://blog.example/go/' . $id . '/', 'https://blog.example/go/' . $id);
    check($redirect === false, 'canonical redirect still rewrites /go/<id> without a slash');
    $_SERVER['REQUEST_URI'] = '/go/' . $id . '/';
    $redirect = apply_filters('redirect_canonical', 'https://blog.example/elsewhere/', 'https://blog.example/go/' . $id . '/');
    check($redirect === false, 'canonical redirect still rewrites /go/<id>/');
    $_SERVER['REQUEST_URI'] = '/a-normal-page';
    $redirect = apply_filters('redirect_canonical', 'https://blog.example/a-normal-page/', 'https://blog.example/a-normal-page');
    check($redirect === 'https://blog.example/a-normal-page/', 'a normal canonical redirect was suppressed');
    $priority = null;
    if (!empty($GLOBALS['action_meta']['template_redirect'])) {
        foreach ($GLOBALS['action_meta']['template_redirect'] as $index => $meta) {
            $callback = $GLOBALS['actions']['template_redirect'][$index];
            if (is_array($callback) && is_object($callback[0]) && $callback[1] === 'handle') {
                $priority = (int) $meta['priority'];
            }
        }
    }
    check($priority === 0, 'the go handler is not registered ahead of redirect_canonical');
}

function affilio_walk_b7_publish_matches_article_url_or_post_id() {
    affilio_walk_boot();
    $id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee';
    $GLOBALS['walk_placement_id'] = $id;
    $post = (object) array(
        'ID' => 77,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Article',
        'post_content' => '<p>Existing article.</p>',
    );
    $GLOBALS['posts'] = array($post);
    $GLOBALS['post'] = $post;
    $GLOBALS['allowed'] = true;
    $GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
    unset($GLOBALS['http_handler']);
    $api = new Affilio_Walk_API();
    $distribution = new Affilio_Distribution($api);
    $request = new Affilio_Walk_Request(array('post_id' => 77, 'placement_id' => $id));

    $api->get = array('id' => $id, 'status' => 'approved', 'plugin_state' => 'pending');
    $flat = $distribution->publish($request);
    check(!is_wp_error($flat) && !empty($flat['published']), 'status payload without an article URL was refused');

    $post->post_content = '<p>Existing article.</p>';
    $api->get = array('id' => $id, 'article_url' => 'https://blog.example/post');
    $by_article = $distribution->publish($request);
    check(!is_wp_error($by_article) && !empty($by_article['published']), 'matching article_url was refused');

    $post->post_content = '<p>Existing article.</p>';
    $api->get = array('id' => $id, 'wp_post_id' => 77);
    $by_post = $distribution->publish($request);
    check(!is_wp_error($by_post) && !empty($by_post['published']), 'matching wp_post_id was refused');

    $post->post_content = '<p>Existing article.</p>';
    $api->get = array('id' => $id, 'article_url' => 'https://blog.example/other', 'post_id' => 77);
    $either = $distribution->publish($request);
    check(!is_wp_error($either) && !empty($either['published']), 'a matching post id was refused when article_url differed');

    $api->get = array('id' => $id, 'article_url' => 'https://blog.example/other');
    $wrong_article = $distribution->publish($request);
    check(is_wp_error($wrong_article) && (int) $wrong_article->data['status'] === 409, 'a different article_url was published');

    $api->get = array('id' => $id, 'post_id' => 8);
    $wrong_post = $distribution->publish($request);
    check(is_wp_error($wrong_post) && (int) $wrong_post->data['status'] === 409, 'a different post id was published');

    $api->get = array('placement' => array('surface_url' => 'https://blog.example/another-post'));
    $wrong_surface = $distribution->publish($request);
    check(is_wp_error($wrong_surface) && (int) $wrong_surface->data['status'] === 409, 'a different surface_url was published');

    $post->post_content = '<p>Existing article.</p>';
    $api->get = array('placement' => array('surface_url' => 'https://blog.example/post/'));
    $surface = $distribution->publish($request);
    check(!is_wp_error($surface) && !empty($surface['published']), 'matching surface_url fallback was refused');
}

function affilio_walk_circuit_opens_only_for_5xx_and_timeouts() {
    affilio_walk_boot();
    $id = 'ffffffff-ffff-ffff-ffff-ffffffffffff';
    update_option(AFFILIO_GO_FALLBACKS_OPTION, array($id => 'https://shop.example/fallback'));
    update_option(AFFILIO_GO_CIRCUIT_OPTION, 0);
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 404), 'headers' => array());
    };
    $go = new Affilio_Go();
    check($go->resolve($id) === 'https://shop.example/fallback', 'a 404 did not use the fallback');
    check((int) get_option(AFFILIO_GO_CIRCUIT_OPTION, 0) === 0, 'a 404 opened the circuit');
    $GLOBALS['http_handler'] = function ($url, $args) {
        return array('response' => array('code' => 503), 'headers' => array());
    };
    $go->resolve($id);
    check((int) get_option(AFFILIO_GO_CIRCUIT_OPTION, 0) >= time() + 59, 'a 503 did not open the circuit');
    update_option(AFFILIO_GO_CIRCUIT_OPTION, 0);
    $GLOBALS['http_handler'] = function ($url, $args) {
        return new WP_Error('http_request_failed', 'cURL error 28: Operation timed out');
    };
    $go->resolve($id);
    check((int) get_option(AFFILIO_GO_CIRCUIT_OPTION, 0) >= time() + 59, 'a timeout did not open the circuit');
    unset($GLOBALS['http_handler']);
}

function affilio_walk_upgrade_then_front_page() {
    affilio_walk_boot();
    unset($GLOBALS['flush_rewrite_rules_hook']);
    unset($GLOBALS['http_handler']);
    $id = 'abababab-abab-abab-abab-abababababab';
    $post = (object) array(
        'ID' => 2615,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Front page story',
        'post_content' => '<p class="affilio-placement-' . $id . '"><a href="/go/' . $id . '" rel="sponsored nofollow">Vedi</a></p>',
    );
    $GLOBALS['posts'] = array($post);
    $GLOBALS['post'] = $post;
    $GLOBALS['options']['affilio_seen_version'] = '0.1.19';
    $GLOBALS['options']['affilio_api_key'] = 'site-key';
    $GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
    delete_option('affilio_go_flush_pending');
    $GLOBALS['http'] = array();
    $GLOBALS['flushes'] = 0;
    $GLOBALS['rewrites'] = array();
    $GLOBALS['get_query_var_calls'] = 0;
    $GLOBALS['wp_query'] = null;
    $GLOBALS['wp_rewrite'] = null;
    $_GET = array();
    $_SERVER['REQUEST_URI'] = '/';
    try {
        affilio_walk_do_action('plugins_loaded');
    } catch (Throwable $error) {
        check(false, 'plugin load fataled the front page: ' . $error->getMessage());
    }
    check(get_option('affilio_seen_version') === '0.1.19', 'the upgrade ran during plugin load');
    check($GLOBALS['rewrites'] === array() || $GLOBALS['rewrites'] === null, 'rewrite rules were added during plugin load');
    check((int) $GLOBALS['get_query_var_calls'] === 0, 'the front page called get_query_var during plugin load');

    $GLOBALS['wp_rewrite'] = new stdClass();
    $GLOBALS['wp_query'] = new stdClass();
    try {
        affilio_walk_do_action('init');
    } catch (Throwable $error) {
        check(false, 'upgrade on init fataled: ' . $error->getMessage());
    }
    check(get_option('affilio_seen_version') === AFFILIO_VERSION, 'the upgrade did not save affilio_seen_version');
    check($GLOBALS['flushes'] >= 1, 'the upgrade did not flush rewrite rules');
    $acks = d033_ack_bodies();
    check(count($acks) === 1 && $acks[0]['body']['action'] === 'placed', 'the upgrade did not acknowledge the existing box');

    $_SERVER['REQUEST_URI'] = '/';
    try {
        affilio_walk_do_action('parse_request');
        affilio_walk_do_action('template_redirect');
    } catch (Throwable $error) {
        check(false, 'the front page fataled after the upgrade: ' . $error->getMessage());
    }
    check(http_response_code() === false || (int) http_response_code() < 500, 'the front page returned an error status');
}

if (!class_exists('Affilio_Walk_Query')) {
    class Affilio_Walk_Query {
        private $id;
        public function __construct($id) { $this->id = $id; }
        public function get($var, $default = '') {
            if ($var === 'affilio_go') {
                return $this->id;
            }
            return $default;
        }
    }
}

function affilio_walk_sync_cron_returns_on_load() {
    affilio_walk_boot();
    $saved_rewrite = isset($GLOBALS['wp_rewrite']) ? $GLOBALS['wp_rewrite'] : null;
    $GLOBALS['wp_rewrite'] = null;
    update_option('affilio_seen_version', AFFILIO_VERSION);
    delete_option('affilio_go_flush_pending');
    $GLOBALS['cron'] = array();
    $GLOBALS['cron_schedule_calls'] = array();
    check(wp_next_scheduled('affilio_sync_plugin_boxes') === false, 'box sync was already scheduled');
    affilio_walk_do_action('init');
    check(isset($GLOBALS['cron']['affilio_sync_plugin_boxes']), 'load did not schedule box sync');
    check($GLOBALS['cron']['affilio_sync_plugin_boxes']['schedule'] === 'affilio_five_minutes', 'load scheduled the wrong recurrence');
    $calls = isset($GLOBALS['cron_schedule_calls']['affilio_sync_plugin_boxes']) ? (int) $GLOBALS['cron_schedule_calls']['affilio_sync_plugin_boxes'] : 0;
    check($calls === 1, 'load scheduled box sync ' . $calls . ' times');
    $stamp = $GLOBALS['cron']['affilio_sync_plugin_boxes']['time'];
    affilio_walk_do_action('init');
    check((int) $GLOBALS['cron_schedule_calls']['affilio_sync_plugin_boxes'] === 1, 'a second load scheduled another box sync');
    check($GLOBALS['cron']['affilio_sync_plugin_boxes']['time'] === $stamp, 'a second load replaced the scheduled event');
    check(count($GLOBALS['cron']) === 1, 'load created more than one cron event');

    wp_clear_scheduled_hook('affilio_sync_plugin_boxes');
    check(!isset($GLOBALS['cron']['affilio_sync_plugin_boxes']), 'unschedule did not remove the event');
    affilio_walk_do_action('init');
    check(isset($GLOBALS['cron']['affilio_sync_plugin_boxes']), 'load did not restore a deleted event');
    check($GLOBALS['cron']['affilio_sync_plugin_boxes']['schedule'] === 'affilio_five_minutes', 'the restored event used the wrong recurrence');
    check((int) $GLOBALS['cron_schedule_calls']['affilio_sync_plugin_boxes'] === 2, 'restoring the event scheduled it more than once');
    check(count($GLOBALS['cron']) === 1, 'restoring the event created a duplicate');
    affilio_walk_do_action('init');
    check((int) $GLOBALS['cron_schedule_calls']['affilio_sync_plugin_boxes'] === 2, 'a later load duplicated the restored event');
    $GLOBALS['wp_rewrite'] = $saved_rewrite;
}

function affilio_walk_scheduled_sync_removes_revoked_box() {
    affilio_walk_boot();
    if (empty($GLOBALS['actions']['affilio_sync_plugin_boxes'])) {
        affilio_init();
    }
    $sync = null;
    foreach ($GLOBALS['actions']['affilio_sync_plugin_boxes'] as $callback) {
        if (is_array($callback) && $callback[1] === 'sync_known_boxes') {
            $sync = $callback;
            break;
        }
    }
    check($sync !== null, 'affilio_sync_plugin_boxes is not bound to sync_known_boxes');
    $GLOBALS['actions']['affilio_sync_plugin_boxes'] = array($sync);
    $id = '12121212-1212-1212-1212-121212121212';
    $post = (object) array(
        'ID' => 4242,
        'post_type' => 'post',
        'post_status' => 'publish',
        'post_title' => 'Revoked on the server',
        'post_content' => affilio_walk_box($id),
    );
    $GLOBALS['posts'] = array($post);
    $GLOBALS['post'] = $post;
    $GLOBALS['options']['affilio_plugin_key'] = 'plugin-only-key';
    update_option('affilio_box_index', array(
        $id => array('post_id' => 4242, 'phase' => 'placed', 'copy' => 'Tool', 'disclosure' => '(link sponsorizzato)'),
    ));
    update_option(Affilio_Distribution::ack_queue_option(), array());
    $GLOBALS['http'] = array();
    unset($GLOBALS['http_handler']);
    $GLOBALS['affilio_api_request'] = function ($method, $path) use ($id) {
        if ($method === 'GET' && strpos((string) $path, '/placements/' . $id) !== false) {
            return array('code' => 200, 'data' => array(
                'id' => $id,
                'status' => 'revoked',
                'plugin_state' => 'pending',
            ));
        }
        return array('code' => 200, 'data' => array());
    };
    affilio_walk_do_action('affilio_sync_plugin_boxes');
    unset($GLOBALS['affilio_api_request']);
    check(strpos($post->post_content, 'affilio-placement-' . $id) === false, 'scheduled sync left a revoked box on the post');
    $acks = d033_ack_bodies();
    check(count($acks) === 1 && $acks[0]['body']['action'] === 'removed', 'scheduled sync did not acknowledge the removal');
}

function affilio_walk_run_all() {
    affilio_walk_b1_front_page_does_not_call_get_query_var();
    affilio_walk_b2_upgrade_saves_marker_after_partial_failure();
    affilio_walk_b3_sync_cron_on_activation_upgrade_and_deactivation();
    affilio_walk_b4_failed_acks_retry_with_backoff();
    affilio_walk_b5_leave_removed_sends_removed_ack();
    affilio_walk_b6_go_without_slash_skips_canonical_redirect();
    affilio_walk_b7_publish_matches_article_url_or_post_id();
    affilio_walk_circuit_opens_only_for_5xx_and_timeouts();
    affilio_walk_upgrade_then_front_page();
    affilio_walk_sync_cron_returns_on_load();
    affilio_walk_scheduled_sync_removes_revoked_box();
    echo "D-033 walk blockers passed.\n";
}

$affilio_walk_script = '';
if (isset($_SERVER['SCRIPT_FILENAME']) && $_SERVER['SCRIPT_FILENAME'] !== '') {
    $affilio_walk_script = (string) realpath($_SERVER['SCRIPT_FILENAME']);
}
if ($affilio_walk_script === '' && isset($argv[0])) {
    $affilio_walk_script = (string) realpath($argv[0]);
}
if ($affilio_walk_script === realpath(__FILE__)) {
    affilio_walk_run_all();
}
