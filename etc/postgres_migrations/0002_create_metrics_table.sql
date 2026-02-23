CREATE USER metrics_writer WITH PASSWORD 'metrics_writer';
CREATE DATABASE metrics OWNER metrics_writer;


CREATE USER metrics_reader WITH PASSWORD 'N32)a:6832+==3YW01';

-- \c metrics

GRANT CONNECT ON DATABASE metrics TO metrics_reader;

GRANT USAGE ON SCHEMA public TO metrics_reader;



CREATE TYPE chapter_action AS ENUM('view', 'run_task', 'change_status');


CREATE TABLE chapters_activity (
    chapter_id varchar NOT NULL,
    user_id BIGINT NOT NULL,
    day date NOT NULL,
    action chapter_action NOT NULL,
    count integer NOT NULL
);
ALTER TABLE chapters_activity ADD UNIQUE (chapter_id, user_id, day, action);

GRANT SELECT ON chapters_activity TO metrics_reader;
ALTER TABLE chapters_activity OWNER TO metrics_writer;


CREATE TABLE weekly_active_users (
    day date NOT NULL,
    count integer NOT NULL
);
ALTER TABLE weekly_active_users ADD UNIQUE (day, count);
ALTER TABLE weekly_active_users OWNER TO metrics_writer;
GRANT SELECT ON weekly_active_users TO metrics_reader;

CREATE TABLE monthly_active_users (
    day date NOT NULL,
    count integer NOT NULL
);
ALTER TABLE monthly_active_users ADD UNIQUE (day, count);
ALTER TABLE monthly_active_users OWNER TO metrics_writer;
GRANT SELECT ON monthly_active_users TO metrics_reader;