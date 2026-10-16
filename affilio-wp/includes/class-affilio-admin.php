<?php

if (class_exists('Affilio_Admin')) {
    return;
}

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
        add_submenu_page('affilio', $this->t('Marketplace', 'Marketplace'), $this->t('Marketplace', 'Marketplace'), 'manage_options', 'affilio-marketplace', array($this, 'render_marketplace'));
        add_submenu_page('affilio', 'Distribution', 'Distribution', 'manage_options', 'affilio-distribution', array($this, 'render_distribution'));
        add_submenu_page('affilio', $this->t('Earnings', 'Guadagni'), $this->t('Earnings', 'Guadagni'), 'manage_options', 'affilio-earnings', array($this, 'render_earnings'));
        add_submenu_page('affilio', $this->t('Analytics', 'Analitiche'), $this->t('Analytics', 'Analitiche'), 'manage_options', 'affilio-analytics', array($this, 'render_analytics'));
        add_submenu_page('affilio', $this->t('Settings', 'Impostazioni'), $this->t('Settings', 'Impostazioni'), 'manage_options', 'affilio-settings', array($this, 'render_settings'));
        if ($this->is_advanced_mode()) {
            add_submenu_page('affilio', $this->t('Discovery Queue', 'Coda Discovery'), $this->t('Discovery Queue', 'Coda Discovery'), 'manage_options', 'affilio-discovery', array($this, 'render_discovery'));
        }
    }

    public function register_routes() {
        register_rest_route('affilio/v1', '/proxy', array(
            'methods' => 'POST',
            'callback' => array($this, 'proxy'),
            'permission_callback' => array($this, 'rest_permission'),
        ));
    }

    public function proxy_allowed() {
        return current_user_can('manage_options');
    }

    public function rest_permission() {
        if ($this->proxy_allowed()) {
            return true;
        }
        return new WP_Error('affilio_forbidden', 'Administrator access required.', array('status' => 403));
    }

    public function proxy($request) {
        if (!$this->proxy_allowed()) {
            return new WP_Error('affilio_forbidden', 'Administrator access required.', array('status' => 403));
        }
        $payload = $request->get_json_params();
        if (!is_array($payload)) {
            $payload = array();
        }
        $method = strtoupper((string) (isset($payload['method']) ? $payload['method'] : ''));
        $path = $this->proxy_target(isset($payload['path']) ? $payload['path'] : '');
        if (!in_array($method, array('GET', 'POST', 'PATCH', 'DELETE'), true) || $path === '') {
            return new WP_Error('affilio_proxy', 'Path not allowed.', array('status' => 404));
        }
        $body = isset($payload['body']) && is_array($payload['body']) ? $payload['body'] : null;
        $bare = strstr($path, '?', true);
        if ($bare === false) {
            $bare = $path;
        }
        if (strpos($bare, '/program-discovery/') === 0) {
            if (is_array($body)) {
                unset($body['actor'], $body['email'], $body['username'], $body['user_login'], $body['display_name']);
            }
            $response = $this->api->admin_request($method, $path, $body);
        } else {
            $response = $this->api->request($method, $path, $body);
        }
        if (is_wp_error($response)) {
            return new WP_Error('affilio_api', 'Affilio is unavailable. Try again shortly.', array('status' => 502));
        }
        return new WP_REST_Response(isset($response['data']) ? $response['data'] : array(), isset($response['code']) ? (int) $response['code'] : 502);
    }

    public function enqueue_sidebar_assets() {
        if (!current_user_can('edit_posts')) {
            return;
        }
        wp_enqueue_script('affilio-sidebar', AFFILIO_PLUGIN_URL . 'admin/build/sidebar.js', array('wp-plugins', 'wp-edit-post', 'wp-element', 'wp-data'), AFFILIO_VERSION, true);
        wp_localize_script('affilio-sidebar', 'AffilioData', array(
            'restBase' => rest_url('affilio/v1'),
            'nonce'  => wp_create_nonce('wp_rest'),
            'settingsUrl' => admin_url('admin.php?page=affilio-settings'),
            'connectionStatus' => $this->api->get_connection_status(),
            'locale' => function_exists('get_user_locale') ? get_user_locale() : get_locale(),
        ));
    }

    public function enqueue_dashboard_assets($hook) {
        if ($hook === 'affilio_page_affilio-distribution') {
            wp_enqueue_script('affilio-distribution', AFFILIO_PLUGIN_URL . 'admin/build/distribution.js', array(), AFFILIO_VERSION, true);
            wp_enqueue_style('affilio-distribution', AFFILIO_PLUGIN_URL . 'admin/build/distribution.css', array(), AFFILIO_VERSION);
            wp_localize_script('affilio-distribution', 'AffilioDistribution', array(
                'restBase' => rest_url('affilio/v1'),
                'nonce' => wp_create_nonce('wp_rest'),
                'noEligibleOffers' => $this->t('Nessuna offerta idonea al momento.', 'Nessuna offerta idonea al momento.'),
                'belowRelevance' => $this->t('Nessuna offerta adatta: la più vicina ha rilevanza %1$d/100, serve almeno %2$d.', 'Nessuna offerta adatta: la più vicina ha rilevanza %1$d/100, serve almeno %2$d.'),
            ));
            return;
        }
        if ($hook === 'affilio_page_affilio-settings') {
            wp_enqueue_style('affilio-admin', AFFILIO_PLUGIN_URL . 'admin/css/affilio-admin.css', array(), AFFILIO_VERSION);
            wp_enqueue_script('affilio-onboarding', AFFILIO_PLUGIN_URL . 'admin/build/onboarding.js', array(), AFFILIO_VERSION, true);
            wp_localize_script('affilio-onboarding', 'AffilioSetup', array(
                'restBase' => rest_url('affilio/v1'),
                'nonce' => wp_create_nonce('wp_rest'),
                'locale' => function_exists('get_user_locale') ? get_user_locale() : get_locale(),
                'serviceUrl' => Affilio_API::resolve_base_url(),
                'copy' => array(
                    'startAnalysis' => __('Start analysis', 'affilio'),
                    'connectedStatus' => __('Your site is connected.', 'affilio'),
                    'analysisInProgress' => __('Analysis in progress', 'affilio'),
                    'analysisComplete' => __('Analysis complete', 'affilio'),
                ),
            ));
            return;
        }
        $affilio_hooks = array('toplevel_page_affilio', 'affilio_page_affilio-discovery', 'affilio_page_affilio-analytics', 'affilio_page_affilio-marketplace', 'affilio_page_affilio-earnings');
        if (!in_array($hook, $affilio_hooks, true)) {
            return;
        }
        if ($hook === 'affilio_page_affilio-discovery') {
            if (!$this->is_advanced_mode()) {
                return;
            }
            wp_enqueue_script('affilio-discovery', AFFILIO_PLUGIN_URL . 'admin/build/discovery.js', array(), AFFILIO_VERSION, true);
            wp_localize_script('affilio-discovery', 'AffilioData', array(
                'apiUrl' => Affilio_API::resolve_base_url(),
                'adminKey' => Affilio_API::mask_key(get_option('affilio_admin_api_key', '')),
                'restBase' => rest_url('affilio/v1'),
                'nonce' => wp_create_nonce('wp_rest'),
                'settingsUrl' => admin_url('admin.php?page=affilio-settings'),
                'locale' => function_exists('get_user_locale') ? get_user_locale() : get_locale(),
            ));
        } elseif ($hook === 'affilio_page_affilio-marketplace' || $hook === 'affilio_page_affilio-earnings') {
            $handle = $hook === 'affilio_page_affilio-marketplace' ? 'affilio-marketplace' : 'affilio-earnings';
            $script = $hook === 'affilio_page_affilio-marketplace' ? 'marketplace.js' : 'earnings.js';
            wp_enqueue_script($handle, AFFILIO_PLUGIN_URL . 'admin/build/' . $script, array(), AFFILIO_VERSION, true);
            wp_localize_script($handle, 'AffilioData', $this->public_script_data());
        } elseif ($hook === 'affilio_page_affilio-analytics') {
            wp_enqueue_script('affilio-analytics', AFFILIO_PLUGIN_URL . 'admin/build/analytics.js', array(), AFFILIO_VERSION, true);
            wp_localize_script('affilio-analytics', 'AffilioData', $this->public_script_data());
        } else {
            wp_enqueue_script('affilio-dashboard', AFFILIO_PLUGIN_URL . 'admin/build/dashboard.js', array(), AFFILIO_VERSION, true);
            wp_localize_script('affilio-dashboard', 'AffilioData', $this->public_script_data());
        }
        wp_enqueue_style('affilio-admin', AFFILIO_PLUGIN_URL . 'admin/css/affilio-admin.css', array(), AFFILIO_VERSION);
    }

    public function render_dashboard() {
        $status = $this->api->get_connection_status();
        echo '<div class="wrap affilio-admin-page">';
        $this->render_dashboard_connection($status);
        echo '<div id="affilio-dashboard-root"></div>';
        if (!$status['connected']) {
            echo '<noscript><div class="affilio-panel affilio-panel-warning">';
            echo '<h2>Affilio Dashboard</h2>';
            echo '<p><strong>' . esc_html($this->t('Not connected.', 'Non connesso.')) . '</strong> ' . esc_html($status['message']) . '</p>';
            echo '<p><a class="button button-primary" href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">' . esc_html($this->t('Open settings', 'Apri impostazioni')) . '</a></p>';
            echo '</div></noscript>';
            echo '</div>';
            return;
        }
        echo '<div class="affilio-panel affilio-panel-actions">';
        echo '<h2>' . esc_html($this->t('Scan', 'Scansione')) . '</h2>';
        echo '<p>' . esc_html($this->t('Run a full scan to refresh article scores and recommendations.', 'Esegui una scansione completa per aggiornare punteggi e raccomandazioni.')) . '</p>';
        $notice = get_transient('affilio_scan_all_notice');
        if (is_string($notice) && $notice !== '') {
            delete_transient('affilio_scan_all_notice');
            echo '<div class="notice notice-info"><p>' . nl2br(esc_html($notice)) . '</p></div>';
        }
        $job = get_option('affilio_scan_all_job', array());
        if (isset($_GET['affilio_scan_continue']) && is_array($job) && $job) {
            $so_far = isset($job['scanned']) ? (int) $job['scanned'] : 0;
            $token = isset($job['continue_token']) ? (string) $job['continue_token'] : '';
            echo '<p>' . esc_html($this->t('Scanning… ' . $so_far . ' posts analyzed so far.', 'Scansione in corso… ' . $so_far . ' articoli analizzati finora.')) . '</p>';
            if ($token !== '') {
                echo '<form id="affilio-scan-continue" method="post" action="' . esc_url(admin_url('admin-post.php')) . '">';
                echo '<input type="hidden" name="action" value="affilio_scan_all">';
                echo '<input type="hidden" name="affilio_scan_continue" value="1">';
                echo '<input type="hidden" name="affilio_scan_token" value="' . esc_attr($token) . '">';
                submit_button($this->t('Continue scan', 'Continua la scansione'));
                echo '</form>';
            }
        }
        echo '<form id="affilio-scan-all" method="post" action="' . esc_url(admin_url('admin-post.php')) . '">';
        wp_nonce_field('affilio_scan_all');
        echo '<input type="hidden" name="action" value="affilio_scan_all">';
        echo '<div data-scan-status role="status"></div>';
        echo '<button type="submit" class="button button-primary button-large" data-scan>' . esc_html($this->t('Scan All Posts', 'Scansiona tutti gli articoli')) . '</button>';
        echo '<div data-scan-fallback hidden><button type="button" class="button button-secondary" data-scan-fallback>' . esc_html($this->t('Send the scan form', 'Invia il modulo di scansione')) . '</button></div>';
        echo '</form>';
        echo '</div>';
        echo '</div>';
    }

    public function render_distribution() {
        echo '<div class="wrap affilio-distribution"><h1>Distributoor</h1><p>Relevant recommendations. Approved by you. Tracked from purchase to commission.</p><div id="affilio-distribution-root"></div></div>';
    }

    public function render_discovery() {
        if (!$this->is_advanced_mode()) {
            echo '<div class="wrap"><h1>' . esc_html($this->t('Program Discovery Queue', 'Coda Discovery Programmi')) . '</h1><p>' . esc_html($this->t('Discovery tools are available only in advanced mode.', 'Gli strumenti discovery sono disponibili solo in modalità avanzata.')) . '</p></div>';
            return;
        }
        if (trim((string) get_option('affilio_admin_api_key', '')) === '') {
            echo '<div class="wrap"><h1>' . esc_html($this->t('Program Discovery Queue', 'Coda Discovery Programmi')) . '</h1><p>' . esc_html($this->t('Add an Admin API Key in advanced settings to use this page.', 'Aggiungi una chiave API Admin nelle impostazioni avanzate per usare questa pagina.')) . '</p></div>';
            return;
        }
        echo '<div id="affilio-discovery-root"></div>';
    }

    public function render_analytics() {
        $status = $this->api->get_connection_status();
        echo '<div class="wrap affilio-admin-page">';
        if (!$status['connected']) {
            echo '<div class="affilio-panel affilio-panel-warning">';
            echo '<h2>' . esc_html($this->t('Affilio Analytics', 'Analitiche Affilio')) . '</h2>';
            echo '<p><strong>' . esc_html($this->t('Not connected.', 'Non connesso.')) . '</strong> ' . esc_html($status['message']) . '</p>';
            echo '<p><a class="button button-primary" href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">' . esc_html($this->t('Open settings', 'Apri impostazioni')) . '</a></p>';
            echo '</div></div>';
            return;
        }
        echo '<div id="affilio-analytics-root"></div>';
        echo '</div>';
    }

    public function render_marketplace() {
        $status = $this->api->get_connection_status();
        echo '<div class="wrap affilio-admin-page">';
        echo '<h1>' . esc_html($this->t('Offer Marketplace', 'Marketplace Offerte')) . '</h1>';
        if (!$status['connected']) {
            echo '<div class="affilio-panel affilio-panel-warning">';
            echo '<p><strong>' . esc_html($this->t('Not connected.', 'Non connesso.')) . '</strong> ' . esc_html($status['message']) . '</p>';
            echo '<p><a class="button button-primary" href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">' . esc_html($this->t('Open settings', 'Apri impostazioni')) . '</a></p>';
            echo '</div></div>';
            return;
        }
        echo '<div id="affilio-marketplace-root"></div>';
        echo '</div>';
    }

    public function render_earnings() {
        $status = $this->api->get_connection_status();
        echo '<div class="wrap affilio-admin-page">';
        echo '<h1>' . esc_html($this->t('Earnings', 'Guadagni')) . '</h1>';
        if (!$status['connected']) {
            echo '<div class="affilio-panel affilio-panel-warning">';
            echo '<p><strong>' . esc_html($this->t('Not connected.', 'Non connesso.')) . '</strong> ' . esc_html($status['message']) . '</p>';
            echo '<p><a class="button button-primary" href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">' . esc_html($this->t('Open settings', 'Apri impostazioni')) . '</a></p>';
            echo '</div></div>';
            return;
        }
        echo '<div id="affilio-earnings-root"></div>';
        echo '</div>';
    }

    public function render_settings() {
        if (!current_user_can('manage_options')) {
            wp_die('Administrator access required.');
        }
        if ($_SERVER['REQUEST_METHOD'] === 'POST') {
            check_admin_referer('affilio_settings');
            $this->keep_or_replace_secret('affilio_api_key', 'affilio_api_key');
            $this->keep_or_replace_secret('affilio_plugin_key', 'affilio_plugin_key');
            $existing_disclosure = isset($_POST['affilio_existing_disclosure']) ? sanitize_text_field(wp_unslash($_POST['affilio_existing_disclosure'])) : '';
            update_option('affilio_existing_disclosure', $existing_disclosure === '1' ? '1' : '');
            $url_error = '';
            if ($this->is_advanced_mode()) {
                $override = isset($_POST['affilio_api_url_override']) ? trim((string) wp_unslash($_POST['affilio_api_url_override'])) : '';
                if (!$this->api_url_override_allowed($override)) {
                    $url_error = "L'URL dell'API deve usare https, salvo localhost.";
                } else {
                    update_option('affilio_api_url_override', Affilio_API::normalize_base_url(esc_url_raw($override)));
                }
                $this->keep_or_replace_secret('affilio_admin_api_key', 'affilio_admin_api_key');
            }
            $this->api->invalidate_status_cache();
            $this->api->get_connection_status(true);
            if ($url_error !== '') {
                echo '<div class="error"><p>' . esc_html($url_error) . '</p></div>';
            } else {
                echo '<div class="updated"><p>' . esc_html($this->t('Settings saved.', 'Impostazioni salvate.')) . '</p></div>';
            }
        }

        $status = $this->api->get_connection_status();
        $default_api_url = Affilio_API::resolve_base_url();
        $override_api_url = esc_attr(get_option('affilio_api_url_override', ''));
        $saved_key = trim((string) get_option('affilio_api_key', ''));
        $key_hint = $saved_key === '' ? '' : sprintf(__('Key saved (%s)', 'affilio'), Affilio_API::mask_key($saved_key));

        echo '<div class="wrap affilio-admin-page"><h1>' . esc_html($this->t('Affilio Settings', 'Impostazioni Affilio')) . '</h1>';
        echo '<div class="affilio-status-row">';
        echo '<span class="affilio-status-pill affilio-status-' . esc_attr($status['state']) . '">' . esc_html($status['label']) . '</span>';
        echo '<span class="affilio-status-message">' . esc_html($status['message']) . '</span>';
        if ($key_hint !== '') {
            echo '<span class="affilio-key-hint">' . esc_html($key_hint) . '</span>';
        }
        echo '</div>';
        echo '<div id="affilio-onboarding-root">';
        if (!empty($status['connected'])) {
            echo '<p data-progress role="status">' . esc_html(__('Your site is connected.', 'affilio')) . '</p>';
            echo '<form method="post" action="' . esc_url(admin_url('admin-post.php')) . '">';
            wp_nonce_field('affilio_scan_all');
            echo '<input type="hidden" name="action" value="affilio_scan_all" />';
            echo '<button type="submit" class="button button-primary button-large" data-connect>' . esc_html(__('Start analysis', 'affilio')) . '</button>';
            echo '</form>';
        }
        echo '</div>';
        echo '<details><summary>' . esc_html($this->t('Existing connection or manual settings', 'Collegamento esistente o impostazioni manuali')) . '</summary>';

        echo '<form class="affilio-settings-form" method="post">';
        wp_nonce_field('affilio_settings');
        echo '<table class="form-table">';
        echo '<tr><th>' . esc_html($this->t('API Key', 'Chiave API')) . '</th><td>';
        $this->secret_input('affilio_api_key', 'affilio_api_key');
        echo '<p class="description">' . esc_html($this->t('Optional manual setup. The Connect button manages this for new installs.', 'Setup manuale opzionale. Il pulsante Collega lo gestisce per le nuove installazioni.')) . '</p></td></tr>';
        echo '<tr><th>' . esc_html($this->t('Plugin key', 'Chiave plugin')) . '</th><td>';
        $this->secret_input('affilio_plugin_key', 'affilio_plugin_key');
        echo '<p class="description">' . esc_html($this->t('Sent only as X-Plugin-Key when the plugin confirms a box. It is not the site API key. Paste the key recorded for this site. Leave the field empty to keep the saved key.', 'Inviata solo come X-Plugin-Key quando il plugin conferma un riquadro. Non è la chiave API del sito. Incolla la chiave registrata per questo sito. Lascia il campo vuoto per conservare la chiave salvata.')) . '</p></td></tr>';
        $existing_disclosure = get_option('affilio_existing_disclosure', '') === '1';
        echo '<tr><th>' . esc_html($this->t('Disclosure', 'Disclosure')) . '</th><td><label><input type="checkbox" name="affilio_existing_disclosure" value="1"' . ($existing_disclosure ? ' checked' : '') . ' /> ' . esc_html($this->t('My articles already contain a disclosure', 'I miei articoli contengono già una disclosure')) . '</label></td></tr>';
        echo '</table>';

        if ($this->is_advanced_mode()) {
            echo '<h2>' . esc_html($this->t('Advanced', 'Avanzate')) . '</h2>';
            echo '<table class="form-table">';
            echo '<tr><th>' . esc_html($this->t('Default API URL', 'URL API predefinito')) . '</th><td><code>' . esc_html($default_api_url) . '</code></td></tr>';
            echo '<tr><th>' . esc_html($this->t('API URL Override', 'Override URL API')) . '</th><td><input name="affilio_api_url_override" value="' . $override_api_url . '" class="regular-text" />';
            echo '<p class="description">' . esc_html($this->t('Optional. For development/self-hosted backends only.', 'Opzionale. Solo per sviluppo/backend self-hosted.')) . '</p></td></tr>';
            echo '<tr><th>' . esc_html($this->t('Admin API Key', 'Chiave API Admin')) . '</th><td>';
            $this->secret_input('affilio_admin_api_key', 'affilio_admin_api_key');
            echo '<p class="description">' . esc_html($this->t('Advanced only. For internal discovery/admin endpoints, not normal customer setup.', 'Solo avanzato. Per endpoint interni discovery/admin, non per setup clienti standard.')) . '</p></td></tr>';
            echo '</table>';
        }

        submit_button($this->t('Save Settings', 'Salva impostazioni'));
        echo '</form></details></div>';
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
        if ($is_affilio_screen) {
            return;
        }
        $is_editor_screen = in_array($screen->base, array('post', 'site-editor'), true);
        if (!$is_editor_screen) {
            return;
        }
        $status = $this->api->get_connection_status();
        if ($status['connected']) {
            return;
        }
        $saved_key = trim((string) get_option('affilio_api_key', ''));
        $key_hint = $saved_key === '' ? '' : ' ' . sprintf(__('Key saved (%s)', 'affilio'), Affilio_API::mask_key($saved_key)) . '.';
        echo '<div class="notice notice-warning"><p><strong>Affilio:</strong> ' . esc_html($status['message'] . $key_hint) . ' ';
        echo '<a href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">' . esc_html($this->t('Open settings', 'Apri impostazioni')) . '</a>.</p></div>';
    }

    public function handle_scan_all() {
        if (!current_user_can('manage_options')) {
            wp_die(esc_html($this->t('Unauthorized', 'Non autorizzato')), '', array('response' => 403));
        }
        $continuing = isset($_POST['affilio_scan_continue']) && (string) $_POST['affilio_scan_continue'] === '1';
        if (!$continuing) {
            check_admin_referer('affilio_scan_all');
        }
        delete_option('affilio_setup_redirect');
        $status = $this->api->get_connection_status();
        if (!$status['connected']) {
            wp_safe_redirect(admin_url('admin.php?page=affilio-settings'));
            exit;
        }
        $result = $this->advance_scan_all();
        wp_safe_redirect($result['redirect']);
        exit;
    }

    public function advance_scan_all() {
        $stored = get_option('affilio_scan_all_job', array());
        if (!is_array($stored)) {
            $stored = array();
        }
        $continuing = isset($_POST['affilio_scan_continue']) && (string) $_POST['affilio_scan_continue'] === '1';
        $job = array();
        if ($continuing) {
            $token = isset($_POST['affilio_scan_token']) ? (string) $_POST['affilio_scan_token'] : '';
            $saved = isset($stored['continue_token']) ? (string) $stored['continue_token'] : '';
            if ($saved === '' || $token === '' || !hash_equals($saved, $token)) {
                return array(
                    'redirect' => admin_url('admin.php?page=affilio'),
                    'done' => false,
                );
            }
            $job = $stored;
        }
        $offset = isset($job['offset']) ? (int) $job['offset'] : 0;
        $batch = $this->scanner->scan_published_batch($offset);
        if (!is_array($batch)) {
            $batch = array();
        }
        $scanned = (isset($job['scanned']) ? (int) $job['scanned'] : 0) + (isset($batch['scanned']) ? (int) $batch['scanned'] : 0);
        $skipped = isset($job['skipped']) && is_array($job['skipped']) ? $job['skipped'] : array();
        if (isset($batch['skipped']) && is_array($batch['skipped'])) {
            $skipped = array_merge($skipped, $batch['skipped']);
        }
        if (empty($batch['done'])) {
            update_option('affilio_scan_all_job', array(
                'offset' => isset($batch['next_offset']) ? (int) $batch['next_offset'] : $offset + 10,
                'scanned' => $scanned,
                'skipped' => $skipped,
                'continue_token' => bin2hex(random_bytes(16)),
            ), false);
            return array(
                'redirect' => admin_url('admin.php?page=affilio&affilio_scan_continue=1'),
                'done' => false,
            );
        }
        delete_option('affilio_scan_all_job');
        $notice = $this->format_scan_result(array('scanned' => $scanned, 'skipped' => $skipped));
        if ($notice !== '') {
            set_transient('affilio_scan_all_notice', $notice, 120);
        }
        return array(
            'redirect' => admin_url('admin.php?page=affilio'),
            'done' => true,
        );
    }

    public function format_scan_result($result) {
        if (!is_array($result)) {
            return '';
        }
        $scanned = isset($result['scanned']) ? (int) $result['scanned'] : 0;
        $skipped = isset($result['skipped']) && is_array($result['skipped']) ? $result['skipped'] : array();
        $lines = array(affilio_scan_summary($scanned, count($skipped)));
        foreach ($skipped as $row) {
            if (!is_array($row)) {
                continue;
            }
            $id = isset($row['wp_post_id']) ? (int) $row['wp_post_id'] : 0;
            $reason = isset($row['reason']) ? (string) $row['reason'] : '';
            $lines[] = $id . ': ' . $reason;
        }
        return implode("\n", $lines);
    }

    private function public_script_data() {
        return array(
            'apiUrl' => Affilio_API::resolve_base_url(),
            'apiKey' => Affilio_API::mask_key(get_option('affilio_api_key', '')),
            'restBase' => rest_url('affilio/v1'),
            'nonce' => wp_create_nonce('wp_rest'),
            'settingsUrl' => admin_url('admin.php?page=affilio-settings'),
            'connectionStatus' => $this->api->get_connection_status(),
            'locale' => function_exists('get_user_locale') ? get_user_locale() : get_locale(),
            'i18n' => array(
                'connected' => __('Connected', 'affilio'),
                'notConnected' => __('Not connected.', 'affilio'),
                'connectSite' => __('Connect your site in Settings.', 'affilio'),
                'analysisInProgress' => $this->t('Analysis in progress', 'Analisi in corso'),
                'analysisComplete' => $this->t('Analysis complete', 'Analisi completata'),
                'scanFailed' => $this->t('Analysis failed. Retry, or send the scan form.', 'Analisi non riuscita. Riprova, oppure invia il modulo di scansione.'),
                'scanTimedOut' => $this->t('Analysis is taking too long. Retry, or send the scan form.', 'L\'analisi sta impiegando troppo tempo. Riprova, oppure invia il modulo di scansione.'),
                'scanFallback' => $this->t('Send the scan form', 'Invia il modulo di scansione'),
            ),
        );
    }

    private function render_dashboard_connection($status) {
        echo '<div id="affilio-dashboard-connection" class="affilio-status-row affilio-dashboard-connection" data-connection-status>';
        if (!empty($status['connected'])) {
            $state = $this->connection_state_class(isset($status['state']) ? $status['state'] : 'connected');
            echo '<span class="affilio-status-pill affilio-status-' . esc_attr($state) . '">' . esc_html(__('Connected', 'affilio')) . '</span>';
            echo '</div>';
            return;
        }
        echo '<p class="affilio-status-message">' . esc_html(__('Not connected.', 'affilio')) . ' ';
        echo '<a class="affilio-next-step" href="' . esc_url(admin_url('admin.php?page=affilio-settings')) . '">' . esc_html(__('Connect your site in Settings.', 'affilio')) . '</a>';
        echo '</p></div>';
    }

    private function connection_state_class($state) {
        $allowed = array('connected', 'missing_key', 'missing_url', 'invalid_key', 'api_error', 'network_error', 'wrong_api_url', 'starting', 'unknown_error');
        $state = (string) $state;
        if (!in_array($state, $allowed, true)) {
            return 'unknown_error';
        }
        return $state;
    }

    private function proxy_target($path) {
        $path = trim((string) $path);
        if ($path === '' || strpos($path, '..') !== false || strpos($path, '://') !== false || strpos($path, '\\') !== false) {
            return '';
        }
        if ($path[0] !== '/') {
            $path = '/' . $path;
        }
        $query = '';
        $bare = $path;
        $split = strpos($path, '?');
        if ($split !== false) {
            $query = substr($path, $split);
            $bare = substr($path, 0, $split);
        }
        $prefixes = array(
            '/articles', '/recommendations', '/generate/content', '/user-programs',
            '/marketplace/offers', '/payouts/', '/track/summary', '/track/trends',
            '/track/funnel', '/track/attribution', '/track/export', '/track/cohorts',
            '/track/recommendation-quality', '/track/retention', '/track/recommendation-tuning',
            '/program-discovery/', '/insert/confirm', '/scan/full', '/scan/single', '/scan/status',
        );
        foreach ($prefixes as $prefix) {
            $boundary = substr($prefix, -1) === '/';
            if ($boundary && strpos($bare, $prefix) === 0) {
                return $bare . $query;
            }
            if (!$boundary && ($bare === $prefix || strpos($bare, $prefix . '/') === 0)) {
                return $bare . $query;
            }
        }
        return '';
    }

    private function keep_or_replace_secret($option, $field) {
        $submitted = sanitize_text_field(isset($_POST[$field]) ? wp_unslash($_POST[$field]) : '');
        if ($submitted !== '') {
            update_option($option, $submitted);
        }
    }

    private function secret_input($name, $option) {
        $stored = trim((string) get_option($option, ''));
        $placeholder = $stored === '' ? '' : sprintf(__('Key saved (%s)', 'affilio'), Affilio_API::mask_key($stored));
        echo '<input type="password" name="' . esc_attr($name) . '" value="" placeholder="' . esc_attr($placeholder) . '" class="regular-text" autocomplete="off" spellcheck="false" />';
    }

    private function api_url_override_allowed($raw) {
        $raw = trim((string) $raw);
        if ($raw === '') {
            return true;
        }
        $parts = wp_parse_url($raw);
        if (!is_array($parts) || empty($parts['scheme']) || empty($parts['host'])) {
            return false;
        }
        $scheme = strtolower((string) $parts['scheme']);
        $host = strtolower((string) $parts['host']);
        if ($scheme === 'https') {
            return true;
        }
        return $scheme === 'http' && ($host === 'localhost' || $host === '127.0.0.1');
    }

    private function is_advanced_mode() {
        $enabled = (defined('AFFILIO_ADVANCED_MODE') && AFFILIO_ADVANCED_MODE) || (defined('WP_DEBUG') && WP_DEBUG);
        return (bool) apply_filters('affilio_advanced_mode', $enabled);
    }

    private function t($en, $it) {
        $locale = function_exists('get_user_locale') ? get_user_locale() : get_locale();
        if (strpos(strtolower((string) $locale), 'it') === 0) {
            return $it;
        }
        return $en;
    }
}
