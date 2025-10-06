package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gammazero/workerpool"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

// An exported global variable to hold the database connection pool
// Completely thread-safe and ok. Fear not, my friend
var DB *sql.DB

var WP *workerpool.WorkerPool

const connStr = "postgresql://senjun:some_password@127.0.0.1:5432/senjun?sslmode=disable"

var Logger *log.Logger

// --------------- METRICS

// /run_task
var countRunTaskTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_task_total",
})

var countRunTaskOk = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_task_ok",
})

var countRunTaskErrClient = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_task_errors_client",
})

var countRunTaskErrServer = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_task_errors_server",
})

// /run_practice
var countRunPracticeTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_practice_total",
})

var countRunPracticeOk = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_practice_ok",
})

var countRunPracticeErrClient = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_practice_errors_client",
})

var countRunPracticeErrServer = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_practice_errors_server",
})

// /run_code
var countRunCodeTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_code_total",
})

var countRunCodeOk = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_code_ok",
})

var countRunCodeErrClient = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_code_errors_client",
})

var countRunCodeErrServer = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_run_code_errors_server",
})

// /get_courses
var countGetCoursesTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_courses_total",
})

var countGetCoursesAnonym = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_courses_anonym",
})

var countGetCoursesAuthorized = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_courses_authorized",
})

var countGetCoursesErrClient = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_courses_err_client",
})

var countGetCoursesErrServer = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_courses_err_server",
})

// update_course_progress
var countUpdateCourseProgressTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_total",
})

var countUpdateCourseProgressServerError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_err_server",
})

var countUpdateCourseProgressStatusError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_err_status",
})

var countUpdateCourseProgressClientError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_err_client",
})

var countUpdateCourseProgressOk = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_ok",
})

var countUpdateCourseProgressOkCompleted = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_ok_completed",
})

var countUpdateCourseProgressNoAction = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_course_progress_no_action",
})

// /update_chapter_progress
var countUpdateChapterProgressTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_total",
})

var countUpdateChapterProgressNoAction = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_no_action",
})

var countUpdateChapterProgressServerError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_err_server",
})

var countUpdateChapterProgressStatusError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_err_status",
})

var countUpdateChapterProgressClientError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_err_client",
})

var countUpdateChapterProgressOk = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_ok",
})

var countUpdateChapterProgressOkCompleted = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_update_chapter_progress_ok_completed",
})

// /get_chapter
var countGetChapterTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_chapter_total",
})

// /get_chapter
var countGetChapterAnonymous = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_chapter_anonymous",
})

var countGetChapterServerError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_chapter_err_server",
})

var countGetChapterClientError = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_chapter_err_client",
})

var countGetChapterOk = promauto.NewCounter(prometheus.CounterOpts{
	Name: "handyman_get_chapter_ok",
})

func ConnectDb() *sql.DB {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		Logger.WithFields(log.Fields{
			"error": err,
		}).Fatal("Couldn't call Open() for db")
	}

	err = db.Ping()

	if err != nil {
		Logger.WithFields(log.Fields{
			"connStr": connStr,
			"error":   err,
		}).Fatal("Couldn't communicate db")
	}

	return db
}

// Returns postgres TYPE edu_material_status
func getEduMaterialStatus(isSolved bool) string {
	if isSolved {
		return "completed"
	}

	return "in_progress"
}

func TryStartCourse(userId string, courseId string) {
	query := `
	INSERT INTO 
	course_progress(user_id, course_id, status)
	VALUES($1, $2, 'in_progress')
	ON CONFLICT ON CONSTRAINT unique_user_course_id
	DO NOTHING
`

	_, err := DB.Exec(query, userId, courseId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":   userId,
			"course_id": courseId,
			"error":     err.Error(),
		}).Error("update course status: couldn't update course status for user")
		return
	}
}

func createPlayground(playgroundId string, langId string, userId string, userCode string) error {
	if len(userId) > 0 {
		query := `
	INSERT INTO 
	playgrounds(playground_id, lang_id, user_id, user_code)
	VALUES($1, $2, $3, $4)
	ON CONFLICT ON CONSTRAINT unique_playground_id
	DO UPDATE SET 
	user_code = EXCLUDED.user_code, 
	dt_last_request = Now()
`
		_, err := DB.Exec(query, playgroundId, langId, userId, userCode)
		return err
	}

	query := `
	INSERT INTO 
	playgrounds(playground_id, lang_id, user_code)
	VALUES($1, $2, $3)
	ON CONFLICT ON CONSTRAINT unique_playground_id
	DO UPDATE SET 
	user_code = EXCLUDED.user_code, 
	dt_last_request = Now()
`
	_, err := DB.Exec(query, playgroundId, langId, userCode)
	return err

}

func GetPlaygroundCode(playroundId string) (string, error) {
	query := `
SELECT user_code FROM playgrounds where playground_id=$1
`
	var userCode string
	row := DB.QueryRow(query, playroundId)
	err := row.Scan(&userCode)
	return userCode, err
}

func SaveTask(userId string, taskId string, chapterId string, courseId string, solutionText string) bool {
	const taskStatus = "in_progress"
	const attemptsCount = 0

	query := `
		INSERT INTO 
		task_progress(user_id, task_id, status, solution_text, attempts_count)
		VALUES($1, $2, $3, $4, $5)
		ON CONFLICT ON CONSTRAINT unique_user_task_id
		DO UPDATE SET 
		status = task_progress.status, 
		solution_text = EXCLUDED.solution_text,
		attempts_count = task_progress.attempts_count
	`
	_, err := DB.Exec(query, userId, taskId, taskStatus, solutionText, attemptsCount)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id": userId,
			"task_id": taskId,
			"error":   err.Error(),
		}).Error("/save_task update task in DB: couldn't update task solution for user")
		return false
	}

	return true
}

func UpdateStatus(userId string, taskId string, chapterId string, courseId string, isSolved bool, solutionText string) bool {
	taskStatus := getEduMaterialStatus(isSolved)
	const attemptsCount = 1

	query := `
		INSERT INTO 
		task_progress(user_id, task_id, status, solution_text, attempts_count)
		VALUES($1, $2, $3, $4, $5)
		ON CONFLICT ON CONSTRAINT unique_user_task_id
		DO UPDATE SET 
		status = EXCLUDED.status, 
		solution_text = EXCLUDED.solution_text,
		attempts_count = task_progress.attempts_count + EXCLUDED.attempts_count
	`
	_, err := DB.Exec(query, userId, taskId, taskStatus, solutionText, attemptsCount)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id": userId,
			"task_id": taskId,
			"error":   err.Error(),
		}).Error("/run_task [worker pool] update task status: couldn't update task status for user")
		return false
	}

	err = UpdateChapterStatus(userId, chapterId, "in_progress")

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":    userId,
			"chapter_id": chapterId,
			"error":      err.Error(),
		}).Error("/run_task [worker pool] update chapter status: couldn't update chapter status for user")
		return false
	}

	TryStartCourse(userId, courseId)

	Logger.WithFields(log.Fields{
		"user_id": userId,
		"task_id": taskId,
		"status":  taskStatus,
	}).Info("/run_task [worker pool] update task status: completed")

	return true
}

