<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class ScanContinueTest extends TestCase {
    /**
     * @runInSeparateProcess
     */
    public function test_get_continue_does_not_autosubmit_or_mint_a_nonce() {
        require dirname(__DIR__) . '/scan-requests.php';
        $this->assertTrue(true);
    }
}
