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
        return $this->scan_posts($posts);
    }

    public function scan_published_batch($offset) {
        $offset = max(0, (int) $offset);
        $posts = get_posts(array(
            'post_type' => 'post',
            'post_status' => 'publish',
            'posts_per_page' => 10,
            'offset' => $offset,
            'orderby' => 'ID',
            'order' => 'ASC',
        ));
        if (!is_array($posts)) {
            $posts = array();
        }
        $result = $this->scan_posts($posts);
        $fetched = count($posts);
        $result['fetched'] = $fetched;
        $result['next_offset'] = $offset + $fetched;
        $result['done'] = $fetched < 10;
        return $result;
    }

    public function scan_posts($posts) {
        $ready = array();
        foreach ($posts as $post) {
            if (isset($post->post_status) && $post->post_status !== 'publish') {
                continue;
            }
            $ready[] = $post;
        }
        $scanned = 0;
        $skipped = array();
        foreach (array_chunk($ready, 10) as $chunk) {
            $articles = array();
            foreach ($chunk as $post) {
                $articles[] = $this->article_payload($post);
            }
            $response = $this->api->request('POST', '/scan/full', array('articles' => $articles));
            if ($this->scan_accepted($response)) {
                $scanned += count($chunk);
                continue;
            }
            foreach ($chunk as $post) {
                $single = $this->api->request('POST', '/scan/single', $this->article_payload($post));
                if ($this->scan_accepted($single)) {
                    $scanned++;
                    continue;
                }
                $skipped[] = array(
                    'wp_post_id' => (int) $post->ID,
                    'reason' => $this->skip_reason($single),
                );
            }
        }
        return array('scanned' => $scanned, 'skipped' => $skipped);
    }

    private function article_payload($post) {
        return array(
            'wp_post_id' => $post->ID,
            'title' => $post->post_title,
            'content' => $post->post_content,
            'url' => get_permalink($post),
        );
    }

    private function scan_accepted($response) {
        return !is_wp_error($response) && isset($response['code']) && (int) $response['code'] >= 200 && (int) $response['code'] < 300;
    }

    private function skip_reason($response) {
        if (is_wp_error($response)) {
            return 'request failed';
        }
        $code = isset($response['code']) ? (int) $response['code'] : 0;
        return 'HTTP ' . $code;
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
