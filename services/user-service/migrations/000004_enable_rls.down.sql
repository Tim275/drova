DROP POLICY IF EXISTS users_service_policy ON users;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS invitations_service_policy ON user_invitations;
ALTER TABLE user_invitations DISABLE ROW LEVEL SECURITY;
