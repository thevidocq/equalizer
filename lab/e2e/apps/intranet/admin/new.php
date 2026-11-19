<?php
require __DIR__ . '/../lib.php';
need_login();
$error = '';
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    if (!hash_equals(token(), $_POST['token'] ?? '')) {
        $error = '<p class="error">Expired form, try again.</p>';
    } else {
        $file = null;
        if (!empty($_FILES['file']['name']) && $_FILES['file']['error'] === UPLOAD_ERR_OK) {
            $file = preg_replace('/[^A-Za-z0-9._-]/', '_', basename($_FILES['file']['name']));
            move_uploaded_file($_FILES['file']['tmp_name'], __DIR__ . '/../uploads/' . $file);
        }
        db()->prepare('insert into notices (title, body, author, file) values (?, ?, ?, ?)')
            ->execute([in($_POST['title']), in($_POST['body']), $_SESSION['user'], $file]);
        header('Location: /notice.php?id=' . db()->lastInsertId());
        exit;
    }
}
page('New notice', $error . '<form method="post" action="/admin/new.php" enctype="multipart/form-data">'
    . '<input type="hidden" name="token" value="' . token() . '">'
    . '<p>Title <input name="title" id="title" size="60"></p><p><textarea name="body" id="body" rows="6" cols="60"></textarea></p>'
    . '<p>Attachment <input type="file" name="file" id="file"></p><p><input type="submit" id="save" value="Publish"></p></form>');
