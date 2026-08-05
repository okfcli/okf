package validate

// Rule IDs are stable identifiers for every check validate performs, named
// okf/<family>/<check>. They are a public contract: agents and CI suppress or
// route findings by rule ID, and SARIF consumers key on them, so renaming one
// is a breaking change (see TestRuleIDSet).
const (
	// frontmatter: required and recommended fields (§4.1)
	RuleTypeRequired           = "okf/frontmatter/type-required"
	RuleTitleRecommended       = "okf/frontmatter/title-recommended"
	RuleDescriptionRecommended = "okf/frontmatter/description-recommended"
	RuleTagsRecommended        = "okf/frontmatter/tags-recommended"
	RuleTimestampFuture        = "okf/frontmatter/timestamp-future"

	// body: markdown body structure (§4.2)
	RuleBodyEmpty = "okf/body/empty"

	// links: cross-link resolution (§6)
	RuleLinkBroken = "okf/links/broken"

	// lifecycle: status and staleness (§5.4, §5.5)
	RuleStatusInvalid     = "okf/lifecycle/status-invalid"
	RuleStaleAfterInvalid = "okf/lifecycle/stale-after-invalid"
	RuleStale             = "okf/lifecycle/stale"

	// trust: generated and verified provenance events (§5.2, §7)
	RuleGeneratedByRequired = "okf/trust/generated-by-required"
	RuleGeneratedAtInvalid  = "okf/trust/generated-at-invalid"
	RuleVerifiedIncomplete  = "okf/trust/verified-incomplete"
	RuleVerifiedAtInvalid   = "okf/trust/verified-at-invalid"
	RuleActorConvention     = "okf/trust/actor-convention"

	// legacy: v0.1 constructs superseded in v0.2 (§13.1)
	RuleLegacyTimestamp = "okf/legacy/timestamp"
	RuleLegacyCitations = "okf/legacy/citations"

	// sources: the provenance family (§5.1)
	RuleSourceResourceRequired = "okf/sources/resource-required"
	RuleUsageWindowMissing     = "okf/sources/usage-window-missing"
	RuleFootnoteUnmatched      = "okf/sources/footnote-unmatched"
	RuleFootnoteUndefined      = "okf/sources/footnote-undefined"
	RuleFootnoteUnreferenced   = "okf/sources/footnote-unreferenced"

	// computation: the Attested Computation contract (§10, §6.2)
	RuleRuntimeRequired     = "okf/computation/runtime-required"
	RuleComputationDup      = "okf/computation/duplicate"
	RuleComputationMissing  = "okf/computation/missing"
	RuleParameterIncomplete = "okf/computation/parameter-incomplete"
	RuleContractPathMissing = "okf/computation/path-missing"

	// reserved: index.md and log.md structure (§8, §9, §12)
	RuleIndexFrontmatter    = "okf/reserved/index-frontmatter"
	RuleIndexFrontmatterKey = "okf/reserved/index-frontmatter-key"
	RuleOKFVersionUnknown   = "okf/reserved/okf-version-unknown"
	RuleLogDateHeading      = "okf/reserved/log-date-heading"
)

// ruleDescriptions gives each rule a short one-line description, used as the
// SARIF shortDescription. Every rule ID must have an entry.
var ruleDescriptions = map[string]string{
	RuleTypeRequired:           "frontmatter 'type' is required (OKF §4.1)",
	RuleTitleRecommended:       "frontmatter 'title' is recommended (OKF §4.1)",
	RuleDescriptionRecommended: "frontmatter 'description' is recommended (OKF §4.1)",
	RuleTagsRecommended:        "frontmatter 'tags' is recommended (OKF §4.1)",
	RuleTimestampFuture:        "frontmatter 'timestamp' is more than a year in the future",

	RuleBodyEmpty: "body is empty, structural markdown is recommended (OKF §4.2)",

	RuleLinkBroken: "cross-link does not resolve to a concept in the bundle (OKF §6)",

	RuleStatusInvalid:     "'status' must be draft, stable, or deprecated (OKF §5.4)",
	RuleStaleAfterInvalid: "'stale_after' must be an absolute YYYY-MM-DD date (OKF §5.5)",
	RuleStale:             "concept is past its stale_after date (OKF §5.5)",

	RuleGeneratedByRequired: "'generated' requires 'by' (OKF §5.2)",
	RuleGeneratedAtInvalid:  "'generated.at' is not an ISO 8601 datetime (OKF §5.2)",
	RuleVerifiedIncomplete:  "verified events should carry both 'by' and 'at' (OKF §5.2)",
	RuleVerifiedAtInvalid:   "'verified[].at' is not an ISO 8601 datetime (OKF §5.2)",
	RuleActorConvention:     "identity does not follow the actor convention: human:<id>, process:<id>, or <producer>/<version> (OKF §7)",

	RuleLegacyTimestamp: "legacy 'timestamp' is superseded by 'generated.at' in OKF v0.2 (§13.1)",
	RuleLegacyCitations: "legacy '# Citations' list is superseded by the 'sources' frontmatter family in OKF v0.2 (§13.1)",

	RuleSourceResourceRequired: "source entries require 'resource' (OKF §5.1)",
	RuleUsageWindowMissing:     "'usage_count' has no framing 'usage_window' (OKF §5.1)",
	RuleFootnoteUnmatched:      "body footnote label has no matching 'sources[].id' (OKF §5.1)",
	RuleFootnoteUndefined:      "body footnote is referenced but never defined (OKF §5.1)",
	RuleFootnoteUnreferenced:   "body footnote is defined but never referenced (OKF §5.1)",

	RuleRuntimeRequired:     "'runtime' is required for type Attested Computation (OKF §10.2)",
	RuleComputationDup:      "computation is provided both inline and via the 'computation' path (OKF §10.3)",
	RuleComputationMissing:  "no computation found: provide a body Computation section or a 'computation' path (OKF §10.3)",
	RuleParameterIncomplete: "parameters should carry 'name' and 'type' (OKF §10.2)",
	RuleContractPathMissing: "path-valued contract field points at a file that does not exist in the bundle (OKF §6.2)",

	RuleIndexFrontmatter:    "index.md files must not contain frontmatter (OKF §8)",
	RuleIndexFrontmatterKey: "root index.md frontmatter may only carry 'okf_version' (OKF §8, §12)",
	RuleOKFVersionUnknown:   "declared okf_version is not recognized by this tool (OKF §12)",
	RuleLogDateHeading:      "log.md date headings must use ISO 8601 YYYY-MM-DD form (OKF §9)",
}
