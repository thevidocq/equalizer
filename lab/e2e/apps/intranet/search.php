<?php
require __DIR__ . '/lib.php';
$q = in($_GET['q'] ?? '');
$st = db()->prepare("select id, title from notices where title like ? or body like ?");
$st->execute(["%$q%", "%$q%"]);
$res = '';
foreach ($st->fetchAll() as $r) { $res .= '<li><a href="/notice.php?id=' . $r['id'] . '">' . h($r['title']) . '</a></li>'; }
page('Search', '<p>Results for <b>' . h($q) . '</b>:</p><ul>' . ($res ?: '<li>Nothing found.</li>') . '</ul>');
