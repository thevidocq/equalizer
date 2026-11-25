<?php
require __DIR__ . '/lib.php';
// The staff directory, by department.
$dept = $_GET['dept'] ?? '';
$menu = '<p>';
foreach (DEPTS as $k => $v) { $menu .= '<a href="/people.php?dept=' . $k . '">' . h($v) . '</a> | '; }
$menu .= '<a href="/people.php">Everyone</a></p>';
if ($dept !== '') {
    $st = db()->prepare('select id, name, dept from people where dept = ? order by name');
    $st->execute([$dept]);
} else {
    $st = db()->query('select id, name, dept from people order by name');
}
$list = '<ul>';
foreach ($st->fetchAll() as $r) { $list .= '<li><a href="/person.php?id=' . $r['id'] . '">' . h($r['name']) . '</a>, ' . h(DEPTS[$r['dept']] ?? $r['dept']) . '</li>'; }
page('People', $menu . $list . '</ul>');
