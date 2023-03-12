# Handyman
Сервис на бэке: выдача и обновление прогресса по курсам, отправка задач в watchman.
Реализован на  go 1.18.4.

## Сборка и запуск
Забрать проект к себе:
```bash
git clone https://gitlab.com/senjun/handyman.git
cd handyman
```

Собрать и запустить сервис в дебаг-сборке:
```bash
cd cmd/handyman
go run .
```

Собрать и запустить сервис в релизной сборке:
```bash
cd cmd/handyman
go build .
./handyman
```

Настроить симлинк на директорию с курсами:
```bash
cd /
sudo mkdir data
sudo ln -s /home/your_user/senjun/courses/courses/ /data
```

Для разработки можно использовать такие IDE как VSCode, LiteIDE и другие.
Иерархия директорий проекта соответствует [распространенным практикам go.](https://github.com/golang-standards/project-layout)

## Добавление модулей
Чтобы добавить сторонний модуль в go-проект, достаточно сначала импортировать его в нужном месте в коде, например:
```go
import "github.com/gorilla/mux"
```

А затем выполнить команду, которая обновит файл с зависимостями `go.mod`:
```bash
go mod tidy
```

## Апишки
`/run_task` - запуск решения пользователя для задачи курса. Решение пользователя закодировано в base64.
```bash
curl -X POST \
  -d '{"task_id":"python_chapter_0010_task_0010", "solution_text":"ZXJyX3NlcnZpY2VfdW5hdmFpbGFibGUgPSA1MDM="}' \
  "http://localhost:8080/run_task?user_id=100"
```

`/get_courses` - получение списка курсов с их характеристиками.
```bash
curl -X POST   "http://localhost:8080/get_courses"

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

## Настройка PostgreSQL в докере для отладки
Чтобы начать работать с handyman, можно запустить контейнер  с постгресом:
```bash
docker run  -e POSTGRES_PASSWORD=senjun_pass -p 5432:5432 -v postgres-senjun-data:/var/lib/postgresql/data -d postgres
```

Зайти в него, применить миграции из `etc/postgres_migrations`:
```bash
docker ps
516cd7b6f4d3   postgres     "docker-entrypoint.s…"  ...

docker exec -it 516cd7b6f4d3 bash
psql -U postgres
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