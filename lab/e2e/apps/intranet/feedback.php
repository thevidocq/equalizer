<?php
require __DIR__ . '/lib.php';
// Anyone may write to the staff office: name, e-mail, topic and message.
$done = '';
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    if (!hash_equals(token(), $_POST['token'] ?? '')) {
        $done = '<p class="error">Expired form, try again.</p>';
    } else {
        db()->prepare('insert into feedback (name, email, topic, message) values (?, ?, ?, ?)')
            ->execute([in($_POST['name'] ?? ''), $_POST['email'] ?? '', $_POST['topic'] ?? '', in($_POST['message'] ?? '')]);
        $done = '<p>Thank you, the staff office will answer.</p>';
    }
}
$topics = '';
foreach (['general' => 'General', 'canteen' => 'Canteen', 'it' => 'IT help', 'building' => 'Building'] as $k => $v) {
    $topics .= '<option value="' . $k . '">' . h($v) . '</option>';
}
page('Feedback', $done . '<form method="post" action="/feedback.php"><input type="hidden" name="token" value="' . token() . '">'
    . '<p>Name <input name="name" id="name" size="40"></p><p>E-mail <input type="email" name="email" id="email" size="40"></p>'
    . '<p>Topic <select name="topic" id="topic">' . $topics . '</select></p>'
    . '<p><textarea name="message" id="message" rows="6" cols="60"></textarea></p><p><input type="submit" id="send" value="Send"></p></form>');
