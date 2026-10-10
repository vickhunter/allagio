<?php

if (!function_exists('add_filter')) {
    function add_filter($name, $callback) {
        if (!isset($GLOBALS['filters']) || !is_array($GLOBALS['filters'])) {
            $GLOBALS['filters'] = array();
        }
        $GLOBALS['filters'][$name][] = $callback;
    }
}

if (!function_exists('remove_filter')) {
    function remove_filter($name, $callback) {
        if (!isset($GLOBALS['filters'][$name]) || !is_array($GLOBALS['filters'][$name])) {
            return;
        }
        $kept = array();
        foreach ($GLOBALS['filters'][$name] as $existing) {
            if ($existing !== $callback) {
                $kept[] = $existing;
            }
        }
        $GLOBALS['filters'][$name] = $kept;
    }
}

if (class_exists('Affilio_Distribution')) {
    return;
}

class Affilio_Distribution {
    private $api;
    private $publisher_checked = false;
    private $publisher_error = null;

    public function __construct($api) {
        $this->api = $api;
    }

    public static function without_revisions($callback) {
        $keep = function ($count = null, $post = null) {
            return 0;
        };
        add_filter('wp_revisions_to_keep', $keep);
        try {
            return call_user_func($callback);
        } finally {
            remove_filter('wp_revisions_to_keep', $keep);
        }
    }

    private function save_content($post_id, $content) {
        return self::without_revisions(function () use ($post_id, $content) {
            return wp_update_post(array('ID' => $post_id, 'post_content' => wp_slash($content)), true);
        });
    }

    public static function disclosure_suppressed($content) {
        return self::has_receipt_disclosure($content, '');
    }

    private static function has_receipt_disclosure($content, $approved) {
        if ($approved !== '' && strpos((string) $content, (string) $approved) !== false) {
            return true;
        }
        $text = mb_strtolower(wp_strip_all_tags((string) $content), 'UTF-8');
        if (mb_strpos($text, 'commissione', 0, 'UTF-8') !== false) {
            return true;
        }
        return (bool) preg_match('/link.{0,60}affiliat|affiliat.{0,60}link/u', $text);
    }

    private static function disclosure_excerpt($content) {
        $text = wp_strip_all_tags((string) $content);
        $lower = mb_strtolower($text, 'UTF-8');
        $at = null;
        if (preg_match('/link.{0,60}affiliat|affiliat.{0,60}link/u', $lower, $match, PREG_OFFSET_CAPTURE)) {
            $at = mb_strlen(substr($lower, 0, $match[0][1]), 'UTF-8');
        }
        $commissione = mb_strpos($lower, 'commissione', 0, 'UTF-8');
        if ($commissione !== false && ($at === null || $commissione < $at)) {
            $at = $commissione;
        }
        if ($at === null) {
            return '';
        }
        return trim(mb_substr($text, max(0, $at - 80), 240, 'UTF-8'));
    }

    private static function receipt_refusal_message($api) {
        $translated = array(
            'published text must include approved copy, disclosure and exact tracked link' => 'Il testo pubblicato deve includere la copia approvata, la disclosure e il link tracciato esatto.',
            'receipt already recorded or placement unavailable' => 'La ricevuta risulta già registrata, oppure il placement non è disponibile.',
            'receipt recording failed' => 'La registrazione della pubblicazione non è riuscita.',
        );
        $detail = isset($translated[$api]) ? $translated[$api] : 'Il servizio ha rifiutato la pubblicazione.';
        return $detail . ' L\'inserimento è stato annullato.';
    }

    public function allowed() {
        return current_user_can('manage_options');
    }

    public function rest_permission() {
        if ($this->allowed()) {
            return true;
        }
        return new WP_Error('affilio_forbidden', 'Administrator access required.', array('status' => 403));
    }

    public function register_routes() {
        $routes = array(
            '/distribution' => array('GET', 'status'),
            '/distribution/connect' => array('POST', 'connect'),
            '/distribution/articles' => array('GET', 'articles'),
            '/distribution/opportunities' => array('POST', 'opportunities'),
            '/distribution/prepare' => array('POST', 'prepare'),
            '/distribution/placements' => array('GET', 'placements'),
            '/distribution/publish' => array('POST', 'publish'),
        );
        foreach ($routes as $path => $route) {
            register_rest_route('affilio/v1', $path, array('methods' => $route[0], 'callback' => array($this, $route[1]), 'permission_callback' => array($this, 'rest_permission')));
        }
    }

