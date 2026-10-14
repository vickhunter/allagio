<?php

class WP_Error {
    public $code;
    public $message;
    public $data;
    public function __construct($code, $message, $data = array()) { $this->code = $code; $this->message = $message; $this->data = $data; }
    public function get_error_message() { return $this->message; }
    public function get_error_data($code = '') { return $this->data; }
}
function is_wp_error($value) { return $value instanceof WP_Error; }
function rest_ensure_response($value) { return $value; }
function current_user_can($capability, $id = null) {
    if ($capability === 'edit_post') {
        return !empty($GLOBALS['can_edit']) && (int) $id === 7;
    }
    return !empty($GLOBALS['allowed']);
}
function absint($value) { return abs((int) $value); }
function get_post($id) { return (int) $id === 7 ? $GLOBALS['post'] : null; }
function get_permalink($post) { return 'https://blog.example/post'; }
function wp_strip_all_tags($value) { return strip_tags((string) $value); }
function esc_html($value) { return htmlspecialchars((string) $value, ENT_QUOTES, 'UTF-8'); }
function esc_url($value) { return (string) $value; }
function wp_slash($value) { return addslashes((string) $value); }
function wp_update_post($values, $error = false) {
    $GLOBALS['post']->post_content = stripslashes($values['post_content']);
    $GLOBALS['updates']++;
    return $values['ID'];
}
function get_option($key, $default = false) {
    return array_key_exists($key, $GLOBALS['options']) ? $GLOBALS['options'][$key] : $default;
}
function update_option($key, $value, $autoload = null) { $GLOBALS['options'][$key] = $value; return true; }
function wp_verify_nonce($nonce, $action) { return $nonce === 'nonce-for-' . $action; }
function wp_nonce_field($action, $name, $referer, $echo) { return '<input type="hidden" name="' . $name . '" value="nonce-for-' . $action . '">'; }
function admin_url($path = '') { return 'https://blog.example/wp-admin/' . $path; }
function add_query_arg($args, $url) {
    return $url . '?' . http_build_query($args);
}
function wp_next_scheduled($hook) { return !empty($GLOBALS['scheduled'][$hook]); }
function wp_schedule_single_event($timestamp, $hook) { $GLOBALS['scheduled'][$hook] = $timestamp; return true; }
class Affilio_API { public static function resolve_base_url() { return 'https://distribution.example/v1'; } }
class Request {
    private $values;
    public function __construct($values) { $this->values = $values; }
    public function get_param($name) { return isset($this->values[$name]) ? $this->values[$name] : null; }
}

$placement_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
$copy = 'Uno strumento utile per il team.';
$disclosure = 'Affiliate disclosure: I may earn a commission if you buy through this link.';
$tracked = '/v1/go/test?placement=' . $placement_id;
$origin = 'https://distribution.example';

class RevertAPI {
    public $removed_mode = 'ok';
    public $paths = array();
    public function request($method, $path, $body = null) {
        global $placement_id, $copy, $disclosure, $tracked;
        $this->paths[] = $method . ' ' . $path;
        $placement = array(
            'id' => $placement_id,
            'surface_url' => 'https://blog.example/post',
            'status' => 'committed',
            'copy' => $copy,
            'disclosure' => $disclosure,
            'receipt' => array('tracked_link' => $tracked, 'publication_status' => 'awaiting_publication'),
        );
        if ($method === 'GET' && strpos($path, '/sites/owner-connection') !== false) {
            return array('code' => 200, 'data' => array('connected' => true));
        }
        if ($method === 'GET' || strpos($path, '/commit') !== false) {
            return array('code' => 200, 'data' => array('placement' => $placement));
        }
        if (strpos($path, '/receipt') !== false) {
            if (!empty($GLOBALS['force_receipt'])) {
                return array('code' => (int) $GLOBALS['force_receipt'], 'data' => array('error' => 'published text must include approved copy, disclosure and exact tracked link'));
            }
            return array('code' => 200, 'data' => array('status' => 'publisher_reported'));
        }
        if (strpos($path, '/removed') !== false) {
            if ($this->removed_mode === 'down') {
                return array('code' => 503, 'data' => array('error' => 'down'));
            }
            if ($this->removed_mode === 'network') {
                return new WP_Error('http_request_failed', 'connection refused');
            }
            if ($this->removed_mode === 'not_published') {
                return array('code' => 409, 'data' => array('error' => 'placement is not published', 'reason' => 'not_published'));
            }
            if ($this->removed_mode === 'forbidden') {
                return array('code' => 403, 'data' => array('error' => 'publisher account is not connected'));
            }
            if ($this->removed_mode === 'unauthorized') {
                return array('code' => 401, 'data' => array('error' => 'invalid api key'));
            }
            if ($this->removed_mode === 'already') {
                return array('code' => 200, 'data' => array('status' => 'removed', 'already_removed' => true));
            }
            return array('code' => 200, 'data' => array('status' => 'removed', 'already_removed' => false));
        }
        return array('code' => 404, 'data' => array());
    }
}

