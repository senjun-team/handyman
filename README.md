# Handyman

Сервис для выдачи и обновления прогресса по курсам, отправки задач в watchman.


## Сборка и запуск

1. Забрать проект к себе:
```bash
git clone https://gitlab.com/senjun/handyman.git
cd handyman
```

2. Определить, в какой директории будут лежать [курсы.](https://github.com/senjun-team/senjun-courses/tree/main) Она должна называться `courses`. Например, `/home/code_runner/courses`.


3. Поднять PostgreSQL в докере.  Чтобы начать работать с handyman, можно запустить контейнер  с постгресом:

```bash
docker run  -e POSTGRES_PASSWORD=senjun_pass -p 5432:5432 -v postgres-senjun-data:/var/lib/postgresql/data -d postgres
```

4. Зайти в контейнер:

```bash
docker ps
516cd7b6f4d3   postgres     "docker-entrypoint.s…"  ...

docker exec -it 516cd7b6f4d3 bash
psql -U postgres
```

5. Применить миграции бд из `etc/postgres_migrations`.

6. Заполнить таблички постгреса данными из курсов. Вам нужен скрипт `import_courses.py` из `etc/scripts`. 

Настройка его окружения:

```bash
cd etc/scripts
python3 -m venv .venv
source .venv/bin/activate
pip3 install -r requirements.txt
```

```bash
python3 import_courses.py --courses_dir=/home/code_runner/courses --postgres_conn=postgresql://senjun:some_password@127.0.0.1:5432/senjun
```

7. Собрать и запустить сервис в дебаг-сборке с указанием пути к курсам:
```bash
cd cmd/handyman
go run . /home/code_runner/courses
```

Либо собрать и запустить сервис в релизной сборке:
```bash
go build .
./handyman /home/code_runner/courses
```

## Апишки

`/run_task` - запуск решения пользователя для задачи курса. Решение пользователя закодировано в base64.
```bash
curl -X POST \
  -d '{"task_id":"python_chapter_0010_task_0010", "solution_text":"ZXJyX3NlcnZpY2VfdW5hdmFpbGFibGUgPSA1MDM="}' \
  "http://localhost:8080/run_task?user_id=100"
```

`/save_task` - сохранение решения пользователя для задачи курса. Решение пользователя закодировано в base64. Выполняется на фронтенде сайта раз в какое-то время, если пользователь редактировал текст. Это нужно, чтобы при уходе со страницы, при перезагрузке страницы, при закрытии браузера, отвале интернета и других неприятностях у пользователя не терялось его решение.
```bash
curl -X POST \
  -d '{"task_id":"python_chapter_0010_task_0010", "solution_text":"ZXJyX3NlcnZpY2VfdW5hdmFpbGFibGUgPSA1MDM="}' \
  "http://localhost:8080/save_task?user_id=1"
```

```bash
curl -X POST \
  -d '{"task_id":"cpp_chapter_0020_task_0020", "solution_text":"IHRydWUgIA==", "task_type":"plain_text"}' \
  "http://localhost:8080/run_task?user_id=1"
```
  
```bash
curl -X POST \
  -d '{"task_id":"rust_chapter_0020_task_0010", "solution_text":"bGV0IG11dCBtID0gMzsKbSA9IG0gKyAyOwpwcmludGxuISgibSA9IHt9IiwgbSk7CmFzc2VydCEobSA9PSA1KTsK", "color_output": false}' \
  "http://localhost:8080/run_task?user_id=1"
```

`/get_courses` - получение списка курсов с их характеристиками.
```bash
curl -X POST   -d '{"status":"all"}'   "http://localhost:8080/get_courses?user_id=100"
```

`/update_course_progress` - обновление прогресса пользователя по курсу. Например, для кнопок "начать" и "завершить".
```bash
curl -X POST   -d '{"course_id":"python", "status":"in_progress"}'   "http://localhost:8080/update_course_progress?user_id=100"
```

`/update_chapter_progress` - обновление прогресса пользователя по главе. Например, для кнопки "следующая глава".
```bash
curl -X POST   -d '{"chapter_id":"python_chapter_0010", "status":"in_progress"}'   "http://localhost:8080/update_chapter_progress?user_id=100"
```


`/get_chapters` - получение списка глав и их статусов для пользоваетля.
```bash
curl -X POST   -d '{"course_id":"python"}'   "http://localhost:8080/get_chapters?user_id=100"
```

`/get_chapter` - получение главы с задачами и их статусами для пользователя.
```bash
curl -X POST   -d '{"chapter_id":"python_chapter_0010"}'   "http://localhost:8080/get_chapter?user_id=100"

curl -X POST   -d '{"course_id":"python"}'   "http://localhost:8080/get_chapter?user_id=4564"
```

`/get_progress` - получение прогресса пользователя по главе.
```bash
curl -X POST   -d '{"chapter_id":"python_chapter_0010"}'   "http://localhost:8080/get_progress?user_id=100"
```

`/get_active_chapter` - получение первой по списку главы с задачами и их статусами для пользователя, которая находится в статусе `not_started` или `in_progress`
```bash
curl -X POST   -d '{"course_id":"python"}'   "http://localhost:8080/get_active_chapter?user_id=100"
```

`/courses_stats` - статистика по прохождению курсов для пользователя. Нужна для страницы "прогресс" на сайте.
Возвращает массив с объектами: id курса, title курса, общее количество глав, количество пройденных пользователем глав, статус курса (в процессе либо завершен).
```bash
curl -X POST   -d '{}'   "http://localhost:8080/courses_stats?user_id=100"
```
Пример ответа:
```json
[{"course_id":"python","title":"Python","total_chapters":6,"finished_chapters":1,"status":"in_progress"}]
```

`/get_course_info` - возвращает в строковом виде теги курса. Теги - это именно строка, а не json. 
```bash
curl -X POST   -d '{"course_id": "python"}'   "http://localhost:8080/get_course_info"
```
Пример ответа:
```json
{"tags":"{\"url\": \"https://www.python.org/\", \"version\": \"Python 3.11.4\", \"chapters\": \"30 глав\", \"projects\": [{\"url\": \"https://docs.celeryq.dev/en/stable/\", \"name\": \"Celery — распределенная очередь задач\"}, {\"url\": \"https://www.openstack.org/\", \"name\": \"OpenStack — платформа для организации облачной инфраструктуры\"}, {\"url\": \"https://www.ansible.com/\", \"name\": \"Ansible — система управления конфигурациями\"}]}"}
```


`/get_task` - получение статуса задачи для пользователя: статус, текст решения.
```bash
curl -X POST   -d '{"task_id":"python_chapter_0010_task_0020"}'   "http://localhost:8080/get_task?user_id=100"
```
Пример удачного ответа:
```json
{"task_id":"python_chapter_0010_task_0020","task_code":"print(\"Returned HTTP code \" + str(200))","status":"completed"}
```
Пример ответа с ошибкой (если что-то пошло не так):
```json
{"error":"Couldn't get task details for: python_chapter_0010_task_0020"}
```

Внутренние апишки для сцены:
`/merge_users` - смерживание прогресса по курсам, главам и задачам для двух пользователей с последующим удалением статистики по второму пользователю. Здесь `new_user_id` присутствует, но не играет роли. 
```bash
curl -X POST   -d '{"cur_user_id": 456, "old_user_id": 982, "new_user_id": 0}'   "http://localhost:8080/merge_users"
```

`/split_users` - дублирование статистики в нового пользователя. Здесь `old_user_id` присутствует, но не играет роли.
```bash
curl -X POST   -d '{"cur_user_id": 456, "old_user_id": 0, "new_user_id": 982}'   "http://localhost:8080/split_users"
```

## Добавление модулей

Чтобы добавить сторонний модуль в go-проект, достаточно сначала импортировать его в нужном месте в коде, например:
```go
import "github.com/gorilla/mux"
```

А затем выполнить команду, которая обновит файл с зависимостями `go.mod`:
```bash
go mod tidy
```

## Полезные SQL-запросы для отладки и разворачивания базы

```sql
-- db size with indices
SELECT pg_size_pretty(pg_database_size('senjun'));


-- show running queries
SELECT pid, age(clock_timestamp(), query_start), usename, query 
FROM pg_stat_activity 
WHERE query != '<IDLE>' AND query NOT ILIKE '%pg_stat_activity%' 
ORDER BY query_start desc;

-- kill running query
SELECT pg_cancel_backend(procpid);

-- kill idle query
SELECT pg_terminate_backend(procpid);

-- vacuum command
VACUUM (VERBOSE, ANALYZE);

-- all database users
select * from pg_stat_activity where current_query not like '<%';

-- all databases and their sizes
select * from pg_user;

-- all tables and their size, with/without indexes
select datname, pg_size_pretty(pg_database_size(datname))
from pg_database
order by pg_database_size(datname) desc;

-- cache hit rates (should not be less than 0.99)
SELECT sum(heap_blks_read) as heap_read, sum(heap_blks_hit)  as heap_hit, (sum(heap_blks_hit) - sum(heap_blks_read)) / sum(heap_blks_hit) as ratio
FROM pg_statio_user_tables;

-- table index usage rates (should not be less than 0.99)
SELECT relname, 100 * idx_scan / (seq_scan + idx_scan) percent_of_times_index_used, n_live_tup rows_in_table
FROM pg_stat_user_tables 
ORDER BY n_live_tup DESC;

-- how many indexes are in cache
SELECT sum(idx_blks_read) as idx_read, sum(idx_blks_hit)  as idx_hit, (sum(idx_blks_hit) - sum(idx_blks_read)) / sum(idx_blks_hit) as ratio
FROM pg_statio_user_indexes;

-- Dump database on remote host to file
$ pg_dump -U username -h hostname databasename > dump.sql

-- Import dump into existing database
$ psql -d newdb -f dump.sql
```