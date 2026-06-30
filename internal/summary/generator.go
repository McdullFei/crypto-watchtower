package summary

import (
	"context"
	"fmt"
	"strings"
)

const defaultDisclaimer = "不构成投资建议"

// Generator produces market summary text from a bounded snapshot.
//
// Author: monsterfei
// Date: 2026-06-30
type Generator interface {
	Generate(context.Context, Snapshot) (string, error)
}

// TemplateGenerator creates deterministic local market summaries.
//
// Author: monsterfei
// Date: 2026-06-30
type TemplateGenerator struct {
	Disclaimer string
}

// NewTemplateGenerator creates a deterministic summary generator.
//
// Author: monsterfei
// Date: 2026-06-30
// @param disclaimer Required risk disclaimer text.
// @returns Template generator instance.
func NewTemplateGenerator(disclaimer string) TemplateGenerator {
	if disclaimer == "" {
		disclaimer = defaultDisclaimer
	}
	return TemplateGenerator{Disclaimer: disclaimer}
}

// Generate renders a deterministic summary from a bounded snapshot.
//
// Author: monsterfei
// Date: 2026-06-30
// @param ctx Request context.
// @param snapshot Bounded market snapshot.
// @returns Summary text containing the configured disclaimer.
func (g TemplateGenerator) Generate(ctx context.Context, snapshot Snapshot) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("市场异动摘要（%s - %s）\n", snapshot.WindowFrom.Format("2006-01-02 15:04"), snapshot.WindowTo.Format("2006-01-02 15:04")))
	builder.WriteString(fmt.Sprintf("alerts=%d, events=%d, funding=%d\n", snapshot.AlertCount, snapshot.EventCount, snapshot.FundingCount))
	if len(snapshot.Alerts) > 0 {
		builder.WriteString("重点告警：\n")
		for _, alert := range snapshot.Alerts {
			builder.WriteString(fmt.Sprintf("- %s %s %s: %s\n", alert.Exchange, alert.Symbol, alert.Type, alert.Title))
		}
	}
	if len(snapshot.Events) > 0 {
		builder.WriteString("样本事件：\n")
		for _, event := range snapshot.Events {
			builder.WriteString(fmt.Sprintf("- %s %s %s notional=%.2f\n", event.Exchange, event.Symbol, event.EventType, event.Notional))
		}
	}
	builder.WriteString(g.Disclaimer)
	return builder.String(), nil
}
