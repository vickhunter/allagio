<?php
/**
 * Plugin Name: Affilio
 * Description: AI Affiliate Copilot for WordPress.
 * Version: 0.1.13
 * Author: Affilio
 */

if (!defined('ABSPATH')) {
    exit;
}

if (defined('AFFILIO_LOADED')) {
    return;
}
define('AFFILIO_LOADED', true);

define('AFFILIO_VERSION', '0.1.13');

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
}
}

add_action('plugins_loaded', 'affilio_init');

register_activation_hook(__FILE__, function () {
    if (get_option('affilio_api_key', '') === '') {
        update_option('affilio_setup_redirect', true, false);
    }
});

add_action('admin_init', function () {
    if (get_option('affilio_setup_redirect', false) && current_user_can('manage_options') && !wp_doing_ajax() && !is_network_admin() && !isset($_GET['activate-multi'])) {
        delete_option('affilio_setup_redirect');
        wp_safe_redirect(admin_url('admin.php?page=affilio-settings'));
        exit;
    }
});
