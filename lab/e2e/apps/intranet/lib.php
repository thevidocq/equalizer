<?php
// Acme Intranet: a deliberately old-style PHP application (one file per
// page, PHP sessions, ISO-8859-1 pages, SQLite) used as the lab's custom
// legacy site. Not a model to follow: a sample of what Equalizer protects.
session_start();
header('Content-Type: text/html; charset=iso-8859-1');

function db() {
    static $pdo;
    if (!$pdo) {
        $pdo = new PDO('sqlite:' . getenv('INTRANET_DB'));
        $pdo->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
        $pdo->exec('create table if not exists notices (id integer primary key, title text, body text, author text, file text)');
        $pdo->exec('create table if not exists users (name text primary key, password text)');
        $pdo->exec('create table if not exists people (id integer primary key, name text, dept text, phone text)');
        $pdo->exec('create table if not exists events (id integer primary key, day text, title text)');
        $pdo->exec('create table if not exists feedback (id integer primary key, name text, email text, topic text, message text)');
        if (!$pdo->query('select count(*) from users')->fetchColumn()) {
            $pdo->prepare('insert into users values (?, ?)')->execute(['editor', password_hash('Lab-editor-pass-1', PASSWORD_DEFAULT)]);
            $pdo->prepare('insert into notices (title, body, author) values (?, ?, ?)')
                ->execute(['Canteen opening hours', 'The canteen opens at 12:00. Specialità del giorno: lasagne, caffè.', 'editor']);
        }
        if (!$pdo->query('select count(*) from people')->fetchColumn()) {
            seed($pdo); // after the first notice, which stays pinned
        }
    }
    return $pdo;
}

// The rest of the intranet: more notices, a staff directory and a calendar,
// as an old company site grows.
function seed($pdo) {
    $n = $pdo->prepare('insert into notices (title, body, author) values (?, ?, ?)');
    foreach (['Parking rules', 'New coffee machine', 'Fire drill on Friday', 'Holiday calendar', 'IT maintenance window',
        'Welcome to the new colleagues', 'Printer on floor 2', 'Summer hours', 'Expense reports', 'Badge renewal',
        'Library opening', 'Bike racks', 'Training week', 'Office move', 'Quarterly meeting'] as $i => $t) {
        $n->execute([$t, "Notice number $i about $t. Più informazioni all'ufficio del personale.", $i % 3 ? 'editor' : 'hr']);
    }
    $p = $pdo->prepare('insert into people (name, dept, phone) values (?, ?, ?)');
    foreach ([['Anna Rossi', 'sales'], ['Luca Bianchi', 'it'], ['Marta Verdi', 'hr'], ['Paolo Neri', 'it'],
        ['Giulia Conti', 'sales'], ['Sara Gallo', 'finance'], ['Marco Fabbri', 'finance'], ['Elena Russo', 'hr']] as $i => $r) {
        $p->execute([$r[0], $r[1], sprintf('+39 02 5550 %04d', 100 + $i)]);
    }
    $e = $pdo->prepare('insert into events (day, title) values (?, ?)');
    foreach ([['2026-12-01', 'Quarterly meeting'], ['2026-12-04', 'Fire drill'], ['2026-12-18', 'Christmas lunch'],
        ['2027-01-12', 'Training week starts']] as $r) {
        $e->execute($r);
    }
}

const DEPTS = ['sales' => 'Sales', 'it' => 'IT', 'hr' => 'Human resources', 'finance' => 'Finance'];

// Pages are written in UTF-8 here and sent as ISO-8859-1, as old sites do.
function h($s) { return htmlspecialchars($s, ENT_QUOTES, 'UTF-8'); }
function out($html) { echo mb_convert_encoding($html, 'ISO-8859-1', 'UTF-8'); }
function in($s) { return mb_convert_encoding($s, 'UTF-8', 'ISO-8859-1'); }

function token() {
    if (empty($_SESSION['token'])) { $_SESSION['token'] = bin2hex(random_bytes(16)); }
    return $_SESSION['token'];
}

function page($title, $body) {
    $user = $_SESSION['user'] ?? null;
    $nav = $user ? '<a href="/admin/new.php">New notice</a> | <a href="/logout.php">Log out ' . h($user) . '</a>'
                 : '<a href="/login.php">Log in</a>';
    out('<!DOCTYPE html><html><head><meta charset="iso-8859-1"><title>' . h($title) . ' - Acme Intranet</title>'
        . '<link rel="stylesheet" href="/style.css"></head><body><div id="top"><b>Acme Intranet</b> | <a href="/">Notices</a> | <a href="/people.php">People</a> | <a href="/events.php">Calendar</a> | '
        . '<a href="/feedback.php">Feedback</a> | <a href="/api.php?action=notices&amp;limit=5">Latest (JSON)</a> | '
        . '<form action="/search.php" method="get" style="display:inline"><input name="q" placeholder="Search"> <input type="submit" value="Go"></form> | '
        . $nav . '</div><div id="main"><h1>' . h($title) . '</h1>' . $body . '</div></body></html>');
}

function need_login() {
    if (empty($_SESSION['user'])) { header('Location: /login.php?back=' . urlencode($_SERVER['REQUEST_URI'])); exit; }
}
