CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_roles (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);
CREATE INDEX IF NOT EXISTS idx_permissions_resource_action ON permissions(resource, action);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

INSERT INTO roles (name, description) VALUES
    ('admin', 'Administrator with full system access'),
    ('user', 'Regular user with limited access')
ON CONFLICT (name) DO NOTHING;

INSERT INTO permissions (name, resource, action, description) VALUES
    ('ekg:submit', 'ekg', 'submit', 'Submit EKG analysis requests'),
    ('job:read', 'job', 'read', 'Read job status and details'),
    ('job:read_own', 'job', 'read_own', 'Read only own jobs'),
    ('job:read_all', 'job', 'read_all', 'Read all jobs in system'),
    ('admin:all', 'admin', 'all', 'Full administrative access')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p 
WHERE r.name = 'user' AND p.name IN ('ekg:submit', 'job:read_own')
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p 
WHERE r.name = 'admin' AND p.name IN ('ekg:submit', 'job:read_all', 'admin:all')
ON CONFLICT (role_id, permission_id) DO NOTHING;

ALTER TABLE requests 
ALTER COLUMN user_id TYPE UUID USING user_id::UUID,
ADD CONSTRAINT fk_requests_user_id FOREIGN KEY (user_id) REFERENCES users(id);