function check($condition, $message) {
    if (!$condition) {
        fwrite(STDERR, $message . "\n");
        exit(1);
    }
}

function parse_blocks($content) {
    $blocks = array();
    $pattern = '/<!--\s+(\/)?wp:([a-z0-9\/-]+)(\s+(\{.*?\}))?\s*(\/)?-->/s';
    $offset = 0;
    $length = strlen($content);
    while ($offset < $length && preg_match($pattern, $content, $match, PREG_OFFSET_CAPTURE, $offset)) {
        $at = $match[0][1];
        $before = substr($content, $offset, $at - $offset);
        if (trim($before) !== '') {
            $blocks[] = array('blockName' => null, 'attrs' => array(), 'innerHTML' => $before);
        }
        $closer = $match[1][0] === '/';
        $name = $match[2][0];
        $void = isset($match[5][0]) && $match[5][0] === '/';
        $end = $at + strlen($match[0][0]);
        if ($closer || $void) {
            $offset = $end;
            continue;
        }
        $close = '<!-- /wp:' . $name . ' -->';
        $close_at = strpos($content, $close, $end);
        if ($close_at === false) {
            $blocks[] = array('blockName' => null, 'attrs' => array(), 'innerHTML' => substr($content, $at));
            return $blocks;
        }
        $attrs = array();
        if (!empty($match[4][0])) {
            $decoded = json_decode($match[4][0], true);
            if (is_array($decoded)) {
                $attrs = $decoded;
            }
        }
        $block_name = strpos($name, '/') === false ? 'core/' . $name : $name;
        $blocks[] = array('blockName' => $block_name, 'attrs' => $attrs, 'innerHTML' => substr($content, $end, $close_at - $end));
        $offset = $close_at + strlen($close);
    }
    $rest = substr($content, $offset);
    if (trim($rest) !== '') {
        $blocks[] = array('blockName' => null, 'attrs' => array(), 'innerHTML' => $rest);
    }
    return $blocks;
}

require_once __DIR__ . '/wp-filters.php';
require __DIR__ . '/../includes/class-affilio-distribution.php';
require __DIR__ . '/../includes/class-affilio-cli.php';

$GLOBALS['allowed'] = true;
$GLOBALS['can_edit'] = true;
$GLOBALS['options'] = array();
$GLOBALS['scheduled'] = array();
$GLOBALS['updates'] = 0;
$GLOBALS['force_receipt'] = 0;

$kept = "<!-- wp:paragraph -->\n<p>Articolo da conservare.</p>\n<!-- /wp:paragraph -->";
$api = new RevertAPI();
$distribution = new Affilio_Distribution($api);
$GLOBALS['post'] = (object) array('ID' => 7, 'post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'Strumenti utili', 'post_content' => $kept);
$result = $distribution->publish(new Request(array('post_id' => 7, 'placement_id' => $placement_id)));
check(!is_wp_error($result), 'insert failed');
$inserted = substr($GLOBALS['post']->post_content, strlen($kept));
$inner = esc_html($copy) . ' <a href="/go/' . $placement_id . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)';
$placement_block = '<!-- wp:paragraph {"className":"affilio-placement-' . $placement_id . '"} -->' . "\n" . '<p class="affilio-placement-' . $placement_id . '">' . $inner . '</p>' . "\n" . '<!-- /wp:paragraph -->';
$disclosure_block = '<!-- wp:paragraph {"className":"affilio-disclosure-' . $placement_id . '"} -->' . "\n" . '<p class="affilio-disclosure-' . $placement_id . '">' . esc_html($disclosure) . '</p>' . "\n" . '<!-- /wp:paragraph -->';
check($inserted === "\n" . $placement_block . "\n" . $disclosure_block, "inserted markup was:\n" . $inserted);
$blocks = parse_blocks($GLOBALS['post']->post_content);
$paragraphs = array();
foreach ($blocks as $block) {
    check($block['blockName'] === 'core/paragraph', 'parse_blocks produced a freeform block');
    $paragraphs[] = $block;
}
$marked = array_values(array_filter($paragraphs, function ($block) use ($placement_id) {
    $class_name = isset($block['attrs']['className']) ? $block['attrs']['className'] : '';
    return in_array('affilio-placement-' . $placement_id, preg_split('/\s+/', $class_name), true);
}));
check(count($marked) === 1, 'parse_blocks did not yield the placement paragraph');
check(strpos($marked[0]['innerHTML'], '<!--') === false, 'the paragraph innerHTML still contains an HTML comment');

