<?php

// PHPUnit bootstrap for the D-033 walk blockers.
// WordPress's own test suite is used when WP_TESTS_DIR or WP_PHPUNIT__DIR is set.
// Otherwise the tests boot the plugin against stubs that fatal the same way
// WordPress does when get_query_var() or add_rewrite_rule() run too early.

$wp_tests = getenv('WP_TESTS_DIR');
if (!$wp_tests) {
    $wp_tests = getenv('WP_PHPUNIT__DIR');
}
if ($wp_tests && is_file($wp_tests . '/includes/functions.php')) {
    require_once $wp_tests . '/includes/functions.php';
    tests_add_filter('muplugins_loaded', function () {
        require dirname(__DIR__, 2) . '/affilio.php';
    });
    require $wp_tests . '/includes/bootstrap.php';
}
