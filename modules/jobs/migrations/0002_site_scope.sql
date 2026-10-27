ALTER TABLE jobs ADD COLUMN site_id TEXT NOT NULL DEFAULT '';
CREATE INDEX jobs_site_created ON jobs (site_id, created_at, job_id);
ALTER TABLE events ADD COLUMN site_id TEXT NOT NULL DEFAULT '';
