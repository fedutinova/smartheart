import { useState, useEffect, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import { ecgChatAPI, type ECGChatMessage as ApiMessage } from '@/services/api';
import type { ECGStructuredResult } from '@/types';

interface ECGChatProps {
  requestId: string;
  structuredResult?: ECGStructuredResult | null;
}

function buildSuggestions(result?: ECGStructuredResult | null): string[] {
  const suggestions: string[] = [];
  const interpretationItems = result?.interpretation?.items ?? [];

  const hasLVH = interpretationItems.some((it) => it.group === 'lvh' && it.status === 'positive');
  const hasRVH = interpretationItems.some((it) => it.group === 'rvh' && it.status === 'positive');
  const hasRhythm = interpretationItems.some((it) => it.group === 'rhythm' && it.status === 'abnormal');
  const hasAbnormalAxis = result?.axis_qrs?.classification && result.axis_qrs.classification !== 'normal';

  if (hasLVH) suggestions.push('Что такое гипертрофия левого желудочка?');
  if (hasRVH) suggestions.push('Что означает гипертрофия правого желудочка?');
  if (hasRhythm) suggestions.push('Опасно ли нарушение ритма на моей ЭКГ?');
  if (hasAbnormalAxis) suggestions.push('Что значит отклонение оси ЭКГ?');

  if (suggestions.length < 3) suggestions.push('Что такое индекс Соколова-Лайона?');
  if (result?.rhythm?.HR_bpm && suggestions.length < 4) suggestions.push('Какая ЧСС считается нормой в покое?');
  if (suggestions.length < 4) suggestions.push('Объясните основные параметры ЭКГ');

  return suggestions.slice(0, 4);
}

export function ECGChat({ requestId, structuredResult }: ECGChatProps) {
  const [messages, setMessages] = useState<ApiMessage[]>([]);
  const [input, setInput] = useState('');
  const [isThinking, setIsThinking] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showDisclaimer, setShowDisclaimer] = useState(true);
  const [thinkingSeconds, setThinkingSeconds] = useState(0);


  const suggestions = useMemo(() => buildSuggestions(structuredResult), [structuredResult]);

  const refreshHistory = async () => {
    setError(null);
    setIsLoading(true);
    try {
      const history = await ecgChatAPI.getMessages(requestId);
      setMessages(history);
    } catch {
      setError('Не удалось загрузить историю чата.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    ecgChatAPI
      .getMessages(requestId)
      .then((history) => {
        if (!cancelled) {
          setMessages(history);
          setError(null);
        }
      })
      .catch(() => {
        if (!cancelled) setError('Не удалось загрузить историю чата.');
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [requestId]);


  // Tick a counter while we're waiting for the assistant so the UI can
  // surface progressively reassuring messages on slow RAG calls.
  useEffect(() => {
    if (!isThinking) {
      setThinkingSeconds(0);
      return;
    }
    const interval = setInterval(() => {
      setThinkingSeconds((s) => s + 1);
    }, 1000);
    return () => clearInterval(interval);
  }, [isThinking]);

  const sendMessage = async (text: string) => {
    const trimmed = text.trim();
    if (!trimmed || isThinking) return;

    const optimisticUserMsg: ApiMessage = {
      id: `pending-${Date.now()}`,
      request_id: requestId,
      user_id: '',
      role: 'user',
      content: trimmed,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, optimisticUserMsg]);
    setInput('');
    setIsThinking(true);
    setError(null);

    try {
      const reply = await ecgChatAPI.sendMessage(requestId, trimmed);
      setMessages((prev) => [...prev, reply]);
    } catch (err: unknown) {
      const isTimeout = err instanceof Error && err.message.includes('timeout');
      setError(
        isTimeout
          ? 'Ответ ещё готовится — нажмите «Обновить», он появится в истории.'
          : 'Не удалось отправить сообщение. Попробуйте ещё раз.',
      );
      setMessages((prev) => prev.filter((m) => m.id !== optimisticUserMsg.id));
    } finally {
      setIsThinking(false);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    sendMessage(input);
  };

  return (
    <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center">
            <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
            </svg>
          </div>
          <div>
            <h2 className="text-lg font-bold text-gray-900">Обсудить эту ЭКГ</h2>
            <p className="text-xs text-gray-500">Задайте вопрос — ассистент ответит с учётом ваших результатов</p>
          </div>
        </div>
      </div>

      {showDisclaimer && messages.length === 0 && !isLoading && (
        <div className="bg-amber-50 border border-amber-200 rounded-lg px-3 py-2 mb-3 text-xs text-amber-800 flex items-start justify-between gap-2">
          <span>
            Ассистент использует базу знаний и результаты ЭКГ для образовательных ответов. Это не медицинская консультация.
          </span>
          <button
            onClick={() => setShowDisclaimer(false)}
            className="text-amber-700 hover:text-amber-900 text-base leading-none"
            aria-label="Скрыть"
          >
            ×
          </button>
        </div>
      )}

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg px-3 py-2 mb-3 text-xs text-red-800 flex items-start justify-between gap-2">
          <span className="flex-1">{error}</span>
          <button
            onClick={refreshHistory}
            disabled={isLoading}
            className="shrink-0 px-2 py-0.5 rounded border border-red-300 hover:bg-red-100 disabled:opacity-50 text-red-800 font-medium"
          >
            Обновить
          </button>
        </div>
      )}

      <div className="space-y-3 max-h-[480px] overflow-y-auto pr-1 mb-3">
        {isLoading && (
          <div className="text-center py-8 text-sm text-gray-400">Загрузка истории...</div>
        )}

        {!isLoading && messages.length === 0 && (
          <div className="text-center py-8">
            <p className="text-sm text-gray-500 mb-4">С чего начать?</p>
            <div className="flex flex-wrap gap-2 justify-center">
              {suggestions.map((s) => (
                <button
                  key={s}
                  onClick={() => sendMessage(s)}
                  disabled={isThinking}
                  className="px-3 py-1.5 rounded-full bg-purple-50 hover:bg-purple-100 disabled:opacity-50 border border-purple-200 text-xs sm:text-sm text-purple-800 transition-colors"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map((msg) => (
          <MessageBubble key={msg.id} message={msg} />
        ))}

        {isThinking && (
          <div className="flex items-start gap-2">
            <div className="w-7 h-7 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex-shrink-0 flex items-center justify-center">
              <svg className="w-3.5 h-3.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375" />
              </svg>
            </div>
            <div className="flex flex-col gap-1">
              <div className="bg-gray-100 rounded-2xl rounded-tl-sm px-4 py-3 self-start">
                <div className="flex gap-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-gray-400 animate-bounce" style={{ animationDelay: '0ms' }} />
                  <span className="w-1.5 h-1.5 rounded-full bg-gray-400 animate-bounce" style={{ animationDelay: '150ms' }} />
                  <span className="w-1.5 h-1.5 rounded-full bg-gray-400 animate-bounce" style={{ animationDelay: '300ms' }} />
                </div>
              </div>
              {thinkingSeconds >= 3 && thinkingSeconds < 15 && (
                <p className="text-xs text-gray-500 italic">
                  Обрабатываем запрос... обычно это занимает до 30 секунд.
                </p>
              )}
              {thinkingSeconds >= 15 && (
                <p className="text-xs text-amber-700 italic">
                  Запрос обрабатывается дольше обычного. Не закрывайте страницу — ответ скоро появится.
                </p>
              )}
            </div>
          </div>
        )}

      </div>

      <form onSubmit={handleSubmit} className="flex gap-2 items-end">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              sendMessage(input);
            }
          }}
          placeholder="Задайте вопрос про вашу ЭКГ..."
          rows={1}
          disabled={isLoading}
          className="flex-1 resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 focus:border-transparent disabled:bg-gray-50"
        />
        <button
          type="submit"
          disabled={!input.trim() || isThinking || isLoading}
          className="px-4 py-2 rounded-lg bg-gradient-to-r from-purple-600 to-blue-600 text-white text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
          </svg>
        </button>
      </form>
    </div>
  );
}

function MessageBubble({ message }: { message: ApiMessage }) {
  const isUser = message.role === 'user';
  const time = new Date(message.created_at);

  return (
    <div className={`flex items-start gap-2 ${isUser ? 'flex-row-reverse' : ''}`}>
      <div
        className={`w-7 h-7 rounded-full flex-shrink-0 flex items-center justify-center ${
          isUser ? 'bg-gray-200' : 'bg-gradient-to-br from-purple-500 to-blue-500'
        }`}
      >
        {isUser ? (
          <svg className="w-4 h-4 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
          </svg>
        ) : (
          <svg className="w-3.5 h-3.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09Z" />
          </svg>
        )}
      </div>

      <div className={`max-w-[85%] ${isUser ? 'items-end' : 'items-start'} flex flex-col`}>
        <div
          className={`px-4 py-2.5 rounded-2xl text-sm ${
            isUser
              ? 'bg-purple-600 text-white rounded-tr-sm'
              : 'bg-gray-100 text-gray-900 rounded-tl-sm'
          }`}
        >
          {isUser ? (
            <p className="whitespace-pre-wrap">{message.content}</p>
          ) : (
            <ReactMarkdown className="prose prose-sm max-w-none prose-gray prose-p:my-1 prose-ul:my-1.5">
              {message.content}
            </ReactMarkdown>
          )}
        </div>

        <p className="text-[10px] text-gray-400 mt-1">
          {time.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
        </p>
      </div>
    </div>
  );
}
