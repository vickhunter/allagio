<?php

require __DIR__ . '/bootstrap.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

$GLOBALS['user_locale'] = 'it_IT';
$GLOBALS['options']['affilio_api_url_override'] = 'https://sleeping.example/v1';
$GLOBALS['options']['affilio_api_key'] = 'site-key';
$GLOBALS['health_timeouts'] = array();
$GLOBALS['health_mode'] = 'recover';

add_filter('pre_http_request', function ($pre, $args, $url) {
    $timeout = isset($args['timeout']) ? (int) $args['timeout'] : 0;
    if (strpos($url, '/health') !== false) {
        $GLOBALS['health_timeouts'][] = $timeout;
        $attempt = count($GLOBALS['health_timeouts']);
        if ($GLOBALS['health_mode'] === 'recover' && $attempt >= 2) {
            return array('response' => array('code' => 200), 'body' => '{"status":"ok","site_installation_registration":true}');
        }
        if ($GLOBALS['health_mode'] === 'ok') {
            return array('response' => array('code' => 200), 'body' => '{"status":"ok","site_installation_registration":true}');
        }
        if ($GLOBALS['health_mode'] === 'dns') {
            return new WP_Error('http_request_failed', 'cURL error 6: Could not resolve host: dns.example');
        }
        if ($GLOBALS['health_mode'] === 'tls') {
            return new WP_Error('http_request_failed', 'cURL error 60: SSL certificate problem: unable to get local issuer certificate');
        }
        if ($GLOBALS['health_mode'] === 'refused') {
            return new WP_Error('http_request_failed', 'cURL error 7: Failed to connect to sleeping.example port 443 after 0 ms: Connection refused');
        }
        if ($GLOBALS['health_mode'] === 'missing') {
            return array('response' => array('code' => 404), 'body' => 'no such path');
        }
        if ($GLOBALS['health_mode'] === 'denied') {
            return array('response' => array('code' => 401), 'body' => 'denied');
        }
        return new WP_Error('http_request_failed', 'cURL error 28: Operation timed out after 8000 milliseconds');
    }
    if (strpos($url, '/articles') !== false) {
        return array('response' => array('code' => 200), 'body' => '[]');
    }
    return array('response' => array('code' => 200), 'body' => '{}');
});

require dirname(__DIR__) . '/affilio.php';

$client = new Affilio_API();
$status = $client->get_connection_status(true);
expect($status['connected'] === true, 'a 13s cold start was not retried with a longer health timeout');
expect($GLOBALS['health_timeouts'] === array(8, 20), 'health retry timeouts were ' . json_encode($GLOBALS['health_timeouts']));

$GLOBALS['health_timeouts'] = array();
$GLOBALS['health_mode'] = 'stay-asleep';
$asleep = $client->get_connection_status(true);
expect($asleep['connected'] === false, 'a still-sleeping service was marked connected');
expect($asleep['message'] === 'Il servizio si sta avviando, riprova tra qualche secondo', 'cold start message was ' . $asleep['message']);
foreach (array('URL API non valido', 'non corrisponde', 'Invalid API URL', 'Check network or API URL', 'cURL', 'sleeping.example') as $wrong) {
    expect(strpos($asleep['message'], $wrong) === false, 'cold start message looked like a bad URL: ' . $wrong);
}

$GLOBALS['health_timeouts'] = array();
$GLOBALS['options']['affilio_api_key'] = '';
$setup = new Affilio_Onboarding($client);
$connect = $setup->connect();
expect($connect instanceof WP_Error, 'connect did not report a sleeping service');
if ($connect instanceof WP_Error) {
    expect($connect->get_error_message() === 'Il servizio si sta avviando, riprova tra qualche secondo', 'connect said: ' . $connect->get_error_message());
    expect(stripos($connect->get_error_message(), 'not ready') === false, 'connect called a cold start an incompatible backend');
}

