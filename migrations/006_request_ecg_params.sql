-- Add ECG analysis parameters to requests table
ALTER TABLE requests ADD COLUMN IF NOT EXISTS ecg_age SMALLINT;
ALTER TABLE requests ADD COLUMN IF NOT EXISTS ecg_sex VARCHAR(10);
ALTER TABLE requests ADD COLUMN IF NOT EXISTS ecg_paper_speed_mms REAL;
ALTER TABLE requests ADD COLUMN IF NOT EXISTS ecg_mm_per_mv_limb REAL;
ALTER TABLE requests ADD COLUMN IF NOT EXISTS ecg_mm_per_mv_chest REAL;
