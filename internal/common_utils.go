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
	CourseId           string `json:"course_id,omitempty"`
	ChapterId          string `json:"chapter_id,omitempty"`
	TaskId             string `json:"task_id,omitempty"`
	SourceCodeOriginal string `json:"solution_text,omitempty"`
	SourceCodeRun      string `json:"text_for_exed,omitempty"`
	SourceCodeTest     string `json:"text_for_test",omitempty`
	Status             string `json:"status,omitempty"`
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

	if len(opts.TaskId) > 0 {
		err := FillOptionsByTaskId(&opts)
		if err != nil {
			return Options{}, err
		}
	} else if len(opts.ChapterId) > 0 {
		err := FillOptionsByChapterId(&opts)
		if err != nil {
			return Options{}, err
		}
	}

	return opts, err
}

const taskIdFixedSize = 9      // task_0042
const chapterIdSuffixSize = 12 // chapter_0015
const splitChar = "_"
const splitCharLen = len(splitChar)
const minChapterIdLen = 1 + splitCharLen + chapterIdSuffixSize
const minTaskIdLen = minChapterIdLen + splitCharLen + taskIdFixedSize

func FillOptionsByTaskId(opts *Options) error {
	if len(opts.TaskId) == 0 {
		return errors.New("empty task id")
	}

	if len(opts.TaskId) < minTaskIdLen {
		return errors.New("invalid length of task id")
	}

	opts.ChapterId = opts.TaskId[:len(opts.TaskId)-taskIdFixedSize-1]
	opts.CourseId = opts.ChapterId[:len(opts.ChapterId)-chapterIdSuffixSize-splitCharLen]

	return nil
}

func FillOptionsByChapterId(opts *Options) error {
	if len(opts.ChapterId) == 0 {
		return errors.New("empty chapter id")
	}

	if len(opts.ChapterId) < minChapterIdLen {
		return errors.New("invalid length of chapter id")
	}

	opts.CourseId = opts.ChapterId[:len(opts.ChapterId)-chapterIdSuffixSize-splitCharLen]

	return nil
}

func GetContainerType(chapterId string) string {
	if strings.HasPrefix(chapterId, "python") {
		return "python"
	}
	if strings.HasPrefix(chapterId, "rust") {
		return "rust"
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

const rootCourses = "/data/courses/"
const injectMarker = "#INJECT-b585472fa"

// Gets root path to courses (for example '/courses'),
// opts.TaskId (for example 'python_chapter_0010_task_0060'),
// returns path to task wrapper
func GetPathToTestWrapper(opts *Options) string {
	return filepath.Join(rootCourses, opts.CourseId, opts.ChapterId, "tasks", opts.TaskId, "wrapper_test")
}

func GetPathToRunWrapper(opts *Options) string {
	pathRun := filepath.Join(rootCourses, opts.CourseId, opts.ChapterId, "tasks", opts.TaskId, "wrapper_run")

	_, err := os.Stat(pathRun)

	if err == nil {
		return pathRun
	}

	return filepath.Join(rootCourses, opts.CourseId, "wrapper_run_fallback")
}

func GetPathToChapterText(courseId string, chapterId string) (string, error) {
	return filepath.Join(rootCourses, courseId, chapterId, "text.md"), nil
}

func ReadTextFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		Logger.WithFields(log.Fields{
			"Error":    err,
			"filepath": path,
		}).Error("Couldn't read file")
		return "", err
	}

	return string(content), err
}

func InjectCodeToTestWrapper(opts *Options) error {
	wrapperPath := GetPathToTestWrapper(opts)
	content, err := ReadTextFile(wrapperPath)

	opts.SourceCodeTest = strings.ReplaceAll(string(content), injectMarker, opts.SourceCodeOriginal)

	Logger.WithFields(log.Fields{
		"filepath": wrapperPath,
	}).Info("Injected test wrapper")
	return err
}

func InjectCodeToRunWrapper(opts *Options) error {
	wrapperPath := GetPathToRunWrapper(opts)
	content, err := ReadTextFile(wrapperPath)

	opts.SourceCodeRun = strings.ReplaceAll(string(content), injectMarker, opts.SourceCodeOriginal)

	Logger.WithFields(log.Fields{
		"filepath": wrapperPath,
	}).Info("Injected run wrapper")
	return err
}

func IsNewStatusValid(curStatus string, newStatus string) bool {
	// Possible statuses:
	// 'not_started' or empty, 'in_progress', 'blocked', 'completed'

	// In case we want to block any course/chapter/task
	if newStatus == "blocked" {
		return true
	}

	if (len(curStatus) == 0 || curStatus == "not_started") && newStatus == "in_progress" {
		return true
	}

	if curStatus == "in_progress" && newStatus == "completed" {
		return true
	}

	return false
}

type CourseForUser struct {
	CourseId    string `json:"course_id"`
	CourseType  string `json:"type"`
	Status      string `json:"status,omitempty"`
	Path        string `json:"-"`
	Title       string `json:"title"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type ChapterForUser struct {
	ChapterId string `json:"chapter_id"`
	Status    string `json:"status"`
	Title     string `json:"title"`
}

type TaskForUser struct {
	TaskId   string `json:"task_id"`
	UserCode string `json:"task_code"`
	Status   string `json:"status"`
}

type ChapterContent struct {
	ChapterForUser
	Content string        `json:"content"`
	Tasks   []TaskForUser `json:"tasks"`
}

type UserProgress struct {
	StatusOnChapter string `json:"user_status_on_chapter"`

	NotCompletedTaskIds []string `json:"not_completed_tasks,omitempty"`
	NextChapterId       string   `json:"next_chapter_id,omitempty"`
	IsCourseCompleted   bool     `json:"is_course_completed,omitempty"`
	CourseId            string   `json:"course_id,omitempty"`
}
