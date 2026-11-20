<?php
// The lab Magento: the shop's name on its home page and a customer test
// account, editor@acme.test, made as the shop itself makes one.
$root = $argv[1];
require $root . '/app/bootstrap.php';
$om = \Magento\Framework\App\Bootstrap::create(BP, $_SERVER)->getObjectManager();
$om->get(\Magento\Framework\App\State::class)->setAreaCode('frontend');
$pages = $om->get(\Magento\Cms\Model\ResourceModel\Page\CollectionFactory::class)->create()
    ->addFieldToFilter('identifier', 'home');
foreach ($pages as $page) {
    $page->setTitle('Acme Store')->setContentHeading('Acme Store')
        ->setContent('<p>Welcome to the Acme Store.</p>')->save();
}
$customers = $om->get(\Magento\Customer\Api\CustomerRepositoryInterface::class);
try {
    $customers->get('editor@acme.test', 1);
} catch (\Magento\Framework\Exception\NoSuchEntityException $e) {
    $c = $om->get(\Magento\Customer\Api\Data\CustomerInterfaceFactory::class)->create()
        ->setEmail('editor@acme.test')->setFirstname('Lab')->setLastname('Editor')->setWebsiteId(1)->setStoreId(1);
    $om->get(\Magento\Customer\Api\AccountManagementInterface::class)->createAccount($c, $argv[2]);
}
