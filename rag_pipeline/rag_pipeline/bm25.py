
from rank_bm25 import BM25Okapi


class BM25Index:
    def __init__(self, tokenizer):
        self.tokenizer = tokenizer
        self._bm25: BM25Okapi | None = None
        self._ids: list[str] = []
        self._corpus_tokens: list[list[str]] = []

    @property
    def ids(self) -> list[str]:
        return self._ids

    def build(self, ids: list[str], documents: list[str]) -> None:
        if len(ids) != len(documents):
            raise ValueError("ids and documents must have the same length")
        self._ids = list(ids)
        self._corpus_tokens = [self.tokenizer(d) for d in documents]
        self._bm25 = BM25Okapi(self._corpus_tokens)

    def search(self, query: str, top_k: int = 20) -> list[tuple[str, float]]:
        if self._bm25 is None:
            raise RuntimeError("BM25Index не построен. Вызовите build().")
        q = self.tokenizer(query)
        scores = self._bm25.get_scores(q)
        ranked = sorted(
            range(len(self._ids)),
            key=lambda i: float(scores[i]),
            reverse=True,
        )
        idxs = ranked[:top_k]
        return [(self._ids[i], float(scores[i])) for i in idxs]
