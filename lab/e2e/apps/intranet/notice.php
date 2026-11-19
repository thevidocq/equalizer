<?php
require __DIR__ . '/lib.php';
$st = db()->prepare('select * from notices where id = ?');
$st->execute([(int)($_GET['id'] ?? 0)]);
$n = $st->fetch();
if (!$n) { http_response_code(404); page('Not found', '<p>No such notice.</p>'); exit; }
$file = $n['file'] ? '<p>Attachment: <a href="/uploads/' . rawurlencode($n['file']) . '">' . h($n['file']) . '</a></p>' : '';
page($n['title'], '<p>' . nl2br(h($n['body'])) . '</p><p><i>by ' . h($n['author']) . '</i></p>' . $file);