$removed = $distribution->revert_inserted(7, $placement_id, false, false);
check(!empty($removed['matched']) && $GLOBALS['post']->post_content === $kept, 'reverting the className layout did not restore the other blocks');
check(strpos($GLOBALS['post']->post_content, 'link sponsorizzato') === false, 'revert left the sponsor label in the post');
check(strpos(implode("\n", $api->paths), '/removed') === false, 'a local-only revert called the API');

$legacy_inner = '<!-- distributoor-placement:' . $placement_id . ' --><p>' . $inner . '</p>';
$legacy = $kept . "\n" . "<!-- wp:paragraph -->\n" . $legacy_inner . "\n<!-- /wp:paragraph -->\n" . '<!-- wp:paragraph -->' . "\n" . '<p class="affiliate-disclosure">' . esc_html($disclosure) . '</p>' . "\n" . '<!-- /wp:paragraph -->';
$GLOBALS['post']->post_content = $legacy;
$legacy_result = Affilio_Distribution::revert_content($legacy, $placement_id);
check($legacy_result['content'] === $kept, "the 0.1.13 layout was not removed cleanly:\n" . $legacy_result['content']);

$resaved = $kept . "\n" . "<!-- wp:paragraph -->\n<p><!-- distributoor-placement:" . $placement_id . " -->" . $inner . "</p>\n<!-- /wp:paragraph -->";
$resaved_result = Affilio_Distribution::revert_content($resaved, $placement_id);
check($resaved_result['content'] === $kept, "the editor re-saved 0.1.13 layout was not removed cleanly:\n" . $resaved_result['content']);

$extra = $kept . "\n" . '<!-- wp:paragraph {"className":"affilio-placement-' . $placement_id . ' my-extra"} -->' . "\n" . '<p class="affilio-placement-' . $placement_id . ' my-extra">' . $inner . '</p>' . "\n" . '<!-- /wp:paragraph -->';
$extra_result = Affilio_Distribution::revert_content($extra, $placement_id);
check($extra_result['content'] === $kept, 'an extra editor class kept the placement in the post');

