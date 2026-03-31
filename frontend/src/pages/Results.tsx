import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import ReactMarkdown from 'react-markdown';
import { requestAPI } from '@/services/api';
import { formatDate, formatStatus, getStatusColor, formatECGParams } from '@/utils/format';
import { Layout } from '@/components/Layout';
import { useEventSource } from '@/hooks/useEventSource';
import { usePendingJobs } from '@/hooks/usePendingJobs';
import type { EKGAnalysisResult, ECGStructuredResult, LVHIndices } from '@/types';

const LEADS_ORDER = ['I', 'II', 'III', 'aVR', 'aVL', 'aVF', 'V1', 'V2', 'V3', 'V4', 'V5', 'V6'];

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
    if (id && request && (request.status === 'completed' || request.status === 'failed')) {
      removeJob(id);
    }
  }, [id, request?.status, removeJob]);

  let ekgResult: EKGAnalysisResult | null = null;
  let isStructured = false;
  if (request?.response?.content) {
    try {
      const parsed = JSON.parse(request.response.content);
      if (parsed?.analysis_type === 'ekg_direct_v2' || parsed?.analysis_type === 'ekg_structured_v1') {
        ekgResult = parsed as EKGAnalysisResult;
        isStructured = parsed.analysis_type === 'ekg_structured_v1';
      }
    } catch {
      // Not JSON — direct GPT text response
    }
  }

  const gptContent = ekgResult
    ? ekgResult.gpt_full_response || null
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
        <div className="text-center py-8 text-red-500">
          Ошибка при загрузке результата
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

        {/* Structured ECG Results */}
        {isStructured && ekgResult?.structured_result && (
          <StructuredResultView result={ekgResult.structured_result} />
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
        {!isStructured && ekgResult && !gptContent && ekgResult.gpt_request_id && (
          <div className="bg-yellow-50 border border-yellow-200 shadow rounded-lg p-6 mb-6">
            <h2 className="text-xl font-bold text-gray-900 mb-2">Заключение</h2>
            <p className="text-sm text-yellow-800">
              {ekgResult.gpt_interpretation_status === 'failed'
                ? 'GPT-интерпретация не удалась. Попробуйте повторить запрос.'
                : 'GPT-интерпретация в обработке...'}
            </p>
          </div>
        )}

        {/* Notes */}
        {ekgResult?.notes && (
          <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
            <h2 className="text-sm font-medium text-gray-400 mb-2">Примечания</h2>
            <p className="text-sm text-gray-600">{ekgResult.notes}</p>
          </div>
        )}

        <p className="mt-6 text-xs text-gray-500 text-center leading-relaxed">
          Результаты анализа носят исключительно информационный характер, не являются медицинским заключением
          и не заменяют консультацию квалифицированного врача.
        </p>
      </div>
    </Layout>
  );
}

// --- Structured Result Components ---

