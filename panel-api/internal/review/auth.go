package review

import "fmt"

// login compares roles.auth: the OIDC login in front of the site.
func (c *cmp) login(b, n obj) {
	ba, na := asObj(b["auth"]), asObj(n["auth"])
	was, is := str(ba["provider"]) == "oidc", str(na["provider"]) == "oidc"
	switch {
	case was && !is:
		c.find("high", "roles.auth", "oidc_off",
			"Login with the identity provider is turned off: visitors reach the site with its own login only.")
	case !was && is:
		c.find("low", "roles.auth", "oidc_on",
			"Login with the identity provider is turned on: register the callback address (Site → Login) before publishing.")
	}
	if !is {
		return
	}
	if was && str(ba["issuer"]) != str(na["issuer"]) {
		c.find("high", "roles.auth.issuer", "oidc_issuer_changed",
			fmt.Sprintf("Another identity provider decides who gets in: %s.", str(na["issuer"])))
	}
	if req, ok := na["required"].(bool); ok && !req {
		if old, ok := ba["required"].(bool); !was || !ok || old {
			c.find("medium", "roles.auth.required", "oidc_optional",
				"Login is no longer required: visitors without it keep the anonymous role.")
		}
	}
	before := map[string]string{}
	for _, r := range asList(ba["roles"]) {
		before[str(asObj(r)["group"])] = str(asObj(r)["role"])
	}
	for _, r := range asList(na["roles"]) {
		g, role := str(asObj(r)["group"]), str(asObj(r)["role"])
		if before[g] == role && was {
			continue
		}
		if g == "*" {
			c.find("medium", "roles.auth.roles", "oidc_everyone",
				fmt.Sprintf("Every user of the identity provider gets the role %s.", role))
		} else if was {
			c.find("low", "roles.auth.roles", "oidc_group",
				fmt.Sprintf("Group %s gets the role %s.", g, role))
		}
	}
}
