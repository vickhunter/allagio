<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class ApproveInsertCopyTest extends TestCase {
    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_disclosure_follows_locale_and_is_a_sanitized_block() {
        $this->boot_stubs();
        $this->require_inserter();

        $italian = 'Nota: alcuni link in questa pagina sono affiliati. Se acquisti tramite questi link, posso ricevere una commissione, senza costi extra per te.';
        $english = 'Disclosure: This post may contain affiliate links, which means we may earn a commission if you click and make a purchase.';
        $inserter = new Affilio_Inserter(null);

        $this->set_post("<!-- wp:paragraph -->\n<p>Intro with a quote's mark.</p>\n<!-- /wp:paragraph -->", 'it_IT');
        $GLOBALS['affilio_site_locale'] = 'en_US';
        $inserter->insert_inline_mention(7, '<p>Una menzione.</p><script>alert(1)</script>', 'end_of_article');
        $stored = $GLOBALS['affilio_post']->post_content;
        $this->assertStringContainsString($italian, $stored, 'it_IT post did not get the approved Italian disclosure');
        $this->assertStringNotContainsString($english, $stored);
        $this->assertMatchesRegularExpression(
            '/<!-- wp:paragraph -->\s*<p class="affilio-disclosure"><em>' . preg_quote($italian, '/') . '<\/em><\/p>\s*<!-- \/wp:paragraph -->/',
            $stored
        );
        $this->assertStringNotContainsString('<script>', $stored, 'mention HTML was not passed through wp_kses_post');
        $this->assertStringContainsString("quote\\'s", $GLOBALS['affilio_update_raw'], 'post content was not wp_slash-ed before wp_update_post');
        $this->assertStringContainsString("quote's", $stored);

        $this->set_post('<p>A plain English post.</p>', 'en_US');
        $inserter->insert_inline_mention(7, '<p>A mention.</p>', 'end_of_article');
        $stored = $GLOBALS['affilio_post']->post_content;
        $this->assertStringContainsString($english, $stored);
        $this->assertStringNotContainsString('commissione', $stored);
        $this->assertStringNotContainsString('<!-- wp:paragraph -->', $stored);

        $this->set_post('<p>Articolo senza locale proprio.</p>', '');
        $GLOBALS['affilio_site_locale'] = 'it_IT';
        $inserter->insert_inline_mention(7, '<p>Una menzione.</p>', 'end_of_article');
        $this->assertStringContainsString($italian, $GLOBALS['affilio_post']->post_content, 'site locale it_IT was not used when the post has no locale');

        $this->set_post('<p>No locale anywhere.</p>', '');
        $GLOBALS['affilio_site_locale'] = '';
        $inserter->insert_inline_mention(7, '<p>A mention.</p>', 'end_of_article');
        $stored = $GLOBALS['affilio_post']->post_content;
        $this->assertStringContainsString($english, $stored, 'missing locale did not fall back to English');
        $this->assertStringNotContainsString('commissione', $stored);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_approve_insert_ignores_caller_html() {
        $this->boot_stubs();
        $this->require_inserter();
        $api = new Affilio_Approve_Insert_Fake_API();
        $proxy = new Affilio_Editor_Proxy($api, new Affilio_Inserter($api));
        $this->set_post('<p>Published article.</p>', 'en_US');
        $leak = '<p>strong contextual fit. Performance-adjusted (+4 from 9 historical outcomes).</p>';

        $response = $this->dispatch_approve($proxy, array(
            'post_id' => 7,
            'recommendation_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
            'html' => $leak,
        ));

        $this->assertInstanceOf(WP_REST_Response::class, $response);
        $generate = $this->generate_body($api);
        $this->assertIsArray($generate);
        $this->assertArrayNotHasKey('html', $generate, 'approve-insert forwarded the caller html field');
        $this->assertStringNotContainsString('strong contextual fit', $GLOBALS['affilio_post']->post_content);
        $this->assertStringNotContainsString('Performance-adjusted', $GLOBALS['affilio_post']->post_content);
        $this->assertStringContainsString('A useful option for this article', $GLOBALS['affilio_post']->post_content);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_approve_insert_language_is_post_then_site_then_english() {
        $this->boot_stubs();
        $this->require_inserter();
        $api = new Affilio_Approve_Insert_Fake_API();
        $proxy = new Affilio_Editor_Proxy($api, new Affilio_Inserter($api));

        $this->set_post('<p>Articolo.</p>', 'it_IT');
        $GLOBALS['affilio_site_locale'] = 'en_US';
        $this->dispatch_approve($proxy, array(
            'post_id' => 7,
            'recommendation_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        ));
        $this->assertSame('it_IT', $this->generate_body($api)['site_language'] ?? null, 'post locale was not sent');

        $api->calls = array();
        $this->set_post('<p>Article without its own locale.</p>', '');
        $GLOBALS['affilio_site_locale'] = 'fr_FR';
        $this->dispatch_approve($proxy, array(
            'post_id' => 7,
            'recommendation_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        ));
        $this->assertSame('fr_FR', $this->generate_body($api)['site_language'] ?? null, 'site locale was not used');

        $api->calls = array();
        $this->set_post('<p>Article with no locale at all.</p>', '');
        $GLOBALS['affilio_site_locale'] = '';
        $this->dispatch_approve($proxy, array(
            'post_id' => 7,
            'recommendation_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        ));
        $this->assertSame('en', $this->generate_body($api)['site_language'] ?? null, 'missing locale did not fall back to en');
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_approve_insert_uses_tracked_go_link() {
        $this->boot_stubs();
        if (!function_exists('home_url')) {
            function home_url($path = '') {
                return 'https://blog.example' . $path;
            }
        }
        if (!function_exists('esc_url')) {
            function esc_url($value) {
                return (string) $value;
            }
        }
        require_once dirname(__DIR__, 2) . '/includes/class-affilio-go.php';
        $this->require_inserter();
        $api = new Affilio_Approve_Insert_Fake_API();
        $id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
        $api->generate = array(
            'code' => 200,
            'data' => array(
                'html' => '<p><a href="https://example.com/tool?ref=staging" rel="nofollow sponsored">Tool</a> — Una frase utile per questo articolo.</p>',
                'suggested_position' => 'end_of_article',
                'tracked_link' => '/v1/go/' . $id,
                'placement_reason' => 'Strong contextual fit. Score 80.',
            ),
        );
        $proxy = new Affilio_Editor_Proxy($api, new Affilio_Inserter($api));
        $this->set_post('<p>Articolo pubblicato.</p>', 'it_IT');

        $response = $this->dispatch_approve($proxy, array(
            'post_id' => 7,
            'recommendation_id' => $id,
        ));

        $this->assertInstanceOf(WP_REST_Response::class, $response);
        $this->assertSame(200, $response->status);
        $stored = $GLOBALS['affilio_post']->post_content;
        $this->assertStringContainsString('https://blog.example/go/' . $id, $stored);
        $this->assertStringNotContainsString('example.com/tool', $stored);
        $this->assertStringNotContainsString('ref=staging', $stored);
        $this->assertStringContainsString('rel="nofollow sponsored"', $stored);
        $italian = 'Nota: alcuni link in questa pagina sono affiliati. Se acquisti tramite questi link, posso ricevere una commissione, senza costi extra per te.';
        $this->assertSame(1, substr_count($stored, $italian));
        $this->assertStringNotContainsString('Strong contextual', $stored);
        $this->assertStringNotContainsString('Score 80', $stored);
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_approve_insert_writes_nothing_when_generate_fails() {
        $this->boot_stubs();
        $this->require_inserter();
        $api = new Affilio_Approve_Insert_Fake_API();
        $api->generate = array('code' => 422, 'data' => array('error' => 'no reader-facing copy'));
        $proxy = new Affilio_Editor_Proxy($api, new Affilio_Inserter($api));
        $this->set_post('<p>Untouched article.</p>', 'en_US');
        $before = $GLOBALS['affilio_post']->post_content;

        $response = $this->dispatch_approve($proxy, array(
            'post_id' => 7,
            'recommendation_id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
            'html' => '<p>strong contextual fit</p>',
        ));

        $this->assertSame($before, $GLOBALS['affilio_post']->post_content);
        $this->assertSame(0, $GLOBALS['affilio_updates']);
        $this->assertInstanceOf(WP_REST_Response::class, $response);
        $this->assertSame(422, $response->status);
    }

    private function set_post($content, $locale) {
        $post = (object) array(
            'ID' => 7,
            'post_status' => 'publish',
            'post_type' => 'post',
            'post_title' => 'Tools',
            'post_content' => $content,
            'locale' => $locale,
        );
        $GLOBALS['affilio_post'] = $post;
        $GLOBALS['posts'] = array($post);
        $GLOBALS['affilio_meta'] = array();
        $GLOBALS['affilio_updates'] = 0;
        $GLOBALS['affilio_update_raw'] = '';
    }

    private function generate_body($api) {
        foreach ($api->calls as $call) {
            if ($call['path'] === '/generate/content') {
                return $call['body'];
            }
        }
        $this->fail('approve-insert did not call /generate/content');
    }

    private function dispatch_approve($proxy, $params) {
        if (function_exists('rest_do_request')) {
            $request = new WP_REST_Request('POST', '/affilio/v1/editor/approve-insert');
            $request->set_body_params($params);
            $request->set_param('post_id', $params['post_id']);
            $request->set_param('recommendation_id', $params['recommendation_id']);
            if (isset($params['html'])) {
                $request->set_param('html', $params['html']);
            }
            return rest_do_request($request);
        }
        $proxy->register_routes();
        $request = new Affilio_Test_Request($params);
        foreach ($GLOBALS['routes'] as $route) {
            if ($route[0] !== 'affilio/v1' || $route[1] !== '/editor/approve-insert') {
                continue;
            }
            $spec = $route[2];
            $allowed = call_user_func($spec['permission_callback'], $request);
            if ($allowed !== true) {
                return new WP_Error('rest_forbidden', 'Sorry, you are not allowed to do that.', array('status' => 403));
            }
            return call_user_func($spec['callback'], $request);
        }
        $this->fail('POST /affilio/v1/editor/approve-insert is not registered');
    }

    private function require_inserter() {
        require_once dirname(__DIR__, 2) . '/includes/class-affilio-distribution.php';
        require_once dirname(__DIR__, 2) . '/includes/class-affilio-inserter.php';
        require_once dirname(__DIR__, 2) . '/includes/class-affilio-editor-proxy.php';
    }

    private function boot_stubs() {
        if (function_exists('affilio_review_stubs_ready')) {
            return;
        }
        function affilio_review_stubs_ready() {
            return true;
        }
        if (!defined('ABSPATH')) {
            define('ABSPATH', __DIR__);
        }
        $GLOBALS['routes'] = array();
        $GLOBALS['affilio_site_locale'] = 'en_US';
        $GLOBALS['affilio_meta'] = array();
        $GLOBALS['affilio_updates'] = 0;
        $GLOBALS['affilio_update_raw'] = '';
        if (!function_exists('add_action')) {
            function add_action($name, $callback, $priority = 10, $accepted_args = 1) {
            }
        }
        if (!function_exists('register_rest_route')) {
            function register_rest_route($namespace, $path, $args) {
                $GLOBALS['routes'][] = array($namespace, $path, $args);
            }
        }
        if (!function_exists('current_user_can')) {
            function current_user_can($capability, $id = null) {
                return true;
            }
        }
        if (!function_exists('get_post')) {
            function get_post($id) {
                $post = isset($GLOBALS['affilio_post']) ? $GLOBALS['affilio_post'] : null;
                if ($post && (int) $post->ID === (int) $id) {
                    return $post;
                }
                return null;
            }
        }
        if (!function_exists('get_locale')) {
            function get_locale() {
                return isset($GLOBALS['affilio_site_locale']) ? (string) $GLOBALS['affilio_site_locale'] : '';
            }
        }
        if (!function_exists('get_post_meta')) {
            function get_post_meta($id, $key, $single = false) {
                return isset($GLOBALS['affilio_meta'][$key]) ? $GLOBALS['affilio_meta'][$key] : '';
            }
        }
        if (!function_exists('update_post_meta')) {
            function update_post_meta($id, $key, $value) {
                $GLOBALS['affilio_meta'][$key] = $value;
                return true;
            }
        }
        if (!function_exists('wp_kses_post')) {
            function wp_kses_post($value) {
                return preg_replace('/<script\b[^>]*>.*?<\/script>/is', '', (string) $value);
            }
        }
        if (!function_exists('wp_slash')) {
            function wp_slash($value) {
                return addslashes((string) $value);
            }
        }
        if (!function_exists('wp_update_post')) {
            function wp_update_post($values, $error = false) {
                $GLOBALS['affilio_updates']++;
                $GLOBALS['affilio_update_raw'] = $values['post_content'];
                $GLOBALS['affilio_post']->post_content = stripslashes($values['post_content']);
                return $values['ID'];
            }
        }
        if (!function_exists('sanitize_key')) {
            function sanitize_key($value) {
                return strtolower((string) $value);
            }
        }
        if (!function_exists('wp_strip_all_tags')) {
            function wp_strip_all_tags($value) {
                return strip_tags((string) $value);
            }
        }
        if (!function_exists('esc_html')) {
            function esc_html($value) {
                return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8');
            }
        }
        if (!function_exists('is_wp_error')) {
            function is_wp_error($value) {
                return $value instanceof WP_Error;
            }
        }
    }
}

if (!class_exists('WP_Error')) {
    class WP_Error {
        public $code;
        public $message;
        public $data;
        public function __construct($code, $message, $data = array()) {
            $this->code = $code;
            $this->message = $message;
            $this->data = $data;
        }
        public function get_error_message() {
            return $this->message;
        }
        public function get_error_data($code = '') {
            return $this->data;
        }
    }
}

if (!class_exists('WP_REST_Response')) {
    class WP_REST_Response {
        public $data;
        public $status;
        public function __construct($data, $status = 200) {
            $this->data = $data;
            $this->status = $status;
        }
    }
}

if (!class_exists('Affilio_Test_Request')) {
    class Affilio_Test_Request {
        private $params;
        public function __construct($params = array()) {
            $this->params = $params;
        }
        public function get_param($name) {
            return isset($this->params[$name]) ? $this->params[$name] : null;
        }
        public function get_json_params() {
            return $this->params;
        }
    }
}

if (!class_exists('Affilio_Approve_Insert_Fake_API')) {
    class Affilio_Approve_Insert_Fake_API {
        public $calls = array();
        public $generate = array(
            'code' => 200,
            'data' => array(
                'html' => '<p><a href="https://example.com/tool">Tool</a> — A useful option for this article.</p>',
                'suggested_position' => 'end_of_article',
            ),
        );
        public function request($method, $path, $body = null) {
            $this->calls[] = array('method' => $method, 'path' => $path, 'body' => $body);
            if ($path === '/articles') {
                return array('code' => 200, 'data' => array('articles' => array(array(
                    'id' => 'article-1',
                    'wp_post_id' => 7,
                ))));
            }
            if (strpos($path, '/articles/') === 0) {
                return array('code' => 200, 'data' => array('recommendations' => array(array(
                    'id' => 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
                ))));
            }
            if ($path === '/generate/content') {
                return $this->generate;
            }
            if (strpos($path, '/recommendations/') === 0) {
                return array('code' => 200, 'data' => array('status' => 'approved'));
            }
            if ($path === '/insert/confirm') {
                return array('code' => 200, 'data' => array('status' => 'confirmed'));
            }
            return array('code' => 404, 'data' => array());
        }
    }
}
