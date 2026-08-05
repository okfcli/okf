// OKF v0.2 checks: provenance, trust, and lifecycle families (§5), the
// Attested Computation contract (§10), reserved-file structure (§8, §9), and
// the okf_version declaration (§12).
//
// Severity follows the spec's own language: violations of a MUST or a
// REQUIRED-within-family rule are errors; departures from SHOULD guidance and
// legacy v0.1 constructs (§13.1) are warnings.

package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
)

// now is injectable for deterministic staleness tests.
var now = time.Now

// knownOKFVersions are the spec revisions this tool understands (§12).
var knownOKFVersions = map[string]bool{"0.1": true, "0.2": true}

// validateV02 runs the v0.2 family checks for one concept.
func validateV02(r *Report, b *bundle.Bundle, c *concept.Concept) {
	fm := c.Frontmatter

	// §5.4 status is a fixed enum when present.
	if s := strings.TrimSpace(fm.Status); s != "" && !concept.ValidStatuses[s] {
		r.add(c.ID, SeverityError, fmt.Sprintf(
			"frontmatter: 'status' must be draft, stable, or deprecated, got %q (OKF §5.4)", fm.Status))
	}

	// §5.5 stale_after is an absolute YYYY-MM-DD date.
	if s := strings.TrimSpace(fm.StaleAfter); s != "" {
		if _, err := time.Parse("2006-01-02", s); err != nil {
			r.add(c.ID, SeverityError, fmt.Sprintf(
				"frontmatter: 'stale_after' must be an absolute YYYY-MM-DD date, got %q (OKF §5.5)", fm.StaleAfter))
		} else if fm.IsStale(now()) {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"concept is stale: today >= stale_after (%s) (OKF §5.5)", fm.StaleAfter))
		}
	}

	// §5.2 generated.by is REQUIRED within generated.
	if fm.Generated != nil {
		if strings.TrimSpace(fm.Generated.By) == "" {
			r.add(c.ID, SeverityError, "frontmatter: 'generated' requires 'by' (OKF §5.2)")
		} else {
			checkActor(r, c.ID, "generated.by", fm.Generated.By)
		}
		if at := strings.TrimSpace(fm.Generated.At); at != "" {
			if _, err := concept.ParseDatetime(at); err != nil {
				r.add(c.ID, SeverityWarning, fmt.Sprintf(
					"frontmatter: 'generated.at' is not an ISO 8601 datetime: %q (OKF §5.2)", fm.Generated.At))
			}
		}
	}

	// §5.2 verified events carry by and at.
	for i, v := range fm.Verified {
		if strings.TrimSpace(v.By) == "" || strings.TrimSpace(v.At) == "" {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"frontmatter: 'verified[%d]' should carry both 'by' and 'at' (OKF §5.2)", i))
			continue
		}
		checkActor(r, c.ID, fmt.Sprintf("verified[%d].by", i), v.By)
		if _, err := concept.ParseDatetime(v.At); err != nil {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"frontmatter: 'verified[%d].at' is not an ISO 8601 datetime: %q (OKF §5.2)", i, v.At))
		}
	}

	// §13.1 legacy v0.1 constructs.
	if !fm.Timestamp.IsZero() {
		r.add(c.ID, SeverityWarning,
			"frontmatter: legacy 'timestamp' is superseded by 'generated.at' in OKF v0.2 (§13.1)")
	}
	if citationsHeading.MatchString(c.Body) {
		r.add(c.ID, SeverityWarning,
			"body: legacy '# Citations' list is superseded by the 'sources' frontmatter family in OKF v0.2 (§13.1)")
	}

	validateSources(r, c)
	if fm.Type == concept.TypeAttestedComputation {
		validateComputationContract(r, b, c)
	}
}

// checkActor warns when an identity field does not follow the actor
// convention of §7 (human:<id>, process:<id>, <producer>/<version>).
func checkActor(r *Report, id, field, actor string) {
	if concept.ActorKind(actor) == concept.ActorUnknown {
		r.add(id, SeverityWarning, fmt.Sprintf(
			"frontmatter: '%s' (%q) does not follow the actor convention: human:<id>, process:<id>, or <producer>/<version> (OKF §7)",
			field, actor))
	}
}

