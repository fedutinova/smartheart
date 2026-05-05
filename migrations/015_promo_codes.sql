-- Promo codes system for discounts (including 100% off)

CREATE TABLE promo_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    discount_percent INTEGER NOT NULL CHECK (discount_percent >= 0 AND discount_percent <= 100),
    max_uses INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    used_count INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_promo_codes_code ON promo_codes(code);
CREATE INDEX idx_promo_codes_active ON promo_codes(expires_at) WHERE expires_at > NOW();

-- Track which users have used which promo code
CREATE TABLE promo_code_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    promo_code_id UUID NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    used_at TIMESTAMP NOT NULL DEFAULT NOW(),
    discount_amount_kopecks INTEGER NOT NULL
);

CREATE INDEX idx_promo_usage_user_code ON promo_code_usage(user_id, promo_code_id);
CREATE INDEX idx_promo_usage_payment ON promo_code_usage(payment_id);
