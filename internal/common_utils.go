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

type OptionsTg struct {
	UserIdCur int `json:"cur_user_id"`
	UserIdOld int `json:"old_user_id"`
	UserIdNew int `json:"new_user_id"`
}

func ParseOptionsTg(r *http.Request) (OptionsTg, error) {
	var opts OptionsTg
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return OptionsTg{}, err
	}

	return opts, err
}

type Options struct {
	CourseId           string `json:"course_id,omitempty"`
	ChapterId          string `json:"chapter_id,omitempty"`
	TaskId             string `json:"task_id,omitempty"`
	TaskType           string `json:"task_type,omitempty"`
	SourceCodeOriginal string `json:"solution_text,omitempty"`
	SourceCodeRun      string `json:"source_run"`
	SourceCodeTest     string `json:"source_test"`

	Status string `json:"status,omitempty"`

	// Filled based on the chapter id prefix
	containerType string `json:"container_type"`

	userId string

	ColorOutput          bool   `json:"color_output,omitempty"`
	RunStaticTypeChecker bool   `json:"run_static_type_checker,omitempty"`
	ExampleId            string `json:"example_id,omitempty"`
}

type OptionsPlayground struct {
	PlaygroundId string `json:"playground_id,omitempty"`
	LangId       string `json:"lang_id,omitempty"`
	UserCode     string `json:"user_code,omitempty"`
	Project      string `json:"project,omitempty"`
	userId       string
}

type WatchmanOptions struct {
	SourceCodeRun  string   `json:"source_run"`
	Project        string   `json:"project,omitempty"`
	SourceCodeTest string   `json:"source_test,omitempty"`
	ContainerType  string   `json:"container_type"`
	CmdLineArgs    []string `json:"cmd_line_args,omitempty"`
}

type Practice struct {
	Title              string `json:"title,omitempty"`
	ChapterId          string `json:"chapter_id,omitempty"`
	NextChapterId      string `json:"next_chapter_id,omitempty"`
	Status             string `json:"status,omitempty"`
	ProjectPath        string `json:"project_path,omitempty"`
	Project            string `json:"project,omitempty"`
	ProjectDescription string `json:"project_description,omitempty"`
	ProjectHint        string `json:"project_hint,omitempty"`
	Tags               string `json:"tags,omitempty"`
	MainFile           string `json:"main_file,omitempty"`
	DefaultCmdLineArgs string `json:"default_cmd_line_args,omitempty"`
}

func ParseOptions(r *http.Request) (Options, error) {
	var opts Options
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return Options{}, err
	}

	opts.userId = GetUserId(r)

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

func GetContainerType(s string) string {
	if strings.HasPrefix(s, "python") {
		return "python"
	}
	if strings.HasPrefix(s, "rust") {
		return "rust"
	}

	if strings.HasPrefix(s, "go") {
		return "golang"
	}

	if strings.HasPrefix(s, "haskell") {
		return "haskell"
	}

	if strings.HasPrefix(s, "cpp") {
		return "cpp"
	}

	return ""
}

func GetUserId(r *http.Request) string {
	urlParams := r.URL.Query()
	return urlParams.Get("user_id")
}

const rootCourses = "/data/courses/"
const injectMarker = "#INJECT-b585472fa"
const injectEscapedMarker = "#INJECT-ESCAPED-b585472fa"

func GetPathToWrapper(opts *Options, filename string) string {
	pathRun := filepath.Join(rootCourses, opts.CourseId, opts.ChapterId, "tasks", opts.TaskId, filename)

	if len(opts.ExampleId) > 0 {
		pathRun = filepath.Join(rootCourses, opts.CourseId, opts.ChapterId, "examples", opts.ExampleId, filename)
	}

	_, err := os.Stat(pathRun)

	if err == nil {
		return pathRun
	}

	return filepath.Join(rootCourses, opts.CourseId, filename+"_fallback")
}

func GetPathToChapterText(courseId string, chapterId string) (string, string) {
	return filepath.Join(rootCourses, courseId, chapterId, "text.md"),
		filepath.Join(rootCourses, courseId, chapterId, "keywords.md")
}

func ReadTextFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		Logger.WithFields(log.Fields{
			"Error":    err,
			"filepath": path,
		}).Warn("Couldn't read file")
		return "", err
	}

	return string(content), err
}

func runeReplacementMapping(r rune) rune {
	switch r {
	case '”':
		return '"'
	case '“':
		return '"'
	case '‘':
		return '\''
	case '’':
		return '\''
	case '\u00A0': // No-Break Space
		return ' '
	default:
		return r
	}
}

