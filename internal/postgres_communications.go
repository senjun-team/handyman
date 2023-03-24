package internal

import (
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

		iconSvg, _ := ReadTextFile(filepath.Join(courses[i].Path, "icon.svg"))
		courses[i].Icon = iconSvg
	}

	Logger.WithFields(log.Fields{
		"user_id": opts.userId,
		"status":  opts.Status,
	}).Info("Successfully got courses")

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
		}).Info("Successfully got chapters")

		json.NewEncoder(w).Encode(chapters)
		return
	}

	chapters := GetChaptersForUser(opts.userId, opts.CourseId)

	Logger.WithFields(log.Fields{
		"user_id":   opts.userId,
		"course_id": opts.CourseId,
	}).Info("Successfully got chapters")

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
	}).Info("Successfully got chapter")

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

	if len(userProgress.NotCompletedTaskIds) > 0 {
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

func HandleCourseStats(w http.ResponseWriter, r *http.Request) {
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
			}).Info("Successfully got chapter")

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
