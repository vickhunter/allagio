<?php

use PHPUnit\Framework\TestCase;

if (!class_exists(TestCase::class)) {
    return;
}

class OneBoxTest extends TestCase {
    /**
     * @runInSeparateProcess
     */
    public function test_commit_one_per_post_does_not_insert() {
        require_once dirname(__DIR__) . '/d033-one-box.php';
        affilio_review_commit_one_per_post_does_not_insert();
        $this->assertTrue(true);
    }

    /**
     * @runInSeparateProcess
     */
    public function test_ack_one_per_post_removes_the_new_box() {
        require_once dirname(__DIR__) . '/d033-one-box.php';
        affilio_review_ack_one_per_post_removes_the_new_box();
        $this->assertTrue(true);
    }

    /**
     * @runInSeparateProcess
     */
    public function test_upgrade_one_per_post_keeps_the_existing_box() {
        require_once dirname(__DIR__) . '/d033-one-box.php';
        affilio_review_upgrade_one_per_post_keeps_the_existing_box();
        $this->assertTrue(true);
    }

    /**
     * @runInSeparateProcess
     */
    public function test_go_fallback_is_never_empty() {
        require_once dirname(__DIR__) . '/d033-one-box.php';
        affilio_review_go_fallback_is_never_empty();
        $this->assertTrue(true);
    }
}
