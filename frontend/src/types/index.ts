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

export interface TokenPair {
  access_token: string;
  refresh_token: string;
}

export interface EKGAnalysisRequest {
  image_temp_url: string;
  notes?: string;
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

export interface EKGAnalysisResult {
  analysis_type: string;
  notes?: string;
  timestamp: string;
  job_id: string;
  gpt_request_id?: string;
  gpt_interpretation_status?: string;
  gpt_interpretation?: string;
  gpt_full_response?: string;
}

