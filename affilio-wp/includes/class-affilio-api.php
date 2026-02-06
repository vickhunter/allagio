<?php

class Affilio_API {
    private $base_url;
    private $api_key;
    private $status_cache_key = 'affilio_connection_status';

    public function __construct() {
        $this->base_url = self::resolve_base_url();
        $this->api_key = get_option('affilio_api_key', '');
    }

    public function request($method, $path, $body = null) {
        $url = trailingslashit($this->base_url) . ltrim($path, '/');
        $args = array(
            'method'  => $method,
            'headers' => array(
                'Content-Type' => 'application/json',
                'X-API-Key'    => $this->api_key,
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
        $default = defined('AFFILIO_API_URL') ? AFFILIO_API_URL : 'https://api.affilio.io/v1';
        $url = trim((string) get_option('affilio_api_url_override', ''));
        if ($url === '') {
            $url = $default;
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
            'label' => 'Missing API Key',
            'message' => 'Enter your Affilio API key to connect this site.',
            'connected' => false,
            'checked_at' => gmdate('c'),
            'http_code' => 0,
        );

        if (trim($this->api_key) === '') {
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        $health = wp_remote_get(trailingslashit($this->base_url) . 'health', array('timeout' => 8));
        if (is_wp_error($health)) {
            $status['state'] = 'network_error';
            $status['label'] = 'Connection Error';
            $status['message'] = 'Cannot reach Affilio API. Check network or API URL.';
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        $probe = $this->request('GET', '/articles');
        if (is_wp_error($probe)) {
            $status['state'] = 'network_error';
            $status['label'] = 'Connection Error';
            $status['message'] = 'Cannot reach Affilio API. Check network or API URL.';
            set_transient($this->status_cache_key, $status, 60);
            return $status;
        }

        $code = isset($probe['code']) ? (int) $probe['code'] : 0;
        $status['http_code'] = $code;
        if ($code >= 200 && $code < 300) {
            $status['state'] = 'connected';
            $status['label'] = 'Connected';
            $status['message'] = 'Affilio is connected and ready.';
            $status['connected'] = true;
        } elseif ($code === 401 || $code === 403) {
            $status['state'] = 'invalid_key';
            $status['label'] = 'Invalid API Key';
            $status['message'] = 'API key rejected. Check or regenerate your key.';
        } elseif ($code === 404) {
            $status['state'] = 'wrong_api_url';
            $status['label'] = 'Invalid API URL';
            $status['message'] = 'API URL does not match a valid Affilio backend.';
        } elseif ($code >= 500) {
            $status['state'] = 'api_error';
            $status['label'] = 'API Error';
            $status['message'] = 'Affilio API returned a server error.';
        } else {
            $status['state'] = 'unknown_error';
            $status['label'] = 'Connection Failed';
            $status['message'] = 'Unexpected API response while validating credentials.';
        }
        set_transient($this->status_cache_key, $status, 60);
        return $status;
    }
}