    private function origin() {
        return preg_replace('#/v1/?$#', '', Affilio_API::resolve_base_url());
    }

    private function paragraph_block($html, $class_name = '') {
        $attrs = '';
        if ($class_name !== '') {
            $attrs = ' ' . self::block_json(array('className' => $class_name));
        }
        return '<!-- wp:paragraph' . $attrs . " -->\n" . $html . "\n<!-- /wp:paragraph -->";
    }

    private static function block_json($data) {
        return json_encode($data, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    }

    private function request($method, $path, $body = null) {
        $response = $this->api->request($method, $path, $body);
        if (is_wp_error($response)) {
            return new WP_Error('affilio_distribution', 'Il servizio non è raggiungibile. Riprova tra qualche istante.', array('status' => 503));
        }
        if ($response['code'] < 200 || $response['code'] >= 300) {
            $message = isset($response['data']['error']) ? $response['data']['error'] : 'The distribution service could not complete this request.';
            return new WP_Error('affilio_distribution', $message, array('status' => $response['code'] >= 400 && $response['code'] < 500 ? $response['code'] : 502));
        }
        return $response['data'];
    }

    private function article($request) {
        $id = absint($request->get_param('post_id'));
        $post = get_post($id);
        if (!$post || $post->post_type !== 'post' || $post->post_status !== 'publish' || !current_user_can('edit_post', $id)) {
            return new WP_Error('affilio_post', 'Choose a published article you can edit.', array('status' => 403));
        }
        return $post;
    }

    private function context($post) {
        $text = $post->post_title . "\n" . wp_strip_all_tags($post->post_content);
        return mb_substr($text, 0, 3000, 'UTF-8');
    }

    public function status() {
        $connection = $this->api->get_connection_status();
        $publisher = $connection['connected'] ? $this->request('GET', '/sites/owner-connection') : null;
        return rest_ensure_response(array('site_connected' => $connection['connected'], 'publisher_connected' => is_array($publisher) && !empty($publisher['connected']), 'native_available' => is_array($publisher), 'owner_url' => $this->origin() . '/owner', 'settings_url' => admin_url('admin.php?page=affilio-settings')));
    }

    public function connect() {
        $data = $this->request('POST', '/sites/owner-connection', array());
        if (is_wp_error($data)) {
            return $data;
        }
        $expected = $this->origin() . '/owner/connect-site?';
        if (!isset($data['connect_url']) || strpos($data['connect_url'], $expected) !== 0) {
            return new WP_Error('affilio_owner_url', 'The service returned an invalid connection link.', array('status' => 502));
        }
        return rest_ensure_response($data);
    }

    public function articles() {
        $posts = get_posts(array('post_type' => 'post', 'post_status' => 'publish', 'posts_per_page' => 100, 'orderby' => 'date', 'order' => 'DESC'));
        $articles = array();
        foreach ($posts as $post) {
            if (current_user_can('edit_post', $post->ID)) {
                $articles[] = array('id' => $post->ID, 'title' => $post->post_title, 'url' => get_permalink($post));
            }
        }
        return rest_ensure_response(array('articles' => $articles));
    }

    public function opportunities($request) {
        $publisher = $this->require_publisher_connection();
        if (is_wp_error($publisher)) {
            return $publisher;
        }
        $post = $this->article($request);
        if (is_wp_error($post)) {
            return $post;
        }
        return $this->request('POST', '/revenue/opportunities', array('context' => $this->context($post)));
    }

    public function prepare($request) {
        $publisher = $this->require_publisher_connection();
        if (is_wp_error($publisher)) {
            return $publisher;
        }
        $post = $this->article($request);
        if (is_wp_error($post)) {
            return $post;
        }
        return $this->request('POST', '/placements', array('offer_id' => (string) $request->get_param('offer_id'), 'surface_url' => get_permalink($post), 'context' => $this->context($post), 'copy' => (string) $request->get_param('copy')));
    }

    public function placements() {
        $data = $this->request('GET', '/placements');
        if (is_wp_error($data) || !is_array($data) || empty($data['placements']) || !is_array($data['placements']) || !function_exists('get_posts') || !function_exists('wp_nonce_url')) {
            return is_array($data) ? rest_ensure_response($data) : $data;
        }
        $by_url = array();
        foreach (get_posts(array('post_type' => 'post', 'post_status' => 'publish', 'posts_per_page' => 100, 'orderby' => 'date', 'order' => 'DESC')) as $post) {
            $by_url[get_permalink($post)] = $post;
        }
        foreach ($data['placements'] as $index => $placement) {
            if (!is_array($placement) || empty($placement['id']) || !empty($placement['removed_at'])) {
                continue;
            }
            $url = isset($placement['surface_url']) ? $placement['surface_url'] : '';
            if (!isset($by_url[$url]) || !current_user_can('edit_post', $by_url[$url]->ID)) {
                continue;
            }
            $post_id = (int) $by_url[$url]->ID;
            $data['placements'][$index]['revert_url'] = wp_nonce_url(admin_url('admin.php?page=affilio-revert&post_id=' . $post_id . '&placement=' . rawurlencode($placement['id'])), 'affilio_revert_' . $post_id . '_' . $placement['id']);
        }
        return rest_ensure_response($data);
    }

    private function require_publisher_connection() {
        if (!$this->publisher_checked) {
            $this->publisher_checked = true;
            $data = $this->request('GET', '/sites/owner-connection');
            if (is_wp_error($data)) {
                $this->publisher_error = $data;
            } elseif (empty($data['connected'])) {
                $this->publisher_error = new WP_Error('affilio_publisher', 'Collega prima il tuo account publisher.', array('status' => 403));
            }
        }
        if ($this->publisher_error !== null) {
            return $this->publisher_error;
        }
        return true;
    }

    public function publish($request) {
        $publisher = $this->require_publisher_connection();
        if (is_wp_error($publisher)) {
            return $publisher;
        }
        $post = $this->article($request);
        if (is_wp_error($post)) {
            return $post;
        }
        $id = (string) $request->get_param('placement_id');
        if (!preg_match('/^[a-f0-9-]{36}$/i', $id)) {
            return new WP_Error('affilio_placement', 'Invalid placement.', array('status' => 400));
        }
        $data = $this->request('GET', '/placements/' . $id);
        if (is_wp_error($data)) {
            return $data;
        }
        $placement = $data['placement'];
        if ($placement['surface_url'] !== get_permalink($post)) {
            return new WP_Error('affilio_surface', 'This placement belongs to another article.', array('status' => 409));
        }
        $data = $this->request('POST', '/placements/' . $id . '/commit', array());
        if (is_wp_error($data)) {
            return $data;
        }
        $placement = $data['placement'];
        $link = $placement['receipt']['tracked_link'];
        if (!is_string($link) || strpos($link, '/v1/go/') !== 0 || strpos($link, 'placement=' . $id) === false) {
            return new WP_Error('affilio_link', 'Invalid tracked link.', array('status' => 502));
        }
        $original = $post->post_content;
        $content = $original;
        $inserted = false;
        if (!self::placement_present($content, $id)) {
            $class = 'affilio-placement-' . $id;
            $inner = esc_html($placement['copy']) . ' <a href="' . esc_url($this->origin() . $link) . '" rel="sponsored nofollow">' . esc_html('Vedi il prodotto') . '</a>';
            $content .= "\n" . $this->paragraph_block('<p class="' . $class . '">' . $inner . '</p>', $class);
            if (!self::disclosure_suppressed($content) && strpos($content, esc_html($placement['disclosure'])) === false) {
                $disclosure_class = 'affilio-disclosure-' . $id;
                $content .= "\n" . $this->paragraph_block('<p class="' . $disclosure_class . '">' . esc_html($placement['disclosure']) . '</p>', $disclosure_class);
            }
            $updated = $this->save_content($post->ID, $content);
            if (is_wp_error($updated)) {
                return $updated;
            }
            $inserted = true;
        }
        $current = get_post($post->ID);
        $reported = isset($placement['receipt']['publication_status']) && $placement['receipt']['publication_status'] === 'publisher_reported';
        if (!$reported) {
            $full = html_entity_decode($current->post_content, ENT_QUOTES, 'UTF-8');
            $published = self::published_slice($full, $id);
            if (strpos($current->post_content, esc_html($placement['disclosure'])) !== false && strpos($published, $placement['disclosure']) === false) {
                $published .= "\n" . $placement['disclosure'];
            }
            if (!self::has_receipt_disclosure($published, $placement['disclosure']) && self::has_receipt_disclosure($full, $placement['disclosure'])) {
                $excerpt = self::disclosure_excerpt($full);
                if ($excerpt !== '') {
                    $published = $excerpt . "\n" . $published;
                }
            }
            $receipt = $this->request('POST', '/placements/' . $id . '/receipt', array('surface_url' => get_permalink($post), 'published_text' => $published, 'evidence' => 'WordPress administrator inserted the approved placement in published post ' . $post->ID));
            if (is_wp_error($receipt)) {
                if ($inserted) {
                    $fresh = get_post($post->ID);
                    $reverted = self::revert_content($fresh->post_content, $id);
                    $this->save_content($post->ID, $reverted['content']);
                }
                $status = 502;
                if (is_array($receipt->data) && isset($receipt->data['status'])) {
                    $code = (int) $receipt->data['status'];
                    if ($code < 200 || $code >= 300) {
                        $status = $code;
                    }
                }
                return new WP_Error('affilio_receipt', self::receipt_refusal_message($receipt->get_error_message()), array('status' => $status));
            }
        }
        return rest_ensure_response(array('published' => true, 'url' => get_permalink($post)));
    }

    public static function placement_present($content, $id) {
        foreach (self::locate_blocks($content) as $block) {
            if (self::block_placement_id($block, $id) !== '') {
                return true;
            }
        }
        return false;
    }

    public static function published_slice($content, $id) {
        foreach (self::locate_blocks($content) as $block) {
            if (self::block_placement_id($block, $id) !== '') {
                return substr($content, $block['start']);
            }
        }
        return '';
    }

    public static function revert_content($content, $placement_id = '') {
        $blocks = self::locate_blocks($content);
        $remove = array();
        $ids = array();
        $removed = array();
        foreach ($blocks as $index => $block) {
            $id = self::block_placement_id($block, $placement_id);
            if ($id === '') {
                continue;
            }
            $remove[$index] = true;
            $ids[$id] = true;
            $removed[] = self::describe_block($block, $id, 'placement');
            $next = $index + 1;
            if (self::is_legacy_placement($block) && isset($blocks[$next]) && self::is_legacy_disclosure($blocks[$next])) {
                $remove[$next] = true;
                $removed[] = self::describe_block($blocks[$next], $id, 'disclosure');
            }
        }
        foreach ($blocks as $index => $block) {
            if (isset($remove[$index])) {
                continue;
            }
            $id = self::block_disclosure_id($block, $placement_id);
            if ($id === '') {
                continue;
            }
            $remove[$index] = true;
            $ids[$id] = true;
            $removed[] = self::describe_block($block, $id, 'disclosure');
        }
        if (!$remove) {
            return array('content' => $content, 'matched' => false, 'removed' => array(), 'placement_ids' => array());
        }
        $spans = array();
        foreach (array_keys($remove) as $index) {
            $start = $blocks[$index]['start'];
            $end = $blocks[$index]['end'];
            if ($start > 0 && substr($content, $start - 1, 1) === "\n") {
                $start--;
            }
            $spans[] = array($start, $end);
        }
        usort($spans, function ($a, $b) { return $b[0] - $a[0]; });
        foreach ($spans as $span) {
            $content = substr($content, 0, $span[0]) . substr($content, $span[1]);
        }
        return array('content' => $content, 'matched' => true, 'removed' => $removed, 'placement_ids' => array_keys($ids));
    }

    public function revert_inserted($post_id, $placement_id = '', $report = true, $dry_run = false) {
        $post = get_post($post_id);
        if (!$post) {
            return array('matched' => false, 'removed' => array(), 'api' => array(), 'content' => '', 'title' => '');
        }
        $result = self::revert_content($post->post_content, $placement_id);
        $result['title'] = $post->post_title;
        $result['api'] = array();
        if (!$result['matched'] || $dry_run) {
            return $result;
        }
        $updated = $this->save_content($post->ID, $result['content']);
        if (is_wp_error($updated)) {
            $result['matched'] = false;
            $result['error'] = $updated;
            return $result;
        }
        if ($report) {
            foreach ($result['placement_ids'] as $id) {
                $result['api'][$id] = $this->report_removed($id);
            }
        }
        return $result;
    }

    public function register_revert_page() {
        add_submenu_page(null, 'Rimuovi inserimento', 'Rimuovi inserimento', 'edit_posts', 'affilio-revert', array($this, 'render_revert_screen'));
    }

    public function render_revert_screen() {
        $post_id = isset($_GET['post_id']) ? absint($_GET['post_id']) : 0;
        $placement = isset($_GET['placement']) ? sanitize_text_field(wp_unslash($_GET['placement'])) : '';
        $nonce = isset($_GET['_wpnonce']) ? sanitize_text_field(wp_unslash($_GET['_wpnonce'])) : '';
        $view = $this->confirmation_view($post_id, $placement, $nonce);
        if ($view['status'] !== 200) {
            wp_die($view['message'], '', array('response' => $view['status']));
        }
        echo $view['html'];
    }

    public function confirmation_view($post_id, $placement, $nonce) {
        if (!current_user_can('edit_post', $post_id)) {
            return array('status' => 403, 'message' => 'You cannot edit this post.', 'html' => '');
        }
        if (!wp_verify_nonce($nonce, 'affilio_revert_' . $post_id . '_' . $placement)) {
            return array('status' => 403, 'message' => 'The link expired. Refresh the placements list and try again.', 'html' => '');
        }
        $post = get_post($post_id);
        if (!$post) {
            return array('status' => 404, 'message' => 'Post not found.', 'html' => '');
        }
        $link = self::removal_link($post->post_content, $placement);
        $title = esc_html($post->post_title);
        $anchor = esc_html($link['anchor']);
        $href = esc_html($link['href']);
        $action = esc_url(admin_url('admin-post.php'));
        $back = esc_url(admin_url('admin.php?page=affilio-distribution'));
        $field = wp_nonce_field('affilio_revert_' . $post_id . '_' . $placement, '_wpnonce', true, false);
        $html = '<h1>Rimuovi inserimento</h1>';
        $html .= '<p>Articolo: ' . $title . '</p>';
        $html .= '<p>Link: ' . $anchor . '</p>';
        $html .= '<p>' . $href . '</p>';
        $html .= '<form method="post" action="' . $action . '">';
        $html .= '<input type="hidden" name="action" value="affilio_revert">';
        $html .= '<input type="hidden" name="post_id" value="' . (int) $post_id . '">';
        $html .= '<input type="hidden" name="placement" value="' . esc_html($placement) . '">';
        $html .= $field;
        $html .= '<a href="' . $back . '">Annulla</a> ';
        $html .= '<button type="submit">Rimuovi</button></form>';
        return array('status' => 200, 'message' => '', 'html' => $html, 'anchor' => $link['anchor'], 'href' => $link['href'], 'title' => $post->post_title);
    }

    public function handle_revert_post() {
        $post_id = isset($_POST['post_id']) ? absint($_POST['post_id']) : 0;
        $placement = isset($_POST['placement']) ? sanitize_text_field(wp_unslash($_POST['placement'])) : '';
        $nonce = isset($_POST['_wpnonce']) ? sanitize_text_field(wp_unslash($_POST['_wpnonce'])) : '';
        $done = $this->perform_revert_action($post_id, $placement, $nonce);
        if ($done['status'] !== 200) {
            wp_die($done['message'], '', array('response' => $done['status']));
        }
        wp_safe_redirect($done['redirect']);
        exit;
    }

    public function perform_revert_action($post_id, $placement, $nonce) {
        $view = $this->confirmation_view($post_id, $placement, $nonce);
        if ($view['status'] !== 200) {
            return $view;
        }
        $result = $this->revert_inserted($post_id, $placement, true, false);
        $state = 'unreachable';
        if (!empty($result['matched']) && isset($result['api'][$placement])) {
            $state = $result['api'][$placement];
        }
        $anchor = '';
        $href = '';
        if (!empty($result['removed'])) {
            $anchor = $result['removed'][0]['anchor'];
            $href = $result['removed'][0]['href'];
        }
        $notice = self::removal_notice($result['title'], $anchor, $href, $state, !empty($result['matched']));
        $redirect = add_query_arg(array(
            'page' => 'affilio-distribution',
            'affilio_reverted' => !empty($result['matched']) ? '1' : '0',
            'affilio_notice' => rawurlencode($notice),
        ), admin_url('admin.php'));
        return array('status' => 200, 'message' => '', 'notice' => $notice, 'redirect' => $redirect, 'result' => $result);
    }

    public function render_revert_notice() {
        if (!isset($_GET['affilio_notice']) || !current_user_can('edit_posts')) {
            return;
        }
        $notice = sanitize_text_field(wp_unslash(rawurldecode($_GET['affilio_notice'])));
        if ($notice === '') {
            return;
        }
        echo '<div class="notice notice-success"><p>' . esc_html($notice) . '</p></div>';
    }

    public static function removal_notice($title, $anchor, $href, $api_state, $matched) {
        if (!$matched) {
            return 'Nessun inserimento da rimuovere.';
        }
        $states = array(
            'reported' => 'API: segnalato',
            'already' => 'API: già rimosso',
            'unreachable' => 'API: non raggiungibile, riprovo più tardi',
        );
        $line = isset($states[$api_state]) ? $states[$api_state] : $states['unreachable'];
        return 'Inserimento rimosso da «' . $title . '»: ' . $anchor . ' ' . $href . '. ' . $line;
    }

    public function retry_removals() {
        $queue = get_option('affilio_removal_queue', array());
        if (!is_array($queue) || !$queue) {
            return;
        }
        $now = time();
        foreach ($queue as $id => $item) {
            if (!is_array($item)) {
                unset($queue[$id]);
                continue;
            }
            $next = isset($item['next_at']) ? (int) $item['next_at'] : 0;
            if ($next > $now) {
                continue;
            }
            $state = $this->post_removed($id);
            if ($state === 'reported' || $state === 'already') {
                unset($queue[$id]);
                continue;
            }
            $attempts = (isset($item['attempts']) ? (int) $item['attempts'] : 0) + 1;
            if ($attempts >= 4) {
                unset($queue[$id]);
                continue;
            }
            $queue[$id] = array('attempts' => $attempts, 'next_at' => $now + (60 * (2 ** $attempts)));
        }
        update_option('affilio_removal_queue', $queue, false);
        if ($queue && function_exists('wp_next_scheduled') && function_exists('wp_schedule_single_event') && !wp_next_scheduled('affilio_retry_removals')) {
            wp_schedule_single_event($now + 60, 'affilio_retry_removals');
        }
    }

    private function report_removed($id) {
        $state = $this->post_removed($id);
        if ($state === 'unreachable') {
            $this->enqueue_removal($id);
        } else {
            $this->dequeue_removal($id);
        }
        return $state;
    }

    private function post_removed($id) {
        $data = $this->request('POST', '/placements/' . $id . '/removed', array());
        if (is_wp_error($data)) {
            return 'unreachable';
        }
        if (is_array($data) && !empty($data['already_removed'])) {
            return 'already';
        }
        return 'reported';
    }

    private function enqueue_removal($id) {
        $queue = get_option('affilio_removal_queue', array());
        if (!is_array($queue)) {
            $queue = array();
        }
        if (!isset($queue[$id])) {
            $queue[$id] = array('attempts' => 0, 'next_at' => time());
        }
        update_option('affilio_removal_queue', $queue, false);
        if (function_exists('wp_next_scheduled') && function_exists('wp_schedule_single_event') && !wp_next_scheduled('affilio_retry_removals')) {
            wp_schedule_single_event(time() + 60, 'affilio_retry_removals');
        }
    }

    private function dequeue_removal($id) {
        $queue = get_option('affilio_removal_queue', array());
        if (!is_array($queue) || !isset($queue[$id])) {
            return;
        }
        unset($queue[$id]);
        update_option('affilio_removal_queue', $queue, false);
    }

    private static function removal_link($content, $placement_id) {
        foreach (self::locate_blocks($content) as $block) {
            $id = self::block_placement_id($block, $placement_id);
            if ($id === '') {
                continue;
            }
            $link = self::link_in_html($block['innerHTML']);
            return array('anchor' => $link[0], 'href' => $link[1]);
        }
        return array('anchor' => '', 'href' => '');
    }

    private static function describe_block($block, $id, $kind) {
        $link = self::link_in_html($block['innerHTML']);
        return array('placement_id' => $id, 'kind' => $kind, 'anchor' => $link[0], 'href' => $link[1]);
    }

    private static function link_in_html($html) {
        if (preg_match('/<a\s[^>]*href="([^"]*)"[^>]*>(.*?)<\/a>/s', $html, $match)) {
            return array(html_entity_decode(strip_tags($match[2]), ENT_QUOTES, 'UTF-8'), html_entity_decode($match[1], ENT_QUOTES, 'UTF-8'));
        }
        return array('', '');
    }

    private static function locate_blocks($content) {
        $blocks = array();
        $pattern = '/<!--\s+wp:([a-z0-9\/-]+)(\s+(\{.*?\}))?\s*-->/s';
        $offset = 0;
        $length = strlen($content);
        while ($offset < $length && preg_match($pattern, $content, $match, PREG_OFFSET_CAPTURE, $offset)) {
            $start = $match[0][1];
            $name = $match[1][0];
            $json = isset($match[3][0]) ? $match[3][0] : '';
            $open_end = $start + strlen($match[0][0]);
            $close = '<!-- /wp:' . $name . ' -->';
            $close_at = strpos($content, $close, $open_end);
            if ($close_at === false) {
                break;
            }
            $end = $close_at + strlen($close);
            $attrs = array();
            if ($json !== '') {
                $decoded = json_decode($json, true);
                if (is_array($decoded)) {
                    $attrs = $decoded;
                }
            }
            $block_name = strpos($name, '/') === false ? 'core/' . $name : $name;
            $blocks[] = array(
                'blockName' => $block_name,
                'attrs' => $attrs,
                'innerHTML' => substr($content, $open_end, $close_at - $open_end),
                'start' => $start,
                'end' => $end,
            );
            $offset = $end;
        }
        return $blocks;
    }

    private static function class_tokens($class_name) {
        $parts = preg_split('/\s+/', trim((string) $class_name));
        return is_array($parts) ? array_values(array_filter($parts, 'strlen')) : array();
    }

    private static function block_placement_id($block, $wanted) {
        $from_class = self::id_from_class(isset($block['attrs']['className']) ? $block['attrs']['className'] : '', 'affilio-placement-');
        if ($from_class === '' && preg_match('/class="([^"]*)"/', $block['innerHTML'], $match)) {
            $from_class = self::id_from_class($match[1], 'affilio-placement-');
        }
        if ($from_class !== '' && ($wanted === '' || $wanted === $from_class)) {
            return $from_class;
        }
        if (preg_match('/<!-- distributoor-placement:([a-f0-9-]{36}) -->/i', $block['innerHTML'], $match)) {
            $id = $match[1];
            if ($wanted === '' || strcasecmp($wanted, $id) === 0) {
                return $id;
            }
        }
        return '';
    }

    private static function block_disclosure_id($block, $wanted) {
        $from_class = self::id_from_class(isset($block['attrs']['className']) ? $block['attrs']['className'] : '', 'affilio-disclosure-');
        if ($from_class === '' && preg_match('/class="([^"]*)"/', $block['innerHTML'], $match)) {
            $from_class = self::id_from_class($match[1], 'affilio-disclosure-');
        }
        if ($from_class !== '' && ($wanted === '' || $wanted === $from_class)) {
            return $from_class;
        }
        return '';
    }

    private static function id_from_class($class_name, $prefix) {
        foreach (self::class_tokens($class_name) as $token) {
            if (strpos($token, $prefix) !== 0) {
                continue;
            }
            $id = substr($token, strlen($prefix));
            if (preg_match('/^[a-f0-9-]{36}$/i', $id)) {
                return $id;
            }
        }
        return '';
    }

    private static function is_legacy_placement($block) {
        return strpos($block['innerHTML'], '<!-- distributoor-placement:') !== false;
    }

    private static function is_legacy_disclosure($block) {
        $class_name = isset($block['attrs']['className']) ? $block['attrs']['className'] : '';
        if (in_array('affiliate-disclosure', self::class_tokens($class_name), true)) {
            return true;
        }
        return strpos($block['innerHTML'], 'affiliate-disclosure') !== false;
    }
}
