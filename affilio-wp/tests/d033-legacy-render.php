<?php

require __DIR__ . '/d033-harness.php';
require __DIR__ . '/../includes/class-affilio-go.php';

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$saved = '<p>Leggi <a href="https://api.example.test/v1/go/' . $placement_id . '">qui</a> e <a href="/v1/go/campaign?placement=' . $placement_id . '">anche</a> e <a href="/v1/go/not-a-uuid">vecchio</a> e <a href="/go/' . $placement_id . '">locale</a>.</p>';
$post = (object) array('ID' => 2615, 'post_content' => $saved);
$GLOBALS['post'] = $post;
$GLOBALS['updates'] = 0;
$GLOBALS['affilio_filter_hooks'] = array();

$rendered = Affilio_Go::rewrite_legacy_links($saved);
check(strpos($rendered, 'https://blog.example/go/' . $placement_id) !== false, 'uuid /v1/go/ was not swapped at render');
check(substr_count($rendered, 'https://blog.example/go/' . $placement_id) === 2, 'placement query was not rewritten to the local go link: ' . $rendered);
check(strpos($rendered, 'href="/go/' . $placement_id . '"') !== false, 'an already-local /go/ link was rewritten');
check(strpos($rendered, '/v1/go/not-a-uuid') !== false, 'a non-uuid code without placement= was rewritten');
check(strpos($rendered, '/v1/go/' . $placement_id) === false, 'legacy uuid link remained');
check($post->post_content === $saved, 'render edited the saved post');
check($GLOBALS['updates'] === 0, 'render called wp_update_post');
$revision_hooks = 0;
foreach ($GLOBALS['affilio_filter_hooks'] as $hook) {
    if ($hook['name'] === 'wp_revisions_to_keep') {
        $revision_hooks++;
    }
}
check($revision_hooks === 0, 'render registered a revision filter');

echo "Legacy /v1/go/ links are rewritten only at render time.\n";
