<?php
// Prepares the lab Joomla after its installation: a featured article on the
// home page and the search index. Usage: php joomla-setup.php <joomla root>
$root = $argv[1];
define('_JEXEC', 1);
require $root . '/configuration.php';
$c = new JConfig();
[$host, $port] = explode(':', $c->host) + [1 => '5432'];
$db = new PDO("pgsql:host=$host;port=$port;dbname={$c->db}", $c->user, $c->password,
    [PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION]);
$p = $c->dbprefix;

if (!$db->query("select count(*) from {$p}content")->fetchColumn()) {
    $cat = $db->query("select id from {$p}categories where extension = 'com_content' and alias = 'uncategorised'")->fetchColumn();
    $user = $db->query("select id from {$p}users order by id limit 1")->fetchColumn();
    $now = gmdate('Y-m-d H:i:s', time() - 3600); // search counts whole minutes: published before now
    $st = $db->prepare("insert into {$p}content (title, alias, introtext, \"fulltext\", state, catid, created, created_by,
        modified, modified_by, publish_up, images, urls, attribs, metadesc, metadata, access, featured, language)
        values (?, ?, ?, '', 1, ?, ?, ?, ?, ?, ?, '{}', '{}', '{}', '', '{}', 1, 1, '*') returning id");
    $st->execute(['Welcome to Acme News', 'welcome-to-acme-news',
        '<p>News from the lab, with accents: città, perché, Größe. Search for the word wombat.</p>',
        $cat, $now, $user, $now, $user, $now]);
    $id = $st->fetchColumn();
    $db->prepare("insert into {$p}content_frontpage (content_id, ordering) values (?, 1)")->execute([$id]);
}
// The lab has no Internet: no update or statistics checks, which would
// make admin pages wait for joomla.org; no guided tour, which starts by
// itself on a new site and leaves the page the tests open.
$db->exec("update {$p}extensions set enabled = 0 where type = 'plugin' and (
    (folder = 'quickicon' and element in ('autoupdate', 'joomlaupdate', 'extensionupdate', 'eos'))
    or (folder = 'system' and element in ('stats', 'schedulerunner', 'guidedtours'))
    or (folder = 'task' and element = 'updatenotification'))");
// Smart Search works on an index; the admin keeps it up to date on save.
passthru('php ' . escapeshellarg($root . '/cli/joomla.php') . ' finder:index -q');
