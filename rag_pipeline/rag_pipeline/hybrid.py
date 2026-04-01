from dataclasses import dataclass
from typing import Any

from sentence_transformers import SentenceTransformer


def rrf_fusion(
    ranked_lists: list[list[str]],
    k: float = 60.0,
    weights: list[float] | None = None,
) -> dict[str, float]:
    if weights is None:
        weights = [1.0] * len(ranked_lists)
    if len(weights) != len(ranked_lists):
        raise ValueError("weights length must match ranked_lists length")

    scores: dict[str, float] = {}
    for lst, w in zip(ranked_lists, weights, strict=True):
        for rank, doc_id in enumerate(lst, start=1):
            scores[doc_id] = scores.get(doc_id, 0.0) + (w / (k + rank))
    return scores

@dataclass
class HybridSearchResult:
    ids: list[str]
    combined_scores: list[float]
    vector_scores: dict[str, float]
    bm25_scores: dict[str, float]
    documents: list[str]
    metadatas: list[dict[str, Any]]

class HybridSearchEngine:
    def __init__(
        self,
        collection,
        embed_model: SentenceTransformer,
        bm25,
        rrf_k: float = 60.0,
        w_vector: float = 1.0,
        w_bm25: float = 1.0,
    ):
        self.collection = collection
        self.embed_model = embed_model
        self.bm25 = bm25
        self.rrf_k = rrf_k
        self.w_vector = w_vector
        self.w_bm25 = w_bm25

    def _embed_query(self, query: str) -> list[float]:
        q = f"query: {query}"
        emb = self.embed_model.encode([q], normalize_embeddings=True)[0]
        return emb.tolist()

    def search(
        self,
        query: str,
        n_results: int = 5,
        vector_k: int = 30,
        bm25_k: int = 30,
    ) -> HybridSearchResult:
        # Vector search
        q_emb = self._embed_query(query)
        vec = self.collection.query(
            query_embeddings=[q_emb],
            n_results=vector_k,
            include=["documents", "metadatas", "distances"],
        )
        vec_ids = vec["ids"][0]
        vec_dist = vec.get("distances", [[None] * len(vec_ids)])[0]

        vec_scores: dict[str, float] = {}
        for i, doc_id in enumerate(vec_ids):
            d = vec_dist[i]
            if d is None:
                continue
            vec_scores[doc_id] = 1.0 - float(d)  # cosine sim

        # BM25 search
        bm = self.bm25.search(query, top_k=bm25_k)
        bm_ids = [doc_id for doc_id, _ in bm]
        bm_scores = dict(bm)

        # RRF fuse
        fused = rrf_fusion(
            ranked_lists=[vec_ids, bm_ids],
            k=self.rrf_k,
            weights=[self.w_vector, self.w_bm25],
        )
        top = sorted(fused.items(), key=lambda x: x[1], reverse=True)
        fused_sorted = top[:n_results]
        final_ids = [doc_id for doc_id, _ in fused_sorted]
        final_scores = [score for _, score in fused_sorted]

        got = self.collection.get(
            ids=final_ids,
            include=["documents", "metadatas"],
        )
        id_to_doc = dict(zip(got["ids"], got["documents"], strict=True))
        id_to_meta = dict(zip(got["ids"], got["metadatas"], strict=True))

        documents = [id_to_doc[i] for i in final_ids]
        metadatas = [id_to_meta[i] for i in final_ids]

        return HybridSearchResult(
            ids=final_ids,
            combined_scores=final_scores,
            vector_scores=vec_scores,
            bm25_scores=bm_scores,
            documents=documents,
            metadatas=metadatas,
        )
