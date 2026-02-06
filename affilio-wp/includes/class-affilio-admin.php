<?php

class Affilio_Admin {
    private $api;
    private $scanner;
    private $inserter;

    public function __construct($api, $scanner, $inserter) {
        $this->api = $api;
        $this->scanner = $scanner;
        $this->inserter = $inserter;
        add_action('admin_post_affilio_scan_all', array($this, 'handle_scan_all'));
        add_action('admin_notices', array($this, 'maybe_render_connection_notice'));
    }

    public function register_menu() {
        add_menu_page(
            'Affilio',
            'Affilio',
            'manage_options',
            'affilio',
            array($this, 'render_dashboard'),
            'dashicons-analytics'
        );
        add_submenu_page('affilio', 'Settings', 'Settings', 'manage_options', 'affilio-settings', array($this, 'render_settings'));
        if ($this->is_advanced_mode()) {
            add_submenu_page('affilio', 'Discovery Queue', 'Discovery Queue', 'manage_options', 'affilio-discovery', array($this, 'render_discovery'));
        }
    }

    public function enqueue_sidebar_assets() {
        wp_enqueue_script('affilio-sidebar', AFFILIO_PLUGIN_URL . 'admin/build/sidebar.js', array('wp-plugins', 'wp-edit-post', 'wp-element', 'wp-data'), AFFILIO_VERSION, true);
        wp_localize_script('affilio-sidebar', 'AffilioData', array(
            'apiUrl' => Affilio_API::resolve_base_url(),
            'apiKey' => get_option('affilio_api_key', ''),
            'nonce'  => wp_create_nonce('wp_rest'),
            'settingsUrl' => admin_url('admin.php?page=affilio-settings'),
            'connectionStatus' => $this->api->get_connection_status(),
        ));
    }

    public function enqueue_dashboard_assets($hook) {
        if ($hook !== 'toplevel_page_affilio' && $hook !== 'affilio_page_affilio-discovery') {
            return;
        }
        if ($hook === 'affilio_page_affilio-discovery') {
            if (!$this->is_advanced_mode()) {
                return;
            }
            wp_enqueue_script('affilio-discovery', AFFILIO_PLUGIN_URL . 'admin/build/discovery.js', array(), AFFILIO_VERSION, true);
            wp_localize_script('affilio-discovery', 'AffilioData', array(
                'apiUrl' => Affilio_API::resolve_base_url(),
                'adminKey' => get_option('affilio_admin_api_key', ''),
                'actor' => wp_get_current_user()->user_login,
                'settingsUrl' => admin_url('admin.php?page=affilio-settings'),
            ));
        } else {
            wp_enqueue_script('affilio-dashboard', AFFILIO_PLUGIN_URL . 'admin/build/dashboard.js', array(), AFFILIO_VERSION, true);
            wp_localize_script('affilio-dashboard', 'AffilioData', array(
                'apiUrl' => Affilio_API::resolve_base_url(),
                'apiKey' => get_option('affilio_api_key', ''),
                'nonce'  => wp_create_nonce('wp_rest'),
                'settingsUrl' => admin_url('admin.php?page=affilio-settings'),
                'connectionStatus' => $this->api->get_connection_status(),
            ));
        }
        wp_enqueue_style('affilio-admin', AFFILIO_PLUGIN_URL . 'admin/css/affilio-admin.css', array(), AFFILIO_VERSION);
    }

    public function render_dashboard() {
        $status = $this->api->get_connection_status();
        echo '<div id="affilio-dashboard-root"></div>';
        if (!$status['connected']) {
            echo '<div class="notice notice-warning"><p><strong>Affilio is not connected.</strong> ' . esc_html($status['message']) . ' <a href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">Open settings</a>.</p></div>';
            return;
        }
        echo '<form method="post" action="' . esc_url(admin_url('admin-post.php')) . '">';
        echo '<input type="hidden" name="action" value="affilio_scan_all">';
        submit_button('Scan All Posts');
        echo '</form>';
    }

    public function render_discovery() {
        if (!$this->is_advanced_mode()) {
            echo '<div class="wrap"><h1>Program Discovery Queue</h1><p>Discovery tools are available only in advanced mode.</p></div>';
            return;
        }
        if (trim((string) get_option('affilio_admin_api_key', '')) === '') {
            echo '<div class="wrap"><h1>Program Discovery Queue</h1><p>Add an Admin API Key in advanced settings to use this page.</p></div>';
            return;
        }
        echo '<div id="affilio-discovery-root"></div>';
    }

