<?php

$root = dirname(__DIR__);
$css = file_get_contents($root . '/public/css/placement.css');
$distribution = file_get_contents($root . '/includes/class-affilio-distribution.php');
$main = file_get_contents($root . '/affilio.php');

check_target(strpos($distribution, 'class="affilio-product-link"') !== false, 'product link has no affilio-product-link class');
check_target(strpos($distribution, 'Vedi&nbsp;il&nbsp;prodotto') !== false, 'product link text changed');
check_target(strpos($css, 'display: inline-block;') !== false, 'tap target is not inline-block');
check_target(strpos($css, 'min-height: 44px;') !== false, 'tap target is shorter than 44px');
check_target(strpos($css, 'padding: 10px 12px;') !== false, 'tap target has no padding');
check_target(strpos($css, 'box-sizing: border-box;') !== false, 'tap target box sizing is missing');
check_target(strpos($main, "public/css/placement.css") !== false, 'placement stylesheet is not enqueued');

require $root . '/tests/bootstrap.php';
require $root . '/affilio.php';
affilio_init();
if (!empty($GLOBALS['actions']['wp_enqueue_scripts'])) {
    foreach ($GLOBALS['actions']['wp_enqueue_scripts'] as $callback) {
        call_user_func($callback);
    }
}
$href = isset($GLOBALS['styles']['affilio-placement']) ? $GLOBALS['styles']['affilio-placement'] : '';
check_target(strpos($href, 'public/css/placement.css') !== false, 'front end did not enqueue placement.css');

echo "Vedi il prodotto is an inline-block at least 44px tall.\n";

function check_target($condition, $message) {
    if (!$condition) {
        fwrite(STDERR, $message . "\n");
        exit(1);
    }
}
