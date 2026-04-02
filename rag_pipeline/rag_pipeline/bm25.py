import logging
import pickle
from pathlib import Path

from rank_bm25 import BM25Okapi

logger = logging.getLogger(__name__)


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

    def save(self, path: str | Path) -> None:
        path = Path(path)
        data = {
            "ids": self._ids,
            "corpus_tokens": self._corpus_tokens,
        }
        path.parent.mkdir(parents=True, exist_ok=True)
        with open(path, "wb") as f:
            pickle.dump(data, f, protocol=pickle.HIGHEST_PROTOCOL)
        logger.info("BM25 index saved to %s (%d docs)", path, len(self._ids))

    def load(self, path: str | Path) -> bool:
        path = Path(path)
        if not path.exists():
            return False
        try:
            with open(path, "rb") as f:
                data = pickle.load(f)
            self._ids = data["ids"]
            self._corpus_tokens = data["corpus_tokens"]
            self._bm25 = BM25Okapi(self._corpus_tokens)
            logger.info("BM25 index loaded from %s (%d docs)", path, len(self._ids))
        except Exception:
            logger.warning("Failed to load BM25 index from %s, will rebuild", path)
            return False
        return True

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
