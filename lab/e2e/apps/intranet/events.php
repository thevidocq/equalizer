<?php
require __DIR__ . '/lib.php';
// The calendar: the events from a day on (today by default).
$from = $_GET['from'] ?? '2026-12-01';
if (!preg_match('/^\d{4}-\d{2}-\d{2}$/', $from)) { $from = '2026-12-01'; }
$st = db()->prepare('select day, title from events where day >= ? order by day');
$st->execute([$from]);
$list = '<p>From <a href="/events.php?from=2026-12-01">December</a> | <a href="/events.php?from=2027-01-01">January</a></p><ul>';
foreach ($st->fetchAll() as $r) { $list .= '<li>' . h($r['day']) . ': ' . h($r['title']) . '</li>'; }
page('Calendar', $list . '</ul>');
