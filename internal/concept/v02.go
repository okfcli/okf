// OKF v0.2 frontmatter families: provenance (§5.1), trust (§5.2–5.3),
// lifecycle (§5.4–5.5), the actor convention (§7), and the Attested
// Computation contract (§10).

package concept

import (
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// TypeAttestedComputation is the concept type carrying a sanctioned
// computation (OKF §10).
const TypeAttestedComputation = "Attested Computation"

// Trust tiers derived from `verified` (OKF §5.3), lowest to highest.
const (
	TierUnverified       = "unverified"
	TierMachineConfirmed = "machine-confirmed"
	TierHumanReviewed    = "human-reviewed"
)

// Actor kinds under the actor convention (OKF §7).
const (
	ActorHuman   = "human"
	ActorProcess = "process"
	ActorAgent   = "agent"
	ActorUnknown = "unknown"
)

// Source is one entry of the `sources` provenance family (OKF §5.1).
// Resource is REQUIRED within an entry; it names either a followable artifact
// (URL or path) or a population/scope descriptor. The credibility signals
// (author, usage_count, last_modified) are optional and objective; OKF records
// signals, not a score.
type Source struct {
	ID           string       `yaml:"id"`
	Resource     string       `yaml:"resource"`
	Title        string       `yaml:"title"`
	Author       string       `yaml:"author"`
	UsageCount   *int64       `yaml:"usage_count"`
	LastModified string       `yaml:"last_modified"`
	UsageWindow  *UsageWindow `yaml:"usage_window"` // per-entry override of the shared window
}

// UsageWindow frames usage_count with a { from, to } date range (OKF §5.1).
type UsageWindow struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// Generated records how the current content was produced (OKF §5.2).
type Generated struct {
	By string `yaml:"by"` // REQUIRED within generated; an actor (§7)
	At string `yaml:"at"` // ISO 8601 datetime of the last meaningful change
}

// Verification is one verification event (OKF §5.2).
type Verification struct {
	By string `yaml:"by"`
	At string `yaml:"at"`
}

// VerifiedList unmarshals the `verified` field. Consumers MUST treat a bare
// { by, at } mapping as a one-element list (OKF §5.2, §11), so both shapes are
// accepted. Malformed shapes are tolerated as empty rather than failing the
// whole bundle load, matching StringList's posture.
type VerifiedList []Verification

// UnmarshalYAML implements yaml.Unmarshaler, accepting mapping-or-sequence.
func (v *VerifiedList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.MappingNode:
		var one Verification
		if err := value.Decode(&one); err != nil {
			*v = nil
			return nil
		}
		*v = VerifiedList{one}
		return nil
	case yaml.SequenceNode:
		var list []Verification
		if err := value.Decode(&list); err != nil {
			*v = nil
			return nil
		}
		*v = VerifiedList(list)
		return nil
	default:
		*v = nil
		return nil
	}
}

// Parameter is one typed, named hole of an Attested Computation (OKF §10.2).
type Parameter struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

// Executor says how a computation is run and what evidence a run must return
// (OKF §10.2).
type Executor struct {
	Resource string   `yaml:"resource"`
	Receipt  []string `yaml:"receipt"`
}

// Attester names deterministic (no-LLM) code that inspects a receipt and
// returns a verdict (OKF §10.2).
type Attester struct {
	Resource string `yaml:"resource"`
}

// ValidStatuses are the lifecycle states of §5.4.
var ValidStatuses = map[string]bool{"draft": true, "stable": true, "deprecated": true}

// EffectiveStatus returns the concept's lifecycle status, defaulting to
// "stable" when absent (OKF §5.4). An invalid value is returned as written;
// validation flags it separately.
func (f *Frontmatter) EffectiveStatus() string {
	if strings.TrimSpace(f.Status) == "" {
		return "stable"
	}
	return f.Status
}

// TrustTier derives the trust tier from `verified` (OKF §5.3): no verified key
// means unverified; only non-human actors means machine-confirmed; any
// human:<id> actor means human-reviewed.
func (f *Frontmatter) TrustTier() string {
	if len(f.Verified) == 0 {
		return TierUnverified
	}
	for _, v := range f.Verified {
		if ActorKind(v.By) == ActorHuman {
			return TierHumanReviewed
		}
	}
	return TierMachineConfirmed
}

// GeneratedAt returns when the content last meaningfully changed: generated.at
// when present, falling back to the legacy v0.1 `timestamp` (OKF §13.1).
// ok is false when neither is present or parseable.
func (f *Frontmatter) GeneratedAt() (at time.Time, ok bool) {
	if f.Generated != nil && f.Generated.At != "" {
		if t, err := ParseDatetime(f.Generated.At); err == nil {
			return t, true
		}
	}
	if !f.Timestamp.IsZero() {
		return f.Timestamp, true
	}
	return time.Time{}, false
}

// IsStale reports whether the concept is stale as of today: stale when
// today >= stale_after (OKF §5.5). Absent or unparseable stale_after is never
// stale; validation flags the unparseable case separately.
func (f *Frontmatter) IsStale(today time.Time) bool {
	if strings.TrimSpace(f.StaleAfter) == "" {
		return false
	}
	d, err := time.Parse("2006-01-02", f.StaleAfter)
	if err != nil {
		return false
	}
	return !today.Before(d)
}

// ActorKind classifies an actor string per the convention of §7:
// human:<id>, process:<id>, or <producer>/<version> for agents and tools.
func ActorKind(actor string) string {
	switch {
	case strings.HasPrefix(actor, "human:") && len(actor) > len("human:"):
		return ActorHuman
	case strings.HasPrefix(actor, "process:") && len(actor) > len("process:"):
		return ActorProcess
	case strings.Contains(actor, "/") && !strings.ContainsAny(actor, " \t"):
		return ActorAgent
	default:
		return ActorUnknown
	}
}

// ParseDatetime parses an ISO 8601 datetime as used by generated.at and
// verified[].at.
func ParseDatetime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
