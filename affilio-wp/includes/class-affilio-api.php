<?php

if (class_exists('Affilio_API')) {
    return;
}

class Affilio_API {
    private $base_url;
    private $status_cache_key = 'affilio_connection_status';

    public function __construct() {
        $this->base_url = self::resolve_base_url();
    }

    private function current_api_key() {
        return trim((string) get_option('affilio_api_key', ''));
    }

    public function request($method, $path, $body = null) {
        $this->base_url = self::resolve_base_url();
        if ($this->base_url === '') {
            return new WP_Error('affilio_not_configured', 'Not connected — set the API URL in Settings');
        }
        $url = trailingslashit($this->base_url) . ltrim($path, '/');
        $args = array(
            'method'  => $method,
            'timeout' => 30,
            'redirection' => 0,
            'headers' => array(
                'Content-Type' => 'application/json',
                'X-API-Key'    => $this->current_api_key(),
            ),
        );
        $path_only = $path;
        $query_at = strpos($path, '?');
        if ($query_at !== false) {
            $path_only = substr($path, 0, $query_at);
        }
        if (rtrim($path_only, '/') === '/sites/register') {
            $body = array(
                'site_url' => home_url(),
                'plugin_version' => defined('AFFILIO_VERSION') ? AFFILIO_VERSION : '',
            );
        }
        if (strtoupper($method) === 'GET' && rtrim($path_only, '/') === '/health') {
            $health = $this->service_health();
            if (is_wp_error($health)) {
                return $health;
            }
            $decoded = json_decode(wp_remote_retrieve_body($health), true);
            return array(
                'code' => (int) wp_remote_retrieve_response_code($health),
                'data' => is_array($decoded) ? $decoded : array(),
            );
        }
        if ($body !== null) {
            $body = $this->encode_scan_body($path_only, $body);
            $args['body'] = wp_json_encode($body);
        }
        $response = wp_remote_request($url, $args);
        if (is_wp_error($response)) {
            return $response;
        }
        $code = wp_remote_retrieve_response_code($response);
        $data = json_decode(wp_remote_retrieve_body($response), true);
        return array('code' => $code, 'data' => $data);
    }

    public function admin_request($method, $path, $body = null) {
        $this->base_url = self::resolve_base_url();
        if ($this->base_url === '') {
            return new WP_Error('affilio_not_configured', 'Not connected — set the API URL in Settings');
        }
        $args = array(
            'method'  => $method,
            'timeout' => 30,
            'redirection' => 0,
            'headers' => array(
                'Content-Type' => 'application/json',
                'X-Admin-Key'  => trim((string) get_option('affilio_admin_api_key', '')),
            ),
        );
        if ($body !== null) {
            $args['body'] = wp_json_encode($body);
        }
        $response = wp_remote_request(trailingslashit($this->base_url) . ltrim($path, '/'), $args);
        if (is_wp_error($response)) {
            return $response;
        }
        return array(
            'code' => wp_remote_retrieve_response_code($response),
            'data' => json_decode(wp_remote_retrieve_body($response), true),
        );
    }

    public static function mask_key($key) {
        $key = trim((string) $key);
        if ($key === '') {
            return '';
        }
        if (strlen($key) < 8) {
            return '••••••••';
        }
        return substr($key, 0, 4) . '…';
    }

    public static function normalize_base_url($url) {
        $url = trim((string) $url);
        if ($url === '') {
            return '';
        }
        $url = rtrim($url, '/');
        if (!preg_match('#/v1$#', $url)) {
            $url .= '/v1';
        }
        return $url;
    }

    public static function resolve_base_url() {
        $url = defined('AFFILIO_API_URL') ? trim((string) AFFILIO_API_URL) : '';
        $override = trim((string) get_option('affilio_api_url_override', ''));
        if ($override !== '') {
            $url = $override;
        }
        return self::normalize_base_url(apply_filters('affilio_api_base_url', $url));
    }

    private function connection_cache_key() {
        $this->base_url = self::resolve_base_url();
        return 'affilio_cs_' . md5($this->base_url . "\n" . $this->current_api_key());
    }

    public function invalidate_status_cache() {
        $index = get_option('affilio_status_cache_keys', array());
        if (is_array($index)) {
            foreach ($index as $key) {
                delete_transient($key);
            }
        }
        delete_option('affilio_status_cache_keys');
        delete_transient($this->connection_cache_key());
        delete_transient($this->status_cache_key);
    }

    private function forget_other_status_caches($current) {
        $index = get_option('affilio_status_cache_keys', array());
        if (!is_array($index)) {
            $index = array();
        }
        $keep = array();
        foreach ($index as $key) {
            if ($key === $current) {
                $keep[] = $key;
                continue;
            }
            delete_transient($key);
        }
        delete_transient($this->status_cache_key);
        if ($keep !== $index) {
            update_option('affilio_status_cache_keys', $keep, false);
        }
    }