var (
	citationsHeading = regexp.MustCompile(`(?mi)^#{1,6}\s+Citations\s*$`)
	// footnoteRef matches inline footnote references [^label] (not definitions).
	footnoteRef = regexp.MustCompile(`\[\^([^\]\s]+)\](?::)?`)
	// footnoteMark matches any footnote marker [^label], reference or
	// definition; footnoteLabels classifies each occurrence by position.
	footnoteMark = regexp.MustCompile(`\[\^([^\]\s]+)\]`)
	isoDate      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// footnoteLabels splits a body's footnote markers into references and
// definitions. A definition is `[^label]:` at the start of a line; every
// other `[^label]` occurrence, including one that happens to be followed by
// a literal colon mid-line, is a reference (OKF §5.1).
func footnoteLabels(body string) (refs, defs map[string]bool) {
	refs = make(map[string]bool)
	defs = make(map[string]bool)
	for _, m := range footnoteMark.FindAllStringSubmatchIndex(body, -1) {
		start, end, labelStart, labelEnd := m[0], m[1], m[2], m[3]
		label := body[labelStart:labelEnd]
		atLineStart := start == 0 || body[start-1] == '\n'
		followedByColon := end < len(body) && body[end] == ':'
		if atLineStart && followedByColon {
			defs[label] = true
		} else {
			refs[label] = true
		}
	}
	return refs, defs
}

// validateSources checks the §5.1 provenance family: resource is REQUIRED
// within an entry, usage_count needs a framing usage_window, and body
// footnote labels must join into sources[].id.
func validateSources(r *Report, c *concept.Concept) {
	fm := c.Frontmatter
	ids := make(map[string]bool)
	for i, s := range fm.Sources {
		if strings.TrimSpace(s.Resource) == "" {
			r.add(c.ID, SeverityError, fmt.Sprintf(
				"frontmatter: 'sources[%d]' requires 'resource' (OKF §5.1)", i))
		}
		if s.ID != "" {
			ids[s.ID] = true
		}
		if s.UsageCount != nil && s.UsageWindow == nil && fm.UsageWindow == nil {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"frontmatter: 'sources[%d].usage_count' has no framing 'usage_window' (entry or shared) (OKF §5.1)", i))
		}
	}

	// §5.1/§13.1 footnote definitions: a reference with no definition renders
	// as a dangling marker, and a definition with no reference renders as
	// nothing, silently dropping a source that reads as cited. Both are
	// warnings regardless of whether the label also joins into sources[].id.
	refs, defs := footnoteLabels(c.Body)
	for label := range refs {
		if !defs[label] {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"body: footnote [^%s] is referenced but never defined - renders as a dangling marker (OKF §5.1)", label))
		}
	}
	for label := range defs {
		if !refs[label] {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"body: footnote [^%s] is defined but never referenced - renders as nothing, so a source that reads as cited is absent from the output (OKF §5.1)", label))
		}
	}

	// Per-claim attribution: footnote labels are join keys into sources[].id.
	// Only meaningful when the concept declares source ids at all.
	if len(ids) == 0 {
		return
	}
	seen := make(map[string]bool)
	for _, m := range footnoteRef.FindAllStringSubmatch(c.Body, -1) {
		label := m[1]
		if seen[label] || ids[label] {
			seen[label] = true
			continue
		}
		seen[label] = true
		r.add(c.ID, SeverityWarning, fmt.Sprintf(
			"body: footnote label [^%s] has no matching 'sources[].id' - labels are the join key for per-claim attribution (OKF §5.1)", label))
	}
}

// hasComputationSection reports whether the body carries a `# Computation`
// section with content (an inline fence or indented code, §10.3).
func hasComputationSection(body string) bool {
	return regexp.MustCompile(`(?mi)^#{1,6}\s+Computation\s*$`).MatchString(body)
}

// validateComputationContract checks the §10.2 contract of an Attested
// Computation concept.
func validateComputationContract(r *Report, b *bundle.Bundle, c *concept.Concept) {
	fm := c.Frontmatter

	// runtime is REQUIRED for this type.
	if strings.TrimSpace(fm.Runtime) == "" {
		r.add(c.ID, SeverityError,
			"frontmatter: 'runtime' is required for type Attested Computation (OKF §10.2)")
	}

	hasInline := hasComputationSection(c.Body)
	hasFile := strings.TrimSpace(fm.Computation) != ""
	switch {
	case hasInline && hasFile:
		r.add(c.ID, SeverityWarning,
			"computation is provided both inline (body Computation section) and via the 'computation' path; provide one (OKF §10.3)")
	case !hasInline && !hasFile:
		r.add(c.ID, SeverityWarning,
			"no computation found: provide a body Computation section or a 'computation' path (OKF §10.3)")
	}

	for i, p := range fm.Parameters {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Type) == "" {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"frontmatter: parameter %d should carry 'name' and 'type' (OKF §10.2)", i))
		}
	}

	// Path-valued contract fields should point at material that exists (§6.2).
	checkLocalPath(r, b, c, "computation", fm.Computation)
	if fm.Executor != nil {
		checkLocalPath(r, b, c, "executor.resource", fm.Executor.Resource)
	}
	if fm.Attester != nil {
		checkLocalPath(r, b, c, "attester.resource", fm.Attester.Resource)
	}
}