func UpdateStatusPractice(userId string, projectId string, courseId string, isSolved bool, project string) bool {
	taskStatus := getEduMaterialStatus(isSolved)
	const attemptsCount = 1

	query := `
		INSERT INTO 
		practice_progress(user_id, project_id, status, solution_text, attempts_count)
		VALUES($1, $2, $3, $4, $5)
		ON CONFLICT ON CONSTRAINT unique_user_practice_id
		DO UPDATE SET 
		status = EXCLUDED.status, 
		solution_text = EXCLUDED.solution_text,
		attempts_count = practice_progress.attempts_count + EXCLUDED.attempts_count
	`
	_, err := DB.Exec(query, userId, projectId, taskStatus, project, attemptsCount)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":    userId,
			"project_id": projectId,
			"error":      err.Error(),
		}).Error("update practice status: couldn't update project status for user")
		return false
	}

	TryStartCourse(userId, courseId)

	Logger.WithFields(log.Fields{
		"user_id":    userId,
		"project_id": projectId,
		"status":     taskStatus,
	}).Info("update practice status: completed")

	return true
}

func SavePractice(userId string, projectId string, courseId string, project string) bool {
	query := `
		INSERT INTO 
		practice_progress(user_id, project_id, status, solution_text, attempts_count)
		VALUES($1, $2, 'in_progress', $3, 1)
		ON CONFLICT ON CONSTRAINT unique_user_practice_id
		DO UPDATE SET 
		status = practice_progress.status, 
		solution_text = EXCLUDED.solution_text,
		attempts_count = practice_progress.attempts_count
	`
	_, err := DB.Exec(query, userId, projectId, project)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":    userId,
			"project_id": projectId,
			"error":      err.Error(),
		}).Error("save practice: couldn't update project for user")
		return false
	}

	TryStartCourse(userId, courseId)

	Logger.WithFields(log.Fields{
		"user_id":    userId,
		"project_id": projectId,
	}).Info("save practice: completed")

	return true
}

func GetCourses() []CourseForUser {
	query := `
		SELECT courses.course_id, courses.path_on_disk, courses.type, courses.title, courses.tags
		FROM courses
	`

	rows, err := DB.Query(query)
	if err != nil {
		return []CourseForUser{}
	}

	defer rows.Close()

	courses := []CourseForUser{}

	for rows.Next() {
		var course CourseForUser

		if err := rows.Scan(&course.CourseId, &course.Path, &course.CourseType, &course.Title, &course.Tags); err != nil {
			Logger.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Couldn't parse row from courses selection")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetPractice(opts Options) (Practice, error) {
	query := `
	SELECT title, chapter_id, main_file, default_cmd_line_args,
		'not_started' as status
		FROM practice 
		WHERE practice.project_id=$1
`

	rows, err := DB.Query(query, opts.TaskId)
	if err != nil {
		return Practice{}, err
	}

	defer rows.Close()

	var p Practice

	if rows.Next() {
		if err := rows.Scan(&p.Title, &p.ChapterId, &p.MainFile, &p.DefaultCmdLineArgs, &p.Status); err != nil {
			Logger.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Couldn't parse row from practice selection")
			return Practice{}, err
		}
	}

	return p, nil
}

func GetPracticeForUser(opts Options) (Practice, error) {
	query := `
	SELECT title, chapter_id, main_file, default_cmd_line_args,
		(CASE WHEN practice_progress.status IS NULL THEN 'not_started' ELSE practice_progress.status::varchar(40) END) as status, 
		(CASE WHEN practice_progress.solution_text IS NULL THEN '' ELSE practice_progress.solution_text END) as project   
		FROM practice 
		LEFT JOIN practice_progress 
		ON practice_progress.user_id=$2 AND practice_progress.project_id=practice.project_id 
		WHERE practice.project_id=$1
`

	rows, err := DB.Query(query, opts.TaskId, opts.userId)
	if err != nil {
		return Practice{}, err
	}

	defer rows.Close()

	var p Practice

	if rows.Next() {
		if err := rows.Scan(&p.Title, &p.ChapterId, &p.MainFile, &p.DefaultCmdLineArgs, &p.Status, &p.Project); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": opts.userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from practice selection for user")
			return Practice{}, err
		}
	}

	return p, nil
}

func GetCourseInfo(courseId string) (string, error) {
	query := `
		SELECT tags FROM courses where course_id=$1
	`

	var tags string
	row := DB.QueryRow(query, courseId)
	err := row.Scan(&tags)
	return tags, err
}

