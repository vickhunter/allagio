<?php

if (class_exists('Affilio_Onboarding')) {
    return;
}

class Affilio_Onboarding {
    private $api;

    public function __construct($api) {
        $this->api = $api;
        add_action('affilio_setup_scan', array($this, 'scan_batch'));
    }

    public function register_routes() {
        register_rest_route('affilio/v1', '/setup', array(
            array('methods' => 'GET', 'callback' => array($this, 'status'), 'permission_callback' => array($this, 'allowed')),
            array('methods' => 'POST', 'callback' => array($this, 'connect'), 'permission_callback' => array($this, 'allowed')),
        ));
    }

    public function allowed() {
        return current_user_can('manage_options');
    }

    public function status() {
        $state = get_option('affilio_setup_state', array('phase' => 'not_started', 'scanned' => 0));
        if ($state['phase'] === 'scanning') {
            $this->schedule_scan();
        }
        $state['connection'] = $this->api->get_connection_status();
        $state['dashboard_url'] = admin_url('admin.php?page=affilio');
        return rest_ensure_response($state);
    }

    private function lock() {
        $previous = (int) get_option('affilio_setup_lock', 0);
        if ($previous && $previous < time() - 180) {
            delete_option('affilio_setup_lock');
        }
        return add_option('affilio_setup_lock', time(), '', false);
    }

    public function connect() {
        if (!$this->allowed()) {
            return new WP_Error('affilio_forbidden', 'Administrator access required.', array('status' => 403));
        }
        if (!$this->lock()) {
            return new WP_Error('affilio_busy', 'Setup is already running. Try again shortly.', array('status' => 409));
        }
        try {
            if (trim((string) get_option('affilio_api_key', '')) === '') {
                $health = $this->api->request('GET', '/health');
                if (is_wp_error($health) || $health['code'] !== 200 || empty($health['data']['site_installation_registration'])) {
                    return new WP_Error('affilio_version', 'This service is not ready for one-click setup. Update the backend before connecting.', array('status' => 503));
                }
                $domain = strtolower((string) wp_parse_url(home_url(), PHP_URL_HOST));
                if (!$domain || strpos($domain, '.') === false) {
                    return new WP_Error('affilio_domain', 'Use a public site domain before connecting.', array('status' => 400));
                }
                add_option('affilio_installation_key', bin2hex(random_bytes(32)), '', false);
                $key = get_option('affilio_installation_key');
                $response = $this->api->request('POST', '/sites/register', array(
                    'domain' => $domain, 'name' => get_bloginfo('name'), 'installation_key' => $key,
                ));
                if (is_wp_error($response) || $response['code'] !== 200 || empty($response['data']['site_id']) || !isset($response['data']['api_key']) || !hash_equals($key, (string) $response['data']['api_key'])) {
                    return new WP_Error('affilio_connect', 'Could not connect. Your setup can be retried safely.', array('status' => 502));
                }
                update_option('affilio_api_key', $key, false);
                update_option('affilio_site_id', $response['data']['site_id'], false);
            }
            $this->api->invalidate_status_cache();
            $connection = $this->api->get_connection_status(true);
            if (!$connection['connected']) {
                return new WP_Error('affilio_connection', $connection['message'], array('status' => 502));
            }
            $state = get_option('affilio_setup_state', array());
            if (isset($state['phase']) && $state['phase'] === 'scan_error') {
                $state['phase'] = 'scanning';
                update_option('affilio_setup_state', $state, false);
            } elseif (!isset($state['phase']) || !in_array($state['phase'], array('scanning', 'complete'), true)) {
                update_option('affilio_setup_state', array('phase' => 'scanning', 'offset' => 0, 'scanned' => 0), false);
            }
            if (get_option('affilio_setup_state')['phase'] === 'scanning') {
                $this->schedule_scan();
            }
            return $this->status();
        } finally {
            delete_option('affilio_setup_lock');
        }
    }

    private function schedule_scan() {
        if (!wp_next_scheduled('affilio_setup_scan')) {
            wp_schedule_single_event(time() + 1, 'affilio_setup_scan');
        }
    }

    public function scan_batch() {
        if (!$this->lock()) {
            $this->schedule_scan();
            return;
        }
        try {
            $state = get_option('affilio_setup_state', array());
            if (!isset($state['phase']) || $state['phase'] !== 'scanning') {
                return;
            }
            $posts = get_posts(array('post_type' => 'post', 'post_status' => 'publish', 'posts_per_page' => 10, 'offset' => $state['offset'], 'orderby' => 'ID', 'order' => 'ASC'));
            if (!$posts) {
                $state['phase'] = 'complete';
                update_option('affilio_setup_state', $state, false);
                return;
            }
            $articles = array();
            foreach ($posts as $post) {
                $articles[] = array('wp_post_id' => $post->ID, 'title' => $post->post_title, 'content' => $post->post_content, 'url' => get_permalink($post));
            }
            $response = $this->api->request('POST', '/scan/full', array('articles' => $articles));
            if (is_wp_error($response) || $response['code'] < 200 || $response['code'] >= 300) {
                $state['phase'] = 'scan_error';
                update_option('affilio_setup_state', $state, false);
                return;
            }
            $state['offset'] += count($posts);
            $state['scanned'] += count($posts);
            if (count($posts) < 10) {
                $state['phase'] = 'complete';
            } else {
                $this->schedule_scan();
            }
            update_option('affilio_setup_state', $state, false);
        } finally {
            delete_option('affilio_setup_lock');
        }
    }
}
