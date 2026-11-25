<?php
require __DIR__ . '/lib.php';
$st = db()->prepare('select * from people where id = ?');
$st->execute([(int)($_GET['id'] ?? 0)]);
$p = $st->fetch();
if (!$p) { http_response_code(404); page('Not found', '<p>No such person.</p>'); exit; }
page($p['name'], '<p>' . h(DEPTS[$p['dept']] ?? $p['dept']) . '</p><p>Phone: ' . h($p['phone']) . '</p>'
    . '<p><a href="/api.php?action=person&amp;id=' . $p['id'] . '">As JSON</a></p>');
