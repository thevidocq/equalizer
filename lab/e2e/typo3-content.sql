-- The lab TYPO3 site: ten pages under the home page and a template that
-- shows the site's name, a menu and the page title.
INSERT INTO pages (pid, tstamp, crdate, sorting, perms_userid, perms_user, perms_group, perms_everybody, doktype, title, slug)
VALUES (1, extract(epoch from now())::int, extract(epoch from now())::int, 256, 1, 31, 27, 1, 1, 'About Acme', '/about-acme'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 512, 1, 31, 27, 1, 1, 'Opening hours', '/opening-hours'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 768, 1, 31, 27, 1, 1, 'Products', '/products'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 1024, 1, 31, 27, 1, 1, 'Services', '/services'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 1280, 1, 31, 27, 1, 1, 'Careers', '/careers'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 1536, 1, 31, 27, 1, 1, 'Press', '/press'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 1792, 1, 31, 27, 1, 1, 'Locations', '/locations'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 2048, 1, 31, 27, 1, 1, 'Partners', '/partners'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 2304, 1, 31, 27, 1, 1, 'Support', '/support'),
       (1, extract(epoch from now())::int, extract(epoch from now())::int, 2560, 1, 31, 27, 1, 1, 'Privacy', '/privacy');
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
