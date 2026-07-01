package dashboard

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const dashboardTimingSlowThreshold = 250 * time.Millisecond

type dashboardTiming struct {
	route  string
	path   string
	start  time.Time
	blocks []dashboardTimingBlock
}

type dashboardTimingBlock struct {
	Name     string
	Duration time.Duration
	Detail   string
}

func newDashboardTiming(route string, r *http.Request) *dashboardTiming {
	path := ""
	if r != nil && r.URL != nil {
		path = r.URL.Path
	}
	return &dashboardTiming{route: route, path: path, start: time.Now()}
}

func (t *dashboardTiming) step(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	t.add(name, time.Since(start), "")
	return err
}

func (t *dashboardTiming) add(name string, duration time.Duration, detail string) {
	if t == nil {
		return
	}
	t.blocks = append(t.blocks, dashboardTimingBlock{Name: name, Duration: duration, Detail: detail})
}

func (t *dashboardTiming) logIfSlow() {
	if t == nil {
		return
	}
	total := time.Since(t.start)
	if total < dashboardTimingSlowThreshold {
		return
	}
	var b strings.Builder
	b.WriteString("dashboard_timing")
	b.WriteString(" route=")
	b.WriteString(t.route)
	if t.path != "" {
		b.WriteString(" path=")
		b.WriteString(t.path)
	}
	b.WriteString(" total_ms=")
	b.WriteString(formatDurationMS(total))
	for _, block := range t.blocks {
		b.WriteString(" block=")
		b.WriteString(block.Name)
		b.WriteString(":")
		b.WriteString(formatDurationMS(block.Duration))
		b.WriteString("ms")
		if block.Detail != "" {
			b.WriteString("(")
			b.WriteString(sanitizeTimingDetail(block.Detail))
			b.WriteString(")")
		}
	}
	log.Print(b.String())
}

func formatDurationMS(duration time.Duration) string {
	return strings.TrimRight(strings.TrimRight(formatFloat(float64(duration)/float64(time.Millisecond)), "0"), ".")
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func sanitizeTimingDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	detail = strings.ReplaceAll(detail, " ", "_")
	detail = strings.ReplaceAll(detail, "\t", "_")
	detail = strings.ReplaceAll(detail, "\n", "_")
	return detail
}