$GLOBALS['health_mode'] = 'recover';
$GLOBALS['health_timeouts'] = array();
$GLOBALS['options']['affilio_api_url_override'] = 'https://old.example/v1';
$GLOBALS['options']['affilio_api_key'] = 'key-one';
$keyed = new Affilio_API();
$keyed->get_connection_status(true);
$first = 'affilio_cs_' . md5("https://old.example/v1\nkey-one");
expect(get_transient($first) !== false, 'connection status was not cached for the first URL');
$GLOBALS['options']['affilio_api_key'] = 'key-two';
$keyed->get_connection_status(true);
$second = 'affilio_cs_' . md5("https://old.example/v1\nkey-two");
expect(get_transient($first) === false, 'the previous API key left a status transient behind');
$GLOBALS['options']['affilio_api_url_override'] = 'https://new.example/v1';
$keyed->get_connection_status(true);
expect(get_transient($second) === false, 'the previous API URL left a status transient behind');
$third = 'affilio_cs_' . md5("https://new.example/v1\nkey-two");
expect(get_transient($third) !== false, 'the current URL and key were not cached');

$refused = 'L\'URL API è sbagliato o è stato rifiutato. Controlla l\'indirizzo in Impostazioni › Offer Rails.';
$starting = 'Il servizio si sta avviando, riprova tra qualche secondo';

$GLOBALS['health_mode'] = 'refused';
$GLOBALS['health_timeouts'] = array();
$known_refused = $client->get_connection_status(true);
expect($known_refused['message'] === $starting, 'connection refused on a known URL said: ' . $known_refused['message']);
expect($GLOBALS['health_timeouts'] === array(8, 20), 'a sleeping known host was not retried: ' . json_encode($GLOBALS['health_timeouts']));

$GLOBALS['options']['affilio_api_url_override'] = 'https://unknown.example/v1';
$GLOBALS['health_timeouts'] = array();
$unknown_refused = (new Affilio_API())->get_connection_status(true);
expect($unknown_refused['message'] === $refused, 'connection refused on an unknown URL said: ' . $unknown_refused['message']);
expect($GLOBALS['health_timeouts'] === array(8), 'an unknown refused URL was retried: ' . json_encode($GLOBALS['health_timeouts']));
expect(strpos($unknown_refused['message'], 'si sta avviando') === false, 'an unknown refused URL looked like a cold start');

foreach (array(
    'dns' => 'https://dns.example/v1',
    'tls' => 'https://tls.example/v1',
    'missing' => 'https://missing.example/v1',
    'denied' => 'https://denied.example/v1',
) as $mode => $url) {
    $GLOBALS['health_mode'] = $mode;
    $GLOBALS['health_timeouts'] = array();
    $GLOBALS['options']['affilio_api_url_override'] = $url;
    $GLOBALS['http'] = array();
    $bad = (new Affilio_API())->get_connection_status(true);
    expect($bad['message'] === $refused, $mode . ' said: ' . $bad['message']);
    expect(count($GLOBALS['health_timeouts']) === 1, $mode . ' was retried: ' . json_encode($GLOBALS['health_timeouts']));
    expect(strpos($bad['message'], 'si sta avviando') === false, $mode . ' looked like a cold start');
    expect(stripos($bad['message'], 'cURL') === false, $mode . ' leaked cURL');
    expect(strpos($bad['message'], 'example') === false, $mode . ' leaked the host');
    $probed = false;
    foreach ($GLOBALS['http'] as $call) {
        if (strpos($call['url'], '/articles') !== false) {
            $probed = true;
        }
    }
    expect($probed === false, $mode . ' still probed articles');
}

$GLOBALS['health_mode'] = 'dns';
$GLOBALS['health_timeouts'] = array();
$GLOBALS['options']['affilio_api_key'] = '';
$GLOBALS['options']['affilio_api_url_override'] = 'https://dns.example/v1';
$dns_connect = (new Affilio_Onboarding(new Affilio_API()))->connect();
expect($dns_connect instanceof WP_Error, 'connect did not report a bad URL');
if ($dns_connect instanceof WP_Error) {
    expect($dns_connect->get_error_message() === $refused, 'connect on DNS failure said: ' . $dns_connect->get_error_message());
    expect(strpos($dns_connect->get_error_message(), 'si sta avviando') === false, 'connect called a DNS failure a cold start');
}

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Cold start retries health once, and old status caches are deleted.\n";
