<?php

if (class_exists('Affilio_Inserter')) {
    return;
}

class Affilio_Inserter {
    private $api;

    public function __construct($api) {
        $this->api = $api;
    }

    public function insert_inline_mention($post_id, $html, $suggested_position = 'end_of_article') {
        $post = get_post($post_id);
        if (!$post) {
            return new WP_Error('affilio_not_found', 'Post not found');
        }
        $content = $post->post_content;
        $content = $this->inject_by_position($content, $html, $suggested_position);
        if (!get_post_meta($post_id, '_affilio_disclosure_inserted', true) && !Affilio_Distribution::disclosure_suppressed($content)) {
            $content .= "\n\n" . $this->disclosure_block();
            update_post_meta($post_id, '_affilio_disclosure_inserted', 1);
        }
        wp_update_post(array(
            'ID'           => $post_id,
            'post_content' => $content,
        ));
        return true;
    }

    private function disclosure_block() {
        return '<p class="affilio-disclosure"><em>Disclosure: This post may contain affiliate links, which means we may earn a commission if you click and make a purchase.</em></p>';
    }

    private function inject_by_position($content, $html, $position) {
        $position = sanitize_key((string) $position);
        $valid_positions = array('after_intro', 'mid_article', 'before_conclusion', 'end_of_article');
        if (!in_array($position, $valid_positions, true)) {
            $position = 'end_of_article';
        }
        if ($position === 'end_of_article' || strpos((string) $content, '<!-- wp:') !== false) {
            return rtrim((string) $content) . "\n\n" . $html;
        }

        $parts = preg_split('/\n\s*\n/', trim((string) $content));
        if (!is_array($parts) || count($parts) < 2) {
            return rtrim((string) $content) . "\n\n" . $html;
        }

        $count = count($parts);
        $start = 1;
        $end = max(1, $count - 1);
        if ($position === 'after_intro') {
            $start = 1;
            $end = min($count - 1, 4);
        } elseif ($position === 'before_conclusion') {
            $start = max(1, $count - 4);
            $end = max($start, $count - 1);
        } elseif ($position === 'mid_article') {
            $start = max(1, (int) floor($count * 0.25));
            $end = max($start, min($count - 1, (int) floor($count * 0.85)));
        }

        $insert_index = $this->best_insertion_index($parts, $html, $start, $end);
        if ($insert_index < 1 || $insert_index > $count) {
            $insert_index = $start;
        }

        array_splice($parts, $insert_index, 0, array($html));
        return implode("\n\n", $parts);
    }

    private function best_insertion_index($paragraphs, $snippet_html, $start, $end) {
        $focus = $this->focus_tokens(wp_strip_all_tags($snippet_html));
        $best_index = $start;
        $best_score = -1;
        $middle = ($start + $end) / 2.0;

        for ($i = $start; $i <= $end; $i++) {
            if (!isset($paragraphs[$i])) {
                continue;
            }
            $plain = wp_strip_all_tags((string) $paragraphs[$i]);
            $tokens = $this->tokenize($plain);
            if (empty($tokens)) {
                continue;
            }
            $score = 0.0;
            $score += $this->overlap_score($tokens, $focus) * 4.0;
            $token_count = count($tokens);
            if ($token_count >= 35 && $token_count <= 220) {
                $score += 2.0;
            }
            $plain_lc = strtolower($plain);
            if ($this->contains_intent_cue($plain_lc)) {
                $score += 2.0;
            }
            if (strpos((string) $paragraphs[$i], 'http') !== false) {
                $score += 0.8;
            }
            $score -= abs($middle - $i) * 0.15;

            if ($score > $best_score) {
                $best_score = $score;
                $best_index = $i;
            }
        }

        return $best_index;
    }

    private function contains_intent_cue($text) {
        $cues = array(
            'review', 'compare', 'comparison', 'best', 'pricing', 'tool', 'software', 'plugin', 'trial',
            'guida', 'confronto', 'miglior', 'prezzo', 'strumento', 'prova', 'consigliamo', 'consiglio', 'scopri',
        );
        foreach ($cues as $cue) {
            if (strpos($text, $cue) !== false) {
                return true;
            }
        }
        return false;
    }

    private function focus_tokens($text) {
        $tokens = $this->tokenize($text);
        $base = array(
            'review', 'compare', 'comparison', 'pricing', 'best', 'tool', 'software', 'plugin', 'guide', 'trial',
            'guida', 'confronto', 'prezzo', 'migliore', 'strumento', 'prova', 'scopri',
        );
        $all = array_unique(array_merge($tokens, $base));
        return array_slice($all, 0, 40);
    }

    private function overlap_score($tokens, $focus) {
        if (empty($tokens) || empty($focus)) {
            return 0;
        }
        $token_set = array_fill_keys($tokens, true);
        $overlap = 0;
        foreach ($focus as $f) {
            if (isset($token_set[$f])) {
                $overlap++;
            }
        }
        return $overlap;
    }

    private function tokenize($text) {
        $text = strtolower((string) $text);
        $parts = preg_split('/[^a-z0-9]+/', $text);
        if (!is_array($parts)) {
            return array();
        }
        $out = array();
        foreach ($parts as $p) {
            $p = trim($p);
            if ($p === '' || strlen($p) < 4) {
                continue;
            }
            $out[] = $p;
        }
        return array_values(array_unique($out));
    }
}
