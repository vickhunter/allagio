<?php
/**
 * Plugin Name: Affilio
 * Description: AI Affiliate Copilot for WordPress.
 * Version: 0.1.30
 * Author: Affilio
 * Text Domain: affilio
 * Domain Path: /languages
 */

if (!defined('ABSPATH')) {
    exit;
}

if (defined('AFFILIO_LOADED')) {
    return;
}
define('AFFILIO_LOADED', true);

define('AFFILIO_VERSION', '0.1.30');

if (!defined('AFFILIO_PLUGIN_ACK_HEADER')) {
    define('AFFILIO_PLUGIN_ACK_HEADER', 'X-Plugin-Key');
}
if (!defined('AFFILIO_PLUGIN_ACK_OPTION')) {
    define('AFFILIO_PLUGIN_ACK_OPTION', 'affilio_plugin_key');
}
if (!defined('AFFILIO_PLUGIN_ACK_PATH')) {
    define('AFFILIO_PLUGIN_ACK_PATH', '/placements/%s/plugin-ack');
}
if (!defined('AFFILIO_PLUGIN_ACK_PLACED')) {
    define('AFFILIO_PLUGIN_ACK_PLACED', 'placed');
}
if (!defined('AFFILIO_PLUGIN_ACK_REMOVED')) {
    define('AFFILIO_PLUGIN_ACK_REMOVED', 'removed');
}
if (!defined('AFFILIO_PLUGIN_ACK_RESTORED')) {
    define('AFFILIO_PLUGIN_ACK_RESTORED', 'restored');
}
if (!defined('AFFILIO_PLUGIN_ACK_FAILED')) {
    define('AFFILIO_PLUGIN_ACK_FAILED', 'failed');
}

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
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-plugin-ack.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-go.php';

if (!function_exists('affilio_load_textdomain')) {
function affilio_load_textdomain() {
    if (function_exists('load_plugin_textdomain') && function_exists('plugin_basename')) {
        load_plugin_textdomain('affilio', false, dirname(plugin_basename(__FILE__)) . '/languages');
    }
}
}

if (!function_exists('affilio_init')) {
function affilio_init() {
    add_action('init', 'affilio_load_textdomain');
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
    add_action('rest_api_init', array($editor_proxy, 'register_routes'));
    add_action('rest_api_init', array($onboarding, 'register_routes'));
    add_action('rest_api_init', array($distribution, 'register_routes'));
    add_action('admin_menu', array($distribution, 'register_revert_page'));
    add_action('admin_post_affilio_revert', array($distribution, 'handle_revert_post'));
    add_action('admin_notices', array($distribution, 'render_revert_notice'));
    add_action('affilio_retry_removals', array($distribution, 'retry_removals'));
    add_action('init', array('Affilio_Go', 'register_rewrites'));
    add_filter('query_vars', array('Affilio_Go', 'query_vars'));
    add_filter('the_content', array('Affilio_Go', 'rewrite_legacy_links'));
    add_filter('robots_txt', array('Affilio_Go', 'robots_txt'), 10, 2);
    add_filter('wpfc_exclude_current_page', array('Affilio_Go', 'exclude_cache'));
    add_action('parse_request', array('Affilio_Go', 'mark_go_request_uncacheable'));
    // Priority 0 runs before redirect_canonical (template_redirect priority 10).
    add_action('template_redirect', array(new Affilio_Go(), 'handle'), 0);
    add_filter('redirect_canonical', array('Affilio_Go', 'bypass_canonical'), 0, 2);
    add_action('affilio_sync_plugin_boxes', array($distribution, 'sync_known_boxes'));
    add_action('admin_init', array($distribution, 'retry_pending_acks'));
    add_action('init', array($distribution, 'maybe_upgrade'), 20);
    add_action('wp_enqueue_scripts', 'affilio_enqueue_placement_styles');
    if (defined('WP_CLI') && WP_CLI) {
        require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-cli.php';
        WP_CLI::add_command('affilio revert', new Affilio_Revert_Command($distribution));
    }
}
}

add_action('plugins_loaded', 'affilio_init');

if (!function_exists('affilio_enqueue_placement_styles')) {
function affilio_enqueue_placement_styles() {
    wp_enqueue_style('affilio-placement', AFFILIO_PLUGIN_URL . 'public/css/placement.css', array(), AFFILIO_VERSION);
}
}

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

if (!function_exists('affilio_activate')) {
function affilio_activate() {
    if (get_option('affilio_api_key', '') === '') {
        update_option('affilio_setup_redirect', true, false);
    }
    if (class_exists('Affilio_Go')) {
        Affilio_Go::register_rewrites();
    }
    if (function_exists('flush_rewrite_rules') && (!class_exists('Affilio_Go') || Affilio_Go::rewrite_api_ready())) {
        flush_rewrite_rules();
    }
    affilio_schedule_box_sync();
}
}

if (!function_exists('affilio_cron_schedules')) {
function affilio_cron_schedules($schedules) {
    if (!is_array($schedules)) {
        $schedules = array();
    }
    $schedules['affilio_five_minutes'] = array(
        'interval' => 300,
        'display' => 'Every five minutes',
    );
    return $schedules;
}
}

if (function_exists('add_filter')) {
    add_filter('cron_schedules', 'affilio_cron_schedules');
}

if (!function_exists('affilio_schedule_box_sync')) {
function affilio_schedule_box_sync() {
    if (!function_exists('wp_next_scheduled') || !function_exists('wp_schedule_event')) {
        return;
    }
    if (wp_next_scheduled('affilio_sync_plugin_boxes')) {
        return;
    }
    wp_schedule_event(time() + 60, 'affilio_five_minutes', 'affilio_sync_plugin_boxes');
}
}

add_action('init', 'affilio_schedule_box_sync');

if (!function_exists('affilio_deactivate')) {
function affilio_deactivate() {
    if (function_exists('wp_clear_scheduled_hook')) {
        wp_clear_scheduled_hook('affilio_sync_plugin_boxes');
    }
}
}

register_activation_hook(__FILE__, 'affilio_activate');
register_deactivation_hook(__FILE__, 'affilio_deactivate');

add_action('admin_init', 'affilio_maybe_setup_redirect');

add_action('plugins_loaded', 'affilio_mark_go_uncacheable', 0);

if (!function_exists('affilio_mark_go_uncacheable')) {
function affilio_mark_go_uncacheable() {
    if (class_exists('Affilio_Go') && Affilio_Go::path_is_go()) {
        Affilio_Go::mark_uncacheable();
    }
}
}
