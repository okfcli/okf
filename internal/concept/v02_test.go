package concept

import (
	"reflect"
	"testing"
	"time"
)

// --- §5.2 generated ---

func TestParse_Generated(t *testing.T) {
	raw := []byte("---\ntype: T\ngenerated: { by: reference_agent/gemini-2.5-pro, at: 2026-06-20T22:53:05Z }\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	g := c.Frontmatter.Generated
	if g == nil {
		t.Fatal("Generated = nil, want parsed mapping")
	}
	if g.By != "reference_agent/gemini-2.5-pro" {
		t.Errorf("Generated.By = %q", g.By)
	}
	if g.At != "2026-06-20T22:53:05Z" {
		t.Errorf("Generated.At = %q", g.At)
	}
}

// --- §5.2 verified: bare mapping MUST be treated as a one-element list ---

func TestParse_VerifiedBareMapping(t *testing.T) {
	raw := []byte("---\ntype: T\nverified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	v := c.Frontmatter.Verified
	if len(v) != 1 {
		t.Fatalf("Verified len = %d, want 1 (bare mapping is a one-element list, OKF §5.2)", len(v))
	}
	if v[0].By != "human:ahormati" || v[0].At != "2026-06-25T09:00:00Z" {
		t.Errorf("Verified[0] = %+v", v[0])
	}
}

func TestParse_VerifiedList(t *testing.T) {
	raw := []byte("---\ntype: T\nverified:\n  - { by: human:ahormati, at: 2026-06-25T09:00:00Z }\n  - { by: process:finance-nightly, at: 2026-06-26T02:00:00Z }\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	v := c.Frontmatter.Verified
	if len(v) != 2 {
		t.Fatalf("Verified len = %d, want 2", len(v))
	}
	if v[1].By != "process:finance-nightly" {
		t.Errorf("Verified[1].By = %q", v[1].By)
	}
}

func TestParse_VerifiedMalformedTolerated(t *testing.T) {
	// A scalar (or otherwise unexpected shape) must not fail the whole parse.
	raw := []byte("---\ntype: T\nverified: yes\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes should tolerate malformed verified, got: %v", err)
	}
	if len(c.Frontmatter.Verified) != 0 {
		t.Errorf("Verified = %+v, want empty", c.Frontmatter.Verified)
	}
}

// --- §5.3 trust tiers ---

func TestTrustTier(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"no verified key", "---\ntype: T\n---\n", TierUnverified},
		{"machine only", "---\ntype: T\nverified: { by: process:nightly, at: 2026-01-01T00:00:00Z }\n---\n", TierMachineConfirmed},
		{"agent only", "---\ntype: T\nverified: { by: agent/v1, at: 2026-01-01T00:00:00Z }\n---\n", TierMachineConfirmed},
		{"human present", "---\ntype: T\nverified:\n  - { by: process:nightly, at: 2026-01-01T00:00:00Z }\n  - { by: human:ahormati, at: 2026-01-02T00:00:00Z }\n---\n", TierHumanReviewed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ParseBytes([]byte(tc.yaml+"body"), "a.md", "/tmp/a.md")
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}
			if got := c.Frontmatter.TrustTier(); got != tc.want {
				t.Errorf("TrustTier() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- §7 actor convention ---

func TestActorKind(t *testing.T) {
	cases := []struct{ actor, want string }{
		{"human:ahormati", ActorHuman},
		{"process:finance-nightly", ActorProcess},
		{"reference_agent/gemini-2.5-pro", ActorAgent},
		{"", ActorUnknown},
		{"just a name", ActorUnknown},
	}
	for _, tc := range cases {
		if got := ActorKind(tc.actor); got != tc.want {
			t.Errorf("ActorKind(%q) = %q, want %q", tc.actor, got, tc.want)
		}
	}
}

// --- §5.4 status ---

func TestEffectiveStatus(t *testing.T) {
	cases := []struct{ yaml, want string }{
		{"---\ntype: T\n---\n", "stable"}, // absent => stable
		{"---\ntype: T\nstatus: draft\n---\n", "draft"},
		{"---\ntype: T\nstatus: deprecated\n---\n", "deprecated"},
	}
	for _, tc := range cases {
		c, err := ParseBytes([]byte(tc.yaml+"body"), "a.md", "/tmp/a.md")
		if err != nil {
			t.Fatalf("ParseBytes: %v", err)
		}
		if got := c.Frontmatter.EffectiveStatus(); got != tc.want {
			t.Errorf("EffectiveStatus() = %q, want %q", got, tc.want)
		}
	}
}

// --- §5.5 stale_after ---

func TestIsStale(t *testing.T) {
	raw := []byte("---\ntype: T\nstale_after: 2026-09-23\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	day := func(s string) time.Time {
		d, _ := time.Parse("2006-01-02", s)
		return d
	}
	if c.Frontmatter.IsStale(day("2026-09-22")) {
		t.Error("IsStale(day before) = true, want false")
	}
	if !c.Frontmatter.IsStale(day("2026-09-23")) {
		t.Error("IsStale(same day) = false, want true (stale when today >= stale_after)")
	}
	if !c.Frontmatter.IsStale(day("2026-10-01")) {
		t.Error("IsStale(after) = false, want true")
	}
}

func TestIsStale_AbsentNeverStale(t *testing.T) {
	c, err := ParseBytes([]byte("---\ntype: T\n---\nbody"), "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if c.Frontmatter.IsStale(time.Now()) {
		t.Error("IsStale with no stale_after = true, want false")
	}
}

// --- §13.1 generated.at with legacy timestamp fallback ---

func TestGeneratedAt_FallbackToLegacyTimestamp(t *testing.T) {
	// v0.1 concept: only timestamp.
	c, err := ParseBytes([]byte("---\ntype: T\ntimestamp: 2026-05-28T22:53:05Z\n---\nbody"), "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	at, ok := c.Frontmatter.GeneratedAt()
	if !ok {
		t.Fatal("GeneratedAt ok = false, want fallback to legacy timestamp (OKF §13.1)")
	}
	if at.Year() != 2026 || at.Month() != 5 {
		t.Errorf("GeneratedAt = %v", at)
	}

	// v0.2 concept: generated.at wins over timestamp.
	c2, err := ParseBytes([]byte("---\ntype: T\ntimestamp: 2020-01-01T00:00:00Z\ngenerated: { by: a/1, at: 2026-06-20T22:53:05Z }\n---\nbody"), "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	at2, ok := c2.Frontmatter.GeneratedAt()
	if !ok || at2.Year() != 2026 {
		t.Errorf("GeneratedAt = %v ok=%v, want generated.at to win", at2, ok)
	}
}

// --- §5.1 sources and usage_window ---

func TestParse_SourcesWithCredibilitySignals(t *testing.T) {
	raw := []byte(`---
type: T
sources:
  - id: ga4-schema
    resource: https://developers.google.com/analytics/bigquery/export-schema
    title: GA4 BigQuery Export schema
    author: team:ga4-docs
    usage_count: 5000
    last_modified: 2026-05-30
  - resource: all queries in BigQuery project X
usage_window: { from: 2026-06-01, to: 2026-06-30 }
---
body`)
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	s := c.Frontmatter.Sources
	if len(s) != 2 {
		t.Fatalf("Sources len = %d, want 2", len(s))
	}
	first := s[0]
	if first.ID != "ga4-schema" || first.Author != "team:ga4-docs" || first.Title != "GA4 BigQuery Export schema" {
		t.Errorf("Sources[0] = %+v", first)
	}
	if first.UsageCount == nil || *first.UsageCount != 5000 {
		t.Errorf("Sources[0].UsageCount = %v, want 5000", first.UsageCount)
	}
	if first.LastModified != "2026-05-30" {
		t.Errorf("Sources[0].LastModified = %q", first.LastModified)
	}
	if s[1].Resource != "all queries in BigQuery project X" {
		t.Errorf("Sources[1].Resource = %q (scope descriptors are valid resources)", s[1].Resource)
	}
	w := c.Frontmatter.UsageWindow
	if w == nil || w.From != "2026-06-01" || w.To != "2026-06-30" {
		t.Errorf("UsageWindow = %+v", w)
	}
}

func TestParse_SourceEntryUsageWindowOverride(t *testing.T) {
	raw := []byte(`---
type: T
sources:
  - id: s1
    resource: dashboards/exec
    usage_count: 10
    usage_window: { from: 2026-01-01, to: 2026-01-31 }
---
body`)
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	w := c.Frontmatter.Sources[0].UsageWindow
	if w == nil || w.From != "2026-01-01" {
		t.Errorf("per-entry UsageWindow = %+v, want override present", w)
	}
}

// --- §10 attested computation contract ---

func TestParse_AttestedComputationContract(t *testing.T) {
	raw := []byte(`---
type: Attested Computation
title: Revenue for fiscal year
runtime: bigquery
parameters:
  - { name: year, type: integer, required: true }
computation: references/computations/lib/revenue.sql
executor:
  resource: references/skills/run-on-bq.md
  receipt: [job_id, executed_sql, result]
attester:
  resource: references/attesters/revenue.py
---
body`)
	c, err := ParseBytes(raw, "computations/revenue.md", "/tmp/revenue.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	fm := c.Frontmatter
	if fm.Type != TypeAttestedComputation {
		t.Errorf("Type = %q, want %q", fm.Type, TypeAttestedComputation)
	}
	if fm.Runtime != "bigquery" {
		t.Errorf("Runtime = %q", fm.Runtime)
	}
	wantParams := []Parameter{{Name: "year", Type: "integer", Required: true}}
	if !reflect.DeepEqual(fm.Parameters, wantParams) {
		t.Errorf("Parameters = %+v, want %+v", fm.Parameters, wantParams)
	}
	if fm.Computation != "references/computations/lib/revenue.sql" {
		t.Errorf("Computation = %q", fm.Computation)
	}
	if fm.Executor == nil || fm.Executor.Resource != "references/skills/run-on-bq.md" {
		t.Fatalf("Executor = %+v", fm.Executor)
	}
	if !reflect.DeepEqual(fm.Executor.Receipt, []string{"job_id", "executed_sql", "result"}) {
		t.Errorf("Executor.Receipt = %v", fm.Executor.Receipt)
	}
	if fm.Attester == nil || fm.Attester.Resource != "references/attesters/revenue.py" {
		t.Errorf("Attester = %+v", fm.Attester)
	}
}

// --- extensions still preserved alongside new fields ---

func TestParse_ExtensionsPreservedWithV02Fields(t *testing.T) {
	raw := []byte("---\ntype: T\nstatus: stable\ncustom_key: hello\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if c.Frontmatter.Extensions["custom_key"] != "hello" {
		t.Errorf("Extensions = %v, want custom_key preserved", c.Frontmatter.Extensions)
	}
	if _, leaked := c.Frontmatter.Extensions["status"]; leaked {
		t.Error("status leaked into Extensions; should be a recognized field")
	}
}

// Upstream reference bundles emit tags as a comma-separated scalar rather
// than a YAML list. Consumers MUST NOT reject such documents (§11), so the
// scalar form is tolerated and split on commas.
func TestParse_TagsScalarTolerated(t *testing.T) {
	raw := []byte("---\ntype: T\ntags: Stack Overflow, Q&A, developer\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes should tolerate scalar tags, got: %v", err)
	}
	want := []string{"Stack Overflow", "Q&A", "developer"}
	if !reflect.DeepEqual([]string(c.Frontmatter.Tags), want) {
		t.Errorf("Tags = %v, want %v", c.Frontmatter.Tags, want)
	}
}

func TestParse_TagsListStillWorks(t *testing.T) {
	raw := []byte("---\ntype: T\ntags: [a, b]\n---\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if !reflect.DeepEqual([]string(c.Frontmatter.Tags), []string{"a", "b"}) {
		t.Errorf("Tags = %v", c.Frontmatter.Tags)
	}
}
