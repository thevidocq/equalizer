<?php
// The lab PrestaShop: plain URLs (PHP's own server has no Apache rewrite
// rules) and a customer test account, editor@acme.test.
$_SERVER['HTTP_HOST'] = 'shop.acme.test';
$_SERVER['REQUEST_URI'] = '/';
$_SERVER['REQUEST_METHOD'] = 'GET';
require $argv[1] . '/config/config.inc.php';
Configuration::updateValue('PS_REWRITING_SETTINGS', 0);
if (!Customer::customerExists('editor@acme.test')) {
    $c = new Customer();
    $c->firstname = 'Lab';
    $c->lastname = 'Editor';
    $c->email = 'editor@acme.test';
    $c->passwd = (new PrestaShop\PrestaShop\Core\Crypto\Hashing())->hash($argv[2]);
    $c->id_default_group = (int) Configuration::get('PS_CUSTOMER_GROUP');
    $c->active = 1;
    $c->add();
    $c->updateGroup([$c->id_default_group]);
}
Tools::clearSf2Cache();
