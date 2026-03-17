import { Layout } from '@/components/Layout';
import { useState, useRef, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import { ragAPI } from '@/services/api';
import type { RAGSource, RAGQueryMeta } from '@/services/api';

interface Message {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  question?: string;
  sources?: RAGSource[];
  elapsedMs?: number;
  meta?: RAGQueryMeta;
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
  const [feedbackGiven, setFeedbackGiven] = useState<Record<number, -1 | 1>>({});
  const pendingFeedback = useRef(new Set<number>());
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
        question: question.trim(),
        sources: res.sources,
        elapsedMs: res.elapsed_ms,
        meta: res.meta,
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

  const handleFeedback = useCallback(async (msg: Message, rating: -1 | 1) => {
    if (!msg.question || feedbackGiven[msg.id] !== undefined || pendingFeedback.current.has(msg.id)) return;
    pendingFeedback.current.add(msg.id);
    try {
      await ragAPI.submitFeedback(msg.question, msg.content, rating);
      setFeedbackGiven((prev) => ({ ...prev, [msg.id]: rating }));
    } catch {
      // silently ignore
    } finally {
      pendingFeedback.current.delete(msg.id);
    }
  }, [feedbackGiven]);

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
                    <div className="mt-2 flex items-center gap-1">
                      {feedbackGiven[msg.id] !== undefined ? (
                        <span className="text-xs text-gray-400">
                          {feedbackGiven[msg.id] === 1 ? 'Спасибо за отзыв!' : 'Спасибо, учтём'}
                        </span>
                      ) : (
                        <>
                          <button
                            onClick={() => handleFeedback(msg, 1)}
                            className="p-1 text-gray-400 hover:text-green-600 transition-colors"
                            title="Полезный ответ"
                          >
                            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6.633 10.25c.806 0 1.533-.446 2.031-1.08a9.041 9.041 0 0 1 2.861-2.4c.723-.384 1.35-.956 1.653-1.715a4.498 4.498 0 0 0 .322-1.672V3a.75.75 0 0 1 .75-.75 2.25 2.25 0 0 1 2.25 2.25c0 1.152-.26 2.243-.723 3.218-.266.558.107 1.282.725 1.282m0 0h3.126c1.026 0 1.945.694 2.054 1.715.045.422.068.85.068 1.285a11.95 11.95 0 0 1-2.649 7.521c-.388.482-.987.729-1.605.729H13.48c-.483 0-.964-.078-1.423-.23l-3.114-1.04a4.501 4.501 0 0 0-1.423-.23H5.904m10.598-9.75H14.25M5.904 18.5c.083.205.173.405.27.602.197.4-.078.898-.523.898h-.908c-.889 0-1.713-.518-1.972-1.368a12 12 0 0 1-.521-3.507c0-1.553.295-3.036.831-4.398C3.387 9.953 4.167 9.5 5 9.5h1.053c.472 0 .745.556.5.96a8.958 8.958 0 0 0-1.302 4.665c0 1.194.232 2.333.654 3.375Z" />
                            </svg>
                          </button>
                          <button
                            onClick={() => handleFeedback(msg, -1)}
                            className="p-1 text-gray-400 hover:text-red-600 transition-colors"
                            title="Неполезный ответ"
                          >
                            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M7.498 15.25H4.372c-1.026 0-1.945-.694-2.054-1.715a12.137 12.137 0 0 1-.068-1.285c0-2.848.992-5.464 2.649-7.521C5.287 4.247 5.886 4 6.504 4h4.016a4.5 4.5 0 0 1 1.423.23l3.114 1.04a4.5 4.5 0 0 0 1.423.23h1.294M7.498 15.25c.618 0 .991.724.725 1.282A7.471 7.471 0 0 0 7.5 19.75 2.25 2.25 0 0 0 9.75 22a.75.75 0 0 0 .75-.75v-.633c0-.573.11-1.14.322-1.672.304-.76.93-1.33 1.653-1.715a9.04 9.04 0 0 0 2.86-2.4c.498-.634 1.226-1.08 2.032-1.08h.384m-10.253 1.5H9.7m8.075-9.75c.01.05.027.1.05.148.593 1.2.925 2.55.925 3.977 0 1.31-.269 2.559-.754 3.695-.124.291.023.654.34.71a.757.757 0 0 0 .888-.524 12.098 12.098 0 0 0 .526-3.506c0-1.553-.295-3.036-.831-4.398A2.204 2.204 0 0 0 17 9.5h-1.053c-.472 0-.745-.556-.5-.96a8.95 8.95 0 0 1 .303-.54" />
                            </svg>
                          </button>
                        </>
                      )}
                    </div>
                    {msg.sources && msg.sources.length > 0 && (
                      <details className="mt-3 pt-3 border-t border-gray-200">
                        <summary className="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
                          Источники ({msg.sources.length})
                          {msg.elapsedMs && <span className="ml-2">| {(msg.elapsedMs / 1000).toFixed(1)}s</span>}
                          {msg.meta && <span className="ml-2">| {msg.meta.model}, t={msg.meta.temperature}</span>}
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
