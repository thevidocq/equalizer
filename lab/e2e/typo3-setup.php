<?php
// The lab TYPO3 answers for its own name only, as a site set up behind a
// known host does (TYPO3 refuses any other Host header).
$file = $argv[1] . '/typo3conf/system/settings.php';
$s = file_get_contents($file);
if (strpos($s, 'trustedHostsPattern') === false) {
    $pattern = "        'trustedHostsPattern' => 'site\\\\.acme\\\\.test',\n";
    $at = strpos($s, "'SYS' => [") + strlen("'SYS' => [\n");
    file_put_contents($file, substr($s, 0, $at) . $pattern . substr($s, $at));
}
