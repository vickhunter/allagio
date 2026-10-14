<?php

$root = dirname(__DIR__);
$main = file_get_contents($root . '/affilio.php');
check_identity(strpos($main, 'Version: 0.1.23') !== false, 'plugin header is not 0.1.23');
check_identity(strpos($main, "define('AFFILIO_VERSION', '0.1.23');") !== false, 'AFFILIO_VERSION is not 0.1.23');
$readme = file_get_contents($root . '/readme.txt');
check_identity(strpos($readme, 'Stable tag: 0.1.23') !== false, 'readme stable tag is not 0.1.23');
check_identity(strpos($readme, '= 0.1.23 =') !== false, 'readme changelog is missing 0.1.23');
check_identity(strpos($readme, '= 0.1.22 =') !== false, 'readme changelog dropped 0.1.22');
check_identity(strpos($readme, '= 0.1.21 =') !== false, 'readme changelog dropped 0.1.21');
check_identity(strpos($readme, '= 0.1.20 =') !== false, 'readme changelog dropped 0.1.20');
check_identity(strpos($readme, '= 0.1.19 =') !== false, 'readme changelog dropped 0.1.19');
check_identity(strpos($readme, '= 0.1.18 =') !== false, 'readme changelog dropped 0.1.18');
check_identity(basename($root . '/affilio.php') === 'affilio.php', 'main file name drifted from v0.1.6 affilio.php');

$dockerfile = file_get_contents(dirname($root) . '/Dockerfile.mvp');
check_identity(strpos($dockerfile, 'wordpress/affilio-wp/affilio.php') !== false, 'release image does not package folder affilio-wp');
check_identity(strpos($dockerfile, 'zip -qr /release/affilio.zip affilio-wp') !== false, 'release ZIP is not rooted at affilio-wp/');
check_identity(strpos($dockerfile, 'COPY affilio-wp/readme.txt') !== false, 'release image does not package readme.txt');

$double = run_php('
require ' . var_export($root . '/tests/bootstrap.php', true) . ';
require ' . var_export($root . '/affilio.php', true) . ';
require ' . var_export($root . '/affilio.php', true) . ';
echo "loaded\n";
');
check_identity($double[0] === 0 && strpos($double[1], 'loaded') !== false, "loading the plugin twice fataled:\n" . $double[1]);

$old = run_php('
function affilio_init() {}
class Affilio_API {}
class Affilio_Scanner {}
class Affilio_Inserter {}
class Affilio_Tracker {}
class Affilio_Admin {}
class Affilio_Editor_Proxy {}
class Affilio_Onboarding {}
class Affilio_Distribution {}
require ' . var_export($root . '/tests/bootstrap.php', true) . ';
require ' . var_export($root . '/affilio.php', true) . ';
echo "loaded\n";
');
check_identity($old[0] === 0 && strpos($old[1], 'loaded') !== false, "load beside v0.1.6 fataled:\n" . $old[1]);

echo "Plugin 0.1.23 packages as affilio-wp/affilio.php and survives a double load.\n";

function check_identity($condition, $message) {
    if (!$condition) {
        fwrite(STDERR, $message . "\n");
        exit(1);
    }
}

function run_php($code) {
    $file = tempnam(sys_get_temp_dir(), 'affilio');
    file_put_contents($file, "<?php\n" . $code);
    $output = array();
    $status = 1;
    exec('php ' . escapeshellarg($file) . ' 2>&1', $output, $status);
    unlink($file);
    return array($status, implode("\n", $output));
}
