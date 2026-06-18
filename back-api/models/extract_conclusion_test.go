package models

import (
	"strings"
	"testing"
)

func TestExtractConclusion_AlreadyStructured(t *testing.T) {
	in := "1. Пункт один\n2. Пункт два"
	out := ExtractConclusion(in)
	if out != in {
		t.Fatalf("expected unchanged structured text, got %q", out)
	}
}

func TestExtractConclusion_FindsConclusionHeader(t *testing.T) {
	in := "## Введение\nТекст\n\n### Заключение\n1. Итог\n2. Рекомендация\n\nИнтерпретация носит информационный характер"
	out := ExtractConclusion(in)
	exp := "1. Итог\n2. Рекомендация"
	if out != exp {
		t.Fatalf("expected %q, got %q", exp, out)
	}
}

func TestExtractConclusion_FindsSummaryHeader(t *testing.T) {
	in := "## Ритм\nТекст\n\n## Итог\n- Первый вывод\n- Второй вывод\n\n## Уверенность\nСредняя"
	out := ExtractConclusion(in)
	exp := "- Первый вывод\n- Второй вывод"
	if out != exp {
		t.Fatalf("expected %q, got %q", exp, out)
	}
}

func TestExtractConclusion_NoMarkerReturnsTrimmed(t *testing.T) {
	in := "   Просто текст без маркера   "
	out := ExtractConclusion(in)
	exp := "Просто текст без маркера"
	if out != exp {
		t.Fatalf("expected %q, got %q", exp, out)
	}
}

func TestWithECGDisclaimer_AppendsOnce(t *testing.T) {
	in := "## Итог\n- Синусовая морфология без блокад."
	out := WithECGDisclaimer(in)
	if !strings.Contains(out, ecgDisclaimerMarker) {
		t.Fatalf("disclaimer not appended: %q", out)
	}
	if !strings.HasPrefix(out, in) {
		t.Fatalf("original interpretation must be preserved, got %q", out)
	}
	// Idempotent: a second call must not duplicate the disclaimer.
	twice := WithECGDisclaimer(out)
	if strings.Count(twice, ecgDisclaimerMarker) != 1 {
		t.Fatalf("disclaimer duplicated on second call: %q", twice)
	}
}

func TestWithECGDisclaimer_EmptyUnchanged(t *testing.T) {
	if out := WithECGDisclaimer("   \n"); strings.Contains(out, ecgDisclaimerMarker) {
		t.Fatalf("empty interpretation must not get a disclaimer, got %q", out)
	}
}
