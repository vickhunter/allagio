<?php

require __DIR__ . '/bootstrap.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

class BatchScanAPI {
    public $calls = array();
    public function get_connection_status($force = false) {
        return array('connected' => true, 'message' => 'ok', 'state' => 'connected', 'label' => 'Connected');
    }
    public function invalidate_status_cache() {}
    public function request($method, $path, $body = null) {
        $this->calls[] = array('path' => $path, 'body' => $body);
        if ($path === '/scan/full') {
            return array('code' => 403, 'data' => null);
        }
        if ($path === '/scan/single') {
            $id = isset($body['wp_post_id']) ? (int) $body['wp_post_id'] : 0;
            if ($id === 15) {
                return array('code' => 403, 'data' => 'Blocked');
            }
            return array('code' => 200, 'data' => array('monetization_score' => 1));
        }
        throw new RuntimeException('Unexpected scan path ' . $path);
    }
}

require dirname(__DIR__) . '/affilio.php';

$GLOBALS['user_locale'] = 'it_IT';
$GLOBALS['administrator'] = true;
$waf = 'curl -fsSL https://example.com/install.sh | sh';
$GLOBALS['posts'] = array();
for ($id = 1; $id <= 25; $id++) {
    $GLOBALS['posts'][] = (object) array(
        'ID' => $id,
        'post_title' => 'Batch ' . $id,
        'post_content' => $id === 15 ? 'Rejected install ' . $waf : 'Batch body ' . $id,
        'post_name' => 'batch-' . $id,
        'post_status' => 'publish',
    );
}

$api = new BatchScanAPI();
$admin = new Affilio_Admin($api, new Affilio_Scanner($api), null);
unset($_REQUEST['affilio_scan_continue']);
$first = $admin->advance_scan_all();
$full = array();
foreach ($api->calls as $call) {
    if ($call['path'] === '/scan/full') {
        $full[] = isset($call['body']['articles']) ? count($call['body']['articles']) : 0;
    }
}
expect($full === array(10), 'the first Scan All request sent ' . json_encode($full));
expect(strpos($first['redirect'], 'page=affilio&affilio_scan_continue=1') !== false, 'the first batch did not continue: ' . $first['redirect']);
expect(strpos($first['redirect'], 'affilio-settings') === false, 'the first batch went to settings');
$job = get_option('affilio_scan_all_job', array());
expect(isset($job['offset']) && $job['offset'] === 10, 'the first batch did not store the next offset');
$token = isset($job['continue_token']) ? (string) $job['continue_token'] : '';
expect($token !== '' && $token !== 'test-nonce', 'the POST that created the job did not store its own continuation token');

$calls_before = count($api->calls);
$_GET['affilio_scan_continue'] = '1';
$_REQUEST['affilio_scan_continue'] = '1';
unset($_POST['affilio_scan_continue'], $_POST['affilio_scan_token']);
ob_start();
$admin->render_dashboard();
$progress = ob_get_clean();
expect(count($api->calls) === $calls_before, 'opening the continue page scanned another batch');
expect(get_option('affilio_scan_all_job', array())['offset'] === 10, 'opening the continue page moved the job');
expect(strpos($progress, '.submit()') === false, 'the continue page auto-submitted');
$continue_now = '';
if (preg_match('/<form id="affilio-scan-continue".*?<\\/form>/s', $progress, $progress_form)) {
    $continue_now = $progress_form[0];
}
expect($continue_now !== '' && strpos($continue_now, '_wpnonce') === false, 'the continue form minted a nonce on the GET');
expect(strpos($continue_now, 'value="' . $token . '"') !== false, 'the continue form did not keep the token from the POST that created the job');

