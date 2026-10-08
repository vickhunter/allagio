<?php

require __DIR__ . '/bootstrap.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

class SkipScanAPI {
    public $calls = array();
    public $reject_id = 5;
    public function invalidate_status_cache() {}
    public function get_connection_status($force = false) {
        return array('connected' => true, 'message' => 'ok', 'state' => 'connected', 'label' => 'Connected');
    }
    public function request($method, $path, $body = null) {
        $this->calls[] = array('path' => $path, 'body' => $body);
        if ($path === '/scan/full') {
            return array('code' => 403, 'data' => null);
        }
        if ($path === '/scan/single') {
            $id = isset($body['wp_post_id']) ? (int) $body['wp_post_id'] : 0;
            if ($id === $this->reject_id) {
                return array('code' => 403, 'data' => 'Blocked');
            }
            return array('code' => 200, 'data' => array('monetization_score' => 1));
        }
        throw new RuntimeException('Unexpected scan path ' . $path);
    }
}

require dirname(__DIR__) . '/affilio.php';

$GLOBALS['user_locale'] = 'it_IT';
$GLOBALS['posts'] = array();
for ($id = 1; $id <= 12; $id++) {
    $GLOBALS['posts'][] = (object) array(
        'ID' => $id,
        'post_title' => 'Post ' . $id,
        'post_content' => 'Body ' . $id,
        'post_name' => 'post-' . $id,
        'post_status' => 'publish',
    );
}
$GLOBALS['posts'][] = (object) array(
    'ID' => 99,
    'post_title' => 'Draft secret launch',
    'post_content' => 'Draft secret body',
    'post_name' => 'draft-secret',
    'post_status' => 'draft',
);

$api = new SkipScanAPI();
$setup = new Affilio_Onboarding($api);
update_option('affilio_setup_state', array('phase' => 'scanning', 'offset' => 0, 'scanned' => 0), false);
$setup->scan_batch();
$singles = array();
foreach ($api->calls as $call) {
    if ($call['path'] === '/scan/single') {
        $singles[] = (int) $call['body']['wp_post_id'];
    }
}
expect($singles === range(1, 10), 'a rejected cron batch was not retried post by post');
$setup->scan_batch();
$state = $setup->status();
expect(isset($state['phase']) && $state['phase'] === 'complete', 'onboarding stayed on analisi non riuscita after a rejected post');
expect(isset($state['summary']) && $state['summary'] === 'Analisi completata: 11 articoli analizzati, 1 saltato', 'setup summary was ' . (isset($state['summary']) ? $state['summary'] : 'missing'));
expect(isset($state['scanned']) && (int) $state['scanned'] === 11, 'analyzed count included the skipped post');
$skipped = isset($state['skipped']) && is_array($state['skipped']) ? $state['skipped'] : array();
expect(count($skipped) === 1, 'skipped list did not contain exactly the rejected post');
if ($skipped) {
    $row = $skipped[0];
    expect((int) $row['wp_post_id'] === 5, 'skipped row did not record the post id');
    expect(isset($row['reason']) && $row['reason'] !== '' && strpos($row['reason'], 'Body') === false && strpos($row['reason'], 'Blocked') === false, 'skipped reason included response or post content');
    expect(!isset($row['content']) && !isset($row['title']), 'skipped row stored post content');
}
$blob = json_encode($state);
expect(strpos($blob, 'Draft secret') === false, 'setup state included an unpublished post');
expect(strpos(json_encode($api->calls), 'Draft secret') === false, 'scan sent an unpublished post');

$onboarding_js = file_get_contents(dirname(__DIR__) . '/admin/build/onboarding.js');
expect(strpos($onboarding_js, 'summary') !== false && strpos($onboarding_js, 'data-skipped') !== false, 'setup screen does not show the summary and skipped list');
expect(strpos($onboarding_js, "phase === 'scan_error'") === false || strpos($onboarding_js, 'data.summary') !== false, 'setup screen can stay on analisi non riuscita');

$batch = new SkipScanAPI();
$batch->reject_id = 15;
$scanner = new Affilio_Scanner($batch);
$GLOBALS['posts'] = array();
for ($id = 1; $id <= 25; $id++) {
    $GLOBALS['posts'][] = (object) array(
        'ID' => $id,
        'post_title' => 'Batch ' . $id,
        'post_content' => 'Batch body ' . $id,
        'post_name' => 'batch-' . $id,
        'post_status' => 'publish',
    );
}
$result = $scanner->scan_all_posts();
$full_sizes = array();
foreach ($batch->calls as $call) {
    if ($call['path'] === '/scan/full') {
        $full_sizes[] = isset($call['body']['articles']) ? count($call['body']['articles']) : 0;
    }
}
expect($full_sizes === array(10, 10, 5), 'Scan All sent ' . json_encode($full_sizes) . ' instead of batches of 10');
expect(is_array($result) && (int) $result['scanned'] === 24, 'Scan All did not count analyzed posts separately from the skipped one');
expect(isset($result['skipped'][0]['wp_post_id']) && (int) $result['skipped'][0]['wp_post_id'] === 15, 'Scan All did not record the skipped post id');
expect(!isset($result['skipped'][0]['content']), 'Scan All skip record stored content');

$admin_src = file_get_contents(dirname(__DIR__) . '/includes/class-affilio-admin.php');
$start = strpos($admin_src, 'function handle_scan_all');
$next = strpos($admin_src, 'function ', $start + 20);
$body = substr($admin_src, $start, $next - $start);
expect(strpos($body, '$result') !== false, 'Scan All still ignores the scanner result');
expect(method_exists('Affilio_Admin', 'format_scan_result'), 'Scan All has no admin-visible result');

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Cron and Scan All retry a rejected batch per post, skip it, and finish.\n";
