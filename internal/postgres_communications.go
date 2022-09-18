package internal

import (
	"database/sql"

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

func getStatusBySeqType(seqType string) string {
	if seqType == "last" {
		return "completed"
	}

	return "in_progress"
}

func UpdateChapterStatus(userId string, chapterId string, taskId string, statusCode int) (needUpdateCourse bool) {
	// If task is not solved correctly we don't need to update chapter status
	if statusCode != 0 {
		return false
	}

	var seqTypeTask string
	row := DB.QueryRow("SELECT seq FROM tasks WHERE task_id = $1", taskId)

	if err := row.Scan(&seqTypeTask); err != nil {
		log.WithFields(log.Fields{
			"user_id":  userId,
			"task_id":  taskId,
			"db_error": err.Error(),
		}).Error("Couldn't get seq from tasks table")
		return false
	}

	if seqTypeTask != "last" && seqTypeTask != "first" {
		return false
	}

	const query = `
		INSERT INTO chapter_progress(user_id, chapter_id, status) 
		VALUES($1, $2, $3) 
		ON CONFLICT ON CONSTRAINT unique_user_chapter_id 
		DO UPDATE SET
		status = EXCLUDED.status
	`

	chapterStatus := getStatusBySeqType(seqTypeTask)
	_, err := DB.Exec(query, userId, chapterId, chapterStatus)
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

	return seqTypeTask == "last"
}

func UpdateCourseStatus(userId string, courseId string, chapterId string) {
	log.WithFields(log.Fields{
		"user_id":   userId,
		"course_id": courseId,
	}).Info("Checking for update for course")

	var seqType string
	row := DB.QueryRow("SELECT seq FROM chapters WHERE chapter_id = $1", chapterId)

	if err := row.Scan(&seqType); err != nil {
		log.WithFields(log.Fields{
			"user_id":    userId,
			"chapter_id": chapterId,
			"db_error":   err.Error(),
		}).Error("Couldn't get seq from chapters table")
		return
	}

	if seqType != "last" && seqType != "first" {
		return
	}

	const query = `
		INSERT INTO course_progress(user_id, course_id, status) 
		VALUES($1, $2, $3) 
		ON CONFLICT ON CONSTRAINT unique_user_course_id 
		DO UPDATE SET
		status = EXCLUDED.status
	`

	courseStatus := getStatusBySeqType(seqType)
	_, err := DB.Exec(query, userId, courseId, courseStatus)
	if err != nil {
		log.WithFields(log.Fields{
			"user_id":   userId,
			"course_id": courseId,
			"db_error":  err.Error(),
		}).Error("Couldn't update course status for user")
		return
	}

	log.WithFields(log.Fields{
		"user_id":   userId,
		"course_id": courseId,
	}).Info("Updated course status for user")
}
