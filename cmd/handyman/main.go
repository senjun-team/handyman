package main

import (
	"net/http"
	"runtime"
	"time"
	"os"

	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"github.com/gorilla/mux"
	"senjun.ru/handyman/internal"
)

const version = "1.0"
const addrHandyman = "127.0.0.1:8080"

const timeoutReplyToUser = 20 * time.Second

func main() {
	internal.Logger = &log.Logger{
        Out:   os.Stderr,
        Level: log.DebugLevel,
        Formatter: &prefixed.TextFormatter{
            DisableColors: true,
            TimestampFormat : "2006-01-02 15:04:05.000",
            FullTimestamp:true,
            ForceFormatting: true,
        },
    }

	internal.Logger.WithFields(log.Fields{
		"version":    version,
		"address":    addrHandyman,
		"GOMAXPROCS": runtime.GOMAXPROCS(-1),
	}).Info("Started handyman")

	internal.DB = internal.ConnectDb()
	defer internal.DB.Close()
	internal.Logger.Info("DB is online, checked connection")

	internal.WP = workerpool.New(2)
	internal.Logger.Info("Created worker pool for DB deferred queries")

	r := mux.NewRouter()
	r.HandleFunc("/get_courses", internal.HandleGetCourses)
	
	r.HandleFunc("/update_course_progress", internal.HandleUpdateCourseProgress)
	r.HandleFunc("/update_chapter_progress", internal.HandleUpdateChapterProgress)
	r.HandleFunc("/run_task", internal.HandleRunTask)
	r.HandleFunc("/get_progress", internal.HandleGetProgress)
	r.HandleFunc("/get_chapter", internal.HandleGetChapter)

	r.HandleFunc("/get_chapters", internal.HandleGetChapters)

	srv := &http.Server{
		Handler:      r,
		Addr:         addrHandyman,
		WriteTimeout: timeoutReplyToUser,
		ReadTimeout:  timeoutReplyToUser,
	}
	internal.Logger.Fatal(srv.ListenAndServe())
}