    private function remember_status_cache($key) {
        $index = get_option('affilio_status_cache_keys', array());
        if (!is_array($index)) {
            $index = array();
        }
        if (!in_array($key, $index, true)) {
            $index[] = $key;
            update_option('affilio_status_cache_keys', $index, false);
        }
    }

    private function store_status($cache_key, $status, $ttl) {
        set_transient($cache_key, $status, $ttl);
        $this->remember_status_cache($cache_key);
        return $status;
    }

    public function get_connection_status($force_refresh = false) {
        $this->base_url = self::resolve_base_url();
        $cache_key = $this->connection_cache_key();
        $this->forget_other_status_caches($cache_key);
        $cached = get_transient($cache_key);
        if (!$force_refresh && is_array($cached)) {
            return $cached;
        }

        $status = array(
            'state' => 'missing_key',
            'label' => $this->t('Ready to connect', 'Pronto per il collegamento'),
            'message' => $this->t('Open Offer Rails settings to connect and find opportunities.', 'Apri le impostazioni Offer Rails per collegarti e trovare opportunità.'),
            'connected' => false,
            'checked_at' => gmdate('c'),
            'http_code' => 0,
        );

        if ($this->base_url === '') {
            $status['state'] = 'missing_url';
            $status['label'] = $this->t('Not connected', 'Non connesso');
            $status['message'] = 'Not connected — set the API URL in Settings';
            return $this->store_status($cache_key, $status, 60);
        }

        if ($this->current_api_key() === '') {
            return $this->store_status($cache_key, $status, 60);
        }

        $health = $this->service_health();
        if (is_wp_error($health)) {
            if ($this->error_code($health) === 'affilio_bad_url') {
                $status['state'] = 'wrong_api_url';
                $status['label'] = $this->t('Invalid API URL', 'URL API non valido');
            } else {
                $status['state'] = 'starting';
                $status['label'] = $this->t('Starting', 'In avvio');
            }
            $status['message'] = $health->get_error_message();
            return $this->store_status($cache_key, $status, 15);
        }

        $probe = $this->request('GET', '/articles');
        if (is_wp_error($probe)) {
            $status['state'] = 'network_error';
            $status['label'] = $this->t('Connection Error', 'Errore di connessione');
            $status['message'] = $this->t('Cannot reach the Offer Rails API. Check network or API URL.', 'Impossibile raggiungere l\'API di Offer Rails. Controlla rete o URL API.');
            return $this->store_status($cache_key, $status, 60);
        }

        $code = isset($probe['code']) ? (int) $probe['code'] : 0;
        $status['http_code'] = $code;
        if ($code >= 200 && $code < 300) {
            $status['state'] = 'connected';
            $status['label'] = $this->t('Connected', 'Connesso');
            $status['message'] = $this->t('Offer Rails is connected and ready.', 'Offer Rails è connesso e pronto.');
            $status['connected'] = true;
        } elseif ($code === 401 || $code === 403) {
            $status['state'] = 'invalid_key';
            $status['label'] = $this->t('Invalid API Key', 'Chiave API non valida');
            $status['message'] = $this->t('API key rejected. Check or regenerate your key.', 'Chiave API rifiutata. Controllala o rigenerala.');
        } elseif ($code === 404) {
            $status['state'] = 'wrong_api_url';
            $status['label'] = $this->t('Invalid API URL', 'URL API non valido');
            $status['message'] = $this->t('API URL does not match a valid Offer Rails backend.', 'L\'URL API non corrisponde a un backend Offer Rails valido.');
        } elseif ($code >= 500) {
            $status['state'] = 'api_error';
            $status['label'] = $this->t('API Error', 'Errore API');
            $status['message'] = $this->t('The Offer Rails API returned a server error.', 'L\'API di Offer Rails ha restituito un errore server.');
        } else {
            $status['state'] = 'unknown_error';
            $status['label'] = $this->t('Connection Failed', 'Connessione non riuscita');
            $status['message'] = $this->t('Unexpected API response while validating credentials.', 'Risposta API inattesa durante la validazione delle credenziali.');
        }
        return $this->store_status($cache_key, $status, 60);
    }

    private function service_health() {
        $this->base_url = self::resolve_base_url();
        $url = trailingslashit($this->base_url) . 'health';
        $timeouts = array(8, 20);
        $health = null;
        foreach ($timeouts as $index => $timeout) {
            $health = wp_remote_get($url, array('timeout' => $timeout));
            if ($this->health_is_client_error($health) || (is_wp_error($health) && !$this->transport_is_cold_start($health))) {
                return new WP_Error('affilio_bad_url', $this->refused_url_message(), array('status' => 502));
            }
            if (!$this->health_needs_retry($health) || $index > 0) {
                break;
            }
        }
        if ($this->health_needs_retry($health)) {
            return new WP_Error(
                'affilio_starting',
                $this->t('The service is starting. Try again in a few seconds.', 'Il servizio si sta avviando, riprova tra qualche secondo'),
                array('status' => 503)
            );
        }
        $code = (int) wp_remote_retrieve_response_code($health);
        if ($code >= 200 && $code < 300) {
            $this->remember_known_base_url();
        }
        return $health;
    }