$GLOBALS['post']->post_content = $extra;
$GLOBALS['can_edit'] = false;
$denied = $distribution->perform_revert_action(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check($denied['status'] === 403 && $GLOBALS['post']->post_content === $extra, 'revert ran without edit_post');
$GLOBALS['can_edit'] = true;
$bad_nonce = $distribution->perform_revert_action(7, $placement_id, 'nope');
check($bad_nonce['status'] === 403 && $GLOBALS['post']->post_content === $extra, 'revert ran with the wrong nonce');
$confirm = $distribution->confirmation_view(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check($confirm['status'] === 200, 'confirmation was refused for an editor');
check(strpos($confirm['html'], 'Strumenti utili') !== false, 'confirmation omitted the post title');
$nbsp_anchor = "Vedi\xc2\xa0il\xc2\xa0prodotto";
check(strpos($confirm['html'], 'Vedi&nbsp;il&nbsp;prodotto') !== false || strpos($confirm['html'], $nbsp_anchor) !== false, 'confirmation omitted the anchor');
check(strpos($confirm['html'], '/go/' . $placement_id) !== false, 'confirmation omitted the go URL');
check(strpos($confirm['html'], 'Annulla') !== false && strpos($confirm['html'], '>Rimuovi<') !== false, 'confirmation omitted Annulla / Rimuovi');

$api->removed_mode = 'down';
$api->paths = array();
$down = $distribution->perform_revert_action(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check($down['status'] === 200 && $GLOBALS['post']->post_content === $kept, 'an API outage rolled the local revert back');
check(strpos($down['notice'], 'API: non raggiungibile, riprovo più tardi') !== false, 'the notice did not say the API will be retried: ' . $down['notice']);
check(isset($GLOBALS['options']['affilio_removal_queue'][$placement_id]), 'a failed report was not queued');
$api->removed_mode = 'ok';
$distribution->retry_removals();
check(empty($GLOBALS['options']['affilio_removal_queue']), 'the queue did not retry the removal report');
$again = $distribution->revert_inserted(7, $placement_id, true, false);
check(empty($again['matched']), 're-reverting matched a placement that was already gone');
check($GLOBALS['post']->post_content === $kept, 're-revert changed the post');

$GLOBALS['post']->post_content = $legacy;
$command = new Affilio_Revert_Command($distribution);
$dry = $command->run(array('7'), array('placement' => $placement_id, 'dry-run' => true));
check($dry === 0 && $GLOBALS['post']->post_content === $legacy, 'dry-run changed the post');
check(strpos(implode("\n", $command->lines), 'Dry run') !== false, 'dry-run did not describe the removal');
$removed_calls = 0;
foreach ($api->paths as $path) { if (strpos($path, '/removed') !== false) { $removed_calls++; } }
$before_calls = $removed_calls;
$api->removed_mode = 'already';
$cli = $command->run(array('7'), array('placement' => $placement_id));
check($cli === 0 && $GLOBALS['post']->post_content === $kept, 'CLI revert left the 0.1.13 layout');
check(strpos(implode("\n", $command->lines), 'API: già rimosso') !== false, 'CLI did not report already removed');
$gone = $command->run(array('7'), array('placement' => $placement_id));
check($gone !== 0 && strpos(implode("\n", $command->lines), 'nothing matched') !== false, 'CLI succeeded when nothing matched');

$GLOBALS['post']->post_content = $resaved;
$api->removed_mode = 'down';
$cli_down = $command->run(array('7'), array('placement' => $placement_id));
check($cli_down === 0 && $GLOBALS['post']->post_content === $kept, 'CLI treated an API failure as a local failure');
check(strpos(implode("\n", $command->lines), 'API: non raggiungibile, riprovo più tardi') !== false, 'CLI omitted the retry line');

$GLOBALS['force_receipt'] = 409;
$GLOBALS['post']->post_content = $kept;
$api->paths = array();
$refused = $distribution->publish(new Request(array('post_id' => 7, 'placement_id' => $placement_id)));
check(is_wp_error($refused) && $GLOBALS['post']->post_content === $kept, 'a refused receipt left the className block in the post');
foreach ($api->paths as $path) {
    check(strpos($path, '/removed') === false, 'a refused receipt called /removed');
}

$GLOBALS['post']->post_content = $extra;
$extra_cli = (new Affilio_Revert_Command($distribution))->run(array('7'), array('placement' => $placement_id));
check($extra_cli === 0 && $GLOBALS['post']->post_content === $kept, 'CLI left the extra-class placement in the post');

$other_id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
$stripped_inner = esc_html($copy) . ' <a href="' . $origin . $tracked . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)';
$other_inner = 'Altro <a href="' . $origin . '/v1/go/other?placement=' . $other_id . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)';
$mention = 'Il placement ' . $placement_id . ' compare solo nel testo.';
$other_block = "<!-- wp:paragraph -->\n<p>" . $other_inner . "</p>\n<!-- /wp:paragraph -->";
$mention_block = "<!-- wp:paragraph -->\n<p>" . $mention . "</p>\n<!-- /wp:paragraph -->";
$stripped = $kept . "\n" . "<!-- wp:paragraph -->\n<p>" . $stripped_inner . "</p>\n<!-- /wp:paragraph -->\n" . $other_block . "\n" . $mention_block;
$stripped_kept = $kept . "\n" . $other_block . "\n" . $mention_block;
$stripped_result = Affilio_Distribution::revert_content($stripped, $placement_id);
check(!empty($stripped_result['matched']) && $stripped_result['content'] === $stripped_kept, "class-stripped go link was not removed:\n" . $stripped_result['content']);
check(strpos($stripped_result['content'], 'link sponsorizzato') !== false, 'revert removed the other placement label');
check(strpos($stripped_result['content'], 'placement=' . $other_id) !== false, 'revert removed another placement');
check(strpos($stripped_result['content'], 'solo nel testo') !== false, 'revert removed a paragraph that only mentions the id');
$local_only = $kept . "\n" . "<!-- wp:paragraph -->\n<p>" . esc_html($copy) . ' <a href="/go/' . $placement_id . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)</p>' . "\n<!-- /wp:paragraph -->";
$local_only_result = Affilio_Distribution::revert_content($local_only, $placement_id);
check(!empty($local_only_result['matched']) && $local_only_result['content'] === $kept, "class-stripped /go/ link was not removed:\n" . $local_only_result['content']);
$prefix = $kept . "\n" . '<!-- wp:paragraph -->' . "\n" . '<p><a href="' . $origin . '/v1/go/test?placement=' . $placement_id . 'ffff" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a></p>' . "\n" . '<!-- /wp:paragraph -->';
$prefix_result = Affilio_Distribution::revert_content($prefix, $placement_id);
check(empty($prefix_result['matched']) && $prefix_result['content'] === $prefix, 'a longer placement id was treated as this one');
$foreign = $kept . "\n" . '<!-- wp:paragraph -->' . "\n" . '<p><a href="https://shop.example/item?placement=' . $placement_id . '" rel="sponsored nofollow">Vedi&nbsp;il&nbsp;prodotto</a> (link sponsorizzato)</p>' . "\n" . '<!-- /wp:paragraph -->';
$foreign_all = Affilio_Distribution::revert_content($foreign, '');
check(empty($foreign_all['matched']) && $foreign_all['content'] === $foreign, 'a non-go link was removed when no placement id was requested');
$foreign_one = Affilio_Distribution::revert_content($foreign, $placement_id);
check(!empty($foreign_one['matched']) && $foreign_one['content'] === $kept, "wp affilio revert --placement did not remove the class-stripped link:\n" . $foreign_one['content']);
$GLOBALS['post']->post_content = $stripped;
$stripped_cli = (new Affilio_Revert_Command($distribution))->run(array('7'), array('placement' => $placement_id));
check($stripped_cli === 0 && $GLOBALS['post']->post_content === $stripped_kept, 'CLI left the class-stripped placement in the post');

