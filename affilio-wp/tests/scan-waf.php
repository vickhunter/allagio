<?php

require __DIR__ . '/bootstrap.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

$dangerous = array(
    'curl -fsSL https://example.com/install.sh | sh',
    '<script>',
    'rm -rf /',
    "'); DROP TABLE users; --",
);
$payload = "Install with " . $dangerous[0] . " then " . $dangerous[1] . " and " . $dangerous[2] . " or " . $dangerous[3];

$GLOBALS['options']['affilio_api_url_override'] = 'https://api.example.test/v1';
$GLOBALS['options']['affilio_api_key'] = 'site-key';
$GLOBALS['posts'] = array(
    (object) array(
        'ID' => 7,
        'post_title' => 'Guide ' . $payload,
        'post_content' => 'Steps: ' . $payload,
        'post_name' => 'safe-slug',
        'post_status' => 'publish',
    ),
    (object) array(
        'ID' => 8,
        'post_title' => 'Draft ' . $payload,
        'post_content' => 'Hidden ' . $payload,
        'post_name' => 'hidden-draft',
        'post_status' => 'draft',
    ),
);

add_filter('pre_http_request', function ($pre, $args, $url) {
    return array('response' => array('code' => 202), 'body' => '{"status":"queued"}');
});

require dirname(__DIR__) . '/affilio.php';
affilio_init();
if (!empty($GLOBALS['actions']['rest_api_init'])) {
    foreach ($GLOBALS['actions']['rest_api_init'] as $callback) {
        call_user_func($callback);
    }
}

$api = new Affilio_API();
$scanner = new Affilio_Scanner($api);
$scanner->scan_all_posts();
$scanner->scan_single_post(7);
$scanner->scan_single_post(8);
$editor = null;
foreach ($GLOBALS['routes'] as $route) {
    if ($route[1] === '/editor/analyze') {
        $editor = $route[2]['callback'];
    }
}
expect(is_callable($editor), 'editor analyze route is missing');
if (is_callable($editor)) {
    call_user_func($editor, new Affilio_Test_Request(array('wp_post_id' => 7)));
    call_user_func($editor, new Affilio_Test_Request(array('wp_post_id' => 8)));
}

$raw = '';
$restored = '';
$saw_scan = false;
foreach ($GLOBALS['http'] as $call) {
    if (strpos($call['url'], '/scan/') === false) {
        continue;
    }
    $saw_scan = true;
    $body = isset($call['args']['body']) ? (string) $call['args']['body'] : '';
    $raw .= $body . "\n";
    $decoded = json_decode($body, true);
    if (!is_array($decoded)) {
        continue;
    }
    $articles = isset($decoded['articles']) ? $decoded['articles'] : array($decoded);
    foreach ($articles as $article) {
        if (!is_array($article)) {
            continue;
        }
        expect(!isset($article['content']) && !isset($article['title']), 'scan request still sent raw title or content');
        expect(isset($article['content_b64']) && isset($article['title_b64']), 'scan request did not send encoded title and content');
        $encoding = isset($article['content_encoding']) ? (string) $article['content_encoding'] : 'base64';
        foreach (array('title_b64', 'content_b64') as $field) {
            $bytes = base64_decode($article[$field], true);
            expect($bytes !== false, 'scan field was not valid base64');
            if ($bytes !== false && $encoding === 'gzip+base64') {
                $bytes = gzdecode($bytes);
                expect($bytes !== false, 'gzip+base64 scan field did not decode');
            }
            if (is_string($bytes)) {
                $restored .= $bytes . "\n";
            }
        }
    }
}
expect($saw_scan, 'no scan request was captured');
$visible = str_replace('\\/', '/', $raw);
foreach ($dangerous as $needle) {
    expect(strpos($raw, $needle) === false && strpos($visible, $needle) === false, 'outgoing scan body still contains ' . $needle);
}
expect(strpos($restored, $dangerous[0]) !== false, 'encoded body dropped the published post text');
expect(strpos($raw, 'Hidden ') === false && strpos($restored, 'Hidden ') === false, 'draft post was sent');
expect(strpos($raw, 'https://blog.example/safe-slug') !== false, 'published permalink was not sent');

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Scan requests hide install snippets from the request body.\n";
