ALTER TABLE agent_sessions ADD COLUMN monitor_kind TEXT;
ALTER TABLE agent_sessions ADD COLUMN automation_id TEXT;
ALTER TABLE agent_sessions ADD COLUMN external_run_id TEXT;
ALTER TABLE agent_sessions ADD COLUMN poll_command TEXT;
ALTER TABLE agent_sessions ADD COLUMN manual_until TEXT;
