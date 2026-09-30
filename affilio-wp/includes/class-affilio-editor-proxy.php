<?php

class Affilio_Editor_Proxy {
    private $api;
    private $inserter;

    public function __construct($api, $inserter) {
        $this->api = $api;
        $this->inserter = $inserter;
    }

    public function register_routes() {
        register_rest_route('affilio/v1', '/editor/analyze', array('methods' => 'POST', 'callback' => array($this, 'analyze'), 'permission_callback' => function ($request) {
            return current_user_can('edit_post', (int) $request->get_param('wp_post_id'));
        }));
        register_rest_route('affilio/v1', '/editor/approve-insert', array('methods' => 'POST', 'callback' => array($this, 'approve_insert'), 'permission_callback' => function ($request) {
            return current_user_can('edit_post', (int) $request->get_param('post_id'));
        }));
    }

    public function analyze($request) {
        $post = get_post((int) $request->get_param('wp_post_id'));
        if (!$post || !current_user_can('edit_post', $post->ID)) {
            return new WP_Error('affilio_post', 'Post not found or not editable.', array('status' => 403));
        }
        return $this->response($this->api->request('POST', '/scan/single', array('wp_post_id' => $post->ID, 'title' => $post->post_title, 'content' => $post->post_content, 'url' => get_permalink($post))));
    }

    public function approve_insert($request) {
        $post_id = (int) $request->get_param('post_id');
        $id = (string) $request->get_param('recommendation_id');
        if (!$post_id || !current_user_can('edit_post', $post_id) || !preg_match('/^[a-f0-9-]{36}$/i', $id)) {
            return new WP_Error('affilio_post', 'Invalid recommendation or post access.', array('status' => 403));
        }
        $articles = $this->api->request('GET', '/articles');
        if (is_wp_error($articles) || $articles['code'] !== 200) {
            return $this->response($articles);
        }
        $article_id = '';
        foreach ($articles['data']['articles'] ?? array() as $article) {
            if ((int) $article['wp_post_id'] === $post_id) {
                $article_id = $article['id'];
                break;
            }
        }
        $article = $article_id ? $this->api->request('GET', '/articles/' . rawurlencode($article_id)) : null;
        $found = false;
        if (is_array($article) && $article['code'] === 200) {
            foreach ($article['data']['recommendations'] ?? array() as $rec) {
                if ($rec['id'] === $id) {
                    $found = true;
                    break;
                }
            }
        }
        if (!$found) {
            return new WP_Error('affilio_recommendation', 'Recommendation does not belong to this post.', array('status' => 404));
        }
        $generated = $this->api->request('POST', '/generate/content', array('recommendation_id' => $id, 'article_context' => get_post($post_id)->post_content));
        if (is_wp_error($generated) || $generated['code'] !== 200 || empty($generated['data']['html'])) {
            return $this->response($generated);
        }
        $approved = $this->api->request('PATCH', '/recommendations/' . rawurlencode($id), array('status' => 'approved'));
        if (is_wp_error($approved) || $approved['code'] < 200 || $approved['code'] >= 300) {
            return $this->response($approved);
        }
        $result = $this->inserter->insert_inline_mention($post_id, wp_kses_post($generated['data']['html']), $generated['data']['suggested_position'] ?? 'end_of_article');
        if (is_wp_error($result)) {
            return $result;
        }
        return $this->response($this->api->request('POST', '/insert/confirm', array('recommendation_id' => $id)));
    }

    private function response($response) {
        if (is_wp_error($response)) {
            return new WP_Error('affilio_api', 'Affilio is unavailable. Try again shortly.', array('status' => 502));
        }
        return new WP_REST_Response($response['data'], $response['code']);
    }
}
