package http

import "testing"

func TestExtractConclusion_AlreadyStructured(t *testing.T) {
	in := "1. Пункт один\n2. Пункт два"
	out := extractConclusion(in)
	if out != in {
		t.Fatalf("expected unchanged structured text, got %q", out)
	}
}

func TestExtractConclusion_FindsConclusionHeader(t *testing.T) {
	in := "## Введение\nТекст\n\n### Заключение\n1. Итог\n2. Рекомендация\n\nИнтерпретация носит информационный характер"
	out := extractConclusion(in)
	exp := "1. Итог\n2. Рекомендация"
	if out != exp {
		t.Fatalf("expected %q, got %q", exp, out)
	}
}

func TestExtractConclusion_NoMarkerReturnsTrimmed(t *testing.T) {
	in := "   Просто текст без маркера   "
	out := extractConclusion(in)
	exp := "Просто текст без маркера"
	if out != exp {
		t.Fatalf("expected %q, got %q", exp, out)
	}
}
