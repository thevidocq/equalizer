<?php
require __DIR__ . '/lib.php';
// A small JSON API the intranet's widgets call: notices and people.
header('Content-Type: application/json');
$action = $_GET['action'] ?? '';
if ($action === 'notices') {
    $st = db()->prepare('select id, title from notices order by id desc limit ?');
    $st->execute([min(50, max(1, (int)($_GET['limit'] ?? 5)))]);
    echo json_encode($st->fetchAll(PDO::FETCH_ASSOC));
} elseif ($action === 'person') {
    $st = db()->prepare('select id, name, dept from people where id = ?');
    $st->execute([(int)($_GET['id'] ?? 0)]);
    echo json_encode($st->fetch(PDO::FETCH_ASSOC) ?: null);
} else {
    http_response_code(400);
    echo json_encode(['error' => 'unknown action']);
}