func GetCoursesForUser(userId string) []CourseForUser {
	query := `
		SELECT courses.course_id, courses.path_on_disk, courses.type, courses.title, courses.tags,
		(CASE WHEN course_progress.status IS NULL THEN 'not_started' ELSE course_progress.status::varchar(40) END) as status 
		FROM courses LEFT JOIN course_progress 
		ON course_progress.course_id = courses.course_id AND course_progress.user_id=$1
		ORDER BY status, courses.course_id
	`

	rows, err := DB.Query(query, userId)
	if err != nil {
		return []CourseForUser{}
	}

	defer rows.Close()

	courses := []CourseForUser{}

	for rows.Next() {
		var course CourseForUser

		if err := rows.Scan(&course.CourseId, &course.Path, &course.CourseType, &course.Title, &course.Tags, &course.Status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from courses selection for user")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetCoursesForUserByStatus(userId string, status string) []CourseForUser {
	query := `
		SELECT courses.course_id, courses.path_on_disk, courses.type, courses.title, courses.tags
		FROM courses LEFT JOIN course_progress 
		ON course_progress.course_id = courses.course_id AND course_progress.user_id=$1
		WHERE status = $2
		ORDER BY courses.course_id
	`

	rows, err := DB.Query(query, userId, status)
	if err != nil {
		return []CourseForUser{}
	}

	defer rows.Close()

	courses := []CourseForUser{}

	for rows.Next() {
		var course CourseForUser

		if err := rows.Scan(&course.CourseId, &course.Path, &course.CourseType, &course.Title, &course.Tags); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from courses selection for user")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetChapters(courseId string) []ChapterForUser {
	query := `
	SELECT q.item_id, q.title, q.status, q.tasks_count FROM (
		SELECT
		chapters.chapter_id as item_id, chapters.chapter_id as item_id_sort, chapters.title as title, 'not_started' as status, 
		(SELECT COUNT(*) FROM tasks WHERE tasks.chapter_id = chapters.chapter_id) AS tasks_count, 1 as x
		FROM chapters 
		WHERE course_id=$1
		UNION
		SELECT
		practice.project_id as item_id, practice.chapter_id as item_id_sort, practice.title as title, 'not_started' as status, 1 as tasks_count, 2 as x
		FROM practice
		WHERE course_id=$1
		) as q ORDER BY q.item_id_sort, q.x
	`

	rows, err := DB.Query(query, courseId)
	if err != nil {
		return []ChapterForUser{}
	}

	defer rows.Close()

	chapters := []ChapterForUser{}

	for rows.Next() {
		var chapter ChapterForUser

		if err := rows.Scan(&chapter.ChapterId, &chapter.Title, &chapter.Status, &chapter.TasksTotal); err != nil {
			Logger.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Couldn't parse row from chapters selection")
			return []ChapterForUser{}
		}

		chapters = append(chapters, chapter)
	}

	return chapters
}

func GetChaptersForUser(userId string, courseId string) []ChapterForUser {
	query := `
	SELECT q.item_id, q.title, q.status, q.tasks_count, q.tasks_count_completed FROM (
		SELECT
		chapters.chapter_id as item_id, chapters.title as title, 
		(  SELECT (CASE WHEN chapter_progress.status IS NULL THEN 'not_started' ELSE chapter_progress.status::varchar(40) END) as status 
		   FROM chapter_progress 
		   WHERE chapters.chapter_id = chapter_progress.chapter_id AND chapter_progress.user_id=$1
		) as status,
		(  SELECT COUNT(*) FROM tasks WHERE tasks.chapter_id = chapters.chapter_id
		) AS tasks_count,
		(  SELECT COUNT(*) FROM task_progress WHERE task_progress.task_id like CONCAT(chapters.chapter_id, '_task%') 
		   AND task_progress.status='completed' AND task_progress.user_id=$1
		) AS tasks_count_completed, chapters.chapter_id as item_id_sort, 1 as x
		FROM chapters
		WHERE course_id=$2
		UNION
		SELECT
		practice.project_id as item_id, practice.title as title, 
		(  SELECT (CASE WHEN practice_progress.status IS NULL THEN 'not_started' ELSE practice_progress.status::varchar(40) END) as status 
		   FROM practice_progress 
		   WHERE practice.project_id = practice_progress.project_id AND practice_progress.user_id=$1
		) as status,
		1 AS tasks_count,
		(  SELECT COUNT(*) FROM practice_progress WHERE practice_progress.project_id = practice.project_id
		   AND practice_progress.status='completed' AND practice_progress.user_id=$1
		) AS tasks_count_completed, practice.chapter_id as item_id_sort, 2 as x
		FROM practice
		WHERE course_id=$2
		) as q ORDER BY q.item_id_sort, q.x
	`

	rows, err := DB.Query(query, userId, courseId)
	if err != nil {
		return []ChapterForUser{}
	}

	defer rows.Close()

	chapters := []ChapterForUser{}

	for rows.Next() {
		var chapter ChapterForUser

		var status sql.NullString
		if err := rows.Scan(&chapter.ChapterId, &chapter.Title, &status, &chapter.TasksTotal, &chapter.TasksCompleted); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from chapters selection for user")
			return []ChapterForUser{}
		}

		if status.Valid {
			chapter.Status = status.String
		} else {
			chapter.Status = "not_started"
		}

		chapters = append(chapters, chapter)
	}

	return chapters
}

func GetFirstChapterId(courseId string) (string, error) {
	query := `
		SELECT chapter_id FROM chapters WHERE course_id=$1 ORDER BY chapter_id ASC LIMIT 1
	`
	var firstChapterId string

	row := DB.QueryRow(query, courseId)
	err := row.Scan(&firstChapterId)
	return firstChapterId, err
}

func GetChapterTitle(chapterId string) (string, error) {
	query := `
		SELECT title FROM chapters WHERE chapter_id = $1 LIMIT 1
	`
	var title string

	row := DB.QueryRow(query, chapterId)
	err := row.Scan(&title)

	return title, err
}

func GetNextChapterId(courseId string, chapterId string, getPractice bool) (string, error) {
	query := `
		SELECT chapter_id FROM chapters WHERE course_id=$1 AND chapter_id > $2 ORDER BY chapter_id ASC LIMIT 1
	`
	var nextChapterId string

	row := DB.QueryRow(query, courseId, chapterId)
	err := row.Scan(&nextChapterId)

	if getPractice && err == nil {
		query := `
		SELECT project_id FROM practice WHERE course_id=$1 AND chapter_id = $2 ORDER BY project_id ASC LIMIT 1
	`
		row := DB.QueryRow(query, courseId, chapterId)
		var nextProjectId string
		errProj := row.Scan(&nextProjectId)

		if errProj == nil {
			return nextProjectId, errProj
		}
	}

	return nextChapterId, err
}

func GetChapterProgress(userId string, chapterId string) (string, error) {
	query := `
		SELECT status FROM chapter_progress WHERE user_id=$1 AND chapter_id=$2
	`
	var status string

	row := DB.QueryRow(query, userId, chapterId)
	err := row.Scan(&status)
	return status, err
}

func GetChapterInfo(userId string, chapterId string) (string, string, error) {
	// Anonymous user
	if len(userId) == 0 {
		query := `
		SELECT
		title
		FROM chapters
		WHERE  chapters.chapter_id=$1
	`
		var title string

		row := DB.QueryRow(query, chapterId)
		err := row.Scan(&title)
		return "not_started", title, err
	}

	query := `
		SELECT
		(CASE WHEN chapter_progress.status IS NULL THEN 'not_started' ELSE chapter_progress.status::varchar(40) END),
		chapters.title
		FROM chapters 
		LEFT JOIN chapter_progress 
		ON chapters.chapter_id = chapter_progress.chapter_id AND user_id=$1
		WHERE  chapters.chapter_id=$2
	`
	var status string
	var title string

	row := DB.QueryRow(query, userId, chapterId)
	err := row.Scan(&status, &title)
	return status, title, err
}

func GetChapterForUser(opts Options) (ChapterContent, error) {
	var chapterContent ChapterContent

	if len(opts.ChapterId) == 0 {
		// We need to get the first chapter in course id
		activeChapterId, err := GetFirstChapterId(opts.CourseId)
		if err != nil {
			return ChapterContent{}, err
		}
		chapterContent.ChapterId = activeChapterId
	} else {
		chapterContent.ChapterId = opts.ChapterId
	}

	status, title, err := GetChapterInfo(opts.userId, chapterContent.ChapterId)
	if err != nil {
		return ChapterContent{}, err
	}
	chapterContent.Status = status
	chapterContent.Title = title

	contentPath, keywordsPath := GetPathToChapterText(opts.CourseId, chapterContent.ChapterId)

	chapterText, _ := ReadTextFile(contentPath)
	keywordsText, err := ReadTextFile(keywordsPath)
	if err == nil {
		chapterContent.Keywords = keywordsText
	}
	chapterContent.Content = chapterText
	chapterContent.Tasks = GetTasks(chapterContent.ChapterId, opts.userId)

	return chapterContent, nil
}

func GetCourseProgressForUser(courseId string, userId string) (string, error) {
	query := `
		SELECT status FROM course_progress WHERE course_id=$1 AND user_id=$2
	`
	var status string
	rows, err := DB.Query(query, courseId, userId)
	if err != nil {
		return "", err
	}

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id":   userId,
				"course_id": courseId,
				"error":     err.Error(),
			}).Info("Couldn't get status from course_progress table")
			return "", err
		}
	}

	return status, nil
}

func GetTasks(chapterId string, userId string) []TaskForUser {
	query := ""

	// Anonymous user
	if len(userId) == 0 {
		query = `
		SELECT 
		tasks.task_id, 'not_started', ''
		FROM tasks 
		WHERE chapter_id = $2
	`
	} else {
		query = `
		SELECT 
		tasks.task_id,
		(CASE WHEN task_progress.status IS NULL THEN 'not_started' ELSE task_progress.status::varchar(40) END),
		(CASE WHEN task_progress.solution_text IS NULL THEN '' ELSE task_progress.solution_text::varchar END)
		FROM tasks 
		LEFT JOIN task_progress
		ON tasks.task_id = task_progress.task_id AND user_id = $1
		WHERE chapter_id = $2
	`
	}

	rows, err := DB.Query(query, userId, chapterId)
	if err != nil {
		return []TaskForUser{}
	}

	defer rows.Close()

	tasks := []TaskForUser{}

	for rows.Next() {
		var task TaskForUser

		if err := rows.Scan(&task.TaskId, &task.Status, &task.UserCode); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from tasks selection for user")
			return []TaskForUser{}
		}

		tasks = append(tasks, task)
	}

	return tasks
}

func UpdateCourseProgressForUser(courseId string, status string, userId string) error {
	const query = `
		INSERT INTO course_progress(user_id, course_id, status) 
		VALUES($1, $2, $3) 
		ON CONFLICT ON CONSTRAINT unique_user_course_id
		DO UPDATE SET 
		status = EXCLUDED.status
	`
	_, err := DB.Exec(query, userId, courseId, status)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":   userId,
			"course_id": courseId,
			"db_error":  err.Error(),
		}).Error("Couldn't update course progress for user")
		return err
	}

	return nil
}

