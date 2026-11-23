package assistant

import (
	"context"
	"encoding/json"
	"fmt"
)

// The tools about the site as a whole: its status, its versions, a version's
// check, what changed between two versions, and the guide.

// SiteSource is what those tools read, for one site.
type SiteSource interface {
	Status(ctx context.Context) (any, error)   // as the Status page shows it
	Versions(ctx context.Context) (any, error) // every version, newest first, without documents
	Verification(ctx context.Context, version int) (any, error)
	Compare(ctx context.Context, from, to int) (any, error) // changes and findings, as the review aid
}

func siteTools() []Tool {
	num := func(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }
	return []Tool{
		{Name: "get_site", Description: "The site's status as the Status page shows it: connector online or not (and its release), the legacy application answering or failing, the published map version, the certificate and its expiry, the isolation check, the last 7 days.",
			Schema: obj(map[string]any{})},
		{Name: "list_versions", Description: "Every version of the site's map, newest first: status (draft, in review, approved, published, retired, rejected), author, approver, dates and note.",
			Schema: obj(map[string]any{})},
		{Name: "get_check", Description: "The check of a map version on the site (verification): pages identical through Equalizer, inputs that refuse every attack, pages broken, what is still open and why, corrections made by the agent.",
			Schema: obj(map[string]any{"version": num("a map version; omit for the published one")})},
		{Name: "compare_versions", Description: "What changed between two map versions and what it risks (the review aid's comparison): entries added, removed or widened, findings by severity.",
			Schema: obj(map[string]any{"from": num("the older version"), "to": num("the newer version")}, "from", "to")},
		{Name: "read_guide", Description: "The Equalizer guide for tenants: how to do something in the panel, what a page or a setting means. Query with a few English words (e.g. 'publish draft', 'test accounts', 'certificate upload'); empty for the list of sections.",
			Schema: obj(map[string]any{"query": map[string]any{"type": "string"}})},
	}
}

// execSite runs one of the site tools; ok is false for another tool.
func execSite(ctx context.Context, src Source, m *Masker, c Call, in map[string]any) (string, bool) {
	ss, _ := src.(SiteSource)
	show := func(v any, err error) string {
		if err != nil {
			return "error: " + err.Error()
		}
		b, _ := json.Marshal(v)
		return fence(clip(m.Text(string(b)), 16000)) // addresses and emails in errors, paths, labels
	}
	switch c.Name {
	case "read_guide":
		q, _ := in["query"].(string)
		return Guide(q), true
	case "get_site", "list_versions", "get_check", "compare_versions":
		if ss == nil {
			return "error: not available here", true
		}
	default:
		return "", false
	}
	switch c.Name {
	case "get_site":
		return show(ss.Status(ctx)), true
	case "list_versions":
		return show(ss.Versions(ctx)), true
	case "get_check":
		return show(ss.Verification(ctx, intOf(in["version"], 0))), true
	default:
		from, to := intOf(in["from"], 0), intOf(in["to"], 0)
		if from <= 0 || to <= 0 || from == to {
			return "error: give two different versions, from and to", true
		}
		v, err := ss.Compare(ctx, from, to)
		if err != nil {
			return "error: " + err.Error(), true
		}
		b, _ := json.Marshal(v)
		return fmt.Sprintf("from version %d to version %d\n", from, to) + fence(clip(m.Text(string(b)), 16000)), true
	}
}
