package site

import (
	"fmt"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// checkAuthz validates the authorisation rules of an entry: known roles,
// known checks, a target among the entry's declared values.
func checkAuthz(m *mapmodel.Model, rules map[string]mapmodel.AuthzRule, specs map[string]map[string]mapmodel.FieldSpec) error {
	for role, r := range rules {
		if _, ok := m.Roles.List[role]; !ok {
			return fmt.Errorf("authz: role %q is not in roles.list", role)
		}
		switch r.Check {
		case "always", "never":
		case "owned_by_user":
			ns, name, ok := strings.Cut(r.Target, ".")
			if !ok {
				return fmt.Errorf("authz %s: target must be ns.name, e.g. params.id", role)
			}
			if _, declared := specs[ns][name]; !declared {
				return fmt.Errorf("authz %s: target %q is not a declared value of the entry", role, r.Target)
			}
		default:
			return fmt.Errorf("authz %s: unknown check %q", role, r.Check)
		}
	}
	return nil
}

// checkIssues validates the issues of an API response: a declared field,
// towards an entry whose owned_by_user rule names that target.
func checkIssues(owned map[string]map[string]string, resp mapmodel.APIResponse) error {
	for field, to := range resp.Issues {
		if _, ok := resp.Fields[field]; !ok {
			return fmt.Errorf("response.issues: %q is not a field of the response", field)
		}
		entry, target, _ := strings.Cut(to, ".")
		found := false
		for _, t := range owned[entry] {
			found = found || t == target
		}
		if !found {
			return fmt.Errorf("response.issues %s: %q has no owned_by_user rule on %q", field, entry, target)
		}
	}
	return nil
}

// IssueKey names the values of one target of one entry in a session.
func IssueKey(entry, target string) string { return entry + ":" + target }

// Owned returns the owned_by_user targets of an entry, by role.
func (s *Site) Owned(entry string) map[string]string {
	return s.owned[entry]
}

func ownedTargets(m *mapmodel.Model) map[string]map[string]string {
	out := map[string]map[string]string{}
	add := func(id string, rules map[string]mapmodel.AuthzRule) {
		for role, r := range rules {
			if r.Check == "owned_by_user" {
				if out[id] == nil {
					out[id] = map[string]string{}
				}
				out[id][role] = r.Target
			}
		}
	}
	for _, r := range m.Routes {
		add(r.ID, r.Authz)
	}
	for _, a := range m.API {
		add(a.ID, a.Authz)
	}
	return out
}