    private function health_needs_retry($health) {
        if (is_wp_error($health)) {
            return $this->transport_is_cold_start($health);
        }
        $code = (int) wp_remote_retrieve_response_code($health);
        return $code === 0 || $code === 502 || $code === 503 || $code === 504;
    }

    private function health_is_client_error($health) {
        if (is_wp_error($health)) {
            return false;
        }
        $code = (int) wp_remote_retrieve_response_code($health);
        return $code >= 400 && $code < 500;
    }

    private function transport_is_cold_start($error) {
        $message = '';
        if (is_object($error) && method_exists($error, 'get_error_message')) {
            $message = strtolower((string) $error->get_error_message());
        }
        $curl = 0;
        if (preg_match('/curl error (\d+)/', $message, $match)) {
            $curl = (int) $match[1];
        }
        $dns = $curl === 6 || strpos($message, 'could not resolve') !== false || strpos($message, 'getaddrinfo') !== false || strpos($message, 'name or service not known') !== false;
        $tls_codes = array(35, 51, 53, 54, 58, 59, 60, 66, 77, 80, 82, 83, 90, 91);
        $tls = in_array($curl, $tls_codes, true) || strpos($message, 'ssl') !== false || strpos($message, 'certificate') !== false || strpos($message, 'tls') !== false;
        if ($dns || $tls) {
            return false;
        }
        $refused = $curl === 7 || strpos($message, 'connection refused') !== false;
        if ($refused) {
            return $this->base_url_is_known();
        }
        return $curl === 28 || $curl === 52 || strpos($message, 'timed out') !== false || strpos($message, 'timeout') !== false;
    }

    private function refused_url_message() {
        return $this->t(
            'The API URL is wrong or was refused. Check the address in Settings › Offer Rails.',
            'L\'URL API è sbagliato o è stato rifiutato. Controlla l\'indirizzo in Impostazioni › Offer Rails.'
        );
    }

    private function remember_known_base_url() {
        $url = (string) $this->base_url;
        if ($url === '') {
            return;
        }
        $known = get_option('affilio_known_api_urls', array());
        if (!is_array($known)) {
            $known = array();
        }
        if (in_array($url, $known, true)) {
            return;
        }
        $known[] = $url;
        if (count($known) > 8) {
            $known = array_slice($known, -8);
        }
        update_option('affilio_known_api_urls', $known, false);
    }

    private function base_url_is_known() {
        $known = get_option('affilio_known_api_urls', array());
        return is_array($known) && in_array((string) $this->base_url, $known, true);
    }

    private function error_code($error) {
        if (is_object($error) && method_exists($error, 'get_error_code')) {
            return (string) $error->get_error_code();
        }
        if (is_object($error) && isset($error->code)) {
            return (string) $error->code;
        }
        return '';
    }

    private function encode_scan_body($path, $body) {
        $bare = rtrim((string) $path, '/');
        if ($bare !== '/scan/full' && $bare !== '/scan/single') {
            return $body;
        }
        if (!is_array($body)) {
            return $body;
        }
        if (isset($body['articles']) && is_array($body['articles'])) {
            foreach ($body['articles'] as $index => $article) {
                $body['articles'][$index] = $this->encode_scan_article($article);
            }
            return $body;
        }
        return $this->encode_scan_article($body);
    }

    private function encode_scan_article($article) {
        if (!is_array($article) || isset($article['content_b64']) || isset($article['title_b64'])) {
            return $article;
        }
        $encoding = function_exists('gzencode') ? 'gzip+base64' : 'base64';
        $encoded = false;
        foreach (array('title' => 'title_b64', 'content' => 'content_b64') as $plain => $field) {
            if (!array_key_exists($plain, $article)) {
                continue;
            }
            $article[$field] = $this->encode_scan_field((string) $article[$plain], $encoding);
            unset($article[$plain]);
            $encoded = true;
        }
        if ($encoded) {
            $article['content_encoding'] = $encoding;
        }
        return $article;
    }

    private function encode_scan_field($value, $encoding) {
        $bytes = $value;
        if ($encoding === 'gzip+base64') {
            $gzipped = gzencode($bytes);
            if ($gzipped !== false) {
                $bytes = $gzipped;
            }
        }
        return base64_encode($bytes);
    }

    private function t($en, $it) {
        $locale = function_exists('get_user_locale') ? get_user_locale() : get_locale();
        if (strpos(strtolower((string) $locale), 'it') === 0) {
            return $it;
        }
        return $en;
    }
}
