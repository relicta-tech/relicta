package memory

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

func BenchmarkMnemosRecordRelease(b *testing.B) {
	store := &eventStore{}
	srv := httptest.NewServer(store)
	defer srv.Close()

	adapter := NewMnemosStore(srv.URL+"/v1", "bench-run", nil)

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
