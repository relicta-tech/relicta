package chronos

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

func BenchmarkChronosRecordRelease(b *testing.B) {
	stub := &stubChronos{}
	srv := httptest.NewServer(stub)
	defer srv.Close()

	adapter := NewChronosAdapter(srv.URL, "bench-scope")

	rec := &memory.ReleaseRecord{
		ID:         "bench-rel",
		Version:    "v0.0.1",
		Repository: "bench/repo",
		Actor:      cgp.Actor{ID: "bench", Kind: cgp.ActorKindHuman},
		RiskScore:  0.5,
		Decision:   cgp.DecisionApproved,
		Outcome:    memory.OutcomeSuccess,
		ReleasedAt: time.Now().UTC(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.RecordRelease(context.Background(), rec)
	}
}
