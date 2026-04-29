-- RLS on users: app queries always filter by id — no cross-user data leaks
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

-- Service role bypasses RLS (drova user is the only DB user, so this is explicit)
CREATE POLICY users_service_policy ON users
  USING (true)
  WITH CHECK (true);

-- RLS on invitations
ALTER TABLE user_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_invitations FORCE ROW LEVEL SECURITY;

CREATE POLICY invitations_service_policy ON user_invitations
  USING (true)
  WITH CHECK (true);
