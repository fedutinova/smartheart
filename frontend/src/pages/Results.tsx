import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
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
import type { ECGAnalysisResult, ECGFeedbackRating, ECGRhythmResult, ECGStructuredResult, InterpretationItem, RequestStatus } from '@/types';

export function Results() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { removeJob } = usePendingJobs();
  const [imageAspectRatio, setImageAspectRatio] = useState<number | null>(null);

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
    ? isStructured
      ? ecgResult.gpt_interpretation || ecgResult.gpt_full_response || null
      : ecgResult.gpt_full_response || ecgResult.gpt_interpretation || null
    : (request?.response && request.response.model !== 'ekg_direct_v2')
      ? request.response.content
      : null;
  const firstFile = request?.files?.[0];
  const isWideImage = imageAspectRatio != null && imageAspectRatio >= 1.35;

  useEffect(() => {
    setImageAspectRatio(null);
  }, [firstFile?.id]);

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
      <div className="max-w-6xl mx-auto px-4 sm:px-6 animate-fade-in">
        <div className="mb-5 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <button
              onClick={() => navigate(-1)}
              className="mb-3 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900 transition-colors"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
              </svg>
              Назад
            </button>
            <h1 className="text-2xl font-semibold text-gray-950">Результаты анализа</h1>
            <p className="mt-1 text-sm text-gray-500">{formatDate(request.created_at)}</p>
          </div>
          <span
            className={`w-fit px-2.5 py-1 inline-flex text-xs leading-5 font-semibold rounded-md ${getStatusColor(request.status)}`}
          >
            {formatStatus(request.status)}
          </span>
        </div>

        <div className="mb-5 rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
          <div className="grid gap-3 p-3 sm:gap-4 sm:p-5 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.75fr)]">
            {/* Hide the rhythm badge when the classifier was unconfident and the
                GPT interpretation disagreed — defer to the interpretation text. */}
            {!ecgResult?.rhythm_result?.suppressed && (
              <RhythmResultView result={ecgResult?.rhythm_result ?? null} status={request.status} />
            )}

            {formatECGParams(request) && (
              <div className="rounded-lg bg-gray-50 p-3 sm:border sm:border-gray-200">
                <p className="mb-2 text-[11px] font-medium uppercase tracking-wide text-gray-500 sm:text-xs">Параметры записи</p>
                <div className="flex flex-wrap gap-1.5 sm:gap-2">
                  {request.ecg_sex && (
                    <RecordParamChip>{request.ecg_sex === 'male' ? 'Мужской' : 'Женский'}</RecordParamChip>
                  )}
                  {request.ecg_age && (
                    <RecordParamChip>{request.ecg_age} лет</RecordParamChip>
                  )}
                  {request.ecg_paper_speed_mms && (
                    <RecordParamChip>{request.ecg_paper_speed_mms} мм/с</RecordParamChip>
                  )}
                  {request.ecg_mm_per_mv_limb && (
                    <RecordParamChip>конечн. {request.ecg_mm_per_mv_limb} мм/мВ</RecordParamChip>
                  )}
                  {request.ecg_mm_per_mv_chest && (
                    <RecordParamChip>грудные {request.ecg_mm_per_mv_chest} мм/мВ</RecordParamChip>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>

        <div className={`grid gap-5 lg:items-start ${firstFile && !isWideImage ? 'lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.25fr)]' : ''}`}>
          {firstFile && (
            <div className={isWideImage ? '' : 'lg:sticky lg:top-20'}>
              {/* Original Image */}
              <RequestImage
                requestId={request.id}
                fileId={firstFile.id}
                onAspectRatioChange={setImageAspectRatio}
                compact={isWideImage}
              />
            </div>
          )}

          <div>
            {/* GPT Interpretation / Analysis Result */}
            {gptContent && (
              <div className="bg-gradient-to-br from-purple-50 to-blue-50 border border-purple-200 shadow-sm rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
                <div className="flex items-center mb-3 sm:mb-4">
                  <h2 className="text-lg sm:text-xl font-bold text-gray-900">
                    {isStructured ? 'Интерпретация ЭКГ' : 'Заключение'}
                  </h2>
                </div>
                <div className="bg-white rounded-lg p-3 sm:p-4 border border-purple-100 mb-3 sm:mb-4">
                  <ReactMarkdown className="prose prose-sm max-w-none prose-gray">
                    {gptContent}
                  </ReactMarkdown>
                </div>
              </div>
            )}

            {/* Structured ECG Results */}
            {isStructured && ecgResult?.structured_result && (
              <StructuredResultView result={ecgResult.structured_result} />
            )}

            {/* GPT interpretation pending/failed message for old EKG requests */}
            {!isStructured && ecgResult && !gptContent && ecgResult.gpt_request_id && (
              <div className="bg-yellow-50 border border-yellow-200 shadow-sm rounded-xl p-6 mb-6">
                <h2 className="text-xl font-bold text-gray-900 mb-2">Заключение</h2>
                <p className="text-sm text-yellow-800">
                  {ecgResult.gpt_interpretation_status === 'failed'
                    ? 'GPT-интерпретация не удалась. Попробуйте повторить запрос.'
                    : 'GPT-интерпретация в обработке...'}
                </p>
              </div>
            )}

            {isStructured && ecgResult && !gptContent && ecgResult.gpt_interpretation_status === 'failed' && (
              <div className="bg-yellow-50 border border-yellow-200 shadow-sm rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
                <h2 className="text-lg sm:text-xl font-bold text-gray-900 mb-2">Интерпретация ЭКГ</h2>
                <p className="text-sm text-yellow-800">
                  Интерпретация не удалась, ниже сохранены структурированные измерения.
                </p>
              </div>
            )}

            {/* Notes */}
            {ecgResult?.notes && (
              <div className="bg-white border border-gray-200 shadow-sm rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
                <h2 className="text-sm font-medium text-gray-400 mb-2">Примечания</h2>
                <p className="text-sm text-gray-600">{ecgResult.notes}</p>
              </div>
            )}

            {ecgResult?.rhythm_result && id && (
              <FeedbackButtons requestId={id} />
            )}
          </div>
        </div>

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

function RecordParamChip({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex max-w-full items-center rounded-md border border-gray-200 bg-white px-2 py-0.5 text-xs text-gray-700 sm:px-2.5 sm:py-1 sm:text-sm">
      {children}
    </span>
  );
}

function RhythmResultView({ result, status }: { result: ECGRhythmResult | null; status: RequestStatus }) {
  const copyText = result?.pred_label_ru ?? '';
  const label = result?.pred_label_ru ?? (status === 'pending' || status === 'processing' ? 'Ожидает обработки' : 'Ритм не определён');

  return (
    <div className="rounded-lg border border-rose-100 bg-gradient-to-br from-rose-50 to-orange-50 p-3 sm:p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="text-xs text-gray-500 mb-1">Предполагаемый ритм:</p>
          <p className="break-words text-xl font-semibold leading-snug text-gray-950 sm:text-2xl">
            {label}
          </p>
        </div>
        {copyText && (
          <div className="sm:shrink-0">
            <CopyButton text={copyText} />
          </div>
        )}
      </div>
    </div>
  );
}

const FEEDBACK_OPTIONS: { value: ECGFeedbackRating; label: string }[] = [
  { value: 'helpful',    label: 'Полезно' },
  { value: 'inaccurate', label: 'Неточно' },
  { value: 'unclear',    label: 'Непонятно' },
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
    <div className="bg-white border border-gray-200 shadow-sm rounded-lg p-4 sm:p-5 mb-4 sm:mb-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm font-medium text-gray-900">Оцените результаты</p>
          <p className="mt-0.5 text-xs text-gray-500">Это помогает улучшать качество автоматического анализа.</p>
        </div>
        <div className="grid grid-cols-3 rounded-lg bg-gray-100 p-1 sm:min-w-[320px]">
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
                className={`relative inline-flex min-h-9 items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition-all ${
                  isCurrent
                    ? 'bg-white text-gray-950 shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                } ${pending && !isPending ? 'opacity-50' : ''} ${pending ? 'cursor-wait' : ''}`}
              >
                {opt.value !== FEEDBACK_OPTIONS[0].value && !isCurrent && (
                  <span className="pointer-events-none absolute left-0 top-1/2 h-4 -translate-y-1/2 border-l border-gray-300" />
                )}
                <span>{opt.label}</span>
                {isPending && <span className="ml-1 text-xs">…</span>}
              </button>
            );
          })}
        </div>
      </div>

      {current === 'inaccurate' && (
        <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-3 sm:p-4">
          <label htmlFor="rhythm-feedback-comment" className="block text-xs text-gray-700">
            Что было неточно? <span className="text-gray-400">(необязательно)</span>
          </label>
          <textarea
            id="rhythm-feedback-comment"
            value={commentDraft}
            onChange={(e) => setCommentDraft(e.target.value.slice(0, COMMENT_MAX))}
            placeholder="Например: реальный ритм — синусовый; модель ошиблась с фибрилляцией."
            rows={3}
            disabled={pending === 'comment'}
            className="mt-2 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 shadow-sm outline-none transition focus:border-gray-300 focus:ring-4 focus:ring-gray-100 disabled:opacity-60"
          />
          <div className="mt-2 flex items-center justify-between gap-3">
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
                className="rounded-md bg-gray-900 px-3 py-1.5 text-xs font-medium text-white shadow-sm transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {pending === 'comment' ? 'Сохраняем…' : 'Сохранить'}
              </button>
            </div>
          </div>
        </div>
      )}

      {current && current !== 'inaccurate' && (
        <p className="mt-3 text-xs text-gray-500">Оценка сохранена. Её можно изменить в любой момент.</p>
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
  const intervalRows = getIntervalRows(result.rhythm);

  return (
    <>
      {!hasMeasurements && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
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
        <div className="bg-white border border-gray-200 shadow-sm rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
          <h2 className="text-sm font-medium text-gray-900 mb-3">Признаки</h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
            {summary.map((s, i) => (
              <SummaryCard key={i} item={s} />
            ))}
          </div>
        </div>
      )}

      {/* Computed indices — numeric values + thresholds for each clinical formula. */}
      <IndicesCard indices={result.indices} rvh={result.rvh} />

      {intervalRows.length > 0 && (
        <div className="bg-white border border-gray-200 shadow-sm rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
          <h2 className="text-sm font-medium text-gray-900 mb-3">Интервалы и ритм</h2>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4 text-center">
            {intervalRows.map((row) => (
              <MetricCard key={row.label} {...row} />
            ))}
          </div>
        </div>
      )}

    </>
  );
}

type MetricRow = {
  label: string;
  value: string;
  unit?: string;
};

function validRange(v: number | null | undefined, min: number, max: number): number | null {
  if (v == null || !Number.isFinite(v) || v <= 0 || v < min || v > max) return null;
  return v;
}

function metric(label: string, value: number | null | undefined, min: number, max: number, unit: string): MetricRow | null {
  const v = validRange(value, min, max);
  if (v == null) return null;
  return { label, value: v.toFixed(0), unit };
}

// Below this QRS width (ms) the JT family adds noise rather than signal: JT/JTc
// exist to assess repolarization independent of a prolonged QRS, so they are
// only shown when QRS is wide.
const WIDE_QRS_MS = 120;

function getIntervalRows(rhythm: ECGStructuredResult['rhythm']): MetricRow[] {
  if (!rhythm) return [];

  // Core intervals, always shown. RR is omitted (redundant with ЧСС) and only
  // one QTc correction (Bazett, the clinical default) is surfaced.
  const rows = [
    metric('PR', rhythm.PR_ms, 80, 320, 'мс'),
    metric('QRS', rhythm.QRS_ms, 60, 240, 'мс'),
    metric('QT', rhythm.QT_ms, 200, 700, 'мс'),
    metric('QTc', rhythm.QTc_bazett_ms, 250, 700, 'мс'),
    metric('ЧСС', rhythm.HR_bpm, 30, 220, 'уд/мин'),
  ];

  const qrs = validRange(rhythm.QRS_ms, 60, 240);
  if (qrs != null && qrs >= WIDE_QRS_MS) {
    rows.push(
      metric('JT', rhythm.JT_ms, 100, 550, 'мс'),
      metric('JTc', rhythm.JTc_bazett_ms, 150, 650, 'мс'),
    );
  }

  return rows.filter((row): row is MetricRow => row != null);
}

type IndexRow = {
  id: string;
  label: string;
  value: number | null | undefined;
  unit: string;
  threshold: string;
  decimals?: number;
};

const LVH_INDEX_DEFS: Omit<IndexRow, 'value'>[] = [
  { id: 'sokolow_lyon',     label: 'Соколов-Лайон',        unit: 'мВ', threshold: '> 3.5 мВ',                 decimals: 2 },
  { id: 'cornell_voltage',  label: 'Корнельский вольтажный', unit: 'мВ', threshold: '> 2.8 мВ (М) / > 2.0 (Ж)', decimals: 2 },
  { id: 'peguero_lo_presti',label: 'Пегеро-Ло Прести',     unit: 'мВ', threshold: '> 2.8 мВ (М) / > 2.3 (Ж)', decimals: 2 },
  { id: 'gubner',           label: 'Губнер',               unit: 'мВ', threshold: '> 2.5 мВ',                 decimals: 2 },
  { id: 'lewis',            label: 'Льюис',                unit: 'мВ', threshold: '> 1.7 мВ',                 decimals: 2 },
];

const RVH_INDEX_DEFS: Omit<IndexRow, 'value'>[] = [
  { id: 'rv1',         label: 'R в V1',    unit: 'мВ', threshold: '> 0.7 мВ',  decimals: 2 },
  { id: 'r_over_s_v1', label: 'R/S в V1',  unit: '',   threshold: '> 1.0',     decimals: 2 },
  { id: 'rv1_plus_sv5',label: 'RV1 + SV5', unit: 'мВ', threshold: '> 1.05 мВ', decimals: 2 },
  { id: 'rv1_plus_sv6',label: 'RV1 + SV6', unit: 'мВ', threshold: '> 1.05 мВ', decimals: 2 },
];

function IndicesCard({
  indices,
  rvh,
}: {
  indices?: ECGStructuredResult['indices'];
  rvh?: ECGStructuredResult['rvh'];
}) {
  const lvhRows: IndexRow[] = LVH_INDEX_DEFS.map((d) => ({
    ...d,
    value:
      d.id === 'sokolow_lyon' ? indices?.sokolow_lyon_mV :
      d.id === 'cornell_voltage' ? indices?.cornell_voltage_mV :
      d.id === 'peguero_lo_presti' ? indices?.peguero_lo_presti_mV :
      d.id === 'gubner' ? indices?.gubner_mV :
      d.id === 'lewis' ? indices?.lewis_mV :
      null,
  })).filter((r) => r.value != null);

  const rvhRows: IndexRow[] = RVH_INDEX_DEFS.map((d) => ({
    ...d,
    value:
      d.id === 'rv1' ? rvh?.RV1_mV :
      d.id === 'r_over_s_v1' ? rvh?.R_over_S_V1 :
      d.id === 'rv1_plus_sv5' ? rvh?.RV1_plus_SV5_mV :
      d.id === 'rv1_plus_sv6' ? rvh?.RV1_plus_SV6_mV :
      null,
  })).filter((r) => r.value != null);

  if (lvhRows.length === 0 && rvhRows.length === 0) return null;

  return (
    <div className="bg-white border border-gray-200 shadow-sm rounded-xl p-4 sm:p-5 mb-4 sm:mb-6">
      <h2 className="text-sm font-medium text-gray-900 mb-3">Индексы</h2>

      {lvhRows.length > 0 && (
        <div className={rvhRows.length > 0 ? 'mb-4' : ''}>
          <p className="text-xs font-medium text-gray-500 mb-2">Гипертрофия левого желудочка</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {lvhRows.map((r) => <IndexRowCard key={r.id} row={r} />)}
          </div>
        </div>
      )}

      {rvhRows.length > 0 && (
        <div>
          <p className="text-xs font-medium text-gray-500 mb-2">Гипертрофия правого желудочка</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {rvhRows.map((r) => <IndexRowCard key={r.id} row={r} />)}
          </div>
        </div>
      )}
    </div>
  );
}

function IndexRowCard({ row }: { row: IndexRow }) {
  const v = row.value!;
  return (
    <div className="bg-gray-50 rounded-lg px-4 py-3 border border-gray-200">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm text-gray-700">{row.label}</p>
        <p className="text-sm font-semibold text-gray-900 font-mono">
          {v.toFixed(row.decimals ?? 1)}
          {row.unit && <span className="text-gray-500 font-sans font-normal"> {row.unit}</span>}
        </p>
      </div>
      <p className="text-[11px] text-gray-400 mt-1">порог {row.threshold}</p>
    </div>
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
      className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-gray-200 bg-white text-gray-700 hover:bg-gray-50 transition-colors"
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

function MetricCard({ label, value, unit }: MetricRow) {
  return (
    <div className="bg-gray-50 rounded-lg border border-gray-200 p-3 transition-colors duration-150 hover:bg-gray-100">
      <p className="text-xs text-gray-500 mb-1">{label}</p>
      <p className="text-lg font-semibold text-gray-900">
        {value}
        {unit && <span className="text-sm font-normal text-gray-500 ml-1">{unit}</span>}
      </p>
    </div>
  );
}
