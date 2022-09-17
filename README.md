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
sudo ln -s /home/your_user/senjun/courses/courses/ /etc/courses
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
```bash
curl -X POST \
  -d '{"task_id":"python_chapter_0010_task_0010", "chapter_id":"python_chapter_0010", "solution_text":"err_service_unavailable = 503"}' \
  "http://localhost:8080/run_task?user_id=mesozoic.drones"
```

