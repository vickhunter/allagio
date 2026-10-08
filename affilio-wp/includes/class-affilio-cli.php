<?php

if (class_exists('Affilio_Revert_Command')) {
    return;
}

class Affilio_Revert_Command {
    private $distribution;
    public $lines = array();

    public function __construct($distribution) {
        $this->distribution = $distribution;
    }

    public function __invoke($args, $assoc_args) {
        $code = $this->run($args, $assoc_args);
        if ($code !== 0 && class_exists('WP_CLI')) {
            WP_CLI::halt($code);
        }
    }

    public function run($args, $assoc_args) {
        $this->lines = array();
        $post_id = isset($args[0]) ? (int) $args[0] : 0;
        $placement = isset($assoc_args['placement']) ? (string) $assoc_args['placement'] : '';
        $dry_run = !empty($assoc_args['dry-run']);
        $result = $this->distribution->revert_inserted($post_id, $placement, !$dry_run, $dry_run);
        if (empty($result['matched'])) {
            $this->line('nothing matched');
            return 1;
        }
        $title = isset($result['title']) ? $result['title'] : '';
        foreach ($result['removed'] as $item) {
            if ($item['kind'] !== 'placement') {
                continue;
            }
            $prefix = $dry_run ? 'Dry run' : 'Rimosso';
            $this->line($prefix . ' post ' . $post_id . ' «' . $title . '»: ' . $item['anchor'] . ' ' . $item['href']);
        }
        if ($dry_run) {
            return 0;
        }
        foreach ($result['placement_ids'] as $id) {
            $state = isset($result['api'][$id]) ? $result['api'][$id] : 'unreachable';
            $this->line(self::api_line($state));
        }
        return 0;
    }

    public static function api_line($state) {
        if ($state === 'reported') {
            return 'API: segnalato';
        }
        if ($state === 'already') {
            return 'API: già rimosso';
        }
        return 'API: non raggiungibile, riprovo più tardi';
    }

    private function line($text) {
        $this->lines[] = $text;
        if (class_exists('WP_CLI')) {
            WP_CLI::log($text);
        }
    }
}
