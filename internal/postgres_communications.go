package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gammazero/workerpool"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

// An exported global variable to hold the database connection pool
// Completely thread-safe and ok. Fear not, my friend
var DB *sql.DB

var WP *workerpool.WorkerPool

const connStr = "postgresql://senjun:some_password@127.0.0.1:5432/senjun?sslmode=disable"

var Logger *log.Logger

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

func UpdateTaskStatus(userId string, taskId string, isSolved bool, solutionText string) {
	taskStatus := getEduMaterialStatus(isSolved)
	const attemptsCount = 1

	const query = `
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
			"user_id":  userId,
			"task_id":  taskId,
			"db_error": err.Error(),
		}).Error("Couldn't update task status for user")
		return
	}

	Logger.WithFields(log.Fields{
		"user_id": userId,
		"task_id": taskId,
		"status":  taskStatus,
	}).Info("Updated task status for user")
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
			}).Info("Couldn't parse row from courses selection")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
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
			}).Info("Couldn't parse row from courses selection for user")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetCourseStatsForUser(userId string, courseId string) CourseStatus {
	var courseStatus CourseStatus
	return courseStatus

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
			}).Info("Couldn't parse row from courses selection for user")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetChapters(courseId string) []ChapterForUser {
	query := `
		SELECT
		chapters.chapter_id, chapters.title, 
		'not_started' as status
		FROM chapters
		WHERE course_id=$1 ORDER BY chapters.chapter_id
	`

	rows, err := DB.Query(query, courseId)
	if err != nil {
		return []ChapterForUser{}
	}

	defer rows.Close()

	chapters := []ChapterForUser{}

	for rows.Next() {
		var chapter ChapterForUser

		if err := rows.Scan(&chapter.ChapterId, &chapter.Title, &chapter.Status); err != nil {
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
		SELECT
		chapters.chapter_id, chapters.title, 
		(CASE WHEN chapter_progress.status IS NULL THEN 'not_started' ELSE chapter_progress.status::varchar(40) END) as status
		FROM chapters
		LEFT JOIN chapter_progress ON
		chapters.chapter_id = chapter_progress.chapter_id AND chapter_progress.user_id=$1
		WHERE course_id=$2 ORDER BY chapters.chapter_id
	`

	rows, err := DB.Query(query, userId, courseId)
	if err != nil {
		return []ChapterForUser{}
	}

	defer rows.Close()

	chapters := []ChapterForUser{}

	for rows.Next() {
		var chapter ChapterForUser

		if err := rows.Scan(&chapter.ChapterId, &chapter.Title, &chapter.Status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Error("Couldn't parse row from chapters selection for user")
			return []ChapterForUser{}
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

func GetPrevChapterId(courseId string, chapterId string) (string, error) {
	query := `
		SELECT chapter_id FROM chapters WHERE course_id=$1 AND chapter_id < $2 ORDER BY chapter_id DESC LIMIT 1
	`
	var prevChapterId string

	row := DB.QueryRow(query, courseId, chapterId)
	err := row.Scan(&prevChapterId)
	return prevChapterId, err
}

func IsUserAllowedToRunTask(opts Options) (bool, error) {
	// We check that current chapter is in progress or finished
	progress, err := GetChapterProgress(opts.userId, opts.ChapterId)
	if err != nil {
		// Chapter is not started
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	if progress == "blocked" {
		return false, nil
	}

	return true, nil
}

func GetNextChapterId(courseId string, chapterId string) (string, error) {
	query := `
		SELECT chapter_id FROM chapters WHERE course_id=$1 AND chapter_id > $2 ORDER BY chapter_id ASC LIMIT 1
	`
	var nextChapterId string

	row := DB.QueryRow(query, courseId, chapterId)
	err := row.Scan(&nextChapterId)
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

func UserHasPermissionsForChapter(opts Options) (bool, error) {
	prevChapterId, err := GetPrevChapterId(opts.CourseId, opts.ChapterId)

	if err != nil {
		// First chapter in course
		if err == sql.ErrNoRows {
			return true, nil
		}

		return false, err
	}

	chapterStatus, err := GetChapterProgress(opts.userId, prevChapterId)
	// In case of strange errors we suppose that user doesn't have permissions
	if err != nil {
		return false, err
	}

	return chapterStatus == "completed", nil
}

func GetChapterForUser(opts Options) (ChapterContent, error) {
	var chapterContent ChapterContent

	if len(opts.ChapterId) == 0 {
		// We need to get the first chapter in course id.
		// We don't need to check user access (for now we don't have paid access).
		activeChapterId, err := GetFirstChapterId(opts.CourseId)
		if err != nil {
			return ChapterContent{}, err
		}
		chapterContent.ChapterId = activeChapterId
	} else {
		userHasPermissions, err := UserHasPermissionsForChapter(opts)
		if err != nil {
			return ChapterContent{}, err
		}

		if !userHasPermissions {
			return ChapterContent{}, errors.New("user doesn't have access to chapter")
		}
		chapterContent.ChapterId = opts.ChapterId
	}

	status, title, err := GetChapterInfo(opts.userId, chapterContent.ChapterId)
	if err != nil {
		return ChapterContent{}, err
	}
	chapterContent.Status = status
	chapterContent.Title = title

	contentPath, err := GetPathToChapterText(opts.CourseId, chapterContent.ChapterId)
	if err != nil {
		return ChapterContent{}, err
	}

	chapterText, _ := ReadTextFile(contentPath)

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

func GetChapterProgressForUser(chapterId string, userId string) (string, error) {
	query := `
		SELECT status FROM chapter_progress WHERE chapter_id=$1 AND user_id=$2
	`
	var status string
	rows, err := DB.Query(query, chapterId, userId)
	if err != nil {
		return "", err
	}

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&status); err != nil {
			Logger.WithFields(log.Fields{
				"user_id":    userId,
				"chapter_id": chapterId,
				"error":      err.Error(),
			}).Info("Couldn't get status from chapter_progress table")
			return "", err
		}
	}

	return status, nil
}

func GetTasks(chapterId string, userId string) []TaskForUser {
	query := `
		SELECT 
		tasks.task_id,
		(CASE WHEN task_progress.status IS NULL THEN 'not_started' ELSE task_progress.status::varchar(40) END),
		(CASE WHEN task_progress.solution_text IS NULL THEN '' ELSE task_progress.solution_text::varchar END)
		FROM tasks 
		LEFT JOIN task_progress
		ON tasks.task_id = task_progress.task_id AND user_id = $1
		WHERE chapter_id = $2
	`

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
			}).Info("Couldn't parse row from tasks selection for user")
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
	select course_id, 
	(select title from courses where courses.course_id = course_progress.course_id) as title, 
	(select count(*) from chapters where chapters.course_id = course_progress.course_id) as chapters_total,
	(select count(*) from chapter_progress where chapter_progress.user_id = $1 and 
	chapter_progress.status = 'completed' 
	and chapter_progress.chapter_id like concat(course_id, '_chapter_%')) as chapters_completed,
	status from course_progress
	where user_id = $1 and status in ('in_progress', 'completed')
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
			}).Info("Couldn't parse row from courses selection for user")
			return []CourseStatus{}
		}

		courseStatuses = append(courseStatuses, cs)
	}

	Logger.WithFields(log.Fields{
		"user_id": userId,
	}).Info("Got course statuses")
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
			}).Error("Couldn't update task status for user")
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
			"db_error":    err.Error(),
		}).Error("Couldn't start transaction in SplitUsers")
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
			"db_error":    err.Error(),
		}).Error("Couldn't split course records for user")
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
		}).Error("Couldn't split chapter records for user")
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
		}).Error("Couldn't split task records for user")
		return -1
	}

	err = tx.Commit()
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_new": userIdNew,
			"db_error":    err.Error(),
		}).Error("Couldn't commit transaction in SplitUsers")
		return -1
	}

	Logger.WithFields(log.Fields{
		"user_id_cur": userIdCur,
		"user_id_new": userIdNew,
	}).Info("Split user progress in SplitUsers")

	return 0
}

