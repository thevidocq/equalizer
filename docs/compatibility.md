# Compatibility

Generated from the agent's web application profiles
(`agent/equalizer_agent/webapps/`) by
`python3 -m equalizer_agent.webapps.matrix`; do not edit by hand.

Every site is protected the same way whatever it is built with: the
gateway rebuilds every request and the whole site stays served by the
map's `/` area. What changes per application is how much the discovery
agent can study and narrow (docs/agent.md), and the map template
offered for the first version.

- **Recognised by**: what tells the agent which application it is.
- **Test account login**: where a test account logs in.
- **Administration**: whether a test administrator's pass studies it.
  It opens only where the lab proved that viewing it changes nothing.
- **Pages never opened**: rules for pages that act when only opened
  (on top of the rules every site shares: action links, one-time
  tokens, sensitive files).
- **Lab**: whether `lab/e2e` runs this application; "not yet" means
  the profile is written from the application's documentation and
  behaviour, not proven.

| Application | Site kind | Recognised by | Test account login | Administration | Pages never opened (rules) | Map template | Lab |
| --- | --- | --- | --- | --- | --- | --- | --- |
| WordPress | `wordpress` | generator tag, paths, extensions | `/wp-login.php` | closed | 4 | yes | yes |
| Joomla | `joomla` | generator tag, paths, extensions | `/index.php?option=com_users&view=login` | read-only (proven in the lab), own login | 2 | yes | yes |
| Drupal | `drupal` | generator tag, paths, extensions | `/user/login` | read-only (proven in the lab) | 5 | yes | yes |
| Magento / Adobe Commerce | `magento` | generator tag, paths, extensions | `/customer/account/login/` | closed | 6 | yes | yes |
| PrestaShop | `prestashop` | generator tag, paths, extensions | `/index.php?controller=authentication` | closed | 4 | yes | yes |
| OpenCart | `opencart` | generator tag, paths, extensions | `/index.php?route=account/login` | closed | 2 | yes | yes |
| TYPO3 | `typo3` | generator tag, paths, extensions | first login form found | closed | 3 | yes | yes |
| Moodle | `moodle` | generator tag, paths, extensions | `/login/index.php` | closed | 5 | yes | not yet |
| MediaWiki | `mediawiki` | generator tag, paths, extensions | `/index.php?title=Special:UserLogin` | closed | 3 | yes | yes |
| phpBB | `phpbb` | generator tag, paths, extensions | `/ucp.php?mode=login` | closed | 5 | yes | yes |
| Custom site | `generic` | not recognised: the fallback | first login form found | closed | 0 | yes | yes |

## Notes

- **WordPress**: Lab: public site, login, comments, search, REST API with nonce, uploads; studied with an administrator account, database unchanged. Administration not studied: WordPress writes to its database when its screens are only viewed.
- **Joomla**: Lab (PostgreSQL): articles, search, login; studied with an administrator account through its own login, database unchanged.
- **Drupal**: Lab: nodes, search, login; studied with an administrator account, database unchanged.
- **Magento / Adobe Commerce**: Lab (MariaDB, no search engine, no products): home, CMS pages, customer login; studied with a customer account, database unchanged. Its precise entries are not reliable yet: the agent's verification sent most of them back to the site area (35 of 39 pages identical after correction; its /static .html templates are refused). Administration closed: its address differs per installation. Cart, wishlist and comparison links are never opened.
- **PrestaShop**: Lab (MariaDB, demo products, plain URLs): catalogue, customer login; studied with a customer account, database unchanged; 220 of 220 pages identical through the draft. Administration closed: its folder is renamed at installation. Cart and order links are never opened.
- **OpenCart**: Lab (MariaDB, demo products): catalogue, search, customer login (in script, the session cookie kept); studied with a customer account, database unchanged; 163 of 165 pages identical through the corrected draft. Administration closed until the lab proves that viewing it changes nothing. Cart and checkout links are never opened.
- **TYPO3**: Lab (MariaDB): pages and menu; no visitors' login out of the box (the report says so), database unchanged; 3 of 3 pages identical through the draft. Administration closed until the lab proves that viewing it changes nothing.
- **Moodle**: Administration closed until the lab proves that viewing it changes nothing.
- **MediaWiki**: Lab (SQLite): pages, special pages, login; studied with the administrator logged in, database unchanged; 38 of 38 pages identical through the draft. Special pages closed until the lab proves that viewing them changes nothing; edit links are never opened.
- **phpBB**: Lab (SQLite): board, topics, login; studied with the administrator logged in on the board, database unchanged; 197 of 197 pages identical through the draft. Moderation and administration closed until the lab proves that viewing them changes nothing; posting and moderation links are never opened.
- **Custom site**: Lab: a custom PHP intranet in ISO-8859-1 (sessions, attachments), served in plain http; studied with a test account, database unchanged. Recognised by nothing in particular: the shared rules apply (no action links, no sensitive files); a usual administration path stays closed.
