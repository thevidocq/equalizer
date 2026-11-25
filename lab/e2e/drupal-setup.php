<?php
// Prepares the lab Drupal after its installation: the lab user, a page,
// an article and the search index. Usage: php drupal-setup.php <drupal root>
use Drupal\Core\DrupalKernel;
use Drupal\node\Entity\Node;
use Drupal\user\Entity\User;
use Symfony\Component\HttpFoundation\Request;

$root = $argv[1];
chdir($root);
$autoloader = require $root . '/autoload.php';
$request = Request::create('/', 'GET', [], [], [], [
    'HTTP_HOST' => 'docs.acme.test', 'SCRIPT_NAME' => '/index.php', 'SCRIPT_FILENAME' => $root . '/index.php',
]);
$kernel = DrupalKernel::createFromRequest($request, $autoloader, 'prod');
$kernel->boot();
$kernel->preHandle($request);

$user = User::load(1);
$user->setUsername('editor');
$user->setPassword('Lab-editor-pass-1');
$user->setEmail('editor@acme.test');
$user->save();

$nodes = \Drupal::entityTypeManager()->getStorage('node');
if (!$nodes->loadByProperties(['title' => 'About Acme Docs'])) {
    Node::create(['type' => 'page', 'title' => 'About Acme Docs', 'uid' => 1, 'status' => 1,
        'path' => ['alias' => '/about'],
        'body' => ['value' => '<p>Documentation for the lab, with accents: città, perché, Größe.</p>', 'format' => 'basic_html'],
    ])->save();
    Node::create(['type' => 'article', 'title' => 'Release notes from the lab', 'uid' => 1, 'status' => 1,
        'promote' => 1, 'path' => ['alias' => '/release-notes'],
        'body' => ['value' => '<p>The lab release notes, searchable by the word quokka.</p>', 'format' => 'basic_html'],
    ])->save();
}
// A documentation site as they grow: articles with tags, which visitors
// read page by page, comment on, and a contact form.
\Drupal::service('module_installer')->install(['comment', 'contact', 'taxonomy']);
if (!$nodes->loadByProperties(['title' => 'Installing the lab'])) {
    $terms = \Drupal::entityTypeManager()->getStorage('taxonomy_term');
    $tags = [];
    foreach (['install', 'security', 'release', 'howto'] as $t) {
        $term = $terms->create(['vid' => 'tags', 'name' => $t]);
        $term->save();
        $tags[] = $term->id();
    }
    $titles = ['Installing the lab', 'Updating the lab', 'Backups', 'Users and roles', 'Search tips', 'Writing articles',
        'Images and files', 'Translations', 'Caching', 'Cron jobs', 'Mail settings', 'Themes', 'Blocks and regions',
        'Menus', 'Taxonomy', 'Comments', 'Contact forms', 'Performance', 'Security advisories', 'Logs and reports'];
    foreach ($titles as $i => $t) {
        $n = Node::create(['type' => 'article', 'title' => $t, 'uid' => 1, 'status' => 1, 'promote' => 1,
            'body' => ['value' => "<p>$t: notes for the lab, number $i.</p>", 'format' => 'basic_html'],
        ]);
        if ($n->hasField('field_tags')) { // a lab installed before the tags recipe has none
            $n->set('field_tags', [['target_id' => $tags[$i % 4]]]);
        }
        $n->save();
    }
}
// The home page lists every article, its tags and the contact form, as a
// site's menus would (the minimal profile has no front-page view).
$home = current($nodes->loadByProperties(['title' => 'Acme Docs'])) ?: null;
if (!$home || $home->get('body')->format !== 'basic_html') {
    $links = '';
    foreach ($nodes->loadByProperties(['type' => 'article']) as $n) {
        $links .= '<li><a href="/node/' . $n->id() . '">' . htmlspecialchars($n->getTitle()) . '</a></li>';
    }
    foreach (\Drupal::entityTypeManager()->getStorage('taxonomy_term')->loadByProperties(['vid' => 'tags']) as $term) {
        $links .= '<li><a href="/taxonomy/term/' . $term->id() . '">Tag: ' . htmlspecialchars($term->getName()) . '</a></li>';
    }
    $home ??= Node::create(['type' => 'page', 'title' => 'Acme Docs', 'uid' => 1, 'status' => 1]);
    $home->set('body', ['value' => '<ul>' . $links . '<li><a href="/about">About</a></li><li><a href="/contact">Contact us</a></li></ul>',
        'format' => 'basic_html']);
    $home->save();
    \Drupal::configFactory()->getEditable('system.site')->set('page.front', '/node/' . $home->id())->save();
}
// The site-wide contact form, for anyone.
$forms = \Drupal::entityTypeManager()->getStorage('contact_form');
if (!$forms->load('feedback')) {
    $forms->create(['id' => 'feedback', 'label' => 'Website feedback', 'recipients' => ['editor@acme.test'],
        'reply' => '', 'weight' => 0])->save();
    \Drupal::configFactory()->getEditable('contact.settings')->set('default_form', 'feedback')->save();
}
// Visitors may search, as on most sites; the index is filled now rather
// than by cron.
\Drupal::service('module_installer')->install(['search', 'search_node']); // search_node: content search since Drupal 11.4
$pages = \Drupal::entityTypeManager()->getStorage('search_page');
if (!$pages->load('node_search')) {
    $pages->create(['id' => 'node_search', 'plugin' => 'node_search', 'label' => 'Content', 'path' => 'node', 'weight' => -10])->save();
    \Drupal::configFactory()->getEditable('search.settings')->set('default_page', 'node_search')->save();
}
foreach (['anonymous', 'authenticated'] as $rid) {
    user_role_grant_permissions($rid, ['access content', 'search content', 'access comments', 'post comments',
        'access site-wide contact form']);
}
$plugin = $pages->load('node_search')->getPlugin();
$plugin->markForReindex();
$plugin->updateIndex();
fprintf(STDERR, "%s: %s\n", get_class($plugin), json_encode($plugin->indexStatus()));
drupal_flush_all_caches();
echo "ok\n";
