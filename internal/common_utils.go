package internal

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Options struct {
	CourseId   string `json:"course_id,omitempty"`
	ChapterId  string `json:"chapter_id,omitempty"`
	TaskId     string `json:"task_id,omitempty"`
	SourceCode string `json:"solution_text,omitempty"`
	Status     string `json:"status,omitempty"`
	// Must be extracted later not from HTTP POST body, but
	// from header with JWT access token
	userId string
	// May be filled later based on the chapter id prefix
	containerType string
}

func ParseOptions(r *http.Request) (Options, error) {
	var opts Options
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return Options{}, err
	}

	opts.userId = GetUserId(r)
	if len(opts.userId) == 0 {
		return Options{}, errors.New("invalid user id")
	}

	return opts, err
}

func GetContainerType(chapterId string) string {
	if strings.HasPrefix(chapterId, "python") {
		return "python_env"
	}
	if strings.HasPrefix(chapterId, "rust") {
		return "rust_env"
	}

	return ""
}

func GetUserId(r *http.Request) string {
	urlParams := r.URL.Query()
	return urlParams.Get("user_id")

	// TODO: when Doorman is ready replace this with
	// parsing of JWT token and extracting bearer:
	// jwtToken := r.Header.Get("Authorization")
	// parseUserFormJwt(jwtToken)
}

const taskIdFixedSize = 9      // task_0042
const chapterIdSuffixSize = 12 // chapter_0015
const splitChar = "_"
const splitCharLen = len(splitChar)
const minTaskIdLen = taskIdFixedSize + splitCharLen*2 + chapterIdSuffixSize + 1

const rootCourses = "/etc/courses/"
const injectMarker = "#INJECT"

// Gets root path to courses (for example '/courses'),
// task id (for example 'python_chapter_0010_task_0060'),
// returns path to task wrapper:
// /courses/python/python_chapter_0010/python_chapter_0010_task_0060/wrapper
func GetPathToTaskWrapper(pathToCourses string, taskId string) (string, error) {
	if len(taskId) < minTaskIdLen {
		return "", errors.New("invalid task id length")
	}

	chapterId := taskId[:len(taskId)-taskIdFixedSize-1]
	courseId := chapterId[:len(chapterId)-chapterIdSuffixSize-splitCharLen]

	return filepath.Join(pathToCourses, courseId, chapterId, "tasks", taskId, "wrapper"), nil
}

func InjectCodeToWrapper(taskId string, userCode string) (string, error) {
	wrapperPath, err := GetPathToTaskWrapper(rootCourses, taskId)
	if err != nil {
		log.WithFields(log.Fields{
			"Error":   err,
			"task_id": taskId,
		}).Error("Couldn't get path to task wrapper")
		return "", err
	}

	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		log.WithFields(log.Fields{
			"Error":           err,
			"task_id":         taskId,
			"path_to_wrapper": wrapperPath,
		}).Error("Couldn't read wrapper text")
		return "", err
	}

	return strings.ReplaceAll(string(content), injectMarker, userCode), nil
}

type CoursesForUser struct {
	Courses []CourseForUser `json:"courses"`
}

type CourseForUser struct {
	CourseId        string `json:"course_id"`
	CourseType      string `json:"type"`
	Status          string `json:"status"`
	Path            string `json:"-"`
	Title           string `json:"title"`
	IconPath        string `json:"path_icon"`
	DescriptionPath string `json:"path_description"`
}
