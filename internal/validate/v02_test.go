package validate

import (
	"strings"
	"testing"
	"time"
)

// ok is a minimal fully-clean concept body (no warnings).
const okConcept = "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\n---\n\nbody"

// findingWith reports whether the report contains a finding of the given
// severity whose message contains substr.
func findingWith(t *testing.T, r *Report, sev Severity, substr string) bool {
	t.Helper()
	for _, f := range r.Findings {
		if f.Severity == sev && strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}

func mustFinding(t *testing.T, r *Report, sev Severity, substr string) {
	t.Helper()
	if !findingWith(t, r, sev, substr) {
		t.Fatalf("expected %s finding containing %q, findings = %+v", sev, substr, r.Findings)
	}
}

func mustNotFinding(t *testing.T, r *Report, substr string) {
	t.Helper()
	for _, f := range r.Findings {
		if strings.Contains(f.Message, substr) {
			t.Fatalf("unexpected finding containing %q: %+v", substr, f)
		}
	}
}

// --- §5.4 status ---

func TestValidate_InvalidStatus(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nstatus: retired\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityError, "'status'")
}

func TestValidate_ValidStatusesAccepted(t *testing.T) {
	for _, s := range []string{"draft", "stable", "deprecated"} {
		b := testBundle(t, map[string]string{
			"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nstatus: " + s + "\n---\n\nbody",
		})
		mustNotFinding(t, Validate(b), "'status'")
	}
}

// --- §5.5 stale_after ---

func TestValidate_MalformedStaleAfter(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nstale_after: soon\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityError, "stale_after")
}

func TestValidate_StaleConceptWarns(t *testing.T) {
	defer func(orig func() time.Time) { now = orig }(now)
	now = func() time.Time { return time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC) }
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nstale_after: 2026-09-23\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityWarning, "stale")
}

func TestValidate_FreshConceptNoStaleWarning(t *testing.T) {
	defer func(orig func() time.Time) { now = orig }(now)
	now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nstale_after: 2026-09-23\n---\n\nbody",
	})
	mustNotFinding(t, Validate(b), "stale")
}

// --- §5.2 generated / verified ---

func TestValidate_GeneratedWithoutBy(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\ngenerated: { at: 2026-06-20T22:53:05Z }\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityError, "generated")
}

func TestValidate_VerifiedEntryMissingFields(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nverified: { by: human:x }\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityWarning, "verified")
}

func TestValidate_ActorConventionWarning(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\ngenerated: { by: just a name, at: 2026-06-20T22:53:05Z }\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityWarning, "actor")
}

// --- §13.1 legacy fields ---

func TestValidate_LegacyTimestampWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\ntimestamp: 2026-05-28T22:53:05Z\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityWarning, "generated.at")
}

func TestValidate_LegacyCitationsWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\n---\n\n# Citations\n- https://example.com\n",
	})
	mustFinding(t, Validate(b), SeverityWarning, "sources")
}

// --- §5.1 sources ---

func TestValidate_SourceEntryWithoutResource(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nsources:\n  - id: s1\n    title: no resource here\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityError, "resource")
}

func TestValidate_UsageCountWithoutWindow(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nsources:\n  - id: s1\n    resource: https://example.com\n    usage_count: 10\n---\n\nbody",
	})
	mustFinding(t, Validate(b), SeverityWarning, "usage_window")
}

func TestValidate_UsageCountWithSharedWindowOK(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nsources:\n  - id: s1\n    resource: https://example.com\n    usage_count: 10\nusage_window: { from: 2026-06-01, to: 2026-06-30 }\n---\n\nbody",
	})
	mustNotFinding(t, Validate(b), "usage_window")
}

// Footnote labels join into sources[].id (§5.1): a cited label with no
// matching source id is a warning.
func TestValidate_FootnoteLabelWithoutMatchingSourceID(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nsources:\n  - id: rev-policy\n    resource: https://example.com\n---\n\nA claim.[^wrong-label]\n\n[^wrong-label]: something\n",
	})
	mustFinding(t, Validate(b), SeverityWarning, "wrong-label")
}

func TestValidate_FootnoteLabelMatchingSourceIDOK(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nsources:\n  - id: rev-policy\n    resource: https://example.com\n---\n\nA claim.[^rev-policy]\n\n[^rev-policy]: Revenue recognition policy\n",
	})
	mustNotFinding(t, Validate(b), "rev-policy")
}

