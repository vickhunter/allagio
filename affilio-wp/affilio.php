<?php
/**
 * Plugin Name: Affilio
 * Description: AI Affiliate Copilot for WordPress.
 * Version: 0.1.0
 * Author: Affilio
 */

if (!defined('ABSPATH')) {
    exit;
}

define('AFFILIO_VERSION', '0.1.0');
define('AFFILIO_PLUGIN_DIR', plugin_dir_path(__FILE__));
define('AFFILIO_PLUGIN_URL', plugin_dir_url(__FILE__));
if (!defined('AFFILIO_API_URL')) {
    // Production API default for customer installs. Can still be overridden in advanced mode.
    define('AFFILIO_API_URL', 'https://affilio-production.up.railway.app/v1');
}

require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-api.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-scanner.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-inserter.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-tracker.php';
require_once AFFILIO_PLUGIN_DIR . 'includes/class-affilio-admin.php';

function affilio_init() {
    $api = new Affilio_API();
    $scanner = new Affilio_Scanner($api);
    $inserter = new Affilio_Inserter($api);
    $tracker = new Affilio_Tracker($api);
    $admin = new Affilio_Admin($api, $scanner, $inserter);

    add_action('admin_menu', array($admin, 'register_menu'));
    add_action('enqueue_block_editor_assets', array($admin, 'enqueue_sidebar_assets'));
    add_action('admin_enqueue_scripts', array($admin, 'enqueue_dashboard_assets'));
    add_action('rest_api_init', array($tracker, 'register_routes'));
    add_action('rest_api_init', array($inserter, 'register_routes'));
}

add_action('plugins_loaded', 'affilio_init');
