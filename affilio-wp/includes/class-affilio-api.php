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
        if ($body !== null) {
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

    public static function resolve_base_url() {
        $url = defined('AFFILIO_API_URL') ? trim((string) AFFILIO_API_URL) : '';
        $override = trim((string) get_option('affilio_api_url_override', ''));
        if ($override !== '') {
            $url = $override;
        }
        return rtrim((string) apply_filters('affilio_api_base_url', $url), '/');
    }

    public function invalidate_status_cache() {
        delete_transient($this->status_cache_key);
    }

    public function get_connection_status($force_refresh = false) {
        $cached = get_transient($this->status_cache_key);
        if (!$force_refresh && is_array($cached)) {
            return $cached;
        }

        $status = array(
            'state' => 'missing_key',
            'label' => $this->t('Ready to connect', 'Pronto per il collegamento'),
            'message' => $this->t('Open Affilio settings to connect and find opportunities.', 'Apri le impostazioni Affilio per collegarti e trovare opportunità.'),
            'connected' => false,
            'checked_at' => gmdate('c'),
            'http_code' => 0,
        );

        if ($this->base_url === '') {
            $status['state'] = 'missing_url';
            $status['label'] = $this->t('Not connected', 'Non connesso');
            $status['message'] = 'Not connected — set the API URL in Settings';
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        if ($this->current_api_key() === '') {
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        $health = wp_remote_get(trailingslashit($this->base_url) . 'health', array('timeout' => 8));
        if (is_wp_error($health)) {
            $status['state'] = 'network_error';
            $status['label'] = $this->t('Connection Error', 'Errore di connessione');
            $status['message'] = $this->t('Cannot reach Affilio API. Check network or API URL.', 'Impossibile raggiungere l\'API di Affilio. Controlla rete o URL API.');
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        $probe = $this->request('GET', '/articles');
        if (is_wp_error($probe)) {
            $status['state'] = 'network_error';
            $status['label'] = $this->t('Connection Error', 'Errore di connessione');
            $status['message'] = $this->t('Cannot reach Affilio API. Check network or API URL.', 'Impossibile raggiungere l\'API di Affilio. Controlla rete o URL API.');
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        $code = isset($probe['code']) ? (int) $probe['code'] : 0;
        $status['http_code'] = $code;
        if ($code >= 200 && $code < 300) {
            $status['state'] = 'connected';
            $status['label'] = $this->t('Connected', 'Connesso');
            $status['message'] = $this->t('Affilio is connected and ready.', 'Affilio è connesso e pronto.');
            $status['connected'] = true;
        } elseif ($code === 401 || $code === 403) {
            $status['state'] = 'invalid_key';
            $status['label'] = $this->t('Invalid API Key', 'Chiave API non valida');
            $status['message'] = $this->t('API key rejected. Check or regenerate your key.', 'Chiave API rifiutata. Controllala o rigenerala.');
        } elseif ($code === 404) {
            $status['state'] = 'wrong_api_url';
            $status['label'] = $this->t('Invalid API URL', 'URL API non valido');
            $status['message'] = $this->t('API URL does not match a valid Affilio backend.', 'L\'URL API non corrisponde a un backend Affilio valido.');
        } elseif ($code >= 500) {
            $status['state'] = 'api_error';
            $status['label'] = $this->t('API Error', 'Errore API');
            $status['message'] = $this->t('Affilio API returned a server error.', 'L\'API di Affilio ha restituito un errore server.');
        } else {
            $status['state'] = 'unknown_error';
            $status['label'] = $this->t('Connection Failed', 'Connessione non riuscita');
            $status['message'] = $this->t('Unexpected API response while validating credentials.', 'Risposta API inattesa durante la validazione delle credenziali.');
        }
        set_transient($this->status_cache_key, $status, 60);
        return $status;
    }

    private function t($en, $it) {
        $locale = function_exists('get_user_locale') ? get_user_locale() : get_locale();
        if (strpos(strtolower((string) $locale), 'it') === 0) {
            return $it;
        }
        return $en;
    }
}
