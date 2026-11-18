-- What a site is built with: selects the template offered for its first map.
ALTER TABLE apps ADD COLUMN kind text CHECK (kind IN ('wordpress', 'joomla', 'drupal', 'moodle', 'generic'));
