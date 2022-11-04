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

func ConnectDb() *sql.DB {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Couldn't call Open() for db")
	}

	err = db.Ping()

	if err != nil {
		log.WithFields(log.Fields{
			"connStr": connStr,
			"error":   err,
		}).Fatal("Couldn't communicate db")
	}

	return db
}

// Returns postgres TYPE edu_material_status
func getEduMaterialStatus(code int) string {
	if code == 0 {
		return "completed"
	}

	return "in_progress"
}

func UpdateTaskStatus(userId string, taskId string, statusCode int, solutionText string) {
	taskStatus := getEduMaterialStatus(statusCode)
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
		log.WithFields(log.Fields{
			"user_id":  userId,
			"task_id":  taskId,
			"db_error": err.Error(),
		}).Error("Couldn't update task status for user")
		return
	}

	log.WithFields(log.Fields{
		"user_id": userId,
		"task_id": taskId,
	}).Info("Updated task status for user")
}

func UpdateChapterStatus(userId string, chapterId string, taskId string, statusCode int) {
	// If task is not solved correctly we don't need to update chapter status
	if statusCode != 0 {
		return
	}
	const queryTasks = `
		SELECT
		(CASE WHEN task_progress.status IS NULL THEN 'not_started' ELSE task_progress.status::varchar(40) END)  
		FROM tasks 
		LEFT JOIN task_progress ON 
		tasks.task_id = task_progress.task_id 
		AND tasks.chapter_id= $1
		AND task_progress.user_id = $2
	`

	rows, err := DB.Query(queryTasks, chapterId, userId)
	if err != nil {
		log.WithFields(log.Fields{
			"user_id":    userId,
			"task_id":    taskId,
			"chapter_id": chapterId,
			"db_error":   err.Error(),
		}).Error("Couldn't get tasks for chapter")
		return
	}

	defer rows.Close()

	chapterStatus := "completed"

	for rows.Next() {
		var status string

		if err := rows.Scan(&status); err != nil {
			log.WithFields(log.Fields{
				"user_id":    userId,
				"task_id":    taskId,
				"chapter_id": chapterId,
				"error":      err.Error(),
			}).Info("Couldn't parse row from tasks selection for user")
			return
		}

		if status != "completed" {
			chapterStatus = "in_progress"
			break
		}
	}

	const query = `
		INSERT INTO chapter_progress(user_id, chapter_id, status) 
		VALUES($1, $2, $3) 
		ON CONFLICT ON CONSTRAINT unique_user_chapter_id 
		DO UPDATE SET
		status = EXCLUDED.status
	`

	_, err = DB.Exec(query, userId, chapterId, chapterStatus)
	if err != nil {
		log.WithFields(log.Fields{
			"user_id":    userId,
			"chapter_id": chapterId,
			"db_error":   err.Error(),
		}).Error("Couldn't update chapter status for user")
		return
	}

	log.WithFields(log.Fields{
		"user_id":    userId,
		"chapter_id": chapterId,
	}).Info("Updated chapter status for user")
}

func GetCoursesForUser(userId string) []CourseForUser {
	query := `
		SELECT courses.course_id, courses.path_on_disk, courses.type, courses.title,
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

		if err := rows.Scan(&course.CourseId, &course.Path, &course.CourseType, &course.Title, &course.Status); err != nil {
			log.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Info("Couldn't parse row from courses selection for user")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetCoursesForUserByStatus(userId string, status string) []CourseForUser {
	query := `
		SELECT courses.course_id, courses.path_on_disk, courses.type, courses.title
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

		if err := rows.Scan(&course.CourseId, &course.Path, &course.CourseType, &course.Title); err != nil {
			log.WithFields(log.Fields{
				"user_id": userId,
				"error":   err.Error(),
			}).Info("Couldn't parse row from courses selection for user")
			return []CourseForUser{}
		}

		courses = append(courses, course)
	}

	return courses
}

