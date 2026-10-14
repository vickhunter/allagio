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
        $exit = 0;
        foreach ($result['placement_ids'] as $id) {
            $state = isset($result['api'][$id]) ? $result['api'][$id] : array('state' => 'unreachable');
            $this->line(self::api_line($state));
            if (is_array($state) && isset($state['state']) && $state['state'] === 'client_error') {
                $exit = 1;
            }
        }
        return $exit;
    }

    public static function api_line($state) {
        $outcome = is_array($state) ? $state : array('state' => (string) $state);
        $name = isset($outcome['state']) ? $outcome['state'] : 'unreachable';
        if ($name === 'reported') {
            return 'API: segnalato';
        }
        if ($name === 'already') {
            return 'API: già rimosso';
        }
        if ($name === 'not_published') {
            return "Rimosso dal post. L'inserimento non era pubblicato, quindi non c'è niente da segnalare.";
        }
        if ($name === 'client_error') {
            $code = isset($outcome['code']) ? (int) $outcome['code'] : 0;
            $reason = isset($outcome['reason']) ? $outcome['reason'] : '';
            return Affilio_Distribution::client_error_line($code, $reason);
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
