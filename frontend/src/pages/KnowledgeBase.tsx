import { Layout } from '@/components/Layout';
import { useState, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import { ragAPI } from '@/services/api';
import type { RAGSource } from '@/services/api';

interface Message {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  sources?: RAGSource[];
  elapsedMs?: number;
}

const EXAMPLE_QUESTIONS = [
  'Признаки фибрилляции предсердий на ЭКГ',
  'Как отличить желудочковую тахикардию от наджелудочковой?',
  'Критерии гипертрофии левого желудочка',
  'Признаки инфаркта миокарда на ЭКГ',
  'АВ-блокады: виды и ЭКГ-признаки',
];

export function KnowledgeBase() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const nextId = useRef(1);

  const scrollToBottom = () => {
    setTimeout(() => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }), 100);
  };

  const handleSubmit = async (question: string) => {
    if (!question.trim() || isLoading) return;

    const userMsg: Message = { id: nextId.current++, role: 'user', content: question.trim() };
    setMessages((prev) => [...prev, userMsg]);
    setInput('');
    setError(null);
    setIsLoading(true);
    scrollToBottom();

    try {
      const res = await ragAPI.query(question.trim());
      const assistantMsg: Message = {
        id: nextId.current++,
        role: 'assistant',
        content: res.answer,
        sources: res.sources,
        elapsedMs: res.elapsed_ms,
      };
      setMessages((prev) => [...prev, assistantMsg]);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Ошибка при получении ответа';
      setError(msg);
    } finally {
      setIsLoading(false);
      scrollToBottom();
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(input);
    }
  };

  return (
    <Layout>
      <div className="px-4 sm:px-0 flex flex-col" style={{ height: 'calc(100vh - 10rem)' }}>
        <div className="mb-4">
          <h1 className="text-3xl font-bold text-gray-900 mb-1">База знаний</h1>
          <p className="text-gray-500 text-sm">
            Задайте вопрос по ЭКГ и кардиологии — ответ формируется на основе медицинской литературы
          </p>
        </div>

        {/* Messages area */}
        <div className="flex-1 overflow-y-auto bg-white rounded-lg shadow border border-gray-200 p-4 mb-4 space-y-4">
          {messages.length === 0 && !isLoading && (
            <div className="flex flex-col items-center justify-center h-full text-center">
              <p className="text-gray-400 mb-6">Задайте вопрос по ЭКГ или выберите из примеров:</p>
              <div className="flex flex-wrap gap-2 justify-center max-w-2xl">
                {EXAMPLE_QUESTIONS.map((q) => (
                  <button
                    key={q}
                    onClick={() => handleSubmit(q)}
                    className="px-3 py-2 text-sm bg-blue-50 text-blue-700 rounded-lg hover:bg-blue-100 border border-blue-200 transition-colors text-left"
                  >
                    {q}
                  </button>
                ))}
              </div>
            </div>
          )}

          {messages.map((msg) => (
            <div key={msg.id} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              <div
                className={`max-w-3xl rounded-lg px-4 py-3 ${
                  msg.role === 'user'
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-50 border border-gray-200 text-gray-900'
                }`}
              >
                {msg.role === 'user' ? (
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                ) : (
                  <>
                    <ReactMarkdown className="prose prose-sm max-w-none prose-gray">
                      {msg.content}
                    </ReactMarkdown>
                    {msg.sources && msg.sources.length > 0 && (
                      <details className="mt-3 pt-3 border-t border-gray-200">
                        <summary className="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
                          Источники ({msg.sources.length})
                          {msg.elapsedMs && <span className="ml-2">| {(msg.elapsedMs / 1000).toFixed(1)}s</span>}
                        </summary>
                        <ul className="mt-2 space-y-1">
                          {msg.sources.map((src, i) => (
                            <li key={i} className="text-xs text-gray-500">
                              <span className="font-medium text-gray-700">{src.doc_name}</span>
                              <span className="text-gray-400"> #{src.chunk_index}</span>
                              <span className="text-gray-400 ml-1">(score: {src.score.toFixed(3)})</span>
                              <p className="text-gray-400 truncate">{src.preview}</p>
                            </li>
                          ))}
                        </ul>
                      </details>
                    )}
                  </>
                )}
              </div>
            </div>
          ))}

          {isLoading && (
            <div className="flex justify-start">
              <div className="bg-gray-50 border border-gray-200 rounded-lg px-4 py-3">
                <div className="flex items-center space-x-2 text-gray-400">
                  <div className="flex space-x-1">
                    <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                    <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                    <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                  </div>
                  <span className="text-sm">Поиск по базе знаний...</span>
                </div>
              </div>
            </div>
          )}

          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <div className="flex gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Задайте вопрос по ЭКГ..."
            rows={1}
            disabled={isLoading}
            className="flex-1 resize-none rounded-lg border border-gray-300 px-4 py-3 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50 disabled:bg-gray-50"
          />
          <button
            onClick={() => handleSubmit(input)}
            disabled={!input.trim() || isLoading}
            className="px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isLoading ? '...' : 'Спросить'}
          </button>
        </div>
      </div>
    </Layout>
  );
}
