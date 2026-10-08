<?php

if (class_exists('Affilio_Tracker')) {
    return;
}

class Affilio_Tracker {
    private $api;

    public function __construct($api) {
        $this->api = $api;
    }
}