func GetChaptersForUser(userId string, courseId string) []ChapterForUser {
	query := `
		SELECT
		chapters.chapter_id, chapters.title, 
		(CASE WHEN chapter_progress.status IS NULL THEN 'not_started' ELSE chapter_progress.status::varchar(40) END) as status
		FROM chapters
		LEFT JOIN chapter_progress ON
		chapters.chapter_id = chapter_progress.chapter_id 
		AND chapter_progress.user_id=$1
		WHERE course_id=$2
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
			log.WithFields(log.Fields{
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
		// We need to check if user has rights to read this chapter
		prevChapterId, err := GetPrevChapterId(opts.CourseId, opts.ChapterId)
		if err != nil && err != sql.ErrNoRows {
			return ChapterContent{}, err
		}
		if err != nil && err == sql.ErrNoRows {
			chapterContent.ChapterId = opts.ChapterId
		} else {
			chapterStatus, err := GetChapterProgress(opts.userId, prevChapterId)
			if err != nil {
				return ChapterContent{}, err
			}

			if chapterStatus != "completed" {
				return ChapterContent{}, errors.New("user didn't complete the previous chapter")
			}
			chapterContent.ChapterId = opts.ChapterId
		}
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

	chapterContent.ContentPath = contentPath
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
			log.WithFields(log.Fields{
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
			log.WithFields(log.Fields{
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
		log.WithFields(log.Fields{
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

	if opts.Status == "all" {
		courses = GetCoursesForUser(opts.userId)
	} else {
		courses = GetCoursesForUserByStatus(opts.userId, opts.Status)
	}

	for i := 0; i < len(courses); i++ {
		courses[i].DescriptionPath = filepath.Join(courses[i].Path, "description.md")
		courses[i].IconPath = filepath.Join(courses[i].Path, "icon.svg")
	}

	log.WithFields(log.Fields{
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

	if len(opts.Status) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Course status is not set in request",
		})
		return
	}

	curStatus, err := GetCourseProgressForUser(opts.CourseId, opts.userId)
	if err != nil {
		log.WithFields(log.Fields{
			"user_id":   opts.userId,
			"course_id": opts.CourseId,
			"error":     err.Error(),
		}).Error("Couldn't get progress")

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user progress on course",
		})
		return
	}

	if len(curStatus) == 0 && opts.Status == "completed" {
		json.NewEncoder(w).Encode(map[string]string{
			"error":         "Couldn't complete course which is not started",
			"course_status": curStatus,
		})
		return
	}

	if len(curStatus) != 0 && curStatus != "blocked" && opts.Status == "in_progress" {
		json.NewEncoder(w).Encode(map[string]string{
			"error":         "Couldn't start course which is already started",
			"course_status": curStatus,
		})
		return
	}

	if curStatus == opts.Status {
		json.NewEncoder(w).Encode(map[string]string{
			"error":         "Course is already in this status",
			"course_status": curStatus,
		})
		return
	}

	err = UpdateCourseProgressForUser(opts.CourseId, opts.Status, opts.userId)
	if err != nil {
		log.WithFields(log.Fields{
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

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
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

	if len(opts.userId) == 0 || len(opts.CourseId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id or course_id",
		})
		return
	}

	chapters := GetChaptersForUser(opts.userId, opts.CourseId)

	log.WithFields(log.Fields{
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
			"error": fmt.Sprintf("Couldn't get chapter for user: %s", err),
		})
		w.Write(body)
		return
	}

	log.WithFields(log.Fields{
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

	var userProgress UserProgress
	tasks := GetTasks(opts.ChapterId, opts.userId)
	for _, task := range tasks {
		if task.Status != "completed" {
			userProgress.NotCompletedTaskIds = append(userProgress.NotCompletedTaskIds, task.TaskId)
		}
	}

	if len(userProgress.NotCompletedTaskIds) > 0 || len(tasks) == 0 {
		userProgress.StatusOnChapter = "chapter_not_completed"
	} else {
		userProgress.StatusOnChapter = "chapter_completed"
		nextChapterId, err := GetNextChapterId(opts.CourseId, opts.ChapterId)
		if err != nil && err == sql.ErrNoRows {
			userProgress.IsCourseCompleted = true
		} else if err != nil {
			log.WithFields(log.Fields{
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
		}
	}

	json.NewEncoder(w).Encode(userProgress)
}