func MergeUsers(userIdCur int, userIdOld int) int {
	ctx := context.Background()
	tx, err := DB.BeginTx(ctx, nil)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
			"db_error":    err.Error(),
		}).Error("Couldn't start transaction in MergeUsers")
		return -1
	}

	if MergeUserCourses(tx, ctx, userIdCur, userIdOld) != 0 {
		tx.Rollback()
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
		}).Error("Couldn't merge user courses in MergeUsers")
		return -1
	}

	if MergeUserChapters(tx, ctx, userIdCur, userIdOld) != 0 {
		tx.Rollback()
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
		}).Error("Couldn't merge user chapters in MergeUsers")
		return -1
	}

	if MergeUserTasks(tx, ctx, userIdCur, userIdOld) != 0 {
		tx.Rollback()
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
		}).Error("Couldn't merge user tasks in MergeUsers")
		return -1
	}

	err = tx.Commit()
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id_cur": userIdCur,
			"user_id_old": userIdOld,
			"db_error":    err.Error(),
		}).Error("Couldn't commit transaction in MergeUsers")
		return -1
	}

	Logger.WithFields(log.Fields{
		"user_id_cur": userIdCur,
		"user_id_old": userIdOld,
	}).Info("Merged user progress in MergeUsers")

	return 0
}

