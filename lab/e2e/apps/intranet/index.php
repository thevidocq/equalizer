<?php
require __DIR__ . '/lib.php';
// The first notice stays pinned; the others ten a page, newest first or by title.
$page = max(1, (int)($_GET['page'] ?? 1));
$sort = ($_GET['sort'] ?? 'date') === 'title' ? 'title' : 'id desc';
$total = (int)db()->query('select count(*) from notices where id > 1')->fetchColumn();
$pin = db()->query('select id, title, author from notices where id = 1')->fetch();
$st = db()->prepare("select id, title, author from notices where id > 1 order by $sort limit 10 offset ?");
$st->execute([($page - 1) * 10]);
$list = $pin ? '<p>Pinned: <a href="/notice.php?id=1">' . h($pin['title']) . '</a> by ' . h($pin['author']) . '</p>' : '';
$list .= '<p>Sort: <a href="/?sort=date">newest</a> | <a href="/?sort=title">title</a></p><ul>';
foreach ($st->fetchAll() as $r) { $list .= '<li><a href="/notice.php?id=' . $r['id'] . '">' . h($r['title']) . '</a> by ' . h($r['author']) . '</li>'; }
$list .= '</ul><p>';
for ($i = 1; $i <= (int)ceil($total / 10); $i++) {
    $list .= $i === $page ? "<b>$i</b> " : '<a href="/?page=' . $i . ($sort === 'title' ? '&amp;sort=title' : '') . '">' . $i . '</a> ';
}
page('Notices', $list . '</p>');
