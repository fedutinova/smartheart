package repository

import "testing"

func TestHasContradiction(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
		cached   string
		want     bool
	}{
		// Antonym pairs — must veto
		{
			name:     "left vs right ventricle",
			incoming: "какие критерии гипертрофии правого желудочка на экг",
			cached:   "какие критерии гипертрофии левого желудочка на экг",
			want:     true,
		},
		{
			name:     "right vs left bundle branch",
			incoming: "как выглядит блокада правой ножки пучка гиса на экг",
			cached:   "как выглядит блокада левой ножки пучка гиса на экг",
			want:     true,
		},
		{
			name:     "QT shortening vs prolongation",
			incoming: "что означает укорочение интервала qt и какие нормы",
			cached:   "что означает удлинение интервала qt и какие нормы",
			want:     true,
		},
		{
			name:     "NSTEMI vs STEMI",
			incoming: "каковы экг-признаки инфаркта миокарда без подъёма сегмента st",
			cached:   "каковы экг-признаки инфаркта миокарда с подъёмом сегмента st",
			want:     true,
		},
		{
			name:     "hypokalemia vs hyperkalemia",
			incoming: "какие экг-изменения характерны для гипокалиемии",
			cached:   "какие экг-отклонения могут быть при гиперкалиемии",
			want:     true,
		},
		{
			name:     "tachycardia vs bradycardia",
			incoming: "как отличить синусовую тахикардию от желудочковой",
			cached:   "синусовая брадикардия и атриовентрикулярная блокада",
			want:     true,
		},
		{
			name:     "atrial fibrillation vs flutter",
			incoming: "признаки трепетания предсердий на экг",
			cached:   "экг-признаки фибрилляции предсердий",
			want:     true,
		},
		// Reformulations of same topic — must not veto
		{
			name:     "same topic rephrased",
			incoming: "как понять по экг что у пациента фибрилляция предсердий",
			cached:   "экг-признаки мерцательной аритмии",
			want:     false,
		},
		{
			name:     "synonym phrasing left ventricle",
			incoming: "как определить гипертрофию левого желудочка по электрокардиограмме",
			cached:   "какие критерии гипертрофии левого желудочка на экг",
			want:     false,
		},
		{
			name:     "QT prolongation rephrased",
			incoming: "какой интервал qt считается удлинённым",
			cached:   "что означает удлинение интервала qt и какие нормы",
			want:     false,
		},
		// Edge: both terms appear in the same question (ambiguous, should not veto)
		{
			name:     "both terms in incoming — no veto",
			incoming: "чем отличается блокада левой ножки от правой ножки пучка гиса",
			cached:   "как выглядит блокада левой ножки пучка гиса на экг",
			want:     false,
		},
		{
			name:     "both terms in cached — no veto",
			incoming: "как выглядит блокада левой ножки пучка гиса на экг",
			cached:   "чем отличается блокада левой ножки от правой ножки пучка гиса",
			want:     false,
		},
		// T wave polarity
		{
			name:     "peaked T vs negative T",
			incoming: "как интерпретировать высокие заострённые зубцы t",
			cached:   "как интерпретировать отрицательные зубцы t в грудных отведениях",
			want:     true,
		},
		// Unrelated questions — no veto
		{
			name:     "completely different topics",
			incoming: "что такое синдром бругада",
			cached:   "как рассчитать электрическую ось сердца по экг",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HasContradiction(tc.incoming, tc.cached)
			if got != tc.want {
				t.Errorf("HasContradiction(%q, %q) = %v, want %v",
					tc.incoming, tc.cached, got, tc.want)
			}
		})
	}
}
