ALTER TABLE jobs
    ADD COLUMN title varchar(200) NOT NULL DEFAULT '',
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN budget numeric(10,2),
    ADD COLUMN deadline date,
    ADD CONSTRAINT chk_job_budget CHECK (budget IS NULL OR budget >= 0),
    ADD CONSTRAINT chk_job_description CHECK (char_length(description) <= 5000);