func GetCourseStatuses(userId string) []CourseStatus {
	query := `
	SELECT course_id, 
	(SELECT title from courses where courses.course_id = course_progress.course_id) as title, 
	(SELECT count(*) from chapters where chapters.course_id = course_progress.course_id) as chapters_total,
	(SELECT count(*) from chapter_progress where chapter_progress.user_id = $1 and 
	chapter_progress.status = 'completed' 
	and chapter_progress.chapter_id like concat(course_id, '_chapter_%')) as chapters_completed,
	status FROM course_progress
	WHERE user_id = $1 and status in ('in_progress', 'completed')
	`

	rows, err := DB.Query(query, userId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id": userId,
			"error":   err.Error(),
		}).Warning("Couldn't get course statuses")

		return []CourseStatus{}
	}

	defer rows.Close()

	courseStatuses := []CourseStatus{}

	for rows.Next() {
		var cs CourseStatus

		if err := rows.Scan(&cs.CourseId, &cs.Title, &cs.TotalChapters, &cs.FinishedChapters, &cs.Status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from courses selection for user")
			return []CourseStatus{}
		}

		courseStatuses = append(courseStatuses, cs)
	}

	return courseStatuses
}

func MergeUserCourses(tx *sql.Tx, ctx context.Context, userIdCur int, userIdOld int) int {
	query := `
	SELECT course_id, status FROM course_progress WHERE user_id = $1
	`

	rows, err := DB.Query(query, userIdOld)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_old": userIdOld,
			"error":       err.Error(),
		}).Warning("Couldn't get course_progress")
		return -1
	}

	defer rows.Close()

	for rows.Next() {
		var courseId string
		var status string
		if err := rows.Scan(&courseId, &status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id_old": userIdOld,
				"error":       err.Error(),
			}).Warning("Couldn't get row from course_progress")
			return -1
		}

		query = `
			INSERT INTO 
			course_progress(user_id, course_id, status)
			VALUES($1, $2, $3)
			ON CONFLICT ON CONSTRAINT unique_user_course_id
			DO UPDATE SET 
			status = max_edu_status(EXCLUDED.status, course_progress.status)
		`
		_, err := tx.ExecContext(ctx, query, userIdCur, courseId, status)
		if err != nil {
			Logger.WithFields(log.Fields{
				"user_id_cur": userIdCur,
				"course_id":   courseId,
				"status":      status,
				"db_error":    err.Error(),
			}).Error("Couldn't update course status for user")
			return -1
		}
	}

	query = `DELETE FROM course_progress WHERE user_id = $1`
	_, err = tx.ExecContext(ctx, query, userIdOld)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_old": userIdOld,
			"db_error":    err.Error(),
		}).Error("Couldn't delete course records for user")
		return -1
	}

	return 0
}

func MergeUserChapters(tx *sql.Tx, ctx context.Context, userIdCur int, userIdOld int) int {
	query := `
	SELECT chapter_id, status FROM chapter_progress WHERE user_id = $1
	`

	rows, err := DB.Query(query, userIdOld)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_old": userIdOld,
			"error":       err.Error(),
		}).Warning("Couldn't get chapter_progress")
		return -1
	}

	defer rows.Close()

	for rows.Next() {
		var chapterId string
		var status string
		if err := rows.Scan(&chapterId, &status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id_old": userIdOld,
				"error":       err.Error(),
			}).Warning("Couldn't get row from chapter_progress")
			return -1
		}

		query = `
			INSERT INTO 
			chapter_progress(user_id, chapter_id, status)
			VALUES($1, $2, $3)
			ON CONFLICT ON CONSTRAINT unique_user_chapter_id
			DO UPDATE SET 
			status = max_edu_status(EXCLUDED.status, chapter_progress.status)
		`
		_, err := tx.ExecContext(ctx, query, userIdCur, chapterId, status)
		if err != nil {
			Logger.WithFields(log.Fields{
				"user_id_cur": userIdCur,
				"chapter_id":  chapterId,
				"status":      status,
				"db_error":    err.Error(),
			}).Error("Couldn't update chapter status for user")
			return -1
		}
	}

	query = `DELETE FROM chapter_progress WHERE user_id = $1`
	_, err = tx.ExecContext(ctx, query, userIdOld)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_old": userIdOld,
			"db_error":    err.Error(),
		}).Error("Couldn't delete chapter records for user")
		return -1
	}

	return 0
}

func MergeUserTasks(tx *sql.Tx, ctx context.Context, userIdCur int, userIdOld int) int {
	query := `
	SELECT task_id, status, solution_text, attempts_count FROM task_progress WHERE user_id = $1
	`

	rows, err := DB.Query(query, userIdOld)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_old": userIdOld,
			"error":       err.Error(),
		}).Warning("Couldn't get task_progress")
		return -1
	}

	defer rows.Close()

	for rows.Next() {
		var taskId string
		var status string
		var solutionText string
		var attemptsCount int
		if err := rows.Scan(&taskId, &status, &solutionText, &attemptsCount); err != nil {
			Logger.WithFields(log.Fields{
				"user_id_old": userIdOld,
				"error":       err.Error(),
			}).Warning("Couldn't get row from task_progress")
			return -1
		}

		query = `
			INSERT INTO 
			task_progress(user_id, task_id, status, solution_text, attempts_count)
			VALUES($1, $2, $3, $4, $5)
			ON CONFLICT ON CONSTRAINT unique_user_task_id
			DO UPDATE SET 
			status = max_edu_status(EXCLUDED.status, task_progress.status),
			attempts_count = task_progress.attempts_count + EXCLUDED.attempts_count,
			solution_text = best_solution(EXCLUDED.status, EXCLUDED.solution_text, task_progress.status, task_progress.solution_text)
		`
		_, err := tx.ExecContext(ctx, query, userIdCur, taskId, status, solutionText, attemptsCount)
		if err != nil {
			Logger.WithFields(log.Fields{
				"user_id_cur": userIdCur,
				"task_id":     taskId,
				"status":      status,
				"db_error":    err.Error(),
			}).Error("Couldn't insert into task_progress for user")
			return -1
		}
	}

	query = `DELETE FROM task_progress WHERE user_id = $1`
	_, err = tx.ExecContext(ctx, query, userIdOld)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_old": userIdOld,
			"db_error":    err.Error(),
		}).Error("Couldn't delete task records for user")
		return -1
	}

	return 0
}

func SplitUsers(userIdCur int, userIdNew int) int {
	ctx := context.Background()
	tx, err := DB.BeginTx(ctx, nil)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_new": userIdNew,
			"error":       err.Error(),
		}).Error("/split_users [SplitUsers()]: couldn't start transaction")
		return -1
	}

	// Split courses
	query := `INSERT INTO course_progress(user_id, course_id, status) 
	SELECT $2, course_id, status FROM course_progress WHERE user_id = $1`

	_, err = tx.ExecContext(ctx, query, userIdCur, userIdNew)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_new": userIdNew,
			"error":       err.Error(),
		}).Error("/split_users [SplitUsers()]: couldn't split course records for user")
		return -1
	}

	// Split chapters
	query = `INSERT INTO chapter_progress(user_id, chapter_id, status) 
	SELECT $2, chapter_id, status FROM chapter_progress WHERE user_id = $1`

	_, err = tx.ExecContext(ctx, query, userIdCur, userIdNew)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_new": userIdNew,
			"db_error":    err.Error(),
		}).Error("/split_users [SplitUsers()]: couldn't split chapter records for user")
		return -1
	}

	// Split tasks
	query = `INSERT INTO task_progress(user_id, task_id, status, solution_text, attempts_count) 
	SELECT $2, task_id, status, solution_text, attempts_count FROM task_progress WHERE user_id = $1`

	_, err = tx.ExecContext(ctx, query, userIdCur, userIdNew)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_new": userIdNew,
			"db_error":    err.Error(),
		}).Error("/split_users [SplitUsers()]: couldn't split task records for user")
		return -1
	}

	err = tx.Commit()
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_new": userIdNew,
			"db_error":    err.Error(),
		}).Error("/split_users [SplitUsers()]: couldn't commit transaction")
		return -1
	}

	return 0
}