func GetTaskForUser(userId string, taskId string) (TaskForUser, error) {
	query := `
	select status, solution_text from task_progress where user_id = $1 and task_id = $2
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
			}).Info("Couldn't parse row from task_progress selection for user")
			return TaskForUser{}, err
		}
	}

	return task, nil
}

func HandleGetCourses(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)

	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		return
	}

	var courses []CourseForUser

	// Case when user is not authorized
	if len(opts.userId) == 0 {
		courses = GetCourses()
	} else {
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

	Logger.WithFields(log.Fields{
		"user_id":     opts.userId,
		"status":      opts.Status,
		"courses_len": len(courses),
	}).Info("Got courses")

	json.NewEncoder(w).Encode(courses)
}

func HandleUpdateCourseProgress(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		return
	}

	if len(opts.userId) == 0 || len(opts.Status) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Required fields are not set in request",
		})
		return
	}

	curStatus, err := GetCourseProgressForUser(opts.CourseId, opts.userId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Error("Couldn't get progress")

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user progress on course",
		})
		return
	}

	if !IsNewStatusValid(curStatus, opts.Status) {
		json.NewEncoder(w).Encode(map[string]string{
			"error":          "Couldn't change status",
			"current_status": curStatus,
			"new_status":     opts.Status,
		})
		return
	}

	err = UpdateCourseProgressForUser(opts.CourseId, opts.Status, opts.userId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"status":    opts.Status,
			"error":     err.Error(),
		}).Error("Couldn't update user progress on course")

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't update user progress on course",
		})
		return
	}

	Logger.WithFields(log.Fields{
		"user_id":   opts.userId,
		"course_id": opts.CourseId,
	}).Info("Updated course status for user")

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

func HandleUpdateChapterProgress(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		return
	}

	if len(opts.userId) == 0 || len(opts.ChapterId) == 0 || len(opts.Status) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id, chapter_id or status",
		})
		return
	}

	curStatus, err := GetChapterProgressForUser(opts.ChapterId, opts.userId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"error":      err.Error(),
		}).Error("Couldn't get progress")

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user progress on chapter",
		})
		return
	}

	if !IsNewStatusValid(curStatus, opts.Status) {
		json.NewEncoder(w).Encode(map[string]string{
			"error":          "Couldn't change status",
			"current_status": curStatus,
			"new_status":     opts.Status,
			"course_id":      opts.CourseId,
		})
		return
	}

	userHasPermissions, err := UserHasPermissionsForChapter(opts)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user permissions on chapter",
		})
		return
	}

	if !userHasPermissions {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "User doesn't have permissions on chapter",
		})
		return
	}

	if opts.Status == "completed" {
		tasks := GetTasks(opts.ChapterId, opts.userId)

		for _, task := range tasks {
			if task.Status != "completed" {
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Not all tasks in chapter are completed",
				})
				return
			}
		}
	}

	err = UpdateChapterStatus(opts.userId, opts.ChapterId, opts.Status)

	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"chapter_id": opts.ChapterId,
			"db_error":   err.Error(),
		}).Error("Couldn't update chapter status for user")

		json.NewEncoder(w).Encode(map[string]string{
			"status":    "error",
			"course_id": opts.CourseId,
		})

		return
	}

	Logger.WithFields(log.Fields{
		"user_id":    opts.userId,
		"chapter_id": opts.ChapterId,
	}).Info("Updated chapter status for user")

	json.NewEncoder(w).Encode(map[string]string{
		"status":    "ok",
		"course_id": opts.CourseId,
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
		return
	}

	if len(opts.CourseId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get course_id in get_chapters",
		})
		return
	}

	// User is not authorized
	if len(opts.userId) == 0 {
		chapters := GetChapters(opts.CourseId)

		Logger.WithFields(log.Fields{
			"course_id": opts.CourseId,
		}).Info("Got chapters for not authorized user")

		json.NewEncoder(w).Encode(chapters)
		return
	}

	chapters := GetChaptersForUser(opts.userId, opts.CourseId)

	Logger.WithFields(log.Fields{
		"user_id":   opts.userId,
		"course_id": opts.CourseId,
	}).Info("Got chapters")

	json.NewEncoder(w).Encode(chapters)
}

func HandleGetChapter(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := ParseOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		return
	}

	if len(opts.userId) == 0 || len(opts.CourseId) == 0 && len(opts.ChapterId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get required request params",
		})
		return
	}

	chapter, err := GetChapterForUser(opts)

	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't get %s chapter for user %s (chapter %s): %s",
				opts.CourseId, opts.userId, opts.ChapterId, err),
		})
		w.Write(body)
		return
	}

	Logger.WithFields(log.Fields{
		"user_id":    opts.userId,
		"course_id":  opts.CourseId,
		"chapter_id": opts.ChapterId,
	}).Info("Got chapter")

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
		return
	}

	if len(opts.userId) == 0 || len(opts.ChapterId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get required request params",
		})
		return
	}

	chapterStatus, err := GetChapterProgress(opts.userId, opts.ChapterId)
	if err != nil || len(chapterStatus) == 0 || chapterStatus == "not_started" {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "User doesn't have permissions on this chapter",
		})
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
		nextChapterId, err := GetNextChapterId(opts.CourseId, opts.ChapterId)
		if err != nil && err == sql.ErrNoRows {
			userProgress.IsCourseCompleted = true
		} else if err != nil {
			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"course_id":  opts.CourseId,
				"chapter_id": opts.ChapterId,
				"error":      err.Error(),
			}).Error("Couldn't get user progress on chapter")

			json.NewEncoder(w).Encode(map[string]string{
				"error": "Couldn't get progress",
			})
			return
		} else {
			userProgress.IsCourseCompleted = false
			userProgress.NextChapterId = nextChapterId
			userProgress.CourseId = opts.CourseId
		}
	}

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
		return
	}

	if len(opts.userId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id",
		})
		return
	}

	courseStatuses := GetCourseStatuses(opts.userId)
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
		return
	}

	status := MergeUsers(opts.UserIdCur, opts.UserIdOld)

	body, _ := json.Marshal(map[string]int{
		"status": status,
	})
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
		return
	}

	status := SplitUsers(opts.UserIdCur, opts.UserIdNew)
	body, _ := json.Marshal(map[string]int{
		"status": status,
	})
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
		return
	}

	if len(opts.userId) == 0 || len(opts.TaskId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id or task_id",
		})
		return
	}

	task, err := GetTaskForUser(opts.userId, opts.TaskId)
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Error("Couldn't get task details")

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't get task details for: %s", opts.TaskId),
		})
		w.Write(body)
		return
	}

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
		return
	}

	if len(opts.userId) == 0 || len(opts.CourseId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id or course_id",
		})
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
		return
	}

	chapters := GetChaptersForUser(opts.userId, opts.CourseId)

	for i := 0; i < len(chapters); i++ {
		if chapters[i].Status == "in_progress" || chapters[i].Status == "not_started" {
			opts.ChapterId = chapters[i].ChapterId

			chapter, err := GetChapterForUser(opts)

			if err != nil {
				body, _ := json.Marshal(map[string]string{
					"error": fmt.Sprintf("Couldn't get chapter for user: %s", err),
				})
				w.Write(body)
				return
			}

			Logger.WithFields(log.Fields{
				"user_id":    opts.userId,
				"course_id":  opts.CourseId,
				"chapter_id": opts.ChapterId,
			}).Info("Got chapter")

			json.NewEncoder(w).Encode(chapter)
			return
		}
	}

	Logger.WithFields(log.Fields{
		"user_id":   opts.userId,
		"course_id": opts.CourseId,
	}).Info("No active chapter for user")

	body, _ := json.Marshal(map[string]string{
		"error": "No active chapter for user",
	})
	w.Write(body)
}
