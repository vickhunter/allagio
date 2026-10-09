<?php

require __DIR__ . '/bootstrap.php';

$GLOBALS['options']['affilio_api_url_override'] = 'https://api.example.test/v1';
$GLOBALS['users'][] = $GLOBALS['current_user'];
$GLOBALS['users'][] = (object) array(
    'ID' => 2,
    'user_login' => 'cara-login',
    'user_email' => 'commenter@example.com',
    'display_name' => 'Cara Comment',
);
$GLOBALS['comments'][] = (object) array(
    'comment_author' => 'Cara Comment',
    'comment_author_email' => 'commenter@example.com',
    'comment_author_IP' => '203.0.113.50',
    'comment_content' => 'Secret comment about the draft',
    'user_id' => 2,
);
$GLOBALS['posts'] = array(
    (object) array('ID' => 11, 'post_title' => 'Published lighthouse guide', 'post_content' => 'Published lighthouse body', 'post_name' => 'published-lighthouse', 'post_status' => 'publish'),
    (object) array('ID' => 12, 'post_title' => 'Draft secret launch', 'post_content' => 'Draft secret body', 'post_name' => 'draft-secret', 'post_status' => 'draft'),
    (object) array('ID' => 13, 'post_title' => 'Private owner notes', 'post_content' => 'Private owner body', 'post_name' => 'private-notes', 'post_status' => 'private'),
);

add_filter('pre_http_request', function ($pre, $args, $url) {
    if (strpos($url, '/health') !== false) {
        return array('response' => array('code' => 200), 'body' => '{"site_installation_registration":true}');
    }
    if (strpos($url, '/sites/register') !== false) {
        return array('response' => array('code' => 200), 'body' => '{"site_id":"site-1","api_key":"server-issued-api-key"}');
    }
    return array('response' => array('code' => 202), 'body' => '{"status":"queued","articles":[]}');
});

require dirname(__DIR__) . '/affilio.php';
affilio_init();
if (!empty($GLOBALS['actions']['rest_api_init'])) {
    foreach ($GLOBALS['actions']['rest_api_init'] as $callback) {
        call_user_func($callback);
    }
}

$onboarding = new Affilio_Onboarding(new Affilio_API());
$onboarding->connect();
$registration = null;
foreach ($GLOBALS['http'] as $call) {
    if (strpos($call['url'], '/sites/register') !== false) {
        $registration = json_decode($call['args']['body'], true);
    }
}
check(is_array($registration), 'site registration made no outbound request');
check(array_keys($registration) === array('site_url', 'plugin_version'), 'registration payload is not limited to site_url and plugin_version');
check($registration['site_url'] === 'https://blog.example', 'registration did not send the site URL');
check($registration['plugin_version'] === '0.1.16', 'registration did not send the plugin version');
$contract = json_decode(file_get_contents(__DIR__ . '/fixtures/sites-register.json'), true);
check($registration === $contract, 'registration payload drifted from the Go contract fixture');
check($contract['plugin_version'] === AFFILIO_VERSION, 'contract fixture plugin_version is not AFFILIO_VERSION');

$scanner = new Affilio_Scanner(new Affilio_API());
$scanner->scan_all_posts();
$scanner->scan_single_post(11);
$scanner->scan_single_post(12);
$scanner->scan_single_post(13);
$editor = null;
foreach ($GLOBALS['routes'] as $route) {
    if ($route[1] === '/editor/analyze') {
        $editor = $route[2]['callback'];
    }
}
check(is_callable($editor), 'editor analyze route is missing');
call_user_func($editor, new Affilio_Test_Request(array('wp_post_id' => 11)));
call_user_func($editor, new Affilio_Test_Request(array('wp_post_id' => 12)));
call_user_func($editor, new Affilio_Test_Request(array('wp_post_id' => 13)));
$state = get_option('affilio_setup_state', array());
if (isset($state['phase']) && $state['phase'] === 'scanning') {
    $onboarding->scan_batch();
}

$captured = array();
foreach ($GLOBALS['http'] as $call) {
    $captured[] = $call['url'];
    if (!empty($call['args']['body'])) {
        $decoded = json_decode($call['args']['body'], true);
        if (is_array($decoded)) {
            $decoded = affilio_privacy_expand($decoded);
            $captured[] = json_encode($decoded, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
        } else {
            $captured[] = $call['args']['body'];
        }
    }
    if (!empty($call['args']['headers'])) {
        $captured[] = json_encode($call['args']['headers']);
    }
}
$blob = implode("\n", $captured);
$forbidden = array(
    'admin@example.com', 'commenter@example.com', '203.0.113.50', 'siteadmin', 'cara-login',
    'Ada Admin', 'Cara Comment', 'Ada Admin Blog', 'Secret comment about the draft',
    'Draft secret launch', 'Draft secret body', 'Private owner notes', 'Private owner body',
);
foreach ($forbidden as $secret) {
    check(strpos($blob, $secret) === false, 'outbound request contained ' . $secret);
}
check(strpos($blob, 'Published lighthouse guide') !== false, 'published title was not sent');
check(strpos($blob, 'Published lighthouse body') !== false, 'published content was not sent');
check(strpos($blob, 'https://blog.example/published-lighthouse') !== false, 'published permalink was not sent');

echo "Registration is allowlisted and scans send only published post data.\n";

function affilio_privacy_expand($value) {
    if (!is_array($value)) {
        return $value;
    }
    if (isset($value['articles']) && is_array($value['articles'])) {
        foreach ($value['articles'] as $index => $article) {
            $value['articles'][$index] = affilio_privacy_expand_article($article);
        }
    }
    return affilio_privacy_expand_article($value);
}

function affilio_privacy_expand_article($article) {
    if (!is_array($article)) {
        return $article;
    }
    $encoding = isset($article['content_encoding']) ? (string) $article['content_encoding'] : 'base64';
    foreach (array('title' => 'title_b64', 'content' => 'content_b64') as $plain => $field) {
        if (!isset($article[$field]) || !is_string($article[$field])) {
            continue;
        }
        $bytes = base64_decode($article[$field], true);
        if ($bytes !== false && $encoding === 'gzip+base64') {
            $bytes = gzdecode($bytes);
        }
        if (is_string($bytes)) {
            $article[$plain] = $bytes;
        }
    }
    return $article;
}
