# Вспомогательные скрипты
В этой директории лежат вспомогательные скрипты на питоне для автоматизации работы с бд, курсами и тд.

Перед первым запуском любого из скриптов нужно создать виртуальное окружение, активироать его и установить все зависимости:
```bash
cd handyman/etc/scripts
python3 -m venv .venv
source .venv/bin/activate
pip3 install -r requirements.txt
```

Для каждой новой консольной сессии нужно активировать виртуальное окружение:
```bash
source .venv/bin/activate
```

## import_courses.py
Для чего нужен: обходит директорию с курсами. Находит в ней курсы, главы, задачи. Импортирует их в постгрес с автоматическим разрешением конфликтов.
Когда нужно запускать: при первом поднятии инфраструктуры сенджуна на машине; каждый раз при добавлении/удалении/изменении состава курсов, глав, задач.

Пример запуска:
```bash
python3 import_courses.py --courses_dir=/data/courses/ --postgres_conn=postgresql://senjun:some_password@127.0.0.1:5432/senjun
```

## move_chapters.py
Для чего нужен: чтобы вставлять в середину курса новую главу. Принимает айди главы, после которой нужно осуществить сдвиг. Начиная с этого айди не включительно скрипт делает следующее:
- Инкрементирует айди глав (переименовывает директорию главы)
- Обновляет ссылки на главу во всех текстах всех глав
- Инкрементирует айди в директориях с задачами
- Обновляет заголовки глав (инкрементирует номер)
- Готовит файл с sql миграцией, который нужно прогнать **после** `import_courses.py`.

Пример запуска:
```bash
python3 move_chapters.py --course_dir=/data/courses/python --chapter=python_chapter_0020
```

В этом примере скрипт сделает главу 3 главой 4, главу 4 - главой 5 и тд. После этого руками нужно будет создать главу 3. Если в главу 3 попадает часть главы 2 (то есть 2 разбивается на две главы), то все связанные с этим манипуляции нужно доделать руками.

Порядок действий:
- Запустить скрипт `move_chapters.py`
- Довнести руками изменения, если нужны (при разбивке существующей главы на две)
- Задеплоить `courses` на прод
- Запустить `import_courses.py`
- Запустить мигацию, подготовленную `move_chapters.py`
- Руками довыполнить мигации, если нужны (при разбивке)

Пример ручных миграций при разбиении 2-ой главы питона на две:

```sql
UPDATE task_progress SET task_id = 'python_chapter_0030_task_0010' WHERE task_id = 'python_chapter_0020_task_0070';
UPDATE task_progress SET task_id = 'python_chapter_0030_task_0020' WHERE task_id = 'python_chapter_0020_task_0080';


WITH s AS (
SELECT user_id from chapter_progress where chapter_id = 'python_chapter_0020' and status = 'in_progress'
)
INSERT INTO chapter_progress SELECT *, 'python_chapter_0030', 'in_progress' FROM s;

WITH s AS (
SELECT user_id from chapter_progress where chapter_id = 'python_chapter_0020' and status = 'completed'
)
INSERT INTO chapter_progress SELECT *, 'python_chapter_0030', 'completed' FROM s;
```