// A footnote referenced in the body but never defined renders as a dangling
// marker (§5.1, §13.1): warn, don't fail validate.
func TestValidate_FootnoteReferencedButNotDefinedWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\n---\n\nA claim.[^orphan]\n",
	})
	r := Validate(b)
	mustFinding(t, r, SeverityWarning, "footnote [^orphan] is referenced but never defined")
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %+v", r.Findings)
	}
}

// A footnote defined but never referenced renders as nothing (§5.1, §13.1):
// the source silently drops out of the rendered document. Warn, don't fail.
func TestValidate_FootnoteDefinedButNotReferencedWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\n---\n\nprose with no footnote marks.\n\n[^unused]: something\n",
	})
	r := Validate(b)
	mustFinding(t, r, SeverityWarning, "footnote [^unused] is defined but never referenced")
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %+v", r.Findings)
	}
}

// A label that is defined and IS present in sources[].id, but never
// referenced in the body, still warns as unreferenced: the sources[].id join
// check and the definition/reference check are independent concerns.
func TestValidate_FootnoteDefinedInSourcesButNotReferencedStillWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\nsources:\n  - id: rev-policy\n    resource: https://example.com\n---\n\nprose with no footnote marks.\n\n[^rev-policy]: Revenue recognition policy\n",
	})
	mustFinding(t, Validate(b), SeverityWarning, "footnote [^rev-policy] is defined but never referenced")
}

// A label both defined and referenced produces neither new warning, whether
// or not it also joins into sources[].id.
func TestValidate_FootnoteDefinedAndReferencedNoWarning(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\n---\n\nA claim.[^n1]\n\n[^n1]: a note\n",
	})
	r := Validate(b)
	mustNotFinding(t, r, "is referenced but never defined")
	mustNotFinding(t, r, "is defined but never referenced")
}

// A body with no footnote marks at all produces no footnote definition
// findings.
func TestValidate_NoFootnotesNoDefinitionFindings(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": okConcept,
	})
	r := Validate(b)
	mustNotFinding(t, r, "is referenced but never defined")
	mustNotFinding(t, r, "is defined but never referenced")
}

// --- §10 attested computations ---

func TestValidate_AttestedComputationRequiresRuntime(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\n---\n\n# Computation\n\n    SELECT 1\n",
	})
	mustFinding(t, Validate(b), SeverityError, "runtime")
}

func TestValidate_AttestedComputationNeedsComputation(t *testing.T) {
	// Neither a `computation` path nor a body `# Computation` section.
	b := testBundle(t, map[string]string{
		"c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\n---\n\nprose only\n",
	})
	mustFinding(t, Validate(b), SeverityWarning, "Computation")
}

func TestValidate_AttestedComputationInlineFenceOK(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\n---\n\n# Computation\n\n    SELECT 1\n",
	})
	r := Validate(b)
	mustNotFinding(t, r, "# Computation")
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %+v", r.Findings)
	}
}

func TestValidate_AttestedComputationBothInlineAndFileWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md":            "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\ncomputation: /lib/revenue.sql\n---\n\n# Computation\n\n    SELECT 1\n",
		"lib/revenue.sql": "SELECT 1",
	})
	mustFinding(t, Validate(b), SeverityWarning, "both")
}

func TestValidate_ParameterMissingNameOrType(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\nparameters:\n  - { name: year }\n---\n\n# Computation\n\n    SELECT 1\n",
	})
	mustFinding(t, Validate(b), SeverityWarning, "parameter")
}

// Path-valued contract fields (§6.2) that point at missing local files warn.
func TestValidate_MissingAttesterFileWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\nattester:\n  resource: attesters/revenue.py\n---\n\n# Computation\n\n    SELECT 1\n",
	})
	mustFinding(t, Validate(b), SeverityWarning, "attesters/revenue.py")
}

func TestValidate_PresentAttesterFileOK(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md":                 "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\nattester:\n  resource: attesters/revenue.py\n---\n\n# Computation\n\n    SELECT 1\n",
		"attesters/revenue.py": "print('hi')",
	})
	mustNotFinding(t, Validate(b), "attesters/revenue.py")
}

func TestValidate_ExternalExecutorURLNotChecked(t *testing.T) {
	b := testBundle(t, map[string]string{
		"c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\nexecutor:\n  resource: https://example.com/runner\n---\n\n# Computation\n\n    SELECT 1\n",
	})
	mustNotFinding(t, Validate(b), "example.com")
}

// --- §8 / §12 index.md frontmatter and okf_version ---

