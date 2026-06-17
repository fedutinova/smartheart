-- Record the user's consent to personal-data processing (152-ФЗ): the timestamp
-- it was given and the policy version agreed to. Nullable: pre-existing users
-- registered before this column was added carry NULL (consent not recorded).
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS consent_given_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS consent_version  VARCHAR(20);