func normalizeCode(opts *Options) {
	opts.SourceCodeOriginal = strings.ReplaceAll(opts.SourceCodeOriginal, "…", "...")
	opts.SourceCodeOriginal = strings.Map(runeReplacementMapping, opts.SourceCodeOriginal)
}

func normalizeCodePlayground(opts *OptionsPlayground) {
	opts.UserCode = strings.ReplaceAll(opts.UserCode, "…", "...")
	opts.UserCode = strings.Map(runeReplacementMapping, opts.UserCode)
}

func InjectCodeToTestWrapper(opts *Options) error {
	wrapperPath := GetPathToWrapper(opts, "wrapper_test")
	content, err := ReadTextFile(wrapperPath)
	if err != nil {
		return err
	}

	sourceCodeEscaped := strings.ReplaceAll(opts.SourceCodeOriginal, "\"\"\"", "\\\"\\\"\\\"")

	opts.SourceCodeTest = strings.ReplaceAll(string(content), injectMarker, opts.SourceCodeOriginal)

	opts.SourceCodeTest = strings.ReplaceAll(opts.SourceCodeTest, injectEscapedMarker, sourceCodeEscaped)
	return nil
}

func InjectCodeToWrapper(opts *Options, wrapperName string) error {
	if opts.TaskType != "code" {
		opts.SourceCodeRun = ""
		return nil
	}

	if wrapperName != "wrapper_playground" {
		wrapperPath := GetPathToWrapper(opts, wrapperName)
		content, err := ReadTextFile(wrapperPath)
		if err != nil {
			return err
		}

		opts.SourceCodeRun = strings.ReplaceAll(string(content), injectMarker, opts.SourceCodeOriginal)
		return nil
	}

	wrapperPath := GetPathToWrapper(opts, wrapperName)
	content, err := ReadTextFile(wrapperPath)
	if err != nil {
		return err
	}

	// Add indendations to code
	s := string(content)
	i := strings.Index(s, injectMarker) // one and only marker index in wrapper

	if i == -1 {
		opts.SourceCodeRun = s
		return nil
	}

	if i > 0 {
		j := strings.LastIndex(s[:i], "\n") + 1 // the next char to the nearest new line to marker. or index 0
		opts.SourceCodeOriginal = strings.ReplaceAll(opts.SourceCodeOriginal, "\n", "\n"+s[j:i])
	}

	opts.SourceCodeRun = strings.ReplaceAll(s, injectMarker, opts.SourceCodeOriginal)
	return nil
}

func IsNewStatusValid(curStatus string, newStatus string) bool {
	// Possible statuses:
	// 'not_started' or empty, 'in_progress', 'blocked', 'completed'

	// In case we want to block any course/chapter/task
	if newStatus == "blocked" {
		return true
	}

	if (len(curStatus) == 0 || curStatus == "not_started") && (newStatus == "in_progress" || newStatus == "completed") {
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
	Description string `json:"description"`
	Tags        string `json:"tags"`
}

type CourseStatus struct {
	CourseId         string `json:"course_id"`
	Title            string `json:"title"`
	TotalChapters    int    `json:"total_chapters"`
	FinishedChapters int    `json:"finished_chapters"`
	Status           string `json:"status,omitempty"`
}

type ChapterForUser struct {
	ChapterId          string `json:"chapter_id"`
	Status             string `json:"status"`
	CourseStatus       string `json:"course_status"`
	Title              string `json:"title"`
	NextChapterId      string `json:"next_chapter_id,omitempty"`
	ParentChapterId    string `json:"parent_chapter_id,omitempty"`
	ParentChapterTitle string `json:"parent_chapter_title,omitempty"`
	TasksTotal         int    `json:"tasks_total"`
	TasksCompleted     int    `json:"tasks_completed"`
}

type TaskForUser struct {
	TaskId   string `json:"task_id"`
	UserCode string `json:"task_code"`
	Status   string `json:"status"`
}

type ChapterContent struct {
	ChapterForUser
	Content    string        `json:"content"`
	Tasks      []TaskForUser `json:"tasks"`
	Keywords   string        `json:"keywords,omitempty"`
	IsPractice bool          `json:"is_practice"`
}

type UserProgress struct {
	StatusOnChapter string `json:"user_status_on_chapter"`

	NotCompletedTaskIds []string          `json:"not_completed_tasks,omitempty"`
	NextChapterId       string            `json:"next_chapter_id,omitempty"`
	IsCourseCompleted   bool              `json:"is_course_completed,omitempty"`
	CourseId            string            `json:"course_id,omitempty"`
	PracticeProjects    []PracticeProject `json:"practice_projects,omitempty"`
}

type PracticeProject struct {
	Title     string `json:"title"`
	ProjectId string `json:"project_id"`
	Status    string `json:"status"`
}