func TestValidate_RootIndexOkfVersionKnown(t *testing.T) {
	b := testBundle(t, map[string]string{
		"index.md": "---\nokf_version: \"0.2\"\n---\n# Index\n\n* [A](a.md) - a concept\n",
		"a.md":     okConcept,
	})
	mustNotFinding(t, Validate(b), "okf_version")
}

func TestValidate_RootIndexOkfVersionUnknownWarns(t *testing.T) {
	b := testBundle(t, map[string]string{
		"index.md": "---\nokf_version: \"9.9\"\n---\n# Index\n\n* [A](a.md) - a concept\n",
		"a.md":     okConcept,
	})
	mustFinding(t, Validate(b), SeverityWarning, "okf_version")
}

func TestValidate_NonRootIndexWithFrontmatterErrors(t *testing.T) {
	b := testBundle(t, map[string]string{
		"sub/index.md": "---\nokf_version: \"0.2\"\n---\n# Index\n",
		"sub/a.md":     okConcept,
	})
	mustFinding(t, Validate(b), SeverityError, "frontmatter")
}

func TestValidate_RootIndexExtraFrontmatterKeysWarn(t *testing.T) {
	b := testBundle(t, map[string]string{
		"index.md": "---\nokf_version: \"0.2\"\ncustom: x\n---\n# Index\n",
		"a.md":     okConcept,
	})
	mustFinding(t, Validate(b), SeverityWarning, "okf_version")
}

// --- §9 log.md date headings ---

func TestValidate_LogDateHeadingsISO(t *testing.T) {
	b := testBundle(t, map[string]string{
		"log.md": "# Update Log\n\n## 2026-05-22\n* **Update**: something.\n\n## May 15th 2026\n* bad heading.\n",
		"a.md":   okConcept,
	})
	mustFinding(t, Validate(b), SeverityError, "YYYY-MM-DD")
}

func TestValidate_LogValidHeadingsOK(t *testing.T) {
	b := testBundle(t, map[string]string{
		"log.md": "# Update Log\n\n## 2026-05-22\n* **Update**: something.\n",
		"a.md":   okConcept,
	})
	mustNotFinding(t, Validate(b), "YYYY-MM-DD")
}

// --- clean v0.2 concept has no v0.2 findings ---

func TestValidate_FullV02ConceptClean(t *testing.T) {
	defer func(orig func() time.Time) { now = orig }(now)
	now = func() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) }
	b := testBundle(t, map[string]string{
		"computations/revenue.md": `---
type: Attested Computation
title: Revenue for fiscal year
description: Recognized revenue for a fiscal year.
tags: [finance]
status: stable
runtime: bigquery
parameters:
  - { name: year, type: integer, required: true }
executor:
  resource: /references/skills/run-on-bq.md
  receipt: [job_id, executed_sql, result]
attester:
  resource: /references/attesters/revenue.py
generated: { by: reference_agent/gemini-2.5-pro, at: 2026-06-20T22:53:05Z }
verified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }
stale_after: 2026-09-23
sources:
  - id: rev-policy
    resource: https://wiki.acme/finance/revenue-recognition
    title: Revenue recognition policy
---

# Computation

    SELECT SUM(amount) AS revenue

Per the recognition policy.[^rev-policy]

[^rev-policy]: Revenue recognition policy
`,
		"references/skills/run-on-bq.md":  "---\ntype: Skill\ntitle: Run on BQ\ndescription: d\ntags: [x]\n---\n\nbody",
		"references/attesters/revenue.py": "print('check')",
	})
	r := Validate(b)
	if len(r.Findings) != 0 {
		t.Fatalf("expected clean report, findings = %+v", r.Findings)
	}
}

// Upstream bundles and the spec's own §10.2 example write contract paths
// relative to the bundle root without a leading slash (e.g.
// `references/skills/run-on-bq.md` from a concept in computations/). A path
// that resolves from either the concept's directory or the bundle root is
// not warned about.
func TestValidate_ContractPathRootRelativeFallback(t *testing.T) {
	b := testBundle(t, map[string]string{
		"computations/c.md": "---\ntype: Attested Computation\ntitle: A\ndescription: d\ntags: [x]\nruntime: bigquery\nexecutor:\n  resource: skills/run.md\nattester:\n  resource: attesters/eq.py\n---\n\n# Computation\n\n    SELECT 1\n",
		"skills/run.md":     "---\ntype: Skill\ntitle: S\ndescription: d\ntags: [x]\n---\n\nbody",
		"attesters/eq.py":   "print('check')",
	})
	r := Validate(b)
	mustNotFinding(t, r, "does not exist")
}
