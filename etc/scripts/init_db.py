import logging
import os
import json
from pathlib import Path
from typing import List

import click
import click_extra
import psycopg2
from click_extra import extra_command, option, argument
from psycopg2 import sql
from psycopg2.extensions import ISOLATION_LEVEL_AUTOCOMMIT, ISOLATION_LEVEL_DEFAULT

logging.basicConfig(
    level=logging.INFO, format="[%(asctime)s] %(levelname)-8s %(message)s"
)
#click_extra.logging.logger.set_logger(logging.getLogger())


def run_cmd(conn, cmd) -> None:
    with conn.cursor() as cursor:
        cursor.execute(cmd)


def init_schema(conn, schema_file: str) -> None:
    logging.info(f"apply migration {schema_file}...")
    with open(schema_file, "r") as f:
        with conn.cursor() as cursor:
            text = f.read().replace("CONCURRENTLY", "")
            cursor.execute(text)


def import_courses(courses_dir: str, conn) -> List:
    courses = []

    for course_id in os.listdir(courses_dir):
        path = os.path.join(courses_dir, course_id)

        course_type = "free"
        try:

            with open(os.path.join(path, "tags.json")) as file_tags:
                tags = file_tags.read()
        except Exception as e:
            logging.warning(f"Couldn't read tags in dir {path}")
            continue

        title = json.loads(tags).get("title", course_id.capitalize())
        course_data = (course_id, path, title, course_type, tags)
        courses.append(course_data)

    course_ids = [c[0] for c in courses]
    logging.info(f"Found {len(courses)} courses: {course_ids}")

    insert = sql.SQL(
        """INSERT INTO courses VALUES {}
        ON CONFLICT (course_id) DO UPDATE
        SET path_on_disk=EXCLUDED.path_on_disk, title=EXCLUDED.title, type=EXCLUDED.type, tags=EXCLUDED.tags"""
    ).format(sql.SQL(",").join(map(sql.Literal, courses)))

    run_cmd(conn, insert)
    logging.info("Imported courses")
    return course_ids


def get_chapter_title(chapter_dir: str) -> str|None:

    chapter_text_path = os.path.join(chapter_dir, "text.md")

    if not os.path.exists(chapter_text_path):
        return None

    # First line of file is chapter's title:
    with open(chapter_text_path) as f:
        for line in f:
            return line.strip("#").strip()


def import_chapters_for_course(course_dir: str, course_id: str, conn) -> None:
    chapters = []

    for chapter_id in os.listdir(course_dir):
        if not chapter_id.startswith(course_id):
            # Skip additional files and directories
            continue

        title = get_chapter_title(os.path.join(course_dir, chapter_id))
        if title is None:
            logging.warn(f"Course {course_id}. Can't find {os.path.join(course_dir, chapter_id)}/text.md")
            continue

        chapter_data = (chapter_id, course_id, title)
        chapters.append(chapter_data)

    logging.info(f"Course {course_id}. Found {len(chapters)} chapters")

    insert = sql.SQL(
        """INSERT INTO chapters VALUES {}
        ON CONFLICT (chapter_id) DO UPDATE
        SET title=EXCLUDED.title"""
    ).format(sql.SQL(",").join(map(sql.Literal, chapters)))

    run_cmd(conn, insert)
    logging.info(f"Imported chapters for course {course_id}")


def import_chapters(courses_dir: str, course_ids: List, conn) -> None:
    for course_id in course_ids:
        course_dir = os.path.join(courses_dir, course_id)
        import_chapters_for_course(course_dir, course_id, conn)


def import_tasks_for_chapter(chapter_id: str, tasks_dir: str, conn) -> None:
    if not os.path.exists(tasks_dir):
        logging.warning(
            f"Tasks directory for {chapter_id} doesn't exist. Skipping"
        )
        return

    tasks = []

    for task_id in os.listdir(tasks_dir):
        if not task_id.startswith(chapter_id):
            # Skip additional files and directories
            continue

        task_data = (task_id, chapter_id)
        tasks.append(task_data)

    logging.info(f"Chapter {chapter_id}. Found {len(tasks)} tasks")

    if len(tasks) == 0:
        return

    insert = sql.SQL(
        """INSERT INTO tasks VALUES {}
        ON CONFLICT (task_id) DO NOTHING"""
    ).format(sql.SQL(",").join(map(sql.Literal, tasks)))

    run_cmd(conn, insert)
    logging.info(f"Imported tasks for chapter {chapter_id}")