function StructuredResultView({ result }: { result: ECGStructuredResult }) {
  return (
    <>
      {/* Measurements Table */}
      <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6 overflow-x-auto animate-fade-in-up">
        <h2 className="text-lg font-bold text-gray-900 mb-4">Измерения по отведениям</h2>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-200">
              <th className="text-left py-2 pr-3 text-gray-500 font-medium">Отведение</th>
              <th className="text-right py-2 px-3 text-gray-500 font-medium">R, мм</th>
              <th className="text-right py-2 pl-3 text-gray-500 font-medium">S, мм</th>
            </tr>
          </thead>
          <tbody>
            {LEADS_ORDER.map((lead) => {
              const rKey = `R_${lead}_mm`;
              const sKey = `S_${lead}_mm`;
              const rVal = result.measurements[rKey];
              const sVal = result.measurements[sKey];
              return (
                <tr key={lead} className="border-b border-gray-100">
                  <td className="py-2 pr-3 font-medium text-gray-800">{lead}</td>
                  <td className="py-2 px-3 text-right font-mono text-gray-700">{fmt(rVal)}</td>
                  <td className="py-2 pl-3 text-right font-mono text-gray-700">{fmt(sVal)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Rhythm & Intervals */}
      {result.rhythm && (
        <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <h2 className="text-lg font-bold text-gray-900 mb-3">Интервалы и ритм</h2>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-center">
            <MetricCard label="QRS" value={fmt(result.rhythm.QRS_ms, 0)} unit="мс" />
            <MetricCard label="RR" value={fmt(result.rhythm.RR_ms, 0)} unit="мс" />
            <MetricCard label="ЧСС" value={fmt(result.rhythm.HR_bpm, 0)} unit="уд/мин" />
          </div>
        </div>
      )}

      {/* LVH Indices */}
      {result.indices && <LVHIndicesView indices={result.indices} sex={result.patient?.sex} />}

      {/* QRS Axis */}
      {result.axis_qrs && (
        <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <h2 className="text-lg font-bold text-gray-900 mb-3">Электрическая ось сердца</h2>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-center">
            <MetricCard label="Ось" value={fmt(result.axis_qrs.axis_deg, 0)} unit="°" />
            <MetricCard label="Классификация" value={result.axis_qrs.classification || '—'} />
            {result.transition_zone_lead && (
              <MetricCard label="Переходная зона" value={result.transition_zone_lead} />
            )}
          </div>
        </div>
      )}

      {/* RVH */}
      {result.rvh && (
        <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
          <h2 className="text-lg font-bold text-gray-900 mb-3">Маркеры ГПЖ</h2>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 text-center">
            <MetricCard label="R V1" value={fmt(result.rvh.RV1_mV)} unit="мВ" />
            <MetricCard label="R/S V1" value={fmt(result.rvh.R_over_S_V1)} />
            <MetricCard label="RV1+SV5" value={fmt(result.rvh.RV1_plus_SV5_mV)} unit="мВ" />
            <MetricCard label="RV1+SV6" value={fmt(result.rvh.RV1_plus_SV6_mV)} unit="мВ" />
          </div>
        </div>
      )}
    </>
  );
}

const LVH_THRESHOLDS: Record<string, { label: string; key: keyof LVHIndices; threshold: number; thresholdF?: number }> = {
  sokolow: { label: 'Соколов-Лайон', key: 'sokolow_lyon_mV', threshold: 3.5 },
  cornell: { label: 'Корнелл', key: 'cornell_voltage_mV', threshold: 2.8, thresholdF: 2.0 },
  peguero: { label: 'Пегуэро', key: 'peguero_lo_presti_mV', threshold: 2.3, thresholdF: 2.3 },
  gubner: { label: 'Губнер', key: 'gubner_mV', threshold: 2.5 },
  lewis: { label: 'Льюис', key: 'lewis_mV', threshold: 1.7 },
};

function LVHIndicesView({ indices, sex }: { indices: LVHIndices; sex?: string }) {
  const entries = Object.values(LVH_THRESHOLDS);
  return (
    <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
      <h2 className="text-lg font-bold text-gray-900 mb-3">Индексы ГЛЖ</h2>
      <div className="space-y-2">
        {entries.map(({ label, key, threshold, thresholdF }) => {
          const val = indices[key];
          if (val == null) return null;
          const thr = (sex === 'female' && thresholdF != null) ? thresholdF : threshold;
          const exceeded = val >= thr;
          return (
            <div key={key} className="flex items-center justify-between py-1.5 border-b border-gray-100 last:border-0">
              <span className="text-sm text-gray-700">{label}</span>
              <div className="flex items-center gap-2">
                <span className="text-sm font-mono font-medium text-gray-900">{fmt(val, 2)} мВ</span>
                <span className={`text-xs px-1.5 py-0.5 rounded ${exceeded ? 'bg-red-100 text-red-700' : 'bg-green-100 text-green-700'}`}>
                  {exceeded ? `≥ ${thr}` : `< ${thr}`}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
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

function RequestImage({ requestId, fileId }: { requestId: string; fileId: string }) {
  const [src, setSrc] = useState<string | null>(null);

  useEffect(() => {
    let revoke: string | null = null;
    requestAPI.getFileURL(requestId, fileId).then((url) => {
      revoke = url;
      setSrc(url);
    }).catch(() => {});
    return () => { if (revoke) URL.revokeObjectURL(revoke); };
  }, [requestId, fileId]);

  if (!src) return (
    <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
      <div className="h-6 w-48 bg-gray-200 rounded mb-4 animate-pulse" />
      <div className="h-48 bg-gray-200 rounded animate-pulse" />
    </div>
  );

  return (
    <div className="bg-white shadow rounded-lg p-4 sm:p-6 mb-4 sm:mb-6">
      <h2 className="text-lg sm:text-xl font-bold text-gray-900 mb-3 sm:mb-4">Исходное изображение</h2>
      <div className="flex justify-center">
        <img
          src={src}
          alt="Исходное ЭКГ изображение"
          className="max-w-full h-auto rounded-lg shadow-md"
        />
      </div>
    </div>
  );
}