// checkLocalPath warns when a path-valued field (§6.2) names a local file
// that does not exist in the bundle. URLs are not checked. Bundle-relative
// paths (leading /) resolve from the bundle root; bare relative paths are
// tried from the concept's own directory and then from the bundle root - the
// spec's §10.2 example and upstream bundles write contract paths root-relative
// without a leading slash.
func checkLocalPath(r *Report, b *bundle.Bundle, c *concept.Concept, field, path string) {
	p := strings.TrimSpace(path)
	if p == "" || strings.Contains(p, "://") || strings.HasPrefix(p, "mailto:") {
		return
	}
	var candidates []string
	if strings.HasPrefix(p, "/") {
		candidates = []string{strings.TrimPrefix(p, "/")}
	} else {
		candidates = []string{normalizePath(dirOf(c.ID) + "/" + p), normalizePath(p)}
	}
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(b.Root, filepath.FromSlash(rel))); err == nil {
			return
		}
	}
	r.add(c.ID, SeverityWarning, fmt.Sprintf(
		"'%s' points at %s, which does not exist in the bundle (OKF §6.2)", field, path))
}

// validateReserved checks index.md and log.md structure (§8, §9, §12).
func validateReserved(r *Report, b *bundle.Bundle) {
	for _, c := range b.Reserved {
		isIndex := c.ID == "index" || strings.HasSuffix(c.ID, "/index")
		isLog := c.ID == "log" || strings.HasSuffix(c.ID, "/log")
		switch {
		case isIndex:
			validateIndexFile(r, c)
		case isLog:
			validateLogFile(r, c)
		}
	}
}

// validateIndexFile enforces §8: index files carry no frontmatter, except a
// bundle-root index.md, which may declare okf_version (§12) and nothing else.
func validateIndexFile(r *Report, c *concept.Concept) {
	if c.RawFront == "" {
		return
	}
	if c.ID != "index" {
		r.add(c.ID, SeverityError,
			"index.md files must not contain frontmatter; only the bundle-root index.md may carry 'okf_version' (OKF §8)")
		return
	}

	keys := frontmatterKeys(c)
	for _, k := range keys {
		if k != "okf_version" {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"root index.md frontmatter carries %q; only 'okf_version' is permitted (OKF §8, §12)", k))
		}
	}
	if v, ok := c.Frontmatter.Extensions["okf_version"]; ok {
		ver := fmt.Sprintf("%v", v)
		if !knownOKFVersions[ver] {
			r.add(c.ID, SeverityWarning, fmt.Sprintf(
				"root index.md declares okf_version %q, which this tool does not recognize (known: 0.1, 0.2) (OKF §12)", ver))
		}
	}
}

// frontmatterKeys lists the keys present in a reserved file's frontmatter.
// Reserved files decode all keys into Extensions except the typed fields;
// typed fields are reported when non-zero.
func frontmatterKeys(c *concept.Concept) []string {
	var keys []string
	for k := range c.Frontmatter.Extensions {
		keys = append(keys, k)
	}
	fm := c.Frontmatter
	for k, present := range map[string]bool{
		"type":        fm.Type != "",
		"title":       fm.Title != "",
		"description": fm.Description != "",
		"resource":    fm.Resource != "",
		"tags":        len(fm.Tags) > 0,
		"status":      fm.Status != "",
	} {
		if present {
			keys = append(keys, k)
		}
	}
	return keys
}

// validateLogFile enforces §9: date headings MUST use ISO 8601 YYYY-MM-DD.
func validateLogFile(r *Report, c *concept.Concept) {
	for _, line := range strings.Split(c.Body, "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		if !isoDate.MatchString(heading) {
			r.add(c.ID, SeverityError, fmt.Sprintf(
				"log.md date heading %q must use ISO 8601 YYYY-MM-DD form (OKF §9)", heading))
		}
	}
}
