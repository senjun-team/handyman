package main

import (
	"net/http"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"senjun.ru/handyman/internal"
)

const version = "1.0"
const addrHandyman = "127.0.0.1:8080"

const timeoutReplyToUser = 40 * time.Second

func main() {
	log.WithFields(log.Fields{
		"version":    version,
		"address":    addrHandyman,
		"GOMAXPROCS": runtime.GOMAXPROCS(-1),
	}).Info("Started handyman")

	internal.DB = internal.ConnectDb()
	defer internal.DB.Close()

	r := mux.NewRouter()
	r.HandleFunc("/run_task", internal.HandleRunTask)
	r.HandleFunc("/update_chapter_status", internal.UpdateChapterStatus)

	srv := &http.Server{
		Handler:      r,
		Addr:         addrHandyman,
		WriteTimeout: timeoutReplyToUser,
		ReadTimeout:  timeoutReplyToUser,
	}
	log.Fatal(srv.ListenAndServe())
}