func MergeUsers(userIdCur int, userIdOld int) int {
	ctx := context.Background()
	tx, err := DB.BeginTx(ctx, nil)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
			"error":       err.Error(),
		}).Error("/merge_users [MergeUsers()]: couldn't start transaction")
		return -1
	}

	if MergeUserCourses(tx, ctx, userIdCur, userIdOld) != 0 {
		tx.Rollback()
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
		}).Error("/merge_users [MergeUsers()]: couldn't merge user courses")
		return -1
	}

	if MergeUserChapters(tx, ctx, userIdCur, userIdOld) != 0 {
		tx.Rollback()
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
		}).Error("/merge_users [MergeUsers()]: couldn't merge user chapters")
		return -1
	}

	if MergeUserTasks(tx, ctx, userIdCur, userIdOld) != 0 {
		tx.Rollback()
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
		}).Error("/merge_users [MergeUsers()]: couldn't merge user tasks")
		return -1
	}

	err = tx.Commit()
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
			"error":       err.Error(),
		}).Error("/merge_users [MergeUsers()]: couldn't commit transaction")
		return -1
	}

	return 0
}

func GetTaskForUser(userId string, taskId string) (TaskForUser, error) {
	query := `
	SELECT status, solution_text FROM task_progress WHERE user_id = $1 and task_id = $2
	`
	rows, err := DB.Query(query, userId, taskId)
	if err != nil {
		return TaskForUser{}, err
	}

	var task TaskForUser
	task.TaskId = taskId
	task.Status = "not_started"

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&task.Status, &task.UserCode); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"task_id": taskId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from task_progress selection for user")
			return TaskForUser{}, err
		}
	}

	return task, nil
}

func AreAllChaptersInCourseCompleted(userId string, courseId string) (bool, error) {
	query := `
	SELECT 
	(SELECT count(*) FROM chapters WHERE course_id = $1) AS chapters_total,
	(SELECT count(*) FROM chapter_progress WHERE chapter_id like CONCAT($1, '_chapter%') and user_id = $2 and status = 'completed') AS chapters_completed
	`
	rows, err := DB.Query(query, courseId, userId)
	if err != nil {
		return false, err
	}

	chaptersTotal := 0
	chaptersCompleted := 0

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&chaptersTotal, &chaptersCompleted); err != nil {
			Logger.WithFields(log.Fields{
				"user_id":   userId,
				"course_id": courseId,
				"error":     err.Error(),
			}).Warning("AreAllChaptersInCourseCompleted: couldn't parse response from postgres for user")
			return false, err
		}
	}

	Logger.WithFields(log.Fields{
		"user_id":            userId,
		"course_id":          courseId,
		"chapters_total":     chaptersTotal,
		"chapters_completed": chaptersCompleted,
	}).Info("AreAllChaptersInCourseCompleted: retrieved stats")
	return chaptersTotal == chaptersCompleted, nil
}

func GetPracticeProjects(userId string, courseId string) []PracticeProject {
	query := `
    SELECT practice.project_id, title,
        (CASE WHEN practice_progress.status IS NULL THEN 'not_started' ELSE practice_progress.status::varchar(40) END) as status
        FROM practice 
        LEFT JOIN practice_progress 
        ON practice_progress.user_id=$2 AND practice_progress.project_id=practice.project_id
		WHERE practice.course_id=$1
        ORDER BY practice.chapter_id
	`
	rows, err := DB.Query(query, courseId, userId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":   userId,
			"course_id": courseId,
			"error":     err.Error(),
		}).Error("Couldn't query from GetUnfinishedPracticeProjects selection for user")
		return []PracticeProject{}
	}

	defer rows.Close()

	projects := []PracticeProject{}

	for rows.Next() {
		var project PracticeProject

		if err := rows.Scan(&project.ProjectId, &project.Title, &project.Status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id":   userId,
				"course_id": courseId,
				"error":     err.Error(),
			}).Error("Couldn't parse row from GetUnfinishedPracticeProjects selection for user")
			return []PracticeProject{}
		}

		projects = append(projects, project)
	}

	return projects
}

func HandleGetCourses(w http.ResponseWriter, r *http.Request) {
	countGetCoursesTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)

	if err != nil {
		countGetCoursesErrClient.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"status":  opts.Status,
			"error":   err.Error(),
		}).Warning("/get_courses: couldn't parse request")
		return
	}

	var courses []CourseForUser

	// Case when user is not authorized
	if len(opts.userId) == 0 {
		countGetCoursesAnonym.Inc()
		courses = GetCourses()
	} else {
		countGetCoursesAuthorized.Inc()
		if opts.Status == "all" {
			courses = GetCoursesForUser(opts.userId)
		} else {
			courses = GetCoursesForUserByStatus(opts.userId, opts.Status)
		}
	}

	for i := 0; i < len(courses); i++ {
		descr, _ := ReadTextFile(filepath.Join(courses[i].Path, "description.md"))
		courses[i].Description = descr
	}

	if len(courses) == 0 {
		countGetCoursesErrServer.Inc()
	}

	Logger.WithFields(log.Fields{
		"user_id":               opts.userId,
		"status":                opts.Status,
		"retrieved_courses_len": len(courses),
	}).Info("/get_courses: completed")

	json.NewEncoder(w).Encode(courses)
}

func HandleUpdateCourseProgress(w http.ResponseWriter, r *http.Request) {
	countUpdateCourseProgressTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		countUpdateCourseProgressClientError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"status":    opts.Status,
			"error":     err.Error(),
		}).Warning("/update_course_progress: couldn't parse request")
		return
	}

	if len(opts.userId) == 0 || len(opts.Status) == 0 || len(opts.CourseId) == 0 {
		countUpdateCourseProgressClientError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Required fields are not set in request",
		})

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"status":    opts.Status,
			"error":     err.Error(),
		}).Warning("/update_course_progress: required fields not set in request")
		return
	}

	curStatus, err := GetCourseProgressForUser(opts.CourseId, opts.userId)
	if err != nil {
		countUpdateCourseProgressServerError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user progress on course",
		})

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"status":    opts.Status,
			"error":     err.Error(),
		}).Warning("/update_course_progress: couldn't get progress on course for user")
		return
	}

	if !IsNewStatusValid(curStatus, opts.Status) {
		if opts.Status == "in_progress" && (curStatus == "in_progress" || curStatus == "completed") {
			json.NewEncoder(w).Encode(map[string]string{
				"status": "no_action",
			})

			Logger.WithFields(log.Fields{
				"user_id":        opts.userId,
				"course_id":      opts.CourseId,
				"current_status": curStatus,
				"new_status":     opts.Status,
			}).Info("/update_course_progress: no action")

			countUpdateCourseProgressNoAction.Inc()
			return

		}
		countUpdateCourseProgressStatusError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error":          "Couldn't change status",
			"current_status": curStatus,
			"new_status":     opts.Status,
		})

		Logger.WithFields(log.Fields{
			"user_id":        opts.userId,
			"course_id":      opts.CourseId,
			"current_status": curStatus,
			"new_status":     opts.Status,
		}).Info("/update_course_progress: invalid course status transmission for user")
		return
	}

	if opts.Status == "completed" {
		isCourseCompleted, _ := AreAllChaptersInCourseCompleted(opts.userId, opts.CourseId)

		if isCourseCompleted {
			countUpdateCourseProgressOkCompleted.Inc()
		} else {
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not all materials in course are completed",
			})

			Logger.WithFields(log.Fields{
				"user_id":        opts.userId,
				"course_id":      opts.CourseId,
				"current_status": curStatus,
				"new_status":     opts.Status,
			}).Info("/update_course_progress: not all materials in course are completed")
			return
		}
	}

	err = UpdateCourseProgressForUser(opts.CourseId, opts.Status, opts.userId)
	if err != nil {
		countUpdateCourseProgressServerError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't update user progress on course",
		})

		Logger.WithFields(log.Fields{
			"user_id":        opts.userId,
			"course_id":      opts.CourseId,
			"current_status": curStatus,
			"new_status":     opts.Status,
			"error":          err.Error(),
		}).Error("/update_course_progress: couldn't update user progress on course")
		return
	}

	Logger.WithFields(log.Fields{
		"user_id":    opts.userId,
		"course_id":  opts.CourseId,
		"old_status": curStatus,
		"new_status": opts.Status,
	}).Info("/update_course_progress: completed")

	countUpdateCourseProgressOk.Inc()

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func UpdateChapterStatus(userId string, chapterId string, status string) error {
	const query = `
		INSERT INTO chapter_progress(user_id, chapter_id, status) 
		VALUES($1, $2, $3) 
		ON CONFLICT ON CONSTRAINT unique_user_chapter_id 
		DO UPDATE SET
		status = EXCLUDED.status
    `

	_, err := DB.Exec(query, userId, chapterId, status)
	return err
}

