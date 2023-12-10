import logging
import os
import sys
from pathlib import Path
from typing import List
from dataclasses import dataclass

import click
import click_extra
from click_extra import extra_command, option


logging.basicConfig(
    level=logging.INFO, format="[%(asctime)s] %(levelname)-8s %(message)s"
)
click_extra.logging.logger.set_logger(logging.getLogger())

DELIM = "_"


@dataclass
class ChapterData:
    chapter_id: str
    chapter_fullpath: str


@dataclass
class TaskMigration:
    old_task_id: str
    new_task_id: str


@dataclass
class ChapterMigration:
    old_chapter: ChapterData
    new_chapter: ChapterData


def get_movable_chapters(course_dir: str, chapter: str) -> List:
    """
    Returns full pathes to chapters which should be renamed
    """
    chapters = []

    for chapter_id in os.listdir(course_dir):
        if chapter_id > chapter:
            path = os.path.join(course_dir, chapter_id)
            if os.path.isdir(path):
                chapters.append(path)

    chapters.sort(reverse=True)
    logging.info(f"Found {len(chapters)} chapters after {chapter}")

    return chapters


def get_chapter_migration(chapter_dir: str) -> ChapterMigration:
    """
    Returns migration for chapter: old and new id and full path
    """

    migration = ChapterMigration(ChapterData("", ""), ChapterData("", ""))

    # python_chapter_0230
    chapter_id = os.path.basename(os.path.normpath(chapter_dir))
    migration.old_chapter.chapter_id = chapter_id
    migration.old_chapter.chapter_fullpath = chapter_dir

    # 0230
    path = chapter_id.split(DELIM)
    num_str = path[-1]

    # 23
    old_num = int(num_str[:-1])

    new_num_str = str(old_num + 1).zfill(3) + "0"

    new_chapter = DELIM.join([*path[:-1], new_num_str])
    migration.new_chapter.chapter_id = new_chapter
    migration.new_chapter.chapter_fullpath = os.path.join(
        os.path.dirname(chapter_dir), new_chapter
    )
    return migration


def update_chapter_title(
    chapter_dir: str, migration: ChapterMigration
) -> None:
    """
    Increments chapter number in title (first chapter line)
    """

    chapter_text_path = os.path.join(chapter_dir, "text.md")

    with open(chapter_text_path) as f:
        data = f.read()

    # Update chapter number in title (first line)
    if chapter_dir >= migration.old_chapter.chapter_fullpath:
        i = data.find("\n")
        first_line = data[0:i]

        # # Глава 1. Ключевые факты
        words = first_line.split(" ")
        num = words[2]
        num = num.strip(".")
        new_num = int(num) + 1

        words[2] = str(new_num) + "."
        new_first_line = " ".join(words)

        data = data.replace(first_line, new_first_line, 1)

    with open(chapter_text_path, "w") as f:
        f.write(data)


def update_links(chapter_dir: str, migration: ChapterMigration) -> None:
    """
    Finds and updates links to old chapter in text
    """

    chapter_text_path = os.path.join(chapter_dir, "text.md")

    with open(chapter_text_path) as f:
        data = f.read()

    data = data.replace(
        migration.old_chapter.chapter_id, migration.new_chapter.chapter_id
    )

    with open(chapter_text_path, "w") as f:
        f.write(data)


def update_tasks_dir(chapter_dir: str, migration) -> list[TaskMigration]:
    """
    Renames tasks directories in chapter
    """
    task_migrations = []

    tasks_dir = os.path.join(chapter_dir, "tasks")
    if not os.path.exists(tasks_dir):
        return []

    for task_id in os.listdir(tasks_dir):
        # python_chapter_0230_task_0020
        parts = task_id.split(DELIM)
        new_task_id = DELIM.join(
            [migration.new_chapter.chapter_id, *parts[-2:]]
        )
        path = os.path.join(tasks_dir, task_id)
        new_path = os.path.join(tasks_dir, new_task_id)
        os.rename(path, new_path)
        task_migrations.append(TaskMigration(task_id, new_task_id))

    return task_migrations


def migrate(
    course_dir: str, migration: ChapterMigration
) -> list[TaskMigration]:
    """
    Updates links in chapter texts, renames task directories, renames chapters directories
    """
    for chapter_id in os.listdir(course_dir):
        path = os.path.join(course_dir, chapter_id)
        if not os.path.isdir(path):
            continue

        update_links(path, migration)

    task_migrations = update_tasks_dir(
        migration.old_chapter.chapter_fullpath, migration
    )
    os.rename(
        migration.old_chapter.chapter_fullpath,
        migration.new_chapter.chapter_fullpath,
    )
    return task_migrations


def create_sql_migration(
    task_migrations: list[TaskMigration],
    chapter_migrations: ChapterMigration,
    chapter_dir: str,
) -> None:
    """
    Creates SQL queries which should be run on server to migrate user data
    """
    chapter_id = os.path.basename(os.path.normpath(chapter_dir))
    filename = f"migration_{chapter_id}.sql"

    with open(filename, "w") as f:
        for m in chapter_migrations:
            chapter_upd = f"UPDATE chapter_progress SET chapter_id = '{m.new_chapter.chapter_id}' WHERE chapter_id = '{m.old_chapter.chapter_id}';"
            f.write(f"{chapter_upd}\n")

        f.write("\n")

        for m in task_migrations:
            task_upd = f"UPDATE task_progress SET task_id = '{m.new_task_id}' WHERE task_id = '{m.old_task_id}';"
            f.write(f"{task_upd}\n")


@extra_command()
@option(
    "--course_dir",
    type=click.Path(
        exists=True, dir_okay=True, path_type=Path, resolve_path=True
    ),
    required=True,
    help="Directory with course",
)
@option(
    "--chapter",
    type=str,
    required=True,
    help="Chapter from which to increment all chapters in course",
)
def main(course_dir: str, chapter: str) -> None:
    try:
        logging.info(
            f"Started courses moving in {course_dir} from {chapter}..."
        )
        chapter_pathes = get_movable_chapters(course_dir, chapter)

        task_migrations = []
        chapter_migrations = []

        for cur_chapter in chapter_pathes:
            migration = get_chapter_migration(cur_chapter)
            update_chapter_title(cur_chapter, migration)
            task_migrations.extend(migrate(course_dir, migration))

            chapter_migrations.append(migration)

        create_sql_migration(task_migrations, chapter_migrations, chapter)

        logging.info(
            f"Completed courses moving in {course_dir} from {chapter}"
        )
    except Exception:
        logging.exception("Error during import")
        sys.exit(1)


if __name__ == "__main__":
    main()
