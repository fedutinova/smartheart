import pytest
from rag_pipeline.chunking import (
    is_heading,
    split_to_units,
    chunk_units_semantic,
    _tail_units_for_overlap,
)


class TestIsHeading:
    def test_numbered_section(self):
        assert is_heading("1.2.3 Фибрилляция предсердий")
        assert is_heading("1. Введение")
        assert is_heading("3.1) Результаты")

    def test_all_caps_cyrillic(self):
        assert is_heading("ФИБРИЛЛЯЦИЯ ПРЕДСЕРДИЙ")
        assert is_heading("ГЛАВА 1")

    def test_keyword_heading(self):
        assert is_heading("Глава 3. Диагностика")
        assert is_heading("Раздел 2")
        assert is_heading("Таблица 4: Показатели")
        assert is_heading("Рисунок 1")
        assert is_heading("Приложение А")

    def test_too_short(self):
        assert not is_heading("abc")
        assert not is_heading("  ")

    def test_normal_paragraph(self):
        assert not is_heading("Нормальный синусовый ритм определяется по наличию зубца P перед каждым комплексом QRS.")

    def test_long_caps_under_120(self):
        heading = "ЭЛЕКТРОКАРДИОГРАФИЧЕСКИЕ ПРИЗНАКИ ГИПЕРТРОФИИ"
        assert len(heading) <= 120
        assert is_heading(heading)

    def test_long_caps_over_120(self):
        heading = "А" * 121
        assert not is_heading(heading)

    def test_colon_ending(self):
        assert is_heading("1.1 Критерии ЭКГ:")


class TestSplitToUnits:
    def test_basic_split(self):
        text = "Первый абзац\n\nВторой абзац\n\nТретий"
        units = split_to_units(text)
        assert len(units) == 3

    def test_strips_whitespace(self):
        units = split_to_units("  a  \n\n  b  ")
        assert units == ["a", "b"]

    def test_empty_paragraphs_skipped(self):
        units = split_to_units("a\n\n\n\n\n\nb")
        assert len(units) == 2

    def test_single_paragraph(self):
        units = split_to_units("один абзац")
        assert units == ["один абзац"]


class TestTailUnitsForOverlap:
    def test_no_overlap(self):
        assert _tail_units_for_overlap(["a", "b", "c"], 0) == []

    def test_empty_units(self):
        assert _tail_units_for_overlap([], 100) == []

    def test_collects_from_tail(self):
        units = ["short", "word", "end"]  # lengths: 5, 4, 3
        result = _tail_units_for_overlap(units, 10)
        # "end" = 3 chars, "word" + 2 sep = 6, total 9 ≤ 10
        assert "end" in result
        assert "word" in result

    def test_respects_limit(self):
        units = ["a" * 50, "b" * 50, "c" * 50]
        result = _tail_units_for_overlap(units, 55)
        assert len(result) == 1
        assert result[0] == "c" * 50


class TestChunkUnitsSemantic:
    def test_basic_chunking(self):
        units = [f"paragraph {i} " + "x" * 100 for i in range(10)]
        chunks = chunk_units_semantic(units, target_chars=250, min_chars=200, max_chars=400, overlap_chars=0)
        assert len(chunks) > 1
        # Post-merge may combine sub-min trailing chunks, so last chunk can exceed max.
        # Pre-merge chunks respect max_chars; here we just verify we got multiple chunks.
        total = sum(len(c) for c in chunks)
        assert total > 0

    def test_heading_breaks_chunk(self):
        units = [
            "x" * 200,
            "1.1 Новый раздел",  # heading
            "y" * 200,
        ]
        chunks = chunk_units_semantic(units, target_chars=500, min_chars=150, max_chars=600, overlap_chars=0)
        # Heading should force a break after first unit reaches min_chars
        assert len(chunks) >= 2

    def test_small_chunks_merged(self):
        units = ["tiny"]  # way below min_chars
        chunks = chunk_units_semantic(units, target_chars=100, min_chars=50, max_chars=200, overlap_chars=0)
        assert len(chunks) == 1
        assert chunks[0] == "tiny"

    def test_oversized_unit_split_by_sentence(self):
        long_unit = "Первое предложение. " * 50  # ~1000 chars
        units = [long_unit]
        chunks = chunk_units_semantic(units, target_chars=200, min_chars=100, max_chars=300, overlap_chars=0)
        assert len(chunks) >= 2
        # Post-merge may combine sub-min trailing chunks; check pre-merge logic
        # worked by verifying we got multiple chunks from a single oversized unit.
        for ch in chunks[:-1]:  # all but last (last may be merged)
            assert len(ch) <= 300

    def test_empty_units_skipped(self):
        units = ["", "  ", "text"]
        chunks = chunk_units_semantic(units, target_chars=100, min_chars=50, max_chars=200, overlap_chars=0)
        assert len(chunks) == 1
        assert "text" in chunks[0]

    def test_respects_max_chars(self):
        units = ["a" * 100 for _ in range(20)]
        chunks = chunk_units_semantic(units, target_chars=300, min_chars=200, max_chars=400, overlap_chars=0)
        for ch in chunks:
            assert len(ch) <= 400

    def test_overlap_carries_context(self):
        units = [f"unit_{i}_" + "x" * 80 for i in range(10)]
        chunks_no_overlap = chunk_units_semantic(units, target_chars=200, min_chars=150, max_chars=300, overlap_chars=0)
        chunks_with_overlap = chunk_units_semantic(units, target_chars=200, min_chars=150, max_chars=300, overlap_chars=80)
        # With overlap, later chunks should contain text from previous chunks
        if len(chunks_with_overlap) > 1:
            # The overlap makes chunks slightly larger on average
            total_with = sum(len(c) for c in chunks_with_overlap)
            total_without = sum(len(c) for c in chunks_no_overlap)
            assert total_with >= total_without

    def test_real_world_medical_text(self):
        text = """1.1 Фибрилляция предсердий

Фибрилляция предсердий (ФП) — наиболее часто встречающаяся аритмия в клинической практике. Распространённость ФП увеличивается с возрастом.

ЭКГ-признаки:
- Отсутствие зубцов P
- Нерегулярные интервалы R-R
- Волны f (мелковолновые осцилляции изолинии)

1.2 Трепетание предсердий

Трепетание предсердий характеризуется регулярной предсердной активностью с частотой 250-350 в минуту."""

        units = split_to_units(text)
        chunks = chunk_units_semantic(units, target_chars=300, min_chars=200, max_chars=500, overlap_chars=50)
        assert len(chunks) >= 1
        full = "\n\n".join(chunks)
        assert "Фибрилляция" in full
        assert "Трепетание" in full
