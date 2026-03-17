from typing import List, Tuple, Optional
from rank_bm25 import BM25Okapi

class BM25Index:
    def __init__(self, tokenizer):
        self.tokenizer = tokenizer
        self._bm25: Optional[BM25Okapi] = None
        self._ids: List[str] = []
        self._corpus_tokens: List[List[str]] = []

    @property
    def ids(self) -> List[str]:
        return self._ids

    def build(self, ids: List[str], documents: List[str]) -> None:
        if len(ids) != len(documents):
            raise ValueError("ids and documents must have the same length")
        self._ids = list(ids)
        self._corpus_tokens = [self.tokenizer(d) for d in documents]
        self._bm25 = BM25Okapi(self._corpus_tokens)

    def search(self, query: str, top_k: int = 20) -> List[Tuple[str, float]]:
        if self._bm25 is None:
            raise RuntimeError("BM25Index не построен. Вызовите build().")
        q = self.tokenizer(query)
        scores = self._bm25.get_scores(q)
        idxs = sorted(range(len(self._ids)), key=lambda i: float(scores[i]), reverse=True)[:top_k]
        return [(self._ids[i], float(scores[i])) for i in idxs]
