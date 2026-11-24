ALTER TABLE connections ADD COLUMN favourite INTEGER NOT NULL DEFAULT 0 CHECK (favourite IN (0, 1));
ALTER TABLE connections ADD COLUMN last_opened_at_ns INTEGER;
UPDATE catalog_meta SET schema_version = 2 WHERE singleton = 1;
