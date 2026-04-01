import pytest
from rag_pipeline.bm25 import BM25Index
from rag_pipeline.tokenization import medical_ru_tokenizer


class TestBM25Index:
    def make_index(self, docs=None):
        idx = BM25Index(tokenizer=medical_ru_tokenizer)
        if docs is None:
            docs = {
                "doc1": "Фибрилляция предсердий характеризуется отсутствием зубцов P",
                "doc2": "Инфаркт миокарда проявляется элевацией сегмента ST",
                "doc3": "Нормальный синусовый ритм с зубцом P перед QRS",
                "doc4": "Гипертрофия левого желудочка по критериям Соколова-Лайона",
                "doc5": "Блокада правой ножки пучка Гиса с широким QRS",
            }
        ids = list(docs.keys())
        texts = list(docs.values())
        idx.build(ids, texts)
        return idx

    def test_build_and_search(self):
        idx = self.make_index()
        results = idx.search("фибрилляция предсердий")
        assert len(results) > 0
        top_id, top_score = results[0]
        assert top_id == "doc1"
        assert top_score > 0

    def test_search_returns_ranked(self):
        idx = self.make_index()
        results = idx.search("зубец P синусовый ритм")
        ids = [r[0] for r in results]
        # doc3 ("Нормальный синусовый ритм с зубцом P") should rank high
        assert "doc3" in ids[:2]

    def test_top_k_limits_results(self):
        idx = self.make_index()
        results = idx.search("ЭКГ", top_k=2)
        assert len(results) == 2

    def test_search_before_build_raises(self):
        idx = BM25Index(tokenizer=medical_ru_tokenizer)
        with pytest.raises(RuntimeError):
            idx.search("test")

    def test_mismatched_ids_documents_raises(self):
        idx = BM25Index(tokenizer=medical_ru_tokenizer)
        with pytest.raises(ValueError):
            idx.build(["a", "b"], ["only one doc"])

    def test_ids_property(self):
        idx = self.make_index()
        assert len(idx.ids) == 5
        assert "doc1" in idx.ids

    def test_all_scores_non_negative(self):
        idx = self.make_index()
        results = idx.search("элевация ST инфаркт")
        for _, score in results:
            assert score >= 0

    def test_unrelated_query_low_scores(self):
        idx = self.make_index()
        results = idx.search("компьютерная томография головного мозга")
        # All scores should be relatively low (no exact match)
        if results:
            top_score = results[0][1]
            related = idx.search("фибрилляция предсердий")
            related_top = related[0][1]
            assert top_score < related_top


class TestBM25WithHybridRRF:
    """Test the rrf_fusion function from hybrid module."""

    @pytest.fixture(autouse=True)
    def _skip_if_no_sentence_transformers(self):
        pytest.importorskip("sentence_transformers", reason="sentence_transformers not installed")

    def test_rrf_basic(self):
        from rag_pipeline.hybrid import rrf_fusion

        fused = rrf_fusion(
            ranked_lists=[["a", "b", "c"], ["b", "c", "a"]],
            k=60.0,
        )
        assert "a" in fused
        assert "b" in fused
        assert "c" in fused
        # "b" is rank 2 in list 1 and rank 1 in list 2 → should have high score
        # "a" is rank 1 in list 1 and rank 3 in list 2
        # Both should score reasonably
        assert fused["b"] > 0

    def test_rrf_with_weights(self):
        from rag_pipeline.hybrid import rrf_fusion

        # Heavily weight the first list
        fused = rrf_fusion(
            ranked_lists=[["a", "b"], ["b", "a"]],
            k=60.0,
            weights=[10.0, 1.0],
        )
        # "a" is rank 1 in the heavily weighted list
        assert fused["a"] > fused["b"]

    def test_rrf_mismatched_weights_raises(self):
        from rag_pipeline.hybrid import rrf_fusion

        with pytest.raises(ValueError):
            rrf_fusion(ranked_lists=[["a"], ["b"]], weights=[1.0])

    def test_rrf_empty_lists(self):
        from rag_pipeline.hybrid import rrf_fusion

        fused = rrf_fusion(ranked_lists=[[], []])
        assert fused == {}

    def test_rrf_single_list(self):
        from rag_pipeline.hybrid import rrf_fusion

        fused = rrf_fusion(ranked_lists=[["x", "y", "z"]])
        assert len(fused) == 3
        # Rank 1 should score highest
        assert fused["x"] > fused["y"] > fused["z"]
