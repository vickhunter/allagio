<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class N2ErrorsTest extends TestCase {
    /**
     * @runInSeparateProcess
     */
    public function test_n2_receipt_error_keeps_server_status_and_reason() {
        require_once dirname(__DIR__) . '/d033-n2-errors.php';
        affilio_n2_receipt_error_keeps_server_status_and_reason();
        $this->assertTrue(true);
    }

    /**
     * @runInSeparateProcess
     */
    public function test_register_response_stores_plugin_key() {
        require_once dirname(__DIR__) . '/d033-n2-errors.php';
        affilio_register_response_stores_plugin_key();
        $this->assertTrue(true);
    }
}
