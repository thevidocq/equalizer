-- The lab TYPO3 site: two pages under the home page and a template that
-- shows the site's name, a menu and the page title.
INSERT INTO pages (pid, tstamp, crdate, sorting, perms_userid, perms_user, perms_group, perms_everybody, doktype, title, slug)
VALUES (1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), 256, 1, 31, 27, 1, 1, 'About Acme', '/about-acme'),
       (1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), 512, 1, 31, 27, 1, 1, 'Opening hours', '/opening-hours');
UPDATE sys_template SET config = 'page = PAGE
page.10 = TEXT
page.10.value = Acme Site
page.10.wrap = <h1>|</h1>
page.20 = HMENU
page.20.special = directory
page.20.special.value = 1
page.20.wrap = <ul>|</ul>
page.20.1 = TMENU
page.20.1.NO.wrapItemAndSub = <li>|</li>
page.30 = TEXT
page.30.field = title
page.30.wrap = <h2>|</h2>
' WHERE pid = 1;