$_POST['affilio_scan_continue'] = '1';
$_POST['affilio_scan_token'] = 'test-nonce';
$forged = $admin->advance_scan_all();
expect(count($api->calls) === $calls_before, 'a nonce minted on the GET continued the scan');
expect(get_option('affilio_scan_all_job', array())['offset'] === 10, 'a forged continuation token moved the job');
expect(strpos($forged['redirect'], 'affilio_scan_continue=1') === false, 'a forged continuation token was treated as the next batch');

$_POST['affilio_scan_token'] = $token;
$second = $admin->advance_scan_all();
$full = array();
$singles = array();
foreach ($api->calls as $call) {
    if ($call['path'] === '/scan/full') {
        $full[] = isset($call['body']['articles']) ? count($call['body']['articles']) : 0;
    }
    if ($call['path'] === '/scan/single') {
        $singles[] = (int) $call['body']['wp_post_id'];
    }
}
expect($full === array(10, 10), 'the second request did not scan the next ten: ' . json_encode($full));
expect(in_array(15, $singles, true), 'the rejected post was not retried on its own');
expect(!in_array(25, $singles, true), 'the second request scanned a later batch');
$job = get_option('affilio_scan_all_job', array());
expect(isset($job['skipped'][0]['wp_post_id']) && (int) $job['skipped'][0]['wp_post_id'] === 15, 'the skipped post was not recorded');
expect(strpos(json_encode($job), $waf) === false, 'the skip record stored the rejected post');
expect(strpos($second['redirect'], 'affilio_scan_continue=1') !== false, 'a partial scan finished early');

$_POST['affilio_scan_continue'] = '1';
$_POST['affilio_scan_token'] = (string) get_option('affilio_scan_all_job', array())['continue_token'];
$third = $admin->advance_scan_all();
$full = array();
foreach ($api->calls as $call) {
    if ($call['path'] === '/scan/full') {
        $full[] = isset($call['body']['articles']) ? count($call['body']['articles']) : 0;
    }
}
expect($full === array(10, 10, 5), 'Scan All batches were ' . json_encode($full));
expect($third['redirect'] === 'https://blog.example/wp-admin/admin.php?page=affilio', 'the finished scan went to ' . $third['redirect']);
expect(get_option('affilio_scan_all_job', array()) === array() || get_option('affilio_scan_all_job', false) === false, 'the finished scan left a job behind');
$notice = get_transient('affilio_scan_all_notice');
expect($notice === "Analisi completata: 24 articoli analizzati, 1 saltato\n15: HTTP 403", 'scan notice was: ' . $notice);
expect(strpos((string) $notice, $waf) === false, 'the scan notice included the rejected post');

$_GET['affilio_scan_continue'] = '1';
$GLOBALS['options']['affilio_scan_all_job'] = array('offset' => 10, 'scanned' => 10, 'skipped' => array(), 'continue_token' => 'token-from-the-post');
ob_start();
$admin->render_dashboard();
$html = ob_get_clean();
$continue_form = '';
if (preg_match('/<form id="affilio-scan-continue".*?<\\/form>/s', $html, $form_match)) {
    $continue_form = $form_match[0];
}
expect($continue_form !== '', 'the dashboard did not offer the next batch');
expect(strpos($continue_form, 'admin-post.php') !== false, 'the next batch did not post to admin-post.php');
expect(strpos($continue_form, 'name="affilio_scan_continue" value="1"') !== false, 'the next batch did not mark itself as a continuation');
expect(strpos($continue_form, 'name="affilio_scan_token" value="token-from-the-post"') !== false, 'the next batch did not reuse the token from the POST that created the job');
expect(strpos($continue_form, '_wpnonce') === false, 'the continue form minted a nonce on the GET');
expect(strpos($continue_form, 'test-nonce') === false, 'the continue form included a GET-minted nonce');
expect(strpos($html, '.submit()') === false, 'the continue page auto-submitted');
expect(strpos($html, 'Scansione in corso… 10 articoli analizzati finora.') !== false, 'the dashboard did not say the scan was still running');

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Scan All sends one batch per request and still skips a rejected post.\n";