func AddUserInteraction(userId string, k string, v string) error {
	const query = `
		INSERT INTO user_interactions(user_id, interaction_key, interaction_val) 
		VALUES($1, $2, $3) 
    `
	_, err := DB.Exec(query, userId, k, v)
	return err
}

func HandleUpdateChapterProgress(w http.ResponseWriter, r *http.Request) {
	countUpdateChapterProgressTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		countUpdateChapterProgressClientError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"status":     opts.Status,
			"error":      err.Error(),
		}).Warning("/update_chapter_progress: couldn't parse request")
		return
	}

	if len(opts.userId) == 0 || len(opts.ChapterId) == 0 || len(opts.Status) == 0 {
		countUpdateChapterProgressClientError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id, chapter_id or status",
		})

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"status":     opts.Status,
		}).Warning("/update_chapter_progress: required fields not set in request")
		return
	}

	curStatus, err := GetChapterProgress(opts.userId, opts.ChapterId)
	if err != nil {
		if err != sql.ErrNoRows {
			countUpdateChapterProgressServerError.Inc()

			json.NewEncoder(w).Encode(map[string]string{
				"error": "Couldn't get user progress on chapter",
			})

			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"chapter_id": opts.ChapterId,
				"error":      err.Error(),
			}).Warning("/update_chapter_progress: couldn't get user progress on chapter")
			return
		}

		curStatus = "not_started"
	}

	if !IsNewStatusValid(curStatus, opts.Status) {
		if opts.Status == "in_progress" && (curStatus == "in_progress" || curStatus == "completed") {
			chapters := GetChaptersForUser(opts.userId, opts.CourseId)
			ret_chapter_id := opts.ChapterId

			for i := 0; i < len(chapters); i++ {
				if chapters[i].Status == "in_progress" || chapters[i].Status == "not_started" {
					ret_chapter_id = chapters[i].ChapterId
					break
				}
			}

			Logger.WithFields(log.Fields{
				"user_id":           opts.userId,
				"chapter_id":        opts.ChapterId,
				"status":            opts.Status,
				"action":            "not_changed",
				"return_chapter_id": ret_chapter_id,
			}).Info("/update_chapter_progress: no action")

			json.NewEncoder(w).Encode(map[string]string{
				"status":     "no_action",
				"chapter_id": ret_chapter_id,
				"course_id":  opts.CourseId,
			})

			countUpdateChapterProgressNoAction.Inc()
			return
		}

		countUpdateChapterProgressStatusError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error":          "Couldn't change status",
			"current_status": curStatus,
			"new_status":     opts.Status,
			"chapter_id":     opts.ChapterId,
			"course_id":      opts.CourseId,
		})

		Logger.WithFields(log.Fields{
			"user_id":        opts.userId,
			"chapter_id":     opts.ChapterId,
			"current_status": curStatus,
			"new_status":     opts.Status,
		}).Info("/update_chapter_progress: invalid chapter status transmission for user")
		return
	}

	if opts.Status == "completed" {
		tasks := GetTasks(opts.ChapterId, opts.userId)

		for _, task := range tasks {
			if task.Status != "completed" {
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Not all tasks in chapter are completed",
				})

				Logger.WithFields(log.Fields{
					"user_id":    opts.userId,
					"chapter_id": opts.ChapterId,
					"new_status": opts.Status,
					"task_id":    task.TaskId,
					"status":     task.Status,
				}).Info("/update_chapter_progress: do nothing because not all tasks are completed")
				return
			}
		}
	}

	Logger.WithFields(log.Fields{
		"user_id":    opts.userId,
		"new_status": opts.Status,
		"chapter_id": opts.ChapterId,
	}).Info("/update_chapter_progress: BEFORE UPDATING IN DB")

	err = UpdateChapterStatus(opts.userId, opts.ChapterId, opts.Status)

	if err != nil {
		countUpdateChapterProgressServerError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error":      "Couldn't update chapter status for user",
			"chapter_id": opts.ChapterId,
			"course_id":  opts.CourseId,
		})

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"old_status": curStatus,
			"new_status": opts.Status,
			"error":      err.Error(),
		}).Error("/update_chapter_progress: couldn't update chapter status for user")
		return
	}

	TryStartCourse(opts.userId, opts.CourseId)

	if opts.Status == "completed" {
		countUpdateChapterProgressOkCompleted.Inc()
	}

	countUpdateChapterProgressOk.Inc()

	Logger.WithFields(log.Fields{
		"user_id":    opts.userId,
		"old_status": curStatus,
		"new_status": opts.Status,
		"chapter_id": opts.ChapterId,
	}).Info("/update_chapter_progress: completed")

	json.NewEncoder(w).Encode(map[string]string{
		"status":     "ok",
		"chapter_id": opts.ChapterId,
		"course_id":  opts.CourseId,
	})
}

func HandleGetChapters(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Warning("/get_chapters: couldn't parse request")
		return
	}

	if len(opts.CourseId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get course_id in get_chapters",
		})

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
		}).Warning("/get_chapters: required fields not set in request")
		return
	}

	// User is not authorized
	if len(opts.userId) == 0 {
		chapters := GetChapters(opts.CourseId)

		Logger.WithFields(log.Fields{
			"course_id": opts.CourseId,
		}).Info("/get_chapters: completed for not authorized user")

		json.NewEncoder(w).Encode(chapters)
		return
	}

	chapters := GetChaptersForUser(opts.userId, opts.CourseId)

	Logger.WithFields(log.Fields{
		"user_id":   opts.userId,
		"course_id": opts.CourseId,
	}).Info("/get_chapters: completed")

	json.NewEncoder(w).Encode(chapters)
}

func HandleGetCourseInfo(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Warning("/get_course_info: couldn't parse request")
		return
	}

	if len(opts.CourseId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get required request params",
		})

		Logger.WithFields(log.Fields{
			"course_id": opts.CourseId,
		}).Warning("/get_course_info: required fields not set in request")
		return
	}

	tags_str, err := GetCourseInfo(opts.CourseId)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't get course info",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Warning("/get_course_info: couldn't get course info")
		return
	}

	body, _ := json.Marshal(map[string]string{
		"tags": tags_str,
	})
	w.Write(body)
}

