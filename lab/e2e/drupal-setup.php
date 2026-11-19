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
// Visitors may search, as on most sites; the index is filled now rather
// than by cron.
\Drupal::service('module_installer')->install(['search', 'search_node']); // search_node: content search since Drupal 11.4
$pages = \Drupal::entityTypeManager()->getStorage('search_page');
if (!$pages->load('node_search')) {
    $pages->create(['id' => 'node_search', 'plugin' => 'node_search', 'label' => 'Content', 'path' => 'node', 'weight' => -10])->save();
    \Drupal::configFactory()->getEditable('search.settings')->set('default_page', 'node_search')->save();
}
foreach (['anonymous', 'authenticated'] as $rid) {
    user_role_grant_permissions($rid, ['access content', 'search content']);
}
$plugin = $pages->load('node_search')->getPlugin();
$plugin->markForReindex();
$plugin->updateIndex();
fprintf(STDERR, "%s: %s\n", get_class($plugin), json_encode($plugin->indexStatus()));
drupal_flush_all_caches();
echo "ok\n";
