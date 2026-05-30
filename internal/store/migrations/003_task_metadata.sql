ALTER TABLE task_definitions ADD COLUMN profile TEXT;
ALTER TABLE task_definitions ADD COLUMN owning_domain TEXT;
ALTER TABLE task_definitions ADD COLUMN owning_layer TEXT;
ALTER TABLE task_definitions ADD COLUMN source_paths TEXT;
ALTER TABLE task_definitions ADD COLUMN target_paths TEXT;
ALTER TABLE task_definitions ADD COLUMN review_domains TEXT;
ALTER TABLE task_definitions ADD COLUMN risk_level TEXT;
ALTER TABLE task_definitions ADD COLUMN migration_type TEXT;
