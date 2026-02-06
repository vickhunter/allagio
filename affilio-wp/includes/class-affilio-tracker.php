<?php

class Affilio_Tracker {
    private $api;

    public function __construct($api) {
        $this->api = $api;
        add_action('wp_enqueue_scripts', array($this, 'enqueue_script'));
    }

    public function enqueue_script() {
        wp_enqueue_script('affilio-track', AFFILIO_PLUGIN_URL . 'public/js/affilio-track.js', array(), AFFILIO_VERSION, true);
    }

    public function register_routes() {
        register_rest_route('affilio/v1', '/track', array(
            'methods'  => 'POST',
            'callback' => array($this, 'track_click'),
            'permission_callback' => '__return_true',
        ));
    }

    public function track_click($request) {
        $params = $request->get_json_params();
        $this->api->request('POST', '/track/click', $params);
        return rest_ensure_response(array('status' => 'ok'));
    }
}
