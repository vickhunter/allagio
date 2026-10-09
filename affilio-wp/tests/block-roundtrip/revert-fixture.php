<?php

class WP_Error {
    public $code;
    public $message;
    public $data;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; $this->data = $data; }
    public function get_error_message() { return $this->message; }
}
function is_wp_error($value) { return $value instanceof WP_Error; }
function wp_strip_all_tags($value) { return strip_tags((string) $value); }
function check($condition, $message) { if (!$condition) { fwrite(STDERR, $message . "\n"); exit(1); } }

$raw = stream_get_contents(STDIN);
if ($raw === '' || $raw === false) {
    exit(0);
}
$payload = json_decode($raw, true);
require dirname(__DIR__, 2) . '/includes/class-affilio-distribution.php';
$id = $payload['id'];
$kept = $payload['kept'];
foreach (array('inserted' => $payload['inserted'], 'extra' => $payload['extra'], 'resaved' => $payload['resaved']) as $label => $content) {
    $result = Affilio_Distribution::revert_content($content, $id);
    $left = rtrim($result['content'], "\n");
    check($result['matched'] && $left === $kept, $label . " was not removed cleanly:\n" . $result['content']);
    check(strpos($result['content'], 'affilio-placement-') === false && strpos($result['content'], 'affilio-disclosure-') === false && strpos($result['content'], 'distributoor-placement:') === false, $label . ' left a placement marker');
}
$again = Affilio_Distribution::revert_content($kept, $id);
check(!$again['matched'], 're-revert of a clean post matched something');
if (!empty($payload['stripped'])) {
    $stripped = Affilio_Distribution::revert_content($payload['stripped'], $id);
    $left = rtrim($stripped['content'], "\n");
    check($stripped['matched'] && $left === $payload['strippedKept'], "class-stripped go link was not removed:\n" . $stripped['content']);
    check(strpos($stripped['content'], 'placement=' . $id) === false, 'stripped revert left this placement id in an href');
    check(strpos($stripped['content'], $payload['otherId']) !== false, 'another placement was removed');
    check(strpos($stripped['content'], 'solo nel testo') !== false, 'a text mention of the id was removed');
    check(strpos($stripped['content'], 'link sponsorizzato') !== false, 'the other placement label was removed');
}
if (!empty($payload['foreign'])) {
    $all = Affilio_Distribution::revert_content($payload['foreign'], '');
    check(!$all['matched'] && $all['content'] === $payload['foreign'], 'a non-go link was removed when no placement id was requested');
    $one = Affilio_Distribution::revert_content($payload['foreign'], $id);
    check($one['matched'] && rtrim($one['content'], "\n") === $kept, "a specific placement id did not remove the class-stripped link:\n" . $one['content']);
}
echo "round-tripped content reverted\n";
