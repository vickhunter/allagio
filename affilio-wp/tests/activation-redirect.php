<?php

require __DIR__ . '/bootstrap.php';
require dirname(__DIR__) . '/affilio.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

$GLOBALS['administrator'] = true;
$GLOBALS['options']['affilio_setup_redirect'] = true;
$GLOBALS['pagenow'] = 'admin-post.php';
$_REQUEST['action'] = 'affilio_scan_all';
$GLOBALS['redirect'] = '';
affilio_maybe_setup_redirect();
expect($GLOBALS['redirect'] === '', 'Scan All on admin-post.php was sent to ' . $GLOBALS['redirect']);
expect(get_option('affilio_setup_redirect', false) === true, 'admin-post.php consumed the setup redirect before Scan All ran');

unset($GLOBALS['pagenow']);
$_SERVER['PHP_SELF'] = '/wp-admin/admin-post.php';
$_REQUEST['action'] = 'affilio_revert';
$GLOBALS['redirect'] = '';
affilio_maybe_setup_redirect();
expect($GLOBALS['redirect'] === '', 'revert on admin-post.php was sent to ' . $GLOBALS['redirect']);
expect(get_option('affilio_setup_redirect', false) === true, 'revert consumed the setup redirect');

$_SERVER['PHP_SELF'] = '/wp-admin/admin.php';
$GLOBALS['pagenow'] = 'admin.php';
unset($_REQUEST['action']);
$_GET['page'] = 'affilio-distribution';
$_GET['affilio_reverted'] = '1';
$GLOBALS['redirect'] = '';
affilio_maybe_setup_redirect();
expect($GLOBALS['redirect'] === '', 'a finished removal was sent to ' . $GLOBALS['redirect']);
expect(get_option('affilio_setup_redirect', false) === false, 'Distribution left the setup redirect armed');

$GLOBALS['options']['affilio_setup_redirect'] = true;
$GLOBALS['options']['affilio_scan_all_job'] = array('offset' => 10, 'scanned' => 10, 'skipped' => array());
$_GET['page'] = 'affilio';
$_GET['affilio_scan_continue'] = '1';
unset($_GET['affilio_reverted']);
$GLOBALS['redirect'] = '';
affilio_maybe_setup_redirect();
expect(strpos((string) $GLOBALS['redirect'], 'affilio-settings') === false, 'the next scan batch was sent to settings');
expect(get_option('affilio_setup_redirect', false) === false, 'a scan in progress left the setup redirect armed');

$out = tempnam(sys_get_temp_dir(), 'affilio-redirect');
$code = '<?php
register_shutdown_function(function () {
    $url = isset($GLOBALS["redirect"]) ? $GLOBALS["redirect"] : "";
    $flag = get_option("affilio_setup_redirect", false) ? "1" : "0";
    file_put_contents(' . var_export($out, true) . ', $url . "\n" . $flag);
});
require ' . var_export(__DIR__ . '/bootstrap.php', true) . ';
require ' . var_export(dirname(__DIR__) . '/affilio.php', true) . ';
$GLOBALS["administrator"] = true;
$GLOBALS["options"]["affilio_setup_redirect"] = true;
$GLOBALS["pagenow"] = "index.php";
$_SERVER["PHP_SELF"] = "/wp-admin/index.php";
unset($_REQUEST["action"]);
affilio_maybe_setup_redirect();
echo "STILL_RUNNING\n";
';
$file = tempnam(sys_get_temp_dir(), 'affilio-redirect-src');
file_put_contents($file, $code);
$ran = shell_exec('php ' . escapeshellarg($file) . ' 2>&1');
unlink($file);
$recorded = file_get_contents($out);
unlink($out);
expect(strpos((string) $ran, 'STILL_RUNNING') === false, 'the ordinary admin redirect did not exit');
expect(strpos((string) $recorded, 'page=affilio-settings') !== false, 'ordinary admin load did not go to settings: ' . $recorded);
expect(strpos((string) $recorded, "\n0") !== false, 'ordinary admin load left the setup redirect set');

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Activation redirect leaves Scan All and Distribution alone.\n";
