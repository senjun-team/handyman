package main

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"senjun.ru/handyman/internal"
)

const version = "1.0"
const addrHandyman = "127.0.0.1:8080"

const timeoutReplyToUser = 50 * time.Second

func main() {
	internal.Logger = &log.Logger{
		Out:   os.Stderr,
		Level: log.DebugLevel,
		Formatter: &prefixed.TextFormatter{
			DisableColors:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			FullTimestamp:   true,
			ForceFormatting: true,
		},
	}

	internal.Logger.WithFields(log.Fields{
		"version":    version,
		"address":    addrHandyman,
		"GOMAXPROCS": runtime.GOMAXPROCS(-1),
	}).Info("Started handyman")

	if len(os.Args) > 1 {
		internal.Logger.WithFields(log.Fields{
			"path": os.Args[1],
		}).Info("Setting courses path from 1st agrument")
		internal.RootCourses = os.Args[1]
	} else {
		internal.Logger.WithFields(log.Fields{
			"default_path": internal.RootCourses,
		}).Info("Using default path to courses. You can redefine it by passing as 1st argument")
	}

	internal.DB = internal.ConnectDb()
	defer internal.DB.Close()
	internal.Logger.Info("DB is online, checked connection")

	internal.WP = workerpool.New(12)
	internal.Logger.Info("Created worker pool for DB deferred queries")

	r := mux.NewRouter()
	r.HandleFunc("/get_courses", internal.HandleGetCourses)

	r.HandleFunc("/update_course_progress", internal.HandleUpdateCourseProgress)
	r.HandleFunc("/update_chapter_progress", internal.HandleUpdateChapterProgress)
	r.HandleFunc("/run_task", internal.HandleRunTask)
	r.HandleFunc("/save_task", internal.HandleSaveTask)
	r.HandleFunc("/get_progress", internal.HandleGetProgress)
	r.HandleFunc("/get_chapter", internal.HandleGetChapter)
	r.HandleFunc("/get_practice", internal.HandleGetPractice)
	r.HandleFunc("/get_course_info", internal.HandleGetCourseInfo)
	r.HandleFunc("/get_chapters", internal.HandleGetChapters)
	r.HandleFunc("/get_course_description", internal.HandleGetCourseDescription)

	r.HandleFunc("/get_active_chapter", internal.HandleGetActiveChapter)
	r.HandleFunc("/courses_stats", internal.HandleCoursesStats)
	r.HandleFunc("/course_stats", internal.HandleCourseStats)
	r.HandleFunc("/get_task", internal.HandleGetTask)

	// APIs for syncing telegram bot account and site account:
	r.HandleFunc("/merge_users", internal.HandleMergeUsers)
	r.HandleFunc("/split_users", internal.HandleSplitUsers)

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/run_code", internal.HandleRunCode)
	r.HandleFunc("/get_playground_code", internal.HandleGetPlaygroundCode)

	r.HandleFunc("/inject_playground_code", internal.HandleInjectPlaygroundCode)

	r.HandleFunc("/get_practice", internal.HandleGetPractice)

	// Run, test or save practice project
	r.HandleFunc("/handle_practice_code", internal.HandlePracticeCode)

	srv := &http.Server{
		Handler:      r,
		Addr:         addrHandyman,
		WriteTimeout: timeoutReplyToUser,
		ReadTimeout:  timeoutReplyToUser,
	}
	internal.Logger.Fatal(srv.ListenAndServe())
}
