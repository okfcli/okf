package validate

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// allRuleIDs is the complete public rule ID surface. Rule IDs are a contract:
// adding a rule extends this list, but renaming or removing one is a breaking
// change and must fail here.
var allRuleIDs = []string{
	"okf/body/empty",
	"okf/computation/duplicate",
	"okf/computation/missing",
	"okf/computation/parameter-incomplete",
	"okf/computation/path-missing",
	"okf/computation/runtime-required",
	"okf/frontmatter/description-recommended",
	"okf/frontmatter/tags-recommended",
	"okf/frontmatter/timestamp-future",
	"okf/frontmatter/title-recommended",
	"okf/frontmatter/type-required",
	"okf/legacy/citations",
	"okf/legacy/timestamp",
	"okf/lifecycle/stale",
	"okf/lifecycle/stale-after-invalid",
	"okf/lifecycle/status-invalid",
	"okf/links/broken",
	"okf/reserved/index-frontmatter",
	"okf/reserved/index-frontmatter-key",
	"okf/reserved/log-date-heading",
	"okf/reserved/okf-version-unknown",
	"okf/sources/footnote-duplicate",
	"okf/sources/footnote-undefined",
	"okf/sources/footnote-unmatched",
	"okf/sources/footnote-unreferenced",
	"okf/sources/id-duplicate",
	"okf/sources/resource-required",
	"okf/sources/usage-window-missing",
	"okf/trust/actor-convention",
	"okf/trust/generated-at-invalid",
	"okf/trust/generated-by-required",
	"okf/trust/verified-at-invalid",
	"okf/trust/verified-incomplete",
}

func TestRuleIDSet(t *testing.T) {
	got := make([]string, 0, len(ruleDescriptions))
	for id, desc := range ruleDescriptions {
		if desc == "" {
			t.Errorf("rule %s has an empty description", id)
		}
		got = append(got, id)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, allRuleIDs) {
		t.Errorf("rule ID set changed:\ngot  %v\nwant %v", got, allRuleIDs)
	}
}

func TestValidate_EveryFindingHasKnownRuleID(t *testing.T) {
	// A bundle that trips checks across families: missing type, empty body,
	// broken link, bad status, unmatched footnote, log heading, index
	// frontmatter, incomplete computation contract.
	b := testBundle(t, map[string]string{
		"a.md":       "---\ntitle: A\n---\n",
		"b.md":       "---\ntype: T\nstatus: retired\nsources:\n  - resource: x\n    id: s1\n---\n\n[gone](/missing.md) and [^orphan]",
		"c.md":       "---\ntype: Attested Computation\ntitle: C\ndescription: d\ntags: [x]\n---\n\nbody",
		"d/index.md": "---\ntype: T\n---\n\nlist",
		"log.md":     "## yesterday\n\nentry",
	})
	r := Validate(b)
	if len(r.Findings) == 0 {
		t.Fatal("expected findings")
	}
	for _, f := range r.Findings {
		if f.RuleID == "" {
			t.Errorf("finding without rule ID: %+v", f)
		} else if _, ok := ruleDescriptions[f.RuleID]; !ok {
			t.Errorf("finding carries unknown rule ID %q: %+v", f.RuleID, f)
		}
	}
}

