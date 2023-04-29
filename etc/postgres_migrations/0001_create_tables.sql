-- To run postgres in docker:
-- docker run --name postgresql -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -v /data:/var/lib/postgresql/data -d postgres

-- Databases, roles, enums, etc
CREATE USER senjun WITH PASSWORD 'some_password';
CREATE DATABASE senjun OWNER senjun;

-- To connect to the created db:
-- \c senjun

CREATE TYPE edu_material_status AS ENUM ('in_progress', 'blocked', 'completed');
CREATE TYPE course_type AS ENUM('free', 'paid');


CREATE FUNCTION max_edu_status(s1 edu_material_status, s2 edu_material_status) RETURNS edu_material_status AS
$BODY$
BEGIN

    IF s1 = 'completed' OR s2 = 'completed' THEN
        RETURN 'completed';
    END IF;

    IF s1 = 'blocked' OR s2 = 'blocked' THEN
        RETURN 'blocked';
    END IF;

    RETURN 'in_progress';
 END;
$BODY$
LANGUAGE plpgsql;

CREATE FUNCTION best_solution(s1 edu_material_status, t1 text, s2 edu_material_status, t2 text) RETURNS text AS
$BODY$
BEGIN

    IF s1 = 'completed' THEN
        RETURN t1;
    END IF;

    IF s2 = 'completed' THEN
        RETURN t2;
    END IF;

    IF len(t1) > 0 THEN
        RETURN t1;
    END IF;

    RETURN t2;
 END;
$BODY$
LANGUAGE plpgsql;

-- Tables & indices

CREATE TABLE courses (
    course_id varchar NOT NULL PRIMARY KEY,
    path_on_disk varchar NOT NULL,
    title varchar NOT NULL,
    type course_type default 'free' NOT NULL,
    tags jsonb NOT NULL
);
ALTER TABLE courses OWNER TO senjun;

CREATE TABLE chapters (
    chapter_id varchar NOT NULL PRIMARY KEY,
    course_id varchar NOT NULL,
    title varchar NOT NULL,
    CONSTRAINT fk_course_id FOREIGN KEY(course_id) REFERENCES courses(course_id)
);
ALTER TABLE chapters OWNER TO senjun;

CREATE TABLE tasks (
    task_id varchar NOT NULL PRIMARY KEY,
    chapter_id varchar NOT NULL,
    CONSTRAINT fk_chapter_id FOREIGN KEY(chapter_id) REFERENCES chapters(chapter_id)
);
ALTER TABLE tasks OWNER TO senjun;

CREATE TABLE course_progress (
    user_id BIGINT NOT NULL,
    course_id varchar NOT NULL,
    status edu_material_status NOT NULL,
    CONSTRAINT fk_course_id FOREIGN KEY(course_id) REFERENCES courses(course_id)
);
CREATE UNIQUE INDEX CONCURRENTLY unique_user_course_id ON course_progress(user_id, course_id);
ALTER TABLE course_progress ADD CONSTRAINT unique_user_course_id UNIQUE USING INDEX unique_user_course_id;
ALTER TABLE course_progress OWNER TO senjun;

CREATE TABLE chapter_progress (
    user_id BIGINT NOT NULL,
    chapter_id varchar NOT NULL,
    status edu_material_status NOT NULL,
    CONSTRAINT fk_chapter_id FOREIGN KEY(chapter_id) REFERENCES chapters(chapter_id)
);
CREATE UNIQUE INDEX CONCURRENTLY unique_user_chapter_id ON chapter_progress(user_id, chapter_id);
ALTER TABLE chapter_progress ADD CONSTRAINT unique_user_chapter_id UNIQUE USING INDEX unique_user_chapter_id;
ALTER TABLE chapter_progress OWNER TO senjun;

CREATE TABLE task_progress (
    user_id BIGINT NOT NULL,
    task_id varchar NOT NULL,
    status edu_material_status NOT NULL,
    solution_text text NOT NULL,
    attempts_count smallint DEFAULT 0 NOT NULL,
    CONSTRAINT fk_task_id FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE UNIQUE INDEX CONCURRENTLY unique_user_task_id ON task_progress(user_id, task_id);
ALTER TABLE task_progress ADD CONSTRAINT unique_user_task_id UNIQUE USING INDEX unique_user_task_id;
ALTER TABLE task_progress OWNER TO senjun;


-- FILL TABLES FOR TEST PURPOSES ONLY

INSERT INTO chapter_progress(user_id, chapter_id, status) VALUES
(100, 'python_chapter_0010', 'in_progress');