<?php
// php -S router for the lab WordPress: real files as they are, the rest to
// index.php (pretty permalinks, as with Apache's rewrite rules).
$path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
if ($path !== '/' && file_exists($_SERVER['DOCUMENT_ROOT'] . $path) && !is_dir($_SERVER['DOCUMENT_ROOT'] . $path)) {
    return false;
}
if (is_dir($_SERVER['DOCUMENT_ROOT'] . $path) && file_exists($_SERVER['DOCUMENT_ROOT'] . rtrim($path, '/') . '/index.php')) {
    $_SERVER['SCRIPT_NAME'] = rtrim($path, '/') . '/index.php';
    require $_SERVER['DOCUMENT_ROOT'] . rtrim($path, '/') . '/index.php';
    return true;
}
$_SERVER['SCRIPT_NAME'] = '/index.php';
require $_SERVER['DOCUMENT_ROOT'] . '/index.php';