$unpublished_sentence = "Rimosso dal post. L'inserimento non era pubblicato, quindi non c'è niente da segnalare.";
$GLOBALS['options'] = array();
$GLOBALS['scheduled'] = array();
$GLOBALS['post']->post_content = $extra;
$api->removed_mode = 'not_published';
$unpublished = $distribution->perform_revert_action(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check($unpublished['notice'] === $unpublished_sentence, 'admin notice for an unpublished placement was: ' . $unpublished['notice']);
check($GLOBALS['post']->post_content === $kept, '409 not_published left the block in the post');
check(empty($GLOBALS['options']['affilio_removal_queue']), '409 not_published was queued');
check(empty($GLOBALS['scheduled']['affilio_retry_removals']), '409 not_published scheduled a retry');
$GLOBALS['post']->post_content = $extra;
$cli_unpublished_command = new Affilio_Revert_Command($distribution);
$cli_unpublished = $cli_unpublished_command->run(array('7'), array('placement' => $placement_id));
check($cli_unpublished === 0, 'CLI exited ' . $cli_unpublished . ' for 409 not_published');
check(strpos(implode("\n", $cli_unpublished_command->lines), $unpublished_sentence) !== false, 'CLI omitted the unpublished sentence');
check(empty($GLOBALS['options']['affilio_removal_queue']), 'CLI queued 409 not_published');

$admin_403 = 'Errore dall\'API (403), non riprovo.';
$cli_403 = 'API: errore 4xx (403): publisher account is not connected. Non riprovo.';
$GLOBALS['options'] = array();
$GLOBALS['scheduled'] = array();
$GLOBALS['post']->post_content = $extra;
$api->removed_mode = 'forbidden';
$forbidden = $distribution->perform_revert_action(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check(strpos($forbidden['notice'], $admin_403) !== false, 'admin notice for 403 was: ' . $forbidden['notice']);
check(strpos($forbidden['notice'], 'publisher account is not connected') === false, 'admin notice kept the English 403 reason');
check(strpos($forbidden['notice'], '<div') === false, 'admin notice changed the removal markup');
check($GLOBALS['post']->post_content === $kept, '403 left the block in the post');
check(empty($GLOBALS['options']['affilio_removal_queue']), '403 was queued');
check(empty($GLOBALS['scheduled']['affilio_retry_removals']), '403 scheduled a retry');
$GLOBALS['post']->post_content = $extra;
$cli_forbidden_command = new Affilio_Revert_Command($distribution);
$cli_forbidden = $cli_forbidden_command->run(array('7'), array('placement' => $placement_id));
check($cli_forbidden === 1, 'CLI exited ' . $cli_forbidden . ' for 403');
check(strpos(implode("\n", $cli_forbidden_command->lines), $cli_403) !== false, 'CLI omitted the 4xx line');
check(empty($GLOBALS['options']['affilio_removal_queue']), 'CLI queued a 403');

$admin_401 = 'Chiave API non valida: controlla la chiave in Impostazioni › Affilio.';
$cli_401 = 'API: errore 4xx (401): invalid api key. Non riprovo.';
$GLOBALS['options'] = array();
$GLOBALS['scheduled'] = array();
$GLOBALS['post']->post_content = $extra;
$api->removed_mode = 'unauthorized';
$unauthorized = $distribution->perform_revert_action(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check(strpos($unauthorized['notice'], $admin_401) !== false, 'admin notice for 401 was: ' . $unauthorized['notice']);
check(strpos($unauthorized['notice'], 'invalid api key') === false, 'admin notice kept the English 401 reason');
check($GLOBALS['post']->post_content === $kept, '401 left the block in the post');
check(empty($GLOBALS['options']['affilio_removal_queue']), '401 was queued');
check(empty($GLOBALS['scheduled']['affilio_retry_removals']), '401 scheduled a retry');
$GLOBALS['post']->post_content = $extra;
$cli_unauthorized_command = new Affilio_Revert_Command($distribution);
$cli_unauthorized = $cli_unauthorized_command->run(array('7'), array('placement' => $placement_id));
check($cli_unauthorized === 1, 'CLI exited ' . $cli_unauthorized . ' for 401');
check(strpos(implode("\n", $cli_unauthorized_command->lines), $cli_401) !== false, 'CLI omitted the 401 code and reason');
check(empty($GLOBALS['options']['affilio_removal_queue']), 'CLI queued a 401');

$GLOBALS['options'] = array();
$GLOBALS['scheduled'] = array();
$GLOBALS['post']->post_content = $extra;
$api->removed_mode = 'network';
$network = $distribution->perform_revert_action(7, $placement_id, 'nonce-for-affilio_revert_7_' . $placement_id);
check(strpos($network['notice'], 'API: non raggiungibile, riprovo più tardi') !== false, 'network notice was: ' . $network['notice']);
check($GLOBALS['post']->post_content === $kept, 'a network error left the block in the post');
check(isset($GLOBALS['options']['affilio_removal_queue'][$placement_id]), 'a network error was not queued');
check(!empty($GLOBALS['scheduled']['affilio_retry_removals']), 'a network error did not schedule a retry');
$GLOBALS['post']->post_content = $extra;
$cli_network_command = new Affilio_Revert_Command($distribution);
$cli_network = $cli_network_command->run(array('7'), array('placement' => $placement_id));
check($cli_network === 0, 'CLI exited ' . $cli_network . ' for a network error');
check(strpos(implode("\n", $cli_network_command->lines), 'API: non raggiungibile, riprovo più tardi') !== false, 'CLI omitted the retry line for a network error');

$GLOBALS['options']['affilio_removal_queue'] = array($placement_id => array('attempts' => 1, 'next_at' => 0));
$GLOBALS['scheduled'] = array();
$api->removed_mode = 'forbidden';
$distribution->retry_removals();
check(empty($GLOBALS['options']['affilio_removal_queue']), 'cron kept a 4xx removal');
check(empty($GLOBALS['scheduled']['affilio_retry_removals']), 'cron scheduled another try after a 4xx');
$GLOBALS['options']['affilio_removal_queue'] = array($placement_id => array('attempts' => 1, 'next_at' => 0));
$api->removed_mode = 'not_published';
$distribution->retry_removals();
check(empty($GLOBALS['options']['affilio_removal_queue']), 'cron kept a 409 not_published removal');

echo "Placement className insert, legacy revert, confirmation and CLI passed.\n";
