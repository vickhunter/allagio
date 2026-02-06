<?php

class Affilio_Inserter {
    private $api;

    public function __construct($api) {
        $this->api = $api;
    }

    public function register_routes() {
        register_rest_route('affilio/v1', '/insert', array(
            'methods'  => 'POST',
            'callback' => array($this, 'handle_insert'),
            'permission_callback' => function () {
                return current_user_can('edit_posts');
            },
        ));
    }

    public function handle_insert($request) {
        $params = $request->get_json_params();
        $post_id = isset($params['post_id']) ? intval($params['post_id']) : 0;
        $html = isset($params['html']) ? $params['html'] : '';
        $rec_id = isset($params['recommendation_id']) ? $params['recommendation_id'] : '';
        if (!$post_id || $html === '') {
            return rest_ensure_response(array('status' => 'error', 'message' => 'invalid payload'));
        }
        $result = $this->insert_inline_mention($post_id, $html);
        if (is_wp_error($result)) {
            return rest_ensure_response(array('status' => 'error', 'message' => $result->get_error_message()));
        }
        if ($rec_id) {
            $this->api->request('POST', '/insert/confirm', array('recommendation_id' => $rec_id));
        }
        return rest_ensure_response(array('status' => 'ok'));
    }

    public function insert_inline_mention($post_id, $html) {
        $post = get_post($post_id);
        if (!$post) {
            return new WP_Error('affilio_not_found', 'Post not found');
        }
        $content = $post->post_content;
        $disclosure = $this->disclosure_block();
        if (!get_post_meta($post_id, '_affilio_disclosure_inserted', true)) {
            $content .= "\n\n" . $disclosure;
            update_post_meta($post_id, '_affilio_disclosure_inserted', 1);
        }
        $content .= "\n\n" . $html;
        wp_update_post(array(
            'ID'           => $post_id,
            'post_content' => $content,
        ));
        return true;
    }

    private function disclosure_block() {
        return '<p class="affilio-disclosure"><em>Disclosure: This post may contain affiliate links, which means we may earn a commission if you click and make a purchase.</em></p>';
    }
}
