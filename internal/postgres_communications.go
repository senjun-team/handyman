package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

// An exported global variable to hold the database connection pool
// Completely thread-safe and ok. Fear not, my friend
var DB *sql.DB

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

type updateChapterStatusParams struct {
	userId    string
	chapterId string
	status    string
}

func extractUpdateChapterStatusParams(r *http.Request) (updateChapterStatusParams, error) {
	urlParams := r.URL.Query()
	var res updateChapterStatusParams

	res.userId = urlParams.Get("user_id")

	if len(res.userId) == 0 {
		return updateChapterStatusParams{}, errors.New("invalid user id")
	}

	res.chapterId = urlParams.Get("chapter_id")

	if len(res.chapterId) == 0 {
		return updateChapterStatusParams{}, errors.New("invalid chapter id")
	}

	res.status = urlParams.Get("status")
	if len(res.status) == 0 {
		return updateChapterStatusParams{}, errors.New("invalid status")
	}

	return res, nil
}

func UpdateChapterStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")
	params, err := extractUpdateChapterStatusParams(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		return
	}

	log.WithFields(log.Fields{
		"userId":    params.userId,
		"chapterId": params.chapterId,
		"status":    params.status,
	}).Debug("Parsed url params")

}
