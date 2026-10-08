<?php

require __DIR__ . '/bootstrap.php';
require dirname(__DIR__) . '/affilio.php';

affilio_init();

if (!empty($GLOBALS['actions']['rest_api_init'])) {
    foreach ($GLOBALS['actions']['rest_api_init'] as $callback) {
        call_user_func($callback);
    }
}
$paths = array();
foreach ($GLOBALS['routes'] as $route) {
    $paths[] = $route[1];
}
check(!in_array('/track', $paths, true), 'public /track route is still registered');

if (!empty($GLOBALS['actions']['wp_enqueue_scripts'])) {
    foreach ($GLOBALS['actions']['wp_enqueue_scripts'] as $callback) {
        call_user_func($callback);
    }
}
check(!isset($GLOBALS['scripts']['affilio-track']), 'front-end tracker script is still enqueued');
check(!in_array('public/js/affilio-track.js', $GLOBALS['scripts'], true), 'tracker script src is still registered');

echo "WordPress tracking route and front-end script are absent.\n";
