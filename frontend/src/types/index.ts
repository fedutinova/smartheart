export interface User {
  id: string;
  username: string;
  email: string;
  roles?: Role[];
  created_at: string;
  updated_at: string;
}

export interface Role {
  id: number;
  name: string;
  description?: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  // consent to personal-data processing (152-ФЗ); the backend rejects false.
  consent: boolean;
}

// ECGLayoutLabel selects how cv_service slices the page into per-lead crops.
// Must match the backend's cv.IsValidLayout allow-list.
export type ECGLayoutLabel = '3x4_rhythm' | '3x4' | '6x2' | '6x2_rhythm' | '12x1';

export const ECG_LAYOUT_OPTIONS: { value: ECGLayoutLabel; label: string; hint: string }[] = [
  { value: '3x4_rhythm', label: '3×4 + ритм', hint: '12 отведений в сетке 3×4 и полоса ритма снизу (чаще всего)' },
  { value: '3x4',        label: '3×4',        hint: '12 отведений в сетке 3×4 без полосы ритма' },
  { value: '6x2',        label: '6×2',        hint: '12 отведений в две колонки по 6' },
  { value: '6x2_rhythm', label: '6×2 + ритм', hint: '6×2 с дополнительной полосой ритма' },
  { value: '12x1',       label: '12×1',       hint: 'Все 12 отведений одним столбцом' },
];

export interface ECGCalibrationParams {
  age?: number;
  sex?: string;
  paper_speed_mms: number;
  mm_per_mv_limb: number;
  mm_per_mv_chest: number;
  layout_label: ECGLayoutLabel;
}

export type RedactionMode = 'band' | 'ocr';

export interface ECGClientMeta {
  redaction_mode: RedactionMode;
  redaction_ms: number;
  boxes_count: number;
  masked_area_ratio: number;
  image_width: number;
  image_height: number;
}

export interface RedactionBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface BandRedactionConfig {
  topRatio: number;
  bottomRatio: number;
  leftRatio: number;
}

export interface Job {
  id: string;
  type: string;
  status: JobStatus;
  enqueued_at: string;
  started_at?: string;
  finished_at?: string;
}

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed';

export interface Request {
  id: string;
  user_id?: string;
  text_query?: string;
  status: RequestStatus;
  created_at: string;
  updated_at: string;
  client_meta?: ECGClientMeta;
  files?: File[];
  response?: Response;
  ecg_age?: number;
  ecg_sex?: string;
  ecg_paper_speed_mms?: number;
  ecg_mm_per_mv_limb?: number;
  ecg_mm_per_mv_chest?: number;
}

export type RequestStatus = 'pending' | 'processing' | 'completed' | 'failed';

export interface File {
  id: string;
  request_id: string;
  original_filename: string;
  file_type?: string;
  file_size?: number;
  s3_key: string;
  s3_url?: string;
  created_at: string;
}

export interface Response {
  id: string;
  request_id: string;
  content: string;
  model?: string;
  tokens_used?: number;
  processing_time_ms?: number;
  created_at: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface QuotaInfo {
  free_limit: number;
  free_analyses_used: number;
  free_remaining: number;
  paid_analyses_remaining: number;
  needs_payment: boolean;
  price_per_analysis_kopecks: number;
  subscription_expires_at?: string;
  subscription_price_kopecks: number;
}

export interface PaymentResult {
  payment_id: string;
  confirmation_url: string;
  amount_rub: string;
}

export interface Payment {
  id: string;
  yookassa_id: string;
  status: 'pending' | 'succeeded' | 'canceled';
  amount_kopecks: number;
  description: string;
  payment_type: 'subscription' | 'analyses';
  created_at: string;
  confirmed_at?: string;
}

export interface ECGAnalysisResult {
  analysis_type: string;
  notes?: string;
  timestamp: string;
  job_id: string;
  gpt_request_id?: string;
  gpt_interpretation_status?: string;
  gpt_interpretation?: string;
  gpt_full_response?: string;
  structured_result?: ECGStructuredResult;
  // rhythm_result is produced by cv_service. Absent for legacy responses and
  // when CV inference was unavailable (graceful degradation).
  rhythm_result?: ECGRhythmResult;
}

export interface ECGRhythmResult {
  pred_code: string;
  pred_label_ru: string;
  layout_label: string;
  preprocess_name: string;
  top3: RhythmClassProb[];
  binary_flags: RhythmBinaryFlag[];
  // explanation is the vision-LLM narrative built on top of the CV result.
  // Absent when the LLM call failed (graceful degradation) — frontend shows
  // pred_label_ru alone in that case.
  explanation?: ECGRhythmExplanation;
  // confidence is the classifier's top-1 probability (0..1).
  confidence?: number;
  // suppressed: hide the rhythm badge — the classifier was not confident and
  // the GPT interpretation disagreed; rely on the interpretation text instead.
  suppressed?: boolean;
}

export interface ECGRhythmExplanation {
  prediction_line: string;
  description_text: string;
  conclusion_text: string;
  note_text: string;
}

// User's evaluation of the rhythm conclusion. Persisted in rhythm_feedback;
// backend's allow-list is the source of truth — keep this union in sync.
export type ECGFeedbackRating = 'helpful' | 'inaccurate' | 'unclear';

// Stored feedback row. Comment is typically filled when rating is "inaccurate"
// so the user can describe what was wrong — a retraining signal.
export interface ECGFeedback {
  rating: ECGFeedbackRating;
  comment?: string;
}

export interface RhythmClassProb {
  code: string;
  label_ru: string;
  prob: number;
}

export interface RhythmBinaryFlag {
  code: string;
  label_ru: string;
  prob: number;
}

export interface InterpretationItem {
  label: string;
  value: string;
  threshold?: string;
  status: 'positive' | 'negative' | 'normal' | 'abnormal';
  group?: 'lvh' | 'rvh' | 'rhythm';
}

export interface ECGInterpretation {
  items: InterpretationItem[];
  summary: InterpretationItem[];
  text_summary?: string;
}

export interface ECGStructuredResult {
  measurements: Record<string, number | null>;
  indices?: LVHIndices;
  rvh?: RVHData;
  axis_qrs?: QRSAxis;
  rhythm?: RhythmTiming;
  transition_zone_lead?: string;
  interpretation?: ECGInterpretation;
  patient: PatientInfo;
  timestamp: string;
  job_id: string;
}

export interface LVHIndices {
  sokolow_lyon_mV?: number;
  cornell_voltage_mV?: number;
  peguero_lo_presti_mV?: number;
  gubner_mV?: number;
  lewis_mV?: number;
}

export interface RVHData {
  RV1_mV?: number;
  R_over_S_V1?: number;
  RV1_plus_SV5_mV?: number;
  RV1_plus_SV6_mV?: number;
}

export interface QRSAxis {
  net_I_mV?: number;
  net_aVF_mV?: number;
  axis_deg?: number;
  classification?: string;
}

export interface RhythmTiming {
  PR_ms?: number;
  QRS_ms?: number;
  RR_ms?: number;
  QT_ms?: number;
  QTc_bazett_ms?: number;
  QTc_fridericia_ms?: number;
  JT_ms?: number;
  JTc_bazett_ms?: number;
  JTc_fridericia_ms?: number;
  HR_bpm?: number;
}

export interface PatientInfo {
  sex?: string;
  age?: number;
}
