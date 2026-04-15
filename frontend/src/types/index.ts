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
}

export interface ECGAnalysisRequest {
  image_temp_url: string;
  notes?: string;
  age?: number;
  sex?: string;
  paper_speed_mms?: number;
  mm_per_mv_limb?: number;
  mm_per_mv_chest?: number;
}

export interface ECGCalibrationParams {
  age?: number;
  sex?: string;
  paper_speed_mms: number;
  mm_per_mv_limb: number;
  mm_per_mv_chest: number;
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
  daily_limit: number;
  used_today: number;
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
  QRS_ms?: number;
  RR_ms?: number;
  HR_bpm?: number;
}

export interface PatientInfo {
  sex?: string;
  age?: number;
}

