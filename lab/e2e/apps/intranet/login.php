<?php
require __DIR__ . '/lib.php';
$error = '';
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $st = db()->prepare('select password from users where name = ?');
    $st->execute([$_POST['user'] ?? '']);
    $hash = $st->fetchColumn();
    if ($hash && password_verify($_POST['password'] ?? '', $hash)) {
        session_regenerate_id(true);
        $_SESSION['user'] = $_POST['user'];
        $back = $_POST['back'] ?? '/';
        header('Location: ' . (str_starts_with($back, '/') && !str_starts_with($back, '//') ? $back : '/'));
        exit;
    }
    $error = '<p class="error">Wrong user name or password.</p>';
}
page('Log in', $error . '<form method="post" action="/login.php"><input type="hidden" name="back" value="' . h($_GET['back'] ?? '/') . '">'
    . '<p>User <input name="user" id="user"></p><p>Password <input type="password" name="password" id="password"></p>'
    . '<p><input type="submit" id="login" value="Log in"></p></form>');
