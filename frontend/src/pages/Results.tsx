import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import ReactMarkdown from 'react-markdown';
import { ecgFeedbackAPI, requestAPI } from '@/services/api';
import { ROUTES } from '@/config';
import { formatDate, formatStatus, getStatusColor, formatECGParams } from '@/utils/format';
import { Layout } from '@/components/Layout';
import { RequestImage } from '@/components/RequestImage';
import { ECGChat } from '@/components/ECGChat';
import { useEventSource } from '@/hooks/useEventSource';
import { usePendingJobs } from '@/hooks/usePendingJobs';
import type { ECGAnalysisResult, ECGFeedbackRating, ECGRhythmResult, ECGStructuredResult, InterpretationItem } from '@/types';

function fmt(v: number | null | undefined, decimals = 1): string {
  if (v == null) return '—';
  return v.toFixed(decimals);
}

export function Results() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { removeJob } = usePendingJobs();

  const onSSEEvent = useCallback(
    (evt: { request_id: string }) => {
      if (evt.request_id === id) {
        queryClient.invalidateQueries({ queryKey: ['request', id] });
      }
    },
    [id, queryClient],
  );
  useEventSource(onSSEEvent);

  const { data: request, isLoading, error } = useQuery({
    queryKey: ['request', id],
    queryFn: () => requestAPI.getRequest(id!),
    enabled: !!id,
    refetchInterval: (query) => {
      if (query.state.fetchStatus === 'paused' || query.state.status === 'error') return false;
      const data = query.state.data;
      if (!data) return false;
      if (data.status === 'pending' || data.status === 'processing') return 2000;
      if (data.response?.content) {
        try {
          const parsed = JSON.parse(data.response.content);
          if (parsed?.analysis_type === 'ekg_direct_v2' &&
              parsed.gpt_interpretation_status &&
              parsed.gpt_interpretation_status !== 'completed' &&
              parsed.gpt_interpretation_status !== 'failed') {
            return 3000;
          }
        } catch { /* not JSON, stop polling */ }
      }
      return false;
    },
  });

  useEffect(() => {
    if (id && (request?.status === 'completed' || request?.status === 'failed')) {
      removeJob(id);
    }
  }, [id, request?.status, removeJob]);

  let ecgResult: ECGAnalysisResult | null = null;
  let isStructured = false;
  if (request?.response?.content) {
    try {
      const parsed = JSON.parse(request.response.content);
      if (parsed?.analysis_type === 'ekg_direct_v2' || parsed?.analysis_type === 'ekg_structured_v1') {
        ecgResult = parsed as ECGAnalysisResult;
        isStructured = parsed.analysis_type === 'ekg_structured_v1';
      }
    } catch {
      // Not JSON — direct GPT text response
    }
  }

  const gptContent = ecgResult
    ? ecgResult.gpt_full_response || null
    : (request?.response && request.response.model !== 'ekg_direct_v2')
      ? request.response.content
      : null;

  if (isLoading) {
    return (
      <Layout>
        <div className="max-w-4xl mx-auto px-4 sm:px-6">
          <div className="animate-pulse">
            <div className="h-4 w-16 bg-gray-200 rounded mb-4" />
            <div className="h-7 w-56 bg-gray-200 rounded mb-6" />
            <div className="bg-white shadow rounded-lg p-6 mb-6">
              <div className="flex justify-between mb-3">
                <div className="h-5 w-24 bg-gray-200 rounded-full" />
                <div className="h-4 w-32 bg-gray-200 rounded" />
              </div>
              <div className="flex gap-2">
                <div className="h-7 w-20 bg-gray-200 rounded-md" />
                <div className="h-7 w-16 bg-gray-200 rounded-md" />
                <div className="h-7 w-24 bg-gray-200 rounded-md" />
              </div>
            </div>
            <div className="bg-white shadow rounded-lg p-6 mb-6">
              <div className="h-6 w-48 bg-gray-200 rounded mb-4" />
              <div className="h-48 bg-gray-200 rounded" />
            </div>
            <div className="bg-white shadow rounded-lg p-6 mb-6">
              <div className="h-6 w-52 bg-gray-200 rounded mb-4" />
              {[1,2,3,4,5,6].map(i => (
                <div key={i} className="flex justify-between py-2 border-b border-gray-100">
                  <div className="h-4 w-12 bg-gray-200 rounded" />
                  <div className="flex gap-4">
                    <div className="h-4 w-12 bg-gray-200 rounded" />
                    <div className="h-4 w-12 bg-gray-200 rounded" />
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </Layout>
    );
  }

  if (error || !request) {
    return (
      <Layout>
        <div className="max-w-md mx-auto text-center py-16 px-4">
          <p className="text-gray-500">Не удалось загрузить результат</p>
          <Link to={ROUTES.DASHBOARD} className="mt-4 inline-block text-sm text-rose-600 hover:underline">
            На главную
          </Link>
        </div>
      </Layout>
    );
  }

  if (request.status === 'failed') {
    return (
      <Layout>
        <div className="max-w-lg mx-auto px-4 py-16 text-center">
          <div className="w-14 h-14 rounded-full bg-red-100 flex items-center justify-center mx-auto mb-5">
            <svg className="w-7 h-7 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
          </div>
          <h1 className="text-xl font-semibold text-gray-900 mb-2">Не удалось выполнить анализ</h1>
          <p className="text-sm text-gray-500 mb-6">
            Сервис не смог обработать это изображение. Попробуйте загрузить другой файл.
          </p>
          <div className="rounded-xl border border-gray-200 bg-gray-50 p-4 text-left mb-8">
            <p className="text-xs font-medium text-gray-500 mb-2">Частые причины</p>
            <ul className="space-y-1.5 text-sm text-gray-600">
              <li className="flex items-start gap-2">
                <span className="mt-0.5 text-gray-400">·</span>
                Изображение нечёткое или сильно повёрнуто
              </li>
              <li className="flex items-start gap-2">
                <span className="mt-0.5 text-gray-400">·</span>
                На фото нет ЭКГ или плёнка обрезана
              </li>
              <li className="flex items-start gap-2">
                <span className="mt-0.5 text-gray-400">·</span>
                Файл повреждён или имеет неподдерживаемый формат
              </li>
            </ul>
          </div>
          <div className="flex flex-col sm:flex-row gap-3 justify-center">
            <Link
              to={ROUTES.ANALYZE}
              className="px-5 py-2.5 bg-rose-600 text-white text-sm font-medium rounded-xl hover:bg-rose-700 transition-colors"
            >
              Попробовать снова
            </Link>
            <Link
              to={ROUTES.DASHBOARD}
              className="px-5 py-2.5 bg-gray-100 text-gray-700 text-sm font-medium rounded-xl hover:bg-gray-200 transition-colors"
            >
              На главную
            </Link>
          </div>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-4xl mx-auto px-4 sm:px-6 animate-fade-in">
        <button
          onClick={() => navigate(-1)}
          className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900 transition-colors mb-4"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
          </svg>
          Назад
        </button>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Результаты анализа</h1>

        {/* Request Info */}
        <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <div className="flex items-center justify-between mb-3">
            <span
              className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(request.status)}`}
            >
              {formatStatus(request.status)}
            </span>
            <span className="text-xs text-gray-400">{formatDate(request.created_at)}</span>
          </div>
          {formatECGParams(request) && (
            <div className="flex flex-wrap gap-2">
              {request.ecg_sex && (
                <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-700">
                  {request.ecg_sex === 'male' ? 'Мужской' : 'Женский'}
                </span>
              )}
              {request.ecg_age && (
                <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-700">
                  {request.ecg_age} лет
                </span>
              )}
              {request.ecg_paper_speed_mms && (
                <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-700">
                  {request.ecg_paper_speed_mms} мм/с
                </span>
              )}
              {request.ecg_mm_per_mv_limb && (
                <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-700">
                  конечн. {request.ecg_mm_per_mv_limb} мм/мВ
                </span>
              )}
              {request.ecg_mm_per_mv_chest && (
                <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-gray-100 text-sm text-gray-700">
                  грудные {request.ecg_mm_per_mv_chest} мм/мВ
                </span>
              )}
            </div>
          )}
        </div>

        {/* Original Image */}
        {request.files && request.files.length > 0 && (
          <RequestImage requestId={request.id} fileId={request.files[0].id} />
        )}

        {/* Rhythm classification (cv_service). Shown above measurements
            because the rhythm answer is the first thing a clinician looks for. */}
        {ecgResult?.rhythm_result && id && (
          <RhythmResultView result={ecgResult.rhythm_result} requestId={id} />
        )}

        {/* Structured ECG Results */}
        {isStructured && ecgResult?.structured_result && (
          <StructuredResultView result={ecgResult.structured_result} />
        )}

        {/* GPT Interpretation / Analysis Result (old format) */}
        {!isStructured && gptContent && (
          <div className="bg-gradient-to-br from-purple-50 to-blue-50 border border-purple-200 shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
            <div className="flex items-center mb-3 sm:mb-4">
              <h2 className="text-lg sm:text-xl font-bold text-gray-900">Заключение</h2>
            </div>
            <div className="bg-white rounded-lg p-3 sm:p-4 border border-purple-100 mb-3 sm:mb-4">
              <ReactMarkdown className="prose prose-sm max-w-none prose-gray">
                {gptContent}
              </ReactMarkdown>
            </div>
          </div>
        )}

        {/* GPT interpretation pending/failed message for old EKG requests */}
        {!isStructured && ecgResult && !gptContent && ecgResult.gpt_request_id && (
          <div className="bg-yellow-50 border border-yellow-200 shadow rounded-lg p-6 mb-6">
            <h2 className="text-xl font-bold text-gray-900 mb-2">Заключение</h2>
            <p className="text-sm text-yellow-800">
              {ecgResult.gpt_interpretation_status === 'failed'
                ? 'GPT-интерпретация не удалась. Попробуйте повторить запрос.'
                : 'GPT-интерпретация в обработке...'}
            </p>
          </div>
        )}

        {/* Notes */}
        {ecgResult?.notes && (
          <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
            <h2 className="text-sm font-medium text-gray-400 mb-2">Примечания</h2>
            <p className="text-sm text-gray-600">{ecgResult.notes}</p>
          </div>
        )}

        {/* ECG-contextual chat */}
        {request?.status === 'completed' && id && (
          <ECGChat
            requestId={id}
            structuredResult={ecgResult?.structured_result}
            rhythmResult={ecgResult?.rhythm_result}
          />
        )}

        <p className="mt-6 text-xs text-gray-500 text-center leading-relaxed">
          Результаты анализа носят исключительно информационный характер, не являются медицинским заключением
          и не заменяют консультацию квалифицированного врача.
        </p>
      </div>
    </Layout>
  );
}

// --- Rhythm classifier (cv_service) + vision-LLM narrative ---

function RhythmResultView({ result, requestId }: { result: ECGRhythmResult; requestId: string }) {
  const exp = result.explanation;
  const copyText = exp
    ? [exp.prediction_line, exp.description_text, exp.conclusion_text].filter(Boolean).join('\n\n')
    : result.pred_label_ru;

  return (
    <div className="bg-gradient-to-br from-rose-50 to-orange-50 border border-rose-200 shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-lg font-bold text-gray-900">Заключение</h2>
        {copyText && <CopyButton text={copyText} />}
      </div>

      <div className="bg-white rounded-lg px-4 py-3 border border-rose-100 mb-4">
        <p className="text-xs text-gray-500 mb-1">Предполагаемый ритм</p>
        <p className="text-xl font-semibold text-gray-900">{result.pred_label_ru}</p>
        {exp?.prediction_line && (
          <p className="mt-2 text-sm text-gray-700">{exp.prediction_line}</p>
        )}
      </div>

      {exp ? (
        <div className="space-y-3 text-sm text-gray-800">
          {exp.description_text && (
            <p className="leading-relaxed whitespace-pre-line">{exp.description_text}</p>
          )}
          {exp.conclusion_text && (
            <div className="bg-white/70 border border-rose-100 rounded-md px-4 py-3">
              <p className="text-[11px] uppercase tracking-wide text-gray-500 mb-1">Заключение</p>
              <p className="leading-relaxed whitespace-pre-line">{exp.conclusion_text}</p>
            </div>
          )}
          {exp.note_text && (
            <p className="text-[11px] text-gray-500 leading-relaxed">{exp.note_text}</p>
          )}
        </div>
      ) : (
        <p className="text-xs text-gray-500">
          Текстовое заключение в этот раз не сформировано — показан только результат классификатора.
        </p>
      )}

      <FeedbackButtons requestId={requestId} />
    </div>
  );
}

const FEEDBACK_OPTIONS: { value: ECGFeedbackRating; label: string; emoji: string }[] = [
  { value: 'helpful',    label: 'Помогло',     emoji: '👍' },
  { value: 'inaccurate', label: 'Не точное',   emoji: '👎' },
  { value: 'unclear',    label: 'Непонятно',   emoji: '🤔' },
];

const COMMENT_MAX = 1000;

function FeedbackButtons({ requestId }: { requestId: string }) {
  const [current, setCurrent] = useState<ECGFeedbackRating | null>(null);
  const [savedComment, setSavedComment] = useState<string>('');
  const [commentDraft, setCommentDraft] = useState<string>('');
  const [pending, setPending] = useState<ECGFeedbackRating | 'comment' | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [commentSaved, setCommentSaved] = useState(false);

  useEffect(() => {
    let cancelled = false;
    ecgFeedbackAPI
      .get(requestId)
      .then((fb) => {
        if (cancelled || !fb) return;
        setCurrent(fb.rating);
        setSavedComment(fb.comment ?? '');
        setCommentDraft(fb.comment ?? '');
      })
      .catch(() => { /* silently fall back to "not voted" */ });
    return () => { cancelled = true; };
  }, [requestId]);

  const vote = async (rating: ECGFeedbackRating) => {
    if (pending) return;
    setPending(rating);
    setError(null);
    setCommentSaved(false);
    try {
      // Re-voting carries the existing comment only when the new rating is
      // still "inaccurate" — otherwise the comment loses context and we
      // drop it on the server side.
      const carryComment = rating === 'inaccurate' ? savedComment : '';
      const fb = await ecgFeedbackAPI.submit(requestId, rating, carryComment);
      setCurrent(fb.rating);
      setSavedComment(fb.comment ?? '');
      if (rating !== 'inaccurate') setCommentDraft('');
    } catch {
      setError('Не удалось сохранить оценку. Попробуйте ещё раз.');
    } finally {
      setPending(null);
    }
  };

  const saveComment = async () => {
    if (pending || current !== 'inaccurate') return;
    const trimmed = commentDraft.trim().slice(0, COMMENT_MAX);
    setPending('comment');
    setError(null);
    try {
      const fb = await ecgFeedbackAPI.submit(requestId, 'inaccurate', trimmed);
      setSavedComment(fb.comment ?? '');
      setCommentDraft(fb.comment ?? '');
      setCommentSaved(true);
      window.setTimeout(() => setCommentSaved(false), 2500);
    } catch {
      setError('Не удалось сохранить комментарий. Попробуйте ещё раз.');
    } finally {
      setPending(null);
    }
  };

  const commentDirty = current === 'inaccurate' && commentDraft.trim() !== savedComment;

  return (
    <div className="mt-5 pt-4 border-t border-rose-200">
      <p className="text-[11px] uppercase tracking-wide text-gray-600 mb-2">Оцените заключение</p>
      <div className="flex flex-wrap gap-2">
        {FEEDBACK_OPTIONS.map((opt) => {
          const isCurrent = current === opt.value;
          const isPending = pending === opt.value;
          return (
            <button
              key={opt.value}
              type="button"
              onClick={() => vote(opt.value)}
              disabled={!!pending}
              aria-pressed={isCurrent}
              className={`inline-flex items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-sm transition-colors ${
                isCurrent
                  ? 'bg-rose-500 border-rose-500 text-white shadow-sm'
                  : 'bg-white border-rose-200 text-gray-800 hover:border-rose-300 hover:bg-rose-50'
              } ${pending && !isPending ? 'opacity-50' : ''} ${pending ? 'cursor-wait' : ''}`}
            >
              <span aria-hidden>{opt.emoji}</span>
              <span>{opt.label}</span>
              {isPending && <span className="ml-1 text-xs">…</span>}
            </button>
          );
        })}
      </div>

      {current === 'inaccurate' && (
        <div className="mt-3 space-y-2">
          <label htmlFor="rhythm-feedback-comment" className="block text-xs text-gray-700">
            Что было неточно? <span className="text-gray-400">(необязательно — поможет улучшить модель)</span>
          </label>
          <textarea
            id="rhythm-feedback-comment"
            value={commentDraft}
            onChange={(e) => setCommentDraft(e.target.value.slice(0, COMMENT_MAX))}
            placeholder="Например: реальный ритм — синусовый; модель ошиблась с фибрилляцией."
            rows={3}
            disabled={pending === 'comment'}
            className="w-full rounded-lg border border-rose-200 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 shadow-sm outline-none focus:border-rose-300 focus:ring-4 focus:ring-rose-100 disabled:opacity-60"
          />
          <div className="flex items-center justify-between gap-3">
            <span className="text-[11px] text-gray-400">
              {commentDraft.length}/{COMMENT_MAX}
            </span>
            <div className="flex items-center gap-2">
              {commentSaved && (
                <span className="text-[11px] text-green-700">Сохранено</span>
              )}
              <button
                type="button"
                onClick={saveComment}
                disabled={!commentDirty || pending === 'comment'}
                className="rounded-lg bg-rose-500 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:bg-rose-600 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {pending === 'comment' ? 'Сохраняем…' : 'Сохранить'}
              </button>
            </div>
          </div>
        </div>
      )}

      {current && current !== 'inaccurate' && (
        <p className="mt-2 text-xs text-gray-600">Спасибо! Ваша оценка сохранена — её можно изменить в любой момент.</p>
      )}
      {error && (
        <p className="mt-2 text-xs text-red-600">{error}</p>
      )}
    </div>
  );
}

// --- Structured Result Components ---
//
// The vision-LLM narrative (RhythmResultView) is the main conclusion surface
// now, so the old "Интерпретация" card was reduced to its summary chips
// (axis classification + LVH/RVH presence). The per-lead R/S table and the
// per-criterion items list were dropped because they showed raw numbers we
// no longer want on screen — the underlying indices are still computed and
// used by the backend, just not visible to the user.

function StructuredResultView({ result }: { result: ECGStructuredResult }) {
  const hasMeasurements = Object.values(result.measurements).some((v) => v != null);
  const summary = result.interpretation?.summary ?? [];

  return (
    <>
      {!hasMeasurements && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <h2 className="text-lg font-bold text-gray-900 mb-2">Анализ ЭКГ</h2>
          <div className="text-sm text-yellow-800 space-y-2">
            <p>Система не смогла автоматически обработать изображение. Это может быть из-за:</p>
            <ul className="list-disc list-inside ml-2">
              <li>Низкого качества изображения или размытости</li>
              <li>Плохого контраста сетки ЭКГ</li>
              <li>Повреждённого или неполного изображения</li>
            </ul>
            <p className="mt-2">Рекомендации:</p>
            <ul className="list-disc list-inside ml-2">
              <li>Используйте высокое разрешение при сканировании</li>
              <li>Убедитесь что сетка четко видна</li>
              <li>Попробуйте загрузить заново</li>
            </ul>
          </div>
        </div>
      )}

      {/* Indices summary — axis classification, LVH / RVH presence. */}
      {summary.length > 0 && (
        <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <h2 className="text-lg font-bold text-gray-900 mb-3">Признаки</h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
            {summary.map((s, i) => (
              <SummaryCard key={i} item={s} />
            ))}
          </div>
        </div>
      )}

      {/* Rhythm & Intervals */}
      {result.rhythm && (result.rhythm.QRS_ms != null || result.rhythm.RR_ms != null || result.rhythm.HR_bpm != null) && (
        <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <h2 className="text-lg font-bold text-gray-900 mb-3">Интервалы и ритм</h2>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-center">
            <MetricCard label="QRS" value={fmt(result.rhythm.QRS_ms, 0)} unit="мс" />
            <MetricCard label="RR" value={fmt(result.rhythm.RR_ms, 0)} unit="мс" />
            <MetricCard label="ЧСС" value={fmt(result.rhythm.HR_bpm, 0)} unit="уд/мин" />
          </div>
        </div>
      )}

    </>
  );
}

function SummaryCard({ item }: { item: InterpretationItem }) {
  return (
    <div className="bg-gray-50 rounded-lg px-4 py-3 border border-gray-200 flex items-center justify-between gap-2">
      <div>
        <p className="text-xs text-gray-500">{item.label}</p>
        <p className="text-sm font-medium text-gray-900">{item.value}</p>
      </div>
      <StatusBadge status={item.status} />
    </div>
  );
}

const STATUS_STYLES: Record<string, string> = {
  positive: 'bg-red-100 text-red-700',
  abnormal: 'bg-red-100 text-red-700',
  negative: 'bg-green-100 text-green-700',
  normal: 'bg-green-100 text-green-700',
};

const STATUS_LABELS: Record<string, string> = {
  positive: 'есть',
  negative: 'нет',
  normal: 'норма',
  abnormal: 'отклонение',
};

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`text-xs px-1.5 py-0.5 rounded whitespace-nowrap ${STATUS_STYLES[status] ?? 'bg-gray-100 text-gray-600'}`}>
      {STATUS_LABELS[status] ?? status}
    </span>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current); }, []);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard API недоступен (например, non-HTTPS iframe)
    }
  };

  return (
    <button
      onClick={handleCopy}
      className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-purple-200 text-purple-700 hover:bg-purple-100 transition-colors"
    >
      {copied ? (
        <>
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
          </svg>
          Скопировано
        </>
      ) : (
        <>
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9.75a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
          </svg>
          Скопировать
        </>
      )}
    </button>
  );
}

function MetricCard({ label, value, unit }: { label: string; value: string; unit?: string }) {
  return (
    <div className="bg-gray-50 rounded-lg p-3 hover:bg-gray-100 transition-colors duration-150">
      <p className="text-xs text-gray-500 mb-1">{label}</p>
      <p className="text-lg font-semibold text-gray-900">
        {value}
        {unit && <span className="text-sm font-normal text-gray-500 ml-1">{unit}</span>}
      </p>
    </div>
  );
}