// sarifOf marshals findings to SARIF and unmarshals for structural asserts.
func sarifOf(t *testing.T, findings []Finding, version string) map[string]any {
	t.Helper()
	out, err := SARIF(findings, version)
	if err != nil {
		t.Fatalf("SARIF: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("SARIF output is not valid JSON: %v", err)
	}
	return doc
}

func TestSARIF_Structure(t *testing.T) {
	findings := []Finding{
		{ConceptID: "tables/users", RuleID: RuleTypeRequired, Severity: SeverityError, Message: "frontmatter: 'type' is required (OKF §4.1)"},
		{ConceptID: "tables/users", RuleID: RuleTitleRecommended, Severity: SeverityWarning, Message: "frontmatter: 'title' is recommended"},
	}
	doc := sarifOf(t, findings, "1.2.3")

	if doc["$schema"] != "https://json.schemastore.org/sarif-2.1.0.json" {
		t.Errorf("$schema = %v", doc["$schema"])
	}
	if doc["version"] != "2.1.0" {
		t.Errorf("version = %v", doc["version"])
	}
	runs := doc["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(runs))
	}
	run := runs[0].(map[string]any)
	driver := run["tool"].(map[string]any)["driver"].(map[string]any)
	if driver["name"] != "okf" || driver["version"] != "1.2.3" {
		t.Errorf("driver = %v", driver)
	}
	if driver["informationUri"] != "https://github.com/okfcli/okf" {
		t.Errorf("informationUri = %v", driver["informationUri"])
	}

	results := run["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	first := results[0].(map[string]any)
	if first["ruleId"] != RuleTypeRequired {
		t.Errorf("results[0].ruleId = %v", first["ruleId"])
	}
	if first["level"] != "error" {
		t.Errorf("results[0].level = %v, want error", first["level"])
	}
	if got := first["message"].(map[string]any)["text"]; got != findings[0].Message {
		t.Errorf("results[0].message.text = %v", got)
	}
	second := results[1].(map[string]any)
	if second["level"] != "warning" {
		t.Errorf("results[1].level = %v, want warning", second["level"])
	}

	locs := first["locations"].([]any)
	if len(locs) != 1 {
		t.Fatalf("got %d locations, want 1", len(locs))
	}
	uri := locs[0].(map[string]any)["physicalLocation"].(map[string]any)["artifactLocation"].(map[string]any)["uri"]
	if uri != "tables/users.md" {
		t.Errorf("artifactLocation.uri = %v, want tables/users.md", uri)
	}
}

func TestSARIF_RulesDeduplicated(t *testing.T) {
	findings := []Finding{
		{ConceptID: "a", RuleID: RuleLinkBroken, Severity: SeverityError, Message: "broken link 1"},
		{ConceptID: "b", RuleID: RuleLinkBroken, Severity: SeverityError, Message: "broken link 2"},
		{ConceptID: "a", RuleID: RuleBodyEmpty, Severity: SeverityWarning, Message: "empty"},
	}
	doc := sarifOf(t, findings, "dev")
	run := doc["runs"].([]any)[0].(map[string]any)
	rules := run["tool"].(map[string]any)["driver"].(map[string]any)["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2 (deduplicated): %v", len(rules), rules)
	}
	// First appearance order.
	if id := rules[0].(map[string]any)["id"]; id != RuleLinkBroken {
		t.Errorf("rules[0].id = %v, want %s", id, RuleLinkBroken)
	}
	if id := rules[1].(map[string]any)["id"]; id != RuleBodyEmpty {
		t.Errorf("rules[1].id = %v, want %s", id, RuleBodyEmpty)
	}
	if desc := rules[0].(map[string]any)["shortDescription"].(map[string]any)["text"]; desc != ruleDescriptions[RuleLinkBroken] {
		t.Errorf("rules[0].shortDescription.text = %v", desc)
	}
	if len(run["results"].([]any)) != 3 {
		t.Errorf("results not preserved: %v", run["results"])
	}
}

func TestSARIF_NoFindings(t *testing.T) {
	doc := sarifOf(t, nil, "dev")
	run := doc["runs"].([]any)[0].(map[string]any)
	// Empty arrays, not null: SARIF consumers require both properties.
	if results, ok := run["results"].([]any); !ok || len(results) != 0 {
		t.Errorf("results = %v, want empty array", run["results"])
	}
	rules := run["tool"].(map[string]any)["driver"].(map[string]any)["rules"]
	if r, ok := rules.([]any); !ok || len(r) != 0 {
		t.Errorf("rules = %v, want empty array", rules)
	}
}
