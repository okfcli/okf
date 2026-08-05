package validate

import "encoding/json"

// SARIF 2.1.0 output for CI code scanning (GitHub Security tab, PR
// annotations via github/codeql-action/upload-sarif). Findings are
// concept-scoped, so each result carries a file-level physicalLocation
// without a region, which is valid SARIF.

const sarifSchemaURI = "https://json.schemastore.org/sarif-2.1.0.json"

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

// sarifLevel maps a Severity to a SARIF result level.
func sarifLevel(s Severity) string {
	if s == SeverityError {
		return "error"
	}
	return "warning"
}

// SARIF renders findings as an indented SARIF 2.1.0 document. toolVersion is
// the okf binary version. The driver rules array lists each rule that appears
// in the results, deduplicated, in order of first appearance; artifact URIs
// are the concept's markdown path relative to the bundle root.
func SARIF(findings []Finding, toolVersion string) ([]byte, error) {
	rules := make([]sarifRule, 0)
	seen := make(map[string]bool)
	results := make([]sarifResult, 0, len(findings))
	for _, f := range findings {
		if !seen[f.RuleID] {
			seen[f.RuleID] = true
			desc := ruleDescriptions[f.RuleID]
			if desc == "" {
				desc = f.RuleID
			}
			rules = append(rules, sarifRule{ID: f.RuleID, ShortDescription: sarifMessage{Text: desc}})
		}
		results = append(results, sarifResult{
			RuleID:  f.RuleID,
			Level:   sarifLevel(f.Severity),
			Message: sarifMessage{Text: f.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: f.ConceptID + ".md"},
				},
			}},
		})
	}

	doc := sarifLog{
		Schema:  sarifSchemaURI,
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "okf",
				Version:        toolVersion,
				InformationURI: "https://github.com/okfcli/okf",
				Rules:          rules,
			}},
			Results: results,
		}},
	}
	return json.MarshalIndent(doc, "", "  ")
}
