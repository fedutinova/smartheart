import pytest

langchain = pytest.importorskip("langchain_core", reason="langchain_core not installed")
from rag_pipeline.generation import build_prompt, retrieve_context, format_response


class TestBuildPrompt:
    def test_has_required_variables(self):
        prompt = build_prompt()
        assert "context" in prompt.input_variables
        assert "question" in prompt.input_variables

    def test_template_contains_rules(self):
        prompt = build_prompt()
        assert "врач функциональной диагностики" in prompt.template
        assert "Тип А" in prompt.template
        assert "Тип Б" in prompt.template

    def test_template_contains_format(self):
        prompt = build_prompt()
        assert "Markdown" in prompt.template
        assert "200–400 слов" in prompt.template


class TestFormatResponse:
    def test_basic_format(self):
        items = [
            {
                "id": "chunk_1",
                "combined": 0.1234,
                "vector": 0.5,
                "bm25": 0.3,
                "doc": "Текст документа " * 20,
                "meta": {"doc_name": "cardio.pdf", "chunk_index": 5},
            },
        ]
        result = format_response("Что такое ФП?", "Фибрилляция предсердий — это...", items)
        assert "Что такое ФП?" in result
        assert "Фибрилляция предсердий" in result
        assert "cardio.pdf#5" in result
        assert "0.1234" in result

    def test_max_sources_limit(self):
        items = [
            {
                "id": f"chunk_{i}",
                "combined": 0.1,
                "vector": 0.05,
                "bm25": 0.05,
                "doc": f"doc {i}",
                "meta": {"doc_name": "test.pdf", "chunk_index": i},
            }
            for i in range(10)
        ]
        result = format_response("q", "a", items, max_sources=3)
        # Should only have 3 source entries
        assert result.count("test.pdf#") == 3

    def test_none_scores(self):
        items = [
            {
                "id": "chunk_1",
                "combined": 0.1,
                "vector": None,
                "bm25": None,
                "doc": "text",
                "meta": {"doc_name": "doc.pdf", "chunk_index": 0},
            }
        ]
        result = format_response("q", "a", items)
        assert "n/a" in result

    def test_missing_meta_fields(self):
        items = [
            {
                "id": "x",
                "combined": 0.1,
                "vector": 0.1,
                "bm25": 0.1,
                "doc": "text",
                "meta": {},
            }
        ]
        result = format_response("q", "a", items)
        assert "unknown_doc" in result
