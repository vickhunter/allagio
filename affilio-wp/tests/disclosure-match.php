<?php

function wp_strip_all_tags($value) { return strip_tags((string) $value); }
function check($condition, $message) {
    if (!$condition) {
        fwrite(STDERR, $message . "\n");
        exit(1);
    }
}

require __DIR__ . '/../includes/class-affilio-distribution.php';

$fixture = json_decode(file_get_contents(__DIR__ . '/fixtures/disclosure-match.json'), true);
check(is_array($fixture) && !empty($fixture['positives']) && !empty($fixture['negatives']), 'shared disclosure fixture is empty');

foreach ($fixture['positives'] as $phrase) {
    $wrapped = '<p><em>' . $phrase . '</em></p>';
    check(Affilio_Distribution::disclosure_suppressed($wrapped), 'positive phrase was not a disclosure: ' . $phrase);
}
foreach ($fixture['negatives'] as $phrase) {
    $wrapped = '<p>' . $phrase . '</p>';
    check(!Affilio_Distribution::disclosure_suppressed($wrapped), 'negative phrase counted as a disclosure: ' . $phrase);
}

echo "Plugin and API disclosure phrases match the shared list.\n";
