package handler

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestJudgeEquivalence_Integration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	h := &RAGHandler{judgeClient: openai.NewClient(apiKey)}

	cases := []struct {
		name     string
		incoming string
		cached   string
		want     bool
	}{
		// Should be equivalent (YES) — same topic rephrased
		{
			name:     "afib synonym",
			incoming: "как понять по экг что у пациента фибрилляция предсердий",
			cached:   "экг-признаки мерцательной аритмии",
			want:     true,
		},
		{
			name:     "qt prolongation rephrased",
			incoming: "какой интервал qt считается удлинённым",
			cached:   "что означает удлинение интервала qt и какие нормы",
			want:     true,
		},
		// Should NOT be equivalent (NO) — different named scoring systems
		{
			name:     "cornell vs sokolow-lyon",
			incoming: "какие критерии корнелла используются для диагностики глж",
			cached:   "что такое критерии соколова-лайона для гипертрофии",
			want:     false,
		},
		// Should NOT be equivalent (NO) — different diagnoses, same domain
		{
			name:     "pericarditis vs myocarditis",
			incoming: "какие экг-изменения характерны для перикардита",
			cached:   "какие экг-изменения характерны для миокардита",
			want:     false,
		},
		{
			name:     "WPW vs bundle branch block",
			incoming: "как проявляется синдром wpw на экг",
			cached:   "как выглядит блокада левой ножки пучка гиса на экг",
			want:     false,
		},
		{
			name:     "vfib vs vt",
			incoming: "как отличить фибрилляцию желудочков от желудочковой тахикардии",
			cached:   "каковы экг-признаки инфаркта миокарда с подъёмом сегмента st",
			want:     false,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := h.judgeEquivalence(ctx, tc.incoming, tc.cached)
			if err != nil {
				t.Fatalf("judgeEquivalence error: %v", err)
			}
			verdict := "NO"
			if got {
				verdict = "YES"
			}
			t.Logf("incoming: %q\ncached:   %q\njudge: %s (want %v, got %v)",
				tc.incoming, tc.cached, verdict, tc.want, got)
			if got != tc.want {
				t.Errorf("want %v, got %v", tc.want, got)
			}
		})
	}
}
