package evidencemodel

import (
	"strings"

	"github.com/subashram/fairway/internal/store"
)

const (
	KindScreenshot   = "screenshot"
	KindVideo        = "video"
	KindBrowserTrace = "browser-trace"
	KindUAT          = "uat"
)

type UXMediaEvidence struct {
	Kind              string         `json:"kind"`
	ArtifactType      string         `json:"artifact_type"`
	ArtifactPath      string         `json:"artifact_path,omitempty"`
	Result            string         `json:"result,omitempty"`
	CommandText       string         `json:"command_text,omitempty"`
	Notes             string         `json:"notes,omitempty"`
	RedactionRequired bool           `json:"redaction_required"`
	Boundary          string         `json:"boundary"`
	Evidence          store.Evidence `json:"evidence"`
}

type UXMediaSummary struct {
	Screenshots   int  `json:"screenshots"`
	Videos        int  `json:"videos"`
	BrowserTraces int  `json:"browser_traces"`
	UAT           int  `json:"uat"`
	Total         int  `json:"total"`
	Exercised     bool `json:"exercised"`
}

func UXMediaRows(evidence []store.Evidence) []UXMediaEvidence {
	var rows []UXMediaEvidence
	for _, ev := range evidence {
		kind, ok := UXMediaKind(ev.ArtifactType)
		if !ok {
			continue
		}
		rows = append(rows, UXMediaEvidence{
			Kind:              kind,
			ArtifactType:      strings.TrimSpace(ev.ArtifactType),
			ArtifactPath:      strings.TrimSpace(ev.ArtifactPath),
			Result:            strings.TrimSpace(ev.Result),
			CommandText:       strings.TrimSpace(ev.CommandText),
			Notes:             strings.TrimSpace(ev.Notes),
			RedactionRequired: true,
			Boundary:          "store references and redacted summaries only; do not store raw secrets, auth tokens, provider-private transcripts, or unredacted user data",
			Evidence:          ev,
		})
	}
	return rows
}

func UXMediaSummaryFor(rows []UXMediaEvidence) UXMediaSummary {
	summary := UXMediaSummary{Total: len(rows), Exercised: len(rows) > 0}
	for _, row := range rows {
		switch row.Kind {
		case KindScreenshot:
			summary.Screenshots++
		case KindVideo:
			summary.Videos++
		case KindBrowserTrace:
			summary.BrowserTraces++
		case KindUAT:
			summary.UAT++
		}
	}
	return summary
}

func UXMediaKind(artifactType string) (string, bool) {
	switch normalizeArtifactType(artifactType) {
	case "screenshot", "screen-shot", "image-proof", "visual-proof":
		return KindScreenshot, true
	case "video", "screen-recording", "recording":
		return KindVideo, true
	case "browser-trace", "playwright-trace", "trace":
		return KindBrowserTrace, true
	case "uat", "uat-proof", "uat-artifact", "owner-usage-proof":
		return KindUAT, true
	default:
		return "", false
	}
}

func normalizeArtifactType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
