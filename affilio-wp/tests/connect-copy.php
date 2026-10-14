<?php

require __DIR__ . '/bootstrap.php';
require dirname(__DIR__) . '/affilio.php';

$failures = array();
function expect($condition, $message) {
    if (!$condition) {
        $GLOBALS['failures'][] = $message;
    }
}

class ConnectCopyAPI {
    public $mode = 'version';
    public function invalidate_status_cache() {}
    public function get_connection_status($force = false) {
        return array('connected' => false, 'message' => 'unused');
    }
    public function request($method, $path, $body = null) {
        if ($path === '/health') {
            $ready = $this->mode !== 'version';
            return array('code' => 200, 'data' => array('site_installation_registration' => $ready));
        }
        return array('code' => 500, 'data' => array());
    }
}

$GLOBALS['user_locale'] = 'it_IT';
$GLOBALS['administrator'] = true;
$GLOBALS['options']['affilio_api_url_override'] = '';
$GLOBALS['options']['affilio_api_key'] = '';

$missing = (new Affilio_Onboarding(new ConnectCopyAPI()))->connect();
expect($missing instanceof WP_Error && $missing->get_error_message() === 'Imposta l\'URL API in Impostazioni prima di collegarti.', 'missing URL said: ' . ($missing instanceof WP_Error ? $missing->get_error_message() : 'not an error'));

$GLOBALS['options']['affilio_api_url_override'] = 'https://api.example.test/v1';
$GLOBALS['options']['affilio_setup_lock'] = time();
$busy = (new Affilio_Onboarding(new ConnectCopyAPI()))->connect();
expect($busy instanceof WP_Error && $busy->get_error_message() === 'La configurazione è già in corso. Riprova tra poco.', 'busy connect said: ' . ($busy instanceof WP_Error ? $busy->get_error_message() : 'not an error'));
unset($GLOBALS['options']['affilio_setup_lock']);

$version = (new Affilio_Onboarding(new ConnectCopyAPI()))->connect();
expect($version instanceof WP_Error && $version->get_error_message() === 'Questo servizio non è pronto per il collegamento in un clic. Aggiorna il backend prima di collegarti.', 'unready service said: ' . ($version instanceof WP_Error ? $version->get_error_message() : 'not an error'));

$api = new ConnectCopyAPI();
$api->mode = 'register';
$failed = (new Affilio_Onboarding($api))->connect();
expect($failed instanceof WP_Error && $failed->get_error_message() === 'Collegamento non riuscito. Puoi riprovare in sicurezza.', 'failed connect said: ' . ($failed instanceof WP_Error ? $failed->get_error_message() : 'not an error'));

$GLOBALS['administrator'] = false;
$denied = (new Affilio_Onboarding(new ConnectCopyAPI()))->connect();
expect($denied instanceof WP_Error && $denied->get_error_message() === 'Serve un amministratore.', 'non-admin connect said: ' . ($denied instanceof WP_Error ? $denied->get_error_message() : 'not an error'));

foreach (array($missing, $busy, $version, $failed, $denied) as $error) {
    if ($error instanceof WP_Error) {
        expect(stripos($error->get_error_message(), 'Could not connect') === false, 'an Italian connect error stayed in English');
        expect(stripos($error->get_error_message(), 'not ready') === false, 'an Italian connect error said not ready');
    }
}

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "Connect errors are Italian for an Italian admin.\n";