def import_tasks_for_course(course_dir, course_id, conn) -> None:
    for chapter_id in os.listdir(course_dir):
        if not chapter_id.startswith(course_id):
            # Skip additional files and directories
            continue

        tasks_dir = os.path.join(course_dir, chapter_id, "tasks")
        import_tasks_for_chapter(chapter_id, tasks_dir, conn)

    logging.info(f"Imported all tasks for course {course_id}")


def import_tasks(courses_dir: str, course_ids: List, conn) -> None:
    for course_id in course_ids:
        course_dir = os.path.join(courses_dir, course_id)
        import_tasks_for_course(course_dir, course_id, conn)


def import_practice_for_course(practice_dir: str, course_id: str, conn) -> None:
    if not os.path.exists(practice_dir):
        logging.error(f"Directory for practice {practice_dir} for course {course_id} doesn't exist")
        return

    for project_id in os.listdir(practice_dir):
        if not project_id.startswith(course_id):
            # Skip additional files and directories
            continue

        with open(os.path.join(practice_dir, project_id, "data.json")) as f:
            data = json.load(f)
        
        logging.info(f"Course {course_id}. Practice {practice_dir}")

        title = get_chapter_title(os.path.join(practice_dir, project_id))

        insert = sql.SQL(
            """INSERT INTO practice VALUES {}
            ON CONFLICT (project_id) DO UPDATE
            SET title=EXCLUDED.title, chapter_id=EXCLUDED.chapter_id, main_file=EXCLUDED.main_file, default_cmd_line_args=EXCLUDED.default_cmd_line_args"""
        ).format(sql.SQL(",").join(map(sql.Literal, [(project_id, title, data["chapter_id"], data["main_file"], data["default_cmd_line_args"], course_id), ])))

        run_cmd(conn, insert)
        logging.info(f"Imported practice {project_id} for course {course_id}")


def import_practice(courses_dir: str, course_ids: List,conn) -> None:
    for course_id in course_ids:
        practice_dir = os.path.join(courses_dir, course_id, "practice")
        import_practice_for_course(practice_dir, course_id, conn)


@extra_command()
@option(
    "--courses_dir",
    type=click.Path(
        exists=True, dir_okay=True, path_type=Path, resolve_path=True
    ),
    required=True,
    help="Directory with courses",
)
@option(
    "--postgres_conn",
    type=str,
    required=True,
    help="Format: postgresql://user:password@host:port/db?params",
)
@argument(
    "migration_dir",
    type=click.Path(
        exists=True, dir_okay=True, path_type=Path, resolve_path=True
    ),
    help="Directory with sql files to be initialized"
)
def main(courses_dir: str, postgres_conn: str, migration_dir: Path) -> None:
    try:
        logging.info(f"Started courses import from {courses_dir} to db...")

        conn = psycopg2.connect(postgres_conn)
        conn.autocommit=True
        migrations_files = [ migration_dir/file for file in os.listdir(migration_dir) ]

        if migrations_files:
            with conn.cursor() as cursor:
                for schema in migrations_files:
                    if "0002_create_metrics_table.sql" in schema:
                        continue
                    logging.info(f"apply migration {schema}...")
                    init_schema(conn, schema)

        course_ids = import_courses(courses_dir, conn)
        import_chapters(courses_dir, course_ids, conn)
        import_tasks(courses_dir, course_ids, conn)
        import_practice(courses_dir, course_ids,conn)

        logging.info(f"Completed courses import from {courses_dir} to db")
    except Exception:
        logging.exception("Error during import")
        exit(1)


if __name__ == "__main__":
    main()