    public function render_settings() {
        if ($_SERVER['REQUEST_METHOD'] === 'POST') {
            check_admin_referer('affilio_settings');
            update_option('affilio_api_key', sanitize_text_field(isset($_POST['affilio_api_key']) ? $_POST['affilio_api_key'] : ''));
            if ($this->is_advanced_mode()) {
                update_option('affilio_api_url_override', esc_url_raw(isset($_POST['affilio_api_url_override']) ? $_POST['affilio_api_url_override'] : ''));
                update_option('affilio_admin_api_key', sanitize_text_field(isset($_POST['affilio_admin_api_key']) ? $_POST['affilio_admin_api_key'] : ''));
            }
            $this->api->invalidate_status_cache();
            $this->api->get_connection_status(true);
            echo '<div class="updated"><p>Settings saved.</p></div>';
        }

        $api_key = esc_attr(get_option('affilio_api_key', ''));
        $status = $this->api->get_connection_status();
        $default_api_url = defined('AFFILIO_API_URL') ? AFFILIO_API_URL : 'https://api.affilio.io/v1';
        $override_api_url = esc_attr(get_option('affilio_api_url_override', ''));
        $admin_key = esc_attr(get_option('affilio_admin_api_key', ''));

        echo '<div class="wrap"><h1>Affilio Settings</h1>';
        echo '<p><strong>Status:</strong> ' . esc_html($status['label']) . ' - ' . esc_html($status['message']) . '</p>';
        echo '<p><a class="button" target="_blank" rel="noopener noreferrer" href="https://app.affilio.io/signup">Create Affilio account</a> ';
        echo '<a class="button button-secondary" target="_blank" rel="noopener noreferrer" href="https://app.affilio.io">Open Affilio app</a></p>';

        if ($status['state'] === 'missing_key') {
            echo '<div class="notice notice-info"><p>Onboarding: create your account, generate an API key in the Affilio app, paste it below, then click Save Settings.</p></div>';
        }

        echo '<form method="post">';
        wp_nonce_field('affilio_settings');
        echo '<table class="form-table">';
        echo '<tr><th>API Key</th><td><input name="affilio_api_key" value="' . $api_key . '" class="regular-text" autocomplete="off" />';
        echo '<p class="description">Required. Used to connect this WordPress site to your Affilio workspace.</p></td></tr>';
        echo '</table>';

        if ($this->is_advanced_mode()) {
            echo '<h2>Advanced</h2>';
            echo '<table class="form-table">';
            echo '<tr><th>Default API URL</th><td><code>' . esc_html($default_api_url) . '</code></td></tr>';
            echo '<tr><th>API URL Override</th><td><input name="affilio_api_url_override" value="' . $override_api_url . '" class="regular-text" />';
            echo '<p class="description">Optional. For development/self-hosted backends only.</p></td></tr>';
            echo '<tr><th>Admin API Key</th><td><input name="affilio_admin_api_key" value="' . $admin_key . '" class="regular-text" autocomplete="off" />';
            echo '<p class="description">Advanced only. For internal discovery/admin endpoints, not normal customer setup.</p></td></tr>';
            echo '</table>';
        }

        submit_button('Save Settings');
        echo '</form></div>';
    }

    public function maybe_render_connection_notice() {
        if (!current_user_can('manage_options')) {
            return;
        }
        $screen = function_exists('get_current_screen') ? get_current_screen() : null;
        if (!$screen) {
            return;
        }
        $is_affilio_screen = strpos((string) $screen->id, 'affilio') !== false;
        $is_editor_screen = in_array($screen->base, array('post', 'site-editor'), true);
        if (!$is_affilio_screen && !$is_editor_screen) {
            return;
        }
        $status = $this->api->get_connection_status();
        if ($status['connected']) {
            return;
        }
        echo '<div class="notice notice-warning"><p><strong>Affilio:</strong> ' . esc_html($status['message']) . ' ';
        echo '<a href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">Open settings</a>.</p></div>';
    }

    public function handle_scan_all() {
        if (!current_user_can('manage_options')) {
            wp_die('Unauthorized');
        }
        $status = $this->api->get_connection_status();
        if (!$status['connected']) {
            wp_safe_redirect(admin_url('admin.php?page=affilio-settings'));
            exit;
        }
        $this->scanner->scan_all_posts();
        wp_safe_redirect(admin_url('admin.php?page=affilio'));
        exit;
    }

    private function is_advanced_mode() {
        $enabled = (defined('AFFILIO_ADVANCED_MODE') && AFFILIO_ADVANCED_MODE) || (defined('WP_DEBUG') && WP_DEBUG);
        return (bool) apply_filters('affilio_advanced_mode', $enabled);
    }
}
