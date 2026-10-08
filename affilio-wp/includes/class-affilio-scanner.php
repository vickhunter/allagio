<?php

if (class_exists('Affilio_Scanner')) {
    return;
}

class Affilio_Scanner {
    private $api;

    public function __construct($api) {
        $this->api = $api;
    }

    public function scan_all_posts() {
        $posts = get_posts(array('post_type' => 'post', 'posts_per_page' => -1, 'post_status' => 'publish'));
        $articles = array();
        foreach ($posts as $post) {
            if (isset($post->post_status) && $post->post_status !== 'publish') {
                continue;
            }
            $articles[] = array(
                'wp_post_id' => $post->ID,
                'title'      => $post->post_title,
                'content'    => $post->post_content,
                'url'        => get_permalink($post),
            );
        }
        return $this->api->request('POST', '/scan/full', array('articles' => $articles));
    }

    public function scan_single_post($post_id) {
        $post = get_post($post_id);
        if (!$post || !isset($post->post_status) || $post->post_status !== 'publish') {
            return new WP_Error('affilio_unpublished', 'Only published posts can be scanned.');
        }
        $payload = array(
            'wp_post_id' => $post->ID,
            'title'      => $post->post_title,
            'content'    => $post->post_content,
            'url'        => get_permalink($post),
        );
        return $this->api->request('POST', '/scan/single', $payload);
    }
}
