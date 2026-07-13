package assurance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const DocumentationClaimValidationSchema = "fairway.documentation-claim-validation.v1"

const maxDocumentationClaimFileSize = 2 << 20

var (
	documentationClaimSchemePattern = regexp.MustCompile(`(?i)\b(iso(?:/iec)?[ -]?[0-9]{4,5}|soc[ -]?[12]|cui|fedramp|fips(?:[ -]140(?:-[23])?)?|eu[ -]?cra|eucc|common[ -]criteria|eal[ -]?[0-9]+|national[ -]cloud|sovereign[ -]cloud|regulatory)\b`)
	documentationClaimStatePattern  = regexp.MustCompile(`(?i)\b(certified|compliant|conformant|authorized|approved|accredited|validated)\b`)
	documentationHighClaimPattern   = regexp.MustCompile(`(?i)\b(certified|compliant|conformant|authorized|approved|accredited)\b`)
	documentationProductPattern     = regexp.MustCompile(`(?i)\b(fairway|product|release|package|deployment)\b`)
	documentationClauseBoundary     = regexp.MustCompile(`(?i)(?:[,.;:!?]|\b(?:and|but|however|yet|although|while|whereas|because|if|unless|before|after|when)\b)`)
	documentationDirectNegation     = regexp.MustCompile(`(?i)(?:\b(?:is|are|was|were|be|been|being|remain(?:s|ed)?|become(?:s|came)?)\s+not(?:\s+(?:currently|yet))?|\b(?:must|shall|should|may|can|could|would|will)\s+not\s+(?:(?:be\s+(?:described|represented|treated|labeled|advertised)\s+as)|(?:state|claim|describe|represent|treat|label|assert|advertise))|\b(?:cannot|can't)\s+be\s+(?:described|represented|treated|labeled|advertised)\s+as|\bdo(?:es)?\s+not\s+(?:state|claim|describe|represent|treat|label|assert|advertise)|\bnever)`)
	documentationProhibitedWording  = regexp.MustCompile(`(?i)^\s*(?:claim|claims|wording|language|statement|statements|assertion|assertions)\s+(?:is|are|remains?|must\s+be|should\s+be)?\s*(?:unsupported|prohibited|rejected|forbidden)\b`)
	documentationClaimContextToken  = regexp.MustCompile(`(?i)^(?:a|an|the|this|or|iso|iec|nist|soc|cui|fedramp|fips|eu|cra|eucc|common|criteria|eal[0-9]*|national|sovereign|cloud|fairway|product|release|package|deployment|configuration|certified|compliant|conformant|authorized|approved|accredited|validated|[0-9]+|v?[0-9]+(?:\.[0-9]+)*)$`)
)

type DocumentationClaimValidation struct {
	Schema            string `json:"schema"`
	Path              string `json:"path"`
	Valid             bool   `json:"valid"`
	LinesChecked      int    `json:"lines_checked"`
	AuthorityBoundary string `json:"authority_boundary"`
}

func ValidateDocumentationClaims(path string) (DocumentationClaimValidation, error) {
	report := DocumentationClaimValidation{
		Schema:            DocumentationClaimValidationSchema,
		Path:              filepath.Clean(path),
		AuthorityBoundary: "wording guard only; not legal review, certification, compliance, authorization, approval, or risk acceptance",
	}
	if strings.TrimSpace(path) == "" || strings.Contains(path, "://") {
		return report, errors.New("documentation claim guard requires a local file")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".txt" {
		return report, errors.New("documentation claim guard accepts only .md or .txt files")
	}
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		return report, errors.New("read documentation claim file")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return report, errors.New("documentation claim file must be a non-symlink regular file")
	}
	if info.Size() > maxDocumentationClaimFileSize {
		return report, fmt.Errorf("documentation claim file exceeds %d bytes", maxDocumentationClaimFileSize)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return report, errors.New("read documentation claim file")
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	report.LinesChecked = len(lines)
	for index, line := range lines {
		if documentationClaimStatePattern.FindStringIndex(line) == nil {
			continue
		}
		schemeClaim := documentationClaimSchemePattern.FindStringIndex(line) != nil
		productClaim := documentationProductPattern.FindStringIndex(line) != nil && documentationHighClaimPattern.FindStringIndex(line) != nil
		if !schemeClaim && !productClaim {
			continue
		}
		if explicitDocumentationNonClaim(line) {
			continue
		}
		return report, fmt.Errorf("unsupported certification or regulatory assertion at line %d", index+1)
	}
	report.Valid = true
	return report, nil
}

func explicitDocumentationNonClaim(line string) bool {
	states := documentationClaimStatePattern.FindAllStringIndex(line, -1)
	if len(states) == 0 {
		return false
	}
	for _, state := range states {
		prefix := line[:state[0]]
		if boundaries := documentationClauseBoundary.FindAllStringIndex(prefix, -1); len(boundaries) > 0 {
			prefix = prefix[boundaries[len(boundaries)-1][1]:]
		}
		if directDocumentationNegation(prefix) {
			continue
		}
		if documentationProhibitedWording.FindStringIndex(line[state[1]:]) != nil {
			continue
		}
		return false
	}
	return true
}

func directDocumentationNegation(prefix string) bool {
	matches := documentationDirectNegation.FindAllStringIndex(prefix, -1)
	if len(matches) == 0 {
		return false
	}
	last := matches[len(matches)-1]
	tail := strings.NewReplacer(
		"\"", " ", "'", " ", "`", " ", "“", " ", "”", " ", "‘", " ", "’", " ",
		"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
		"/", " ", "-", " ", "_", " ", "*", " ", "#", " ", ">", " ",
	).Replace(prefix[last[1]:])
	tokens := strings.Fields(tail)
	if len(tokens) > 8 {
		return false
	}
	for _, token := range tokens {
		if !documentationClaimContextToken.MatchString(token) {
			return false
		}
	}
	return true
}
