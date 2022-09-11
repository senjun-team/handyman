-- To run postgres in docker:
-- docker run --name postgresql -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -v /data:/var/lib/postgresql/data -d postgres

-- Databases, roles, enums, etc
CREATE USER senjun WITH PASSWORD 'some_password';
CREATE DATABASE senjun OWNER senjun;

CREATE TYPE edu_material_status AS ENUM ('completed', 'in_progress', 'blocked');
CREATE TYPE course_type AS ENUM('free', 'paid');

-- To connect to the created db:
-- \c senjun

-- Tables & indices
CREATE TABLE users (
    user_id bigserial NOT NULL PRIMARY KEY,
    login varchar NOT NULL,
    pass_hash varchar NOT NULL,
    created timestamptz default current_timestamp,
    is_blocked boolean,
    name varchar,
    surname varchar
);

CREATE TABLE courses (
    course_id varchar NOT NULL PRIMARY KEY,
    path_on_disk varchar NOT NULL,
    type course_type default 'free' NOT NULL,
);

CREATE TABLE accesses (
    access_id bigserial NOT NULL PRIMARY KEY,
    user_id bigserial,
    course_id varchar,
    CONSTRAINT fk_user_id FOREIGN KEY(user_id) REFERENCES users(user_id),
    CONSTRAINT fk_course_id FOREIGN KEY(course_id) REFERENCES courses(course_id)
);

CREATE TABLE chapters (
    chapter_id varchar NOT NULL PRIMARY KEY,
    course_id varchar,
    CONSTRAINT fk_course_id FOREIGN KEY(course_id) REFERENCES courses(course_id)
);

CREATE TABLE tasks (
    task_id varchar NOT NULL PRIMARY KEY,
    course_id varchar,
    CONSTRAINT fk_course_id FOREIGN KEY(course_id) REFERENCES courses(course_id)
);

CREATE TABLE course_progress (
    course_progress_id bigserial NOT NULL PRIMARY KEY,
    user_id bigserial,
    course_id varchar,
    status edu_material_status NOT NULL,
    CONSTRAINT fk_user_id FOREIGN KEY(user_id) REFERENCES users(user_id),
    CONSTRAINT fk_course_id FOREIGN KEY(course_id) REFERENCES courses(course_id)
);

CREATE TABLE chapter_progress (
    chapter_progress_id bigserial NOT NULL PRIMARY KEY,
    user_id bigserial,
    chapter_id varchar,
    status edu_material_status NOT NULL,
    CONSTRAINT fk_user_id FOREIGN KEY(user_id) REFERENCES users(user_id),
    CONSTRAINT fk_chapter_id FOREIGN KEY(chapter_id) REFERENCES chapters(chapter_id)
);

CREATE TABLE task_progress (
    task_progress_id bigserial NOT NULL PRIMARY KEY,
    user_id bigserial,
    task_id varchar,
    status edu_material_status NOT NULL,
    solution_text text,
    attempts_count smallint DEFAULT 0 NOT NULL,
    CONSTRAINT fk_user_id FOREIGN KEY(user_id) REFERENCES users(user_id),
    CONSTRAINT fk_task_id FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);

-- FILL TABLES

INSERT INTO courses(course_id, path_on_disk) VALUES('python', '/courses/python');
INSERT INTO courses(course_id, path_on_disk) VALUES('rust', '/courses/rust');

INSERT INTO users(login, pass_hash, is_blocked, name, surname) 
VALUES('mesozoic.drones', 'fec790f175bef65ca00c3887fa85af51', false, 'Olga', 'Khlopkova');

