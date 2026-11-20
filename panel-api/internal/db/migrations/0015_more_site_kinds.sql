-- Site kinds follow the agent's web application profiles
-- (agent/equalizer_agent/webapps/, docs/compatibility.md).
ALTER TABLE apps DROP CONSTRAINT IF EXISTS apps_kind_check;
ALTER TABLE apps ADD CONSTRAINT apps_kind_check CHECK (kind IN ('wordpress', 'joomla', 'drupal', 'magento', 'prestashop',
    'opencart', 'typo3', 'moodle', 'mediawiki', 'phpbb', 'generic'));
