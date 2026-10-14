<?php

if (!function_exists('add_filter')) {
    function add_filter($name, $callback, $priority = 10, $accepted_args = 1) {
        if (!isset($GLOBALS['filters']) || !is_array($GLOBALS['filters'])) {
            $GLOBALS['filters'] = array();
        }
        if (!isset($GLOBALS['affilio_filter_hooks']) || !is_array($GLOBALS['affilio_filter_hooks'])) {
            $GLOBALS['affilio_filter_hooks'] = array();
        }
        $GLOBALS['filters'][$name][] = array(
            'callback' => $callback,
            'priority' => $priority,
            'accepted_args' => $accepted_args,
        );
        $GLOBALS['affilio_filter_hooks'][] = array('op' => 'add', 'name' => $name, 'priority' => $priority);
    }
}

if (!function_exists('remove_filter')) {
    function remove_filter($name, $callback, $priority = 10) {
        if (!isset($GLOBALS['affilio_filter_hooks']) || !is_array($GLOBALS['affilio_filter_hooks'])) {
            $GLOBALS['affilio_filter_hooks'] = array();
        }
        $GLOBALS['affilio_filter_hooks'][] = array('op' => 'remove', 'name' => $name, 'priority' => $priority);
        if (!isset($GLOBALS['filters'][$name]) || !is_array($GLOBALS['filters'][$name])) {
            return false;
        }
        $kept = array();
        $removed = false;
        foreach ($GLOBALS['filters'][$name] as $existing) {
            $existing_callback = $existing;
            $existing_priority = 10;
            if (is_array($existing) && array_key_exists('callback', $existing)) {
                $existing_callback = $existing['callback'];
                $existing_priority = $existing['priority'];
            }
            if (!$removed && $existing_callback === $callback && $existing_priority === $priority) {
                $removed = true;
                continue;
            }
            $kept[] = $existing;
        }
        $GLOBALS['filters'][$name] = $kept;
        return $removed;
    }
}

if (!function_exists('affilio_test_run_filters')) {
    function affilio_test_run_filters($name, $passed) {
        if (!isset($GLOBALS['filters'][$name]) || !is_array($GLOBALS['filters'][$name])) {
            return $passed;
        }
        $entries = $GLOBALS['filters'][$name];
        usort($entries, function ($a, $b) {
            $pa = is_array($a) && isset($a['priority']) ? $a['priority'] : 10;
            $pb = is_array($b) && isset($b['priority']) ? $b['priority'] : 10;
            if ($pa === $pb) {
                return 0;
            }
            return ($pa < $pb) ? -1 : 1;
        });
        foreach ($entries as $entry) {
            $callback = is_array($entry) && array_key_exists('callback', $entry) ? $entry['callback'] : $entry;
            $passed[0] = call_user_func_array($callback, $passed);
        }
        return $passed;
    }
}

if (isset($_SERVER['SCRIPT_FILENAME']) && realpath($_SERVER['SCRIPT_FILENAME']) === realpath(__FILE__)) {
    echo "WordPress filter stubs live in the test bootstrap.\n";
}
