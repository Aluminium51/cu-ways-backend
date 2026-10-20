ALTER TABLE jobs
    DROP CONSTRAINT chk_job_description,
    DROP CONSTRAINT chk_job_budget,
    DROP COLUMN deadline,
    DROP COLUMN budget,
    DROP COLUMN description,
    DROP COLUMN title;
