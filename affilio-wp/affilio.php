<?php
/**
 * Plugin Name: Affilio
 * Description: AI Affiliate Copilot for WordPress.
 * Version: 0.1.17
 * Author: Affilio
 */

if (!defined('ABSPATH')) {
    exit;
}

if (defined('AFFILIO_LOADED')) {
    return;
}
define('AFFILIO_LOADED', true);

define('AFFILIO_VERSION', '0.1.17');

if (!function_exists('affilio_scan_summary')) {
function affilio_scan_summary($scanned, $skipped) {
    $scanned = (int) $scanned;
    $skipped = (int) $skipped;
    $locale = 'en_US';
    if (isset($GLOBALS['user_locale']) && $GLOBALS['user_locale'] !== '') {
        $locale = $GLOBALS['user_locale'];
    } elseif (function_exists('get_user_locale')) {
        $locale = get_user_locale();
    } elseif (function_exists('get_locale')) {
        $locale = get_locale();
    }
    if (strpos(strtolower((string) $locale), 'it') === 0) {
        $analyzed = $scanned === 1 ? '1 articolo analizzato' : $scanned . ' articoli analizzati';
        $skipped_label = $skipped === 1 ? '1 saltato' : $skipped . ' saltati';
        return 'Analisi completata: ' . $analyzed . ', ' . $skipped_label;
    }
    $analyzed = $scanned === 1 ? '1 post analyzed' : $scanned . ' posts analyzed';
    $skipped_label = $skipped === 1 ? '1 skipped' : $skipped . ' skipped';
    return 'Analysis complete: ' . $analyzed . ', ' . $skipped_label;
}
}
define('AFFILIO_PLUGIN_DIR', plugin_dir_path(__FILE__));
define('AFFILIO_PLUGIN_URL', plugin_dir_url(__FILE__));

require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-api.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-scanner.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-inserter.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-tracker.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-admin.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-editor-proxy.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-onboarding.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-distribution.php';

if (!function_exists('affilio_init')) {
function affilio_init() {
    $api = new Affilio_API();
    $scanner = new Affilio_Scanner($api);
    $inserter = new Affilio_Inserter($api);
    $tracker = new Affilio_Tracker($api);
    $admin = new Affilio_Admin($api, $scanner, $inserter);
    $editor_proxy = new Affilio_Editor_Proxy($api, $inserter);
    $onboarding = new Affilio_Onboarding($api);
    $distribution = new Affilio_Distribution($api);

    add_action('admin_menu', array($admin, 'register_menu'));
    add_action('enqueue_block_editor_assets', array($admin, 'enqueue_sidebar_assets'));
    add_action('admin_enqueue_scripts', array($admin, 'enqueue_dashboard_assets'));
    add_action('rest_api_init', array($admin, 'register_routes'));
    add_action('rest_api_init', array($inserter, 'register_routes'));
    add_action('rest_api_init', array($editor_proxy, 'register_routes'));
    add_action('rest_api_init', array($onboarding, 'register_routes'));
    add_action('rest_api_init', array($distribution, 'register_routes'));
    add_action('admin_menu', array($distribution, 'register_revert_page'));
    add_action('admin_post_affilio_revert', array($distribution, 'handle_revert_post'));
    add_action('admin_notices', array($distribution, 'render_revert_notice'));
    add_action('affilio_retry_removals', array($distribution, 'retry_removals'));
    if (defined('WP_CLI') && WP_CLI) {
        require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-cli.php';
        WP_CLI::add_command('affilio revert', new Affilio_Revert_Command($distribution));
    }
}
}

add_action('plugins_loaded', 'affilio_init');

if (!function_exists('affilio_request_is_admin_post')) {
function affilio_request_is_admin_post() {
    if (isset($GLOBALS['pagenow']) && $GLOBALS['pagenow'] === 'admin-post.php') {
        return true;
    }
    $self = isset($_SERVER['PHP_SELF']) ? basename((string) $_SERVER['PHP_SELF']) : '';
    if ($self === 'admin-post.php') {
        return true;
    }
    $action = isset($_REQUEST['action']) ? (string) $_REQUEST['action'] : '';
    return $action === 'affilio_scan_all' || $action === 'affilio_revert';
}
}

if (!function_exists('affilio_maybe_setup_redirect')) {
function affilio_maybe_setup_redirect() {
    if (!get_option('affilio_setup_redirect', false)) {
        return;
    }
    if (!current_user_can('manage_options') || wp_doing_ajax() || is_network_admin() || isset($_GET['activate-multi'])) {
        return;
    }
    if (affilio_request_is_admin_post()) {
        return;
    }
    $page = isset($_GET['page']) ? (string) $_GET['page'] : '';
    $scan_job = get_option('affilio_scan_all_job', array());
    if ($page === 'affilio' && isset($_GET['affilio_scan_continue']) && is_array($scan_job) && $scan_job) {
        delete_option('affilio_setup_redirect');
        return;
    }
    if ($page === 'affilio-distribution' && isset($_GET['affilio_reverted'])) {
        delete_option('affilio_setup_redirect');
        return;
    }
    delete_option('affilio_setup_redirect');
    wp_safe_redirect(admin_url('admin.php?page=affilio-settings'));
    exit;
}
}

register_activation_hook(__FILE__, function () {
    if (get_option('affilio_api_key', '') === '') {
        update_option('affilio_setup_redirect', true, false);
    }
});

add_action('admin_init', 'affilio_maybe_setup_redirect');