func HandlePracticeCode(w http.ResponseWriter, r *http.Request) {
	countRunPracticeTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	var opts PracticeReq
	opts.userId = GetUserId(r)

	err := json.NewDecoder(r.Body).Decode(&opts)

	if err != nil {
		countRunPracticeErrClient.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"project_id": opts.ProjectId,
			"course_id":  opts.CourseId,
			"action":     opts.Action,
			"error":      err.Error(),
		}).Warning("/handle_practice_code: couldn't parse request")

		return
	}

	Logger.WithFields(log.Fields{
		"user_id":    opts.userId,
		"project_id": opts.ProjectId,
		"course_id":  opts.CourseId,
		"action":     opts.Action,
	}).Warning("/handle_practice_code: parsed request")

	if len(opts.userId) == 0 || len(opts.ProjectId) == 0 || len(opts.CourseId) == 0 {
		countRunPracticeErrClient.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get some fields",
		})

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"project_id": opts.ProjectId,
			"course_id":  opts.CourseId,
			"action":     opts.Action,
		}).Warning("/handle_practice_code: couldn't get required fields")
		return
	}

	res := new(RunTaskResult)

	if opts.Action == "save" {
		if SavePractice(opts.userId, opts.ProjectId, opts.CourseId, opts.ProjectContents) {
			countRunPracticeOk.Inc()
		} else {
			countRunPracticeErrServer.Inc()
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Couldn't save project",
			})

			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"project_id": opts.ProjectId,
				"course_id":  opts.CourseId,
				"action":     opts.Action,
			}).Warning("/handle_practice_code: couldn't save project")
			return
		}
	} else {
		bodyReq, err := json.Marshal(opts)

		if err != nil {
			countRunPracticeErrServer.Inc()

			body, _ := json.Marshal(map[string]string{
				"error": "Couldn't communicate with tasks runner",
			})
			w.Write(body)

			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"project_id": opts.ProjectId,
				"action":     opts.Action,
				"error":      err.Error(),
			}).Error("/handle_practice_code: error creating json for watchman")
			return
		}

		bodyResp, err := sendRequestToWatchman(addrWatchmanPractice, &bodyReq)

		if err != nil {
			countRunPracticeErrServer.Inc()

			body, _ := json.Marshal(map[string]string{
				"error": "Couldn't communicate with tasks runner",
			})
			w.Write(body)

			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"project_id": opts.ProjectId,
				"action":     opts.Action,
				"error":      err.Error(),
			}).Error("/handle_practice_code: error communicating watchman")
			return
		}

		err = json.Unmarshal(bodyResp, &res)

		if err != nil {
			countRunPracticeErrServer.Inc()

			body, _ := json.Marshal(map[string]string{
				"error": "Couldn't communicate with tasks runner",
			})
			w.Write(body)

			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"project_id": opts.ProjectId,
				"action":     opts.Action,
				"error":      err.Error(),
			}).Error("/handle_practice_code: error extracting json from watchman resp")
			return
		}

		if UpdateStatusPractice(opts.userId, opts.ProjectId, opts.CourseId,
			opts.Action == "test" && res.StatusCode == 0, opts.ProjectContents) {
			countRunPracticeOk.Inc()
		} else {
			countRunPracticeErrServer.Inc()
		}
	}

	Logger.WithFields(log.Fields{
		"user_id":     opts.userId,
		"project_id":  opts.ProjectId,
		"course_id":   opts.CourseId,
		"action":      opts.Action,
		"status_code": res.StatusCode,
	}).Info("/handle_practice_code: completed")

	json.NewEncoder(w).Encode(res)
}

func HandleGetPractice(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	var opts Options
	opts.userId = GetUserId(r)

	err := json.NewDecoder(r.Body).Decode(&opts)

	if err != nil {
		//countGetChapterClientError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"task_id":   opts.TaskId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Warning("/get_practice: couldn't parse request")
		return
	}

	if len(opts.CourseId) == 0 || len(opts.TaskId) == 0 {
		//countGetChapterClientError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get required request params",
		})

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"task_id":   opts.TaskId,
			"course_id": opts.CourseId,
		}).Warning("/get_practice: required fields not set in request")
		return
	}

	var practice Practice

	if len(opts.userId) > 0 {
		practice, err = GetPracticeForUser(opts)
	} else {
		practice, err = GetPractice(opts)
	}

	if err != nil {
		//countGetChapterServerError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't get practice for user",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"task_id":   opts.TaskId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Error("/get_practice: couldn't get practice for user")
		return
	}

	practice.NextChapterId, _ = GetNextChapterId(opts.CourseId, practice.ChapterId, false)

	pathToText := filepath.Join(rootCourses, opts.CourseId, "practice", opts.TaskId, "text.md")
	practice.ProjectDescription, err = ReadTextFile(pathToText)

	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't get practice for user",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"task_id":   opts.TaskId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Error("/get_practice: couldn't get practice text for user")
		return
	}

	pathToHint := filepath.Join(rootCourses, opts.CourseId, "practice", opts.TaskId, "hint.md")
	practice.ProjectHint, err = ReadTextFile(pathToHint)

	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't get practice hint for user",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"task_id":   opts.TaskId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Error("/get_practice: couldn't get practice hint for user")
		return
	}

	practice.ProjectPath = filepath.Join(rootCourses, opts.CourseId, "practice", opts.TaskId, "project")

	practice.Tags, err = GetCourseInfo(opts.CourseId)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't get course info",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Warning("/get_practice: couldn't get course info")
		return
	}

	// countGetChapterOk.Inc()
	Logger.WithFields(log.Fields{
		"user_id":         opts.userId,
		"course_id":       opts.CourseId,
		"task_id":         opts.TaskId,
		"next_chapter_id": practice.NextChapterId,
	}).Info("/get_practice: completed")

	json.NewEncoder(w).Encode(practice)

}

func HandleGetChapter(w http.ResponseWriter, r *http.Request) {
	countGetChapterTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		countGetChapterClientError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"course_id":  opts.CourseId,
			"error":      err.Error(),
		}).Warning("/get_chapter: couldn't parse request")
		return
	}

	if len(opts.CourseId) == 0 && len(opts.ChapterId) == 0 {
		countGetChapterClientError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get required request params",
		})

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"course_id":  opts.CourseId,
		}).Warning("/get_chapter: required fields not set in request")
		return
	}

	chapter, err := GetChapterForUser(opts)

	if err != nil {
		countGetChapterServerError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't get %s chapter for user %s (chapter %s): %s",
				opts.CourseId, opts.userId, opts.ChapterId, err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"course_id":  opts.CourseId,
			"error":      err.Error(),
		}).Error("/get_chapter: couldn't get chapter for user")
		return
	}

	chapter.NextChapterId, _ = GetNextChapterId(opts.CourseId, opts.ChapterId, true)
	if len(opts.userId) == 0 {
		countGetChapterAnonymous.Inc()
		chapter.CourseStatus = "not_started"
	} else {
		chapter.CourseStatus, _ = GetCourseProgressForUser(opts.CourseId, opts.userId)
	}

	if !strings.HasSuffix(chapter.ChapterId, "0") { // subchapter
		chapter.ParentChapterId = chapter.ChapterId[:len(chapter.ChapterId)-1] + string('0')
		chapter.ParentChapterTitle, _ = GetChapterTitle(chapter.ParentChapterId)
	}

	countGetChapterOk.Inc()
	Logger.WithFields(log.Fields{
		"user_id":         opts.userId,
		"course_id":       opts.CourseId,
		"chapter_id":      opts.ChapterId,
		"next_chapter_id": chapter.NextChapterId,
	}).Info("/get_chapter: completed")

	json.NewEncoder(w).Encode(chapter)
}

