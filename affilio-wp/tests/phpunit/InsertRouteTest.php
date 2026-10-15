<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class InsertRouteTest extends TestCase {
    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_legacy_insert_route_is_not_registered() {
        $this->boot_plugin_routes();

        $registered = false;
        foreach ($GLOBALS['routes'] as $route) {
            if ($route[0] === 'affilio/v1' && $route[1] === '/insert') {
                $registered = true;
            }
        }

        $this->assertFalse($registered, 'POST /affilio/v1/insert is still registered');
    }

    /**
     * @runInSeparateProcess
     * @preserveGlobalState disabled
     */
    public function test_post_insert_request_returns_404() {
        $this->boot_plugin_routes();

        $this->assertSame(404, $this->dispatch('POST', '/affilio/v1/insert'));
    }

    private function boot_plugin_routes() {
        require dirname(__DIR__) . '/bootstrap.php';
        require dirname(__DIR__, 2) . '/affilio.php';
        affilio_init();
        if (!empty($GLOBALS['actions']['rest_api_init'])) {
            foreach ($GLOBALS['actions']['rest_api_init'] as $callback) {
                call_user_func($callback);
            }
        }
    }

    private function dispatch($method, $route) {
        $namespace = 'affilio/v1';
        $prefix = '/' . $namespace;
        if (strpos($route, $prefix) !== 0) {
            return 404;
        }
        $path = substr($route, strlen($prefix));
        if ($path === '') {
            $path = '/';
        }
        foreach ($GLOBALS['routes'] as $registered) {
            if ($registered[0] !== $namespace || $registered[1] !== $path) {
                continue;
            }
            $spec = $registered[2];
            $methods = isset($spec['methods']) ? $spec['methods'] : '';
            $allowed = is_array($methods) ? $methods : array($methods);
            if (in_array($method, $allowed, true)) {
                return 200;
            }
        }
        return 404;
    }
}
