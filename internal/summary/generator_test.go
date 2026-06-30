package summary

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/renfei198727/crypto-watchtower/internal/model"
)

// TestTemplateGeneratorIncludesDisclaimer verifies deterministic summaries include risk text.
//
// Author: monsterfei
// Date: 2026-06-30
func TestTemplateGeneratorIncludesDisclaimer(t *testing.T) {
	generator := NewTemplateGenerator("不构成投资建议")

	content, err := generator.Generate(context.Background(), Snapshot{
		WindowFrom: time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
		WindowTo:   time.Date(2026, 6, 29, 12, 15, 0, 0, time.UTC),
		AlertCount: 1,
		EventCount: 1,
	})
	if err != nil {
		t.Fatalf("generate template summary: %v", err)
	}
	if !strings.Contains(content, "不构成投资建议") {
		t.Fatalf("expected disclaimer in content: %s", content)
	}
}

// TestTemplateGeneratorIncludesFundingContext verifies funding events are visible to operators.
//
// Author: monsterfei
// Date: 2026-06-30
func TestTemplateGeneratorIncludesFundingContext(t *testing.T) {
	generator := NewTemplateGenerator("不构成投资建议")

	content, err := generator.Generate(context.Background(), Snapshot{
		WindowFrom:   time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
		WindowTo:     time.Date(2026, 6, 29, 12, 15, 0, 0, time.UTC),
		EventCount:   1,
		FundingCount: 1,
		Events: []model.MarketEvent{
			{Exchange: "okx", Symbol: "ETHUSDT", EventType: "funding", EventTime: time.Date(2026, 6, 29, 12, 14, 0, 0, time.UTC)},
		},
	})
	if err != nil {
		t.Fatalf("generate template summary: %v", err)
	}
	if !strings.Contains(strings.ToLower(content), "funding") {
		t.Fatalf("expected funding context in content: %s", content)
	}
}
