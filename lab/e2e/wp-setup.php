<?php
// Lab WordPress settings after the install: pretty permalinks (as most
// real sites), a page, comments without moderation. php wp-setup.php <wp dir>
$_SERVER['HTTP_HOST'] = 'www.acme.test';
require $argv[1] . '/wp-load.php';
update_option('permalink_structure', '/%postname%/');
update_option('comment_moderation', 0);
update_option('comment_previously_approved', 0);
update_option('comment_registration', 0);
update_option('require_name_email', 1);
if (!get_page_by_path('about')) {
    wp_insert_post(['post_type' => 'page', 'post_status' => 'publish', 'post_title' => 'About us', 'post_name' => 'about',
        'post_content' => '<p>Acme makes things. Contact: info@acme.test</p>']);
}
if (!get_page_by_path('news-from-the-lab', OBJECT, 'post')) {
    wp_insert_post(['post_type' => 'post', 'post_status' => 'publish', 'post_title' => 'News from the lab', 'post_name' => 'news-from-the-lab',
        'post_content' => '<p>Città, perché, über, ñandú: accents and symbols survive the gateway.</p>']);
}
flush_rewrite_rules();
echo "wordpress ready\n";
