<?php

if (class_exists('Affilio_Distribution')) {
    return;
}

class Affilio_Distribution {
    private $api;

    public function __construct($api) {
        $this->api = $api;
    }

    public static function disclosure_suppressed($content) {
        $setting = get_option('affilio_existing_disclosure', '');
        if ($setting === '1' || $setting === 1) {
            return true;
        }
        $text = strtolower(wp_strip_all_tags((string) $content));
        $patterns = array('link affiliat', 'link di affiliazione', 'affiliate link', 'commissione', 'disclosure', 'sponsorizzat', 'contiene link');
        foreach ($patterns as $pattern) {
            if (strpos($text, $pattern) !== false) {
                return true;
            }
        }
        return false;
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
        $post = $this->article($request);
        if (is_wp_error($post)) {
            return $post;
        }
        return $this->request('POST', '/revenue/opportunities', array('context' => $this->context($post)));
    }

    public function prepare($request) {
        $post = $this->article($request);
        if (is_wp_error($post)) {
            return $post;
        }
        return $this->request('POST', '/placements', array('offer_id' => (string) $request->get_param('offer_id'), 'surface_url' => get_permalink($post), 'context' => $this->context($post), 'copy' => (string) $request->get_param('copy')));
    }

    public function placements() {
        return $this->request('GET', '/placements');
    }

    public function publish($request) {
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
        $marker = '<!-- distributoor-placement:' . $id . ' -->';
        $content = $post->post_content;
        if (strpos($content, $marker) === false) {
            $content .= "\n" . $marker . '<p>' . esc_html($placement['copy']) . ' <a href="' . esc_url($this->origin() . $link) . '" rel="sponsored nofollow">View product</a></p>';
            if (!self::disclosure_suppressed($content) && strpos($content, esc_html($placement['disclosure'])) === false) {
                $content .= '<p class="affiliate-disclosure">' . esc_html($placement['disclosure']) . '</p>';
            }
            $updated = wp_update_post(array('ID' => $post->ID, 'post_content' => wp_slash($content)), true);
            if (is_wp_error($updated)) {
                return $updated;
            }
        }
        $current = get_post($post->ID);
        $reported = isset($placement['receipt']['publication_status']) && $placement['receipt']['publication_status'] === 'publisher_reported';
        if (!$reported) {
            $offset = strpos($current->post_content, $marker);
            $published = $offset === false ? '' : html_entity_decode(substr($current->post_content, $offset), ENT_QUOTES, 'UTF-8');
            if (strpos($current->post_content, esc_html($placement['disclosure'])) !== false && strpos($published, $placement['disclosure']) === false) {
                $published .= "\n" . $placement['disclosure'];
            }
            $receipt = $this->request('POST', '/placements/' . $id . '/receipt', array('surface_url' => get_permalink($post), 'published_text' => $published, 'evidence' => 'WordPress administrator inserted the approved placement in published post ' . $post->ID));
            if (is_wp_error($receipt)) {
                return new WP_Error('affilio_receipt', 'The recommendation was inserted, but its publication record failed. Retry to record it without inserting twice.', array('status' => 502));
            }
        }
        return rest_ensure_response(array('published' => true, 'url' => get_permalink($post)));
    }
}
