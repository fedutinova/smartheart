import { describe, it, expect } from 'vitest';
import { prettySourceName, uniqueSourceNames, uniqueCitationNames } from '../sourceName';
import type { RAGSource, ECGChatCitation } from '@/services/api';

describe('prettySourceName', () => {
  it('expands КР guideline prefix and drops "только диагностика"', () => {
    expect(prettySourceName('КР Гипертрофическая  кардиомиопатия-только диагностика.pdf'))
      .toBe('Клинические рекомендации: Гипертрофическая кардиомиопатия');
    expect(prettySourceName('КР стабильная ишемическая болезнь сердца - только диагностика.pdf'))
      .toBe('Клинические рекомендации: стабильная ишемическая болезнь сердца');
    expect(prettySourceName('КР Перикардиты только диагностика.pdf'))
      .toBe('Клинические рекомендации: Перикардиты');
  });

  it('strips "страницы-удалены" and trailing punctuation, keeps author', () => {
    expect(prettySourceName('И.Ю. Зудбинов Азбука ЭКГ-страницы-удалены.pdf'))
      .toBe('И.Ю. Зудбинов Азбука ЭКГ');
    expect(prettySourceName('Суворов А. В. Клиническая электрокардиография. -страницы-удалены.pdf'))
      .toBe('Суворов А. В. Клиническая электрокардиография');
    expect(prettySourceName('Шлык Н.И. Сердечный ритм и тип регуляции у детей, подростков и спортсменов.-страницы-удалены.pdf'))
      .toBe('Шлык Н.И. Сердечный ритм и тип регуляции у детей, подростков и спортсменов');
  });

  it('handles empty / unknown gracefully', () => {
    expect(prettySourceName('')).toBe('');
    expect(prettySourceName('unknown')).toBe('unknown');
  });
});

describe('uniqueSourceNames', () => {
  it('dedupes by document and preserves order', () => {
    const sources: RAGSource[] = [
      { doc_name: 'КР ФП и ТП-только диагностика.pdf', chunk_index: 1, score: 0.9, preview: '' },
      { doc_name: 'КР ФП и ТП-только диагностика.pdf', chunk_index: 4, score: 0.8, preview: '' },
      { doc_name: 'И.Ю. Зудбинов Азбука ЭКГ-страницы-удалены.pdf', chunk_index: 2, score: 0.7, preview: '' },
    ];
    expect(uniqueSourceNames(sources)).toEqual([
      'Клинические рекомендации: ФП и ТП',
      'И.Ю. Зудбинов Азбука ЭКГ',
    ]);
  });

  it('returns [] for empty/undefined', () => {
    expect(uniqueSourceNames(undefined)).toEqual([]);
    expect(uniqueSourceNames([])).toEqual([]);
  });
});

describe('uniqueCitationNames', () => {
  it('prettifies and dedupes ECG-chat citations (filename in title)', () => {
    const citations: ECGChatCitation[] = [
      { title: 'Орлов В Н Руководство по электрокардиографии -страницы-удалены.pdf', source: 'Чанк 12 (релевантность 0.91)', excerpt: '' },
      { title: 'Орлов В Н Руководство по электрокардиографии -страницы-удалены.pdf', source: 'Чанк 7 (релевантность 0.84)', excerpt: '' },
      { title: 'КР Желудочковые тахикардии-только диагностика.pdf', source: 'Чанк 3 (релевантность 0.80)', excerpt: '' },
    ];
    expect(uniqueCitationNames(citations)).toEqual([
      'Орлов В Н Руководство по электрокардиографии',
      'Клинические рекомендации: Желудочковые тахикардии',
    ]);
  });

  it('returns [] for empty/undefined', () => {
    expect(uniqueCitationNames(undefined)).toEqual([]);
    expect(uniqueCitationNames([])).toEqual([]);
  });
});
