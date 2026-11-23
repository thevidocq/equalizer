<?php
// Efficacy study: records every request that reaches the legacy site (what
// Equalizer let through), then hands it to the site's own router
// (STUDY_ROUTER) or to PHP's server.
$body = file_get_contents('php://input');
$h = [];
foreach ($_SERVER as $k => $v) {
    if (strpos($k, 'HTTP_') === 0 || $k === 'CONTENT_TYPE') {
        $h[$k] = $v;
    }
}
file_put_contents(getenv('STUDY_LOG'), json_encode(['m' => $_SERVER['REQUEST_METHOD'], 'u' => $_SERVER['REQUEST_URI'],
    'b' => base64_encode(substr($body, 0, 65536)), 'h' => $h]) . "\n", FILE_APPEND | LOCK_EX);
$router = getenv('STUDY_ROUTER');
if ($router) {
    return require $router;
}
return false;