func HandleGetProgress(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"error":      err.Error(),
		}).Warning("/get_progress: couldn't parse request")
		return
	}

	if len(opts.userId) == 0 || len(opts.ChapterId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get required request params",
		})

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
		}).Warning("/get_progress: required fields not set in request")
		return
	}

	chapterStatus, err := GetChapterProgress(opts.userId, opts.ChapterId)

	if err != nil && err != sql.ErrNoRows {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get chapter progress",
		})

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"error":      err.Error(),
		}).Warning("/get_progress: couldn't get chapter progress (sql query)")
		return
	}

	var userProgress UserProgress

	tasks := GetTasks(opts.ChapterId, opts.userId)

	for _, task := range tasks {
		if task.Status != "completed" {
			userProgress.NotCompletedTaskIds = append(userProgress.NotCompletedTaskIds, task.TaskId)
		}
	}

	if len(userProgress.NotCompletedTaskIds) > 0 && chapterStatus != "completed" {
		userProgress.StatusOnChapter = "chapter_not_completed"
	} else {
		userProgress.StatusOnChapter = "chapter_completed"
	}

	nextChapterId, err := GetNextChapterId(opts.CourseId, opts.ChapterId, true)
	if err != nil && err != sql.ErrNoRows {
		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"course_id":  opts.CourseId,
			"chapter_id": opts.ChapterId,
			"error":      err.Error(),
		}).Error("/get_progress: couldn't get user progress on chapter")

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get progress",
		})
		return
	}

	userProgress.NextChapterId = nextChapterId
	userProgress.CourseId = opts.CourseId

	if userProgress.StatusOnChapter == "chapter_completed" {
		userProgress.IsCourseCompleted, err = AreAllChaptersInCourseCompleted(opts.userId, opts.CourseId)

		if err != nil {
			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"course_id":  opts.CourseId,
				"chapter_id": opts.ChapterId,
				"error":      err.Error(),
			}).Error("/get_progress: couldn't check if all chapters are completed")

			json.NewEncoder(w).Encode(map[string]string{
				"error": "Couldn't get progress",
			})
			return
		}

		if userProgress.IsCourseCompleted {
			userProgress.PracticeProjects = GetPracticeProjects(opts.userId, opts.CourseId)
		}
	}

	Logger.WithFields(log.Fields{
		"user_id":             opts.userId,
		"chapter_id":          opts.ChapterId,
		"course_id":           opts.CourseId,
		"chapter_status":      userProgress.StatusOnChapter,
		"next_chapter_id":     userProgress.NextChapterId,
		"is_course_completed": userProgress.IsCourseCompleted,
		"not_completed_tasks": userProgress.NotCompletedTaskIds,
	}).Info("/get_progress: completed")

	json.NewEncoder(w).Encode(userProgress)
}

func HandleCoursesStats(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"error":   err.Error(),
		}).Warning("/courses_stats: couldn't parse request")
		return
	}

	if len(opts.userId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id",
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
		}).Warning("/courses_stats: required fields not set in request")
		return
	}

	courseStatuses := GetCourseStatuses(opts.userId)

	Logger.WithFields(log.Fields{
		"user_id":             opts.userId,
		"course_statuses_len": len(courseStatuses),
	}).Info("/courses_stats: completed")

	json.NewEncoder(w).Encode(courseStatuses)
}

func HandleMergeUsers(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptionsTg(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id_cur": opts.UserIdCur,
			"user_id_old": opts.UserIdOld,
			"error":       err.Error(),
		}).Warning("/merge_users: couldn't parse request")
		return
	}

	status := MergeUsers(opts.UserIdCur, opts.UserIdOld)

	body, _ := json.Marshal(map[string]int{
		"status": status,
	})

	if status == 0 {
		Logger.WithFields(log.Fields{
			"user_id_cur": opts.UserIdCur,
			"user_id_old": opts.UserIdOld,
		}).Info("/merge_users: completed")
	}

	w.Write(body)
}

func HandleSplitUsers(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptionsTg(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id_cur": opts.UserIdCur,
			"user_id_new": opts.UserIdNew,
			"error":       err.Error(),
		}).Warning("/split_users: couldn't parse request")
		return
	}

	status := SplitUsers(opts.UserIdCur, opts.UserIdNew)
	body, _ := json.Marshal(map[string]int{
		"status": status,
	})

	if status == 0 {
		Logger.WithFields(log.Fields{
			"user_id_cur": opts.UserIdCur,
			"user_id_new": opts.UserIdNew,
		}).Info("/split_users: completed")
	}

	w.Write(body)
}

func HandleGetTask(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Warning("/get_task: couldn't parse request")
		return
	}

	if len(opts.userId) == 0 || len(opts.TaskId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id or task_id",
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
		}).Warning("/get_task: required fields not set in request")
		return
	}

	task, err := GetTaskForUser(opts.userId, opts.TaskId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Error("/get_task: couldn't get task details")

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't get task details for: %s", opts.TaskId),
		})
		w.Write(body)
		return
	}

	Logger.WithFields(log.Fields{
		"user_id": opts.userId,
		"task_id": opts.TaskId,
	}).Warning("/get_task: completed")

	json.NewEncoder(w).Encode(task)
}

func HandleGetActiveChapter(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Warning("/get_active_chapter: couldn't parse request")
		return
	}

	if len(opts.userId) == 0 || len(opts.CourseId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id or course_id",
		})

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
		}).Warning("/get_active_chapter: required fields not set in request")
		return
	}

	courses := GetCoursesForUserByStatus(opts.userId, "in_progress")
	hasAccess := false
	for i := 0; i < len(courses); i++ {
		if courses[i].CourseId == opts.CourseId {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		body, _ := json.Marshal(map[string]string{
			"error": "Course is not in state 'in_progress' for user",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
		}).Warning("/get_active_chapter: course is not in state 'in_progress' for user")
		return
	}

	chapters := GetChaptersForUser(opts.userId, opts.CourseId)

	for i := 0; i < len(chapters); i++ {
		if chapters[i].Status == "in_progress" || chapters[i].Status == "not_started" {
			opts.ChapterId = chapters[i].ChapterId

			var chapter ChapterContent
			chapter.IsPractice = false

			prefixLen := len(opts.ChapterId) - 4 // NNNN
			if prefixLen < len(opts.ChapterId) {
				_, err := strconv.Atoi(opts.ChapterId[prefixLen:])
				// Chapter is practice: it is not in format coursename_NNNN
				if err != nil {
					chapter.ChapterId = opts.ChapterId
					chapter.IsPractice = true
				}
			}

			if !chapter.IsPractice {
				chapter, err = GetChapterForUser(opts)
				if err != nil {
					body, _ := json.Marshal(map[string]string{
						"error": fmt.Sprintf("Couldn't get chapter for user: %s", err),
					})
					w.Write(body)

					Logger.WithFields(log.Fields{
						"user_id":    opts.userId,
						"course_id":  opts.CourseId,
						"chapter_id": opts.ChapterId,
						"error":      err.Error(),
					}).Error("/get_active_chapter: couldn't get chapter for user")
					return
				}
			}

			Logger.WithFields(log.Fields{
				"user_id":             opts.userId,
				"course_id":           opts.CourseId,
				"chapter_id":          opts.ChapterId,
				"returned_chapter_id": chapter.ChapterId,
				"is_practice":         chapter.IsPractice,
			}).Info("/get_active_chapter: completed")

			json.NewEncoder(w).Encode(chapter)
			return
		}
	}

	Logger.WithFields(log.Fields{
		"user_id":   opts.userId,
		"course_id": opts.CourseId,
	}).Info("/get_active_chapter: completed with no active chapter for user")

	body, _ := json.Marshal(map[string]string{
		"error": "No active chapter for user",
	})
	w.Write(body)
}
