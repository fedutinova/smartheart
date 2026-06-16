import type { RAGSource, ECGChatCitation } from '@/services/api';

// Technical suffixes baked into the source PDF filenames that should never be
// shown to users (page-range exports, "diagnosis-only" excerpts).
const TECHNICAL_SUFFIXES: RegExp[] = [
  /\s*-?\s*страницы[\s-]удалены\s*$/i,
  /\s*-?\s*только\s+диагностика\s*$/i,
];

/**
 * prettySourceName turns a raw RAG document filename into a human-readable
 * title. Examples:
 *   "КР Гипертрофическая  кардиомиопатия-только диагностика.pdf"
 *     → "Клинические рекомендации: Гипертрофическая кардиомиопатия"
 *   "И.Ю. Зудбинов Азбука ЭКГ-страницы-удалены.pdf"
 *     → "И.Ю. Зудбинов Азбука ЭКГ"
 */
export function prettySourceName(docName: string): string {
  let name = (docName ?? '').trim();
  if (!name) return '';

  // Drop the file extension.
  name = name.replace(/\.(pdf|txt|docx?|md|html?|rtf)$/i, '');
  // Strip technical suffixes.
  for (const re of TECHNICAL_SUFFIXES) {
    name = name.replace(re, '');
  }
  // Trim trailing punctuation/dashes/dots left behind by the suffix removal.
  name = name.replace(/[\s.\-–—]+$/u, '').trim();
  // Expand the "КР" guideline prefix into a readable label.
  name = name.replace(/^КР\s+/u, 'Клинические рекомендации: ');
  // Collapse repeated whitespace.
  name = name.replace(/\s{2,}/gu, ' ').trim();

  return name || docName;
}

// dedupePretty prettifies each raw filename and returns the unique titles,
// preserving the (relevance-ranked) order of first appearance.
function dedupePretty(rawNames: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of rawNames) {
    const name = prettySourceName(raw);
    if (name && !seen.has(name)) {
      seen.add(name);
      out.push(name);
    }
  }
  return out;
}

/**
 * uniqueSourceNames returns deduplicated pretty document titles for RAG
 * knowledge-base sources. A single query often returns several chunks from the
 * same document, so each document is shown once.
 */
export function uniqueSourceNames(sources: RAGSource[] | undefined): string[] {
  return dedupePretty((sources ?? []).map((s) => s.doc_name));
}

/**
 * uniqueCitationNames is the same as uniqueSourceNames for ECG-chat citations,
 * whose document filename lives in the `title` field.
 */
export function uniqueCitationNames(citations: ECGChatCitation[] | undefined): string[] {
  return dedupePretty((citations ?? []).map((c) => c.title));
}
