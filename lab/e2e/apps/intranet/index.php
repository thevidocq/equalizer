<?php
require __DIR__ . '/lib.php';
$rows = db()->query('select id, title, author from notices order by id desc')->fetchAll();
$list = '<ul>';
foreach ($rows as $r) { $list .= '<li><a href="/notice.php?id=' . $r['id'] . '">' . h($r['title']) . '</a> by ' . h($r['author']) . '</li>'; }
page('Notices', $list . '</ul>');
