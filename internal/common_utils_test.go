package internal

import (
	"strings"
	"testing"
)

func TestGetContainerType(t *testing.T) {
	chapterId := "python_chapter_0031"
	plan := "senjun_courses_python"
	fact := GetContainerType(chapterId)

	if plan != fact {
		t.Fatalf(`Wrong container type. Plan: %v Fact: %v`, plan, fact)
	}
}

func TestFillOptionsByTaskId(t *testing.T) {
	var opts Options
	opts.TaskId = "python_chapter_0010_task_0040"

	err := FillOptionsByTaskId(&opts)
	if err != nil {
		t.Fatalf(`Couldn't fill options by task id: %v`, err)
	}

	planChapterId := "python_chapter_0010"

	if opts.ChapterId != planChapterId {
		t.Fatalf(`Wrong chapter id. Plan: %v Fact: %v`, planChapterId, opts.ChapterId)
	}

	planCourseId := "python"
	if opts.CourseId != planCourseId {
		t.Fatalf(`Wrong course id. Plan: %v Fact: %v`, planCourseId, opts.CourseId)
	}
}

func TestFillOptionsByChapterId(t *testing.T) {
	var opts Options
	opts.ChapterId = "rust_chapter_0021"

	err := FillOptionsByChapterId(&opts)
	if err != nil {
		t.Fatalf(`Couldn't fill options by chapter id: %v`, err)
	}

	planCourseId := "rust"
	if opts.CourseId != planCourseId {
		t.Fatalf(`Wrong course id. Plan: %v Fact: %v`, planCourseId, opts.CourseId)
	}
}

func TestGetPathToTaskWrapper(t *testing.T) {
	var opts Options
	opts.TaskId = "go_chapter_0006_task_0001"
	err := FillOptionsByTaskId(&opts)
	if err != nil {
		t.Fatalf(`Couldn't fill options by task id: %v`, err)
	}

	plan := "/data/courses/go/go_chapter_0006/tasks/go_chapter_0006_task_0001/wrapper"
	fact := GetPathToTaskWrapper(opts)

	if plan != fact {
		t.Fatalf(`Wrong path. Plan: %v Fact: %v`, plan, fact)
	}
}

func TestGetPathToChapterText(t *testing.T) {
	courseId := "rust"
	chapterId := "rust_chapter_0052"
	plan := "/data/courses/rust/rust_chapter_0052/text.md"
	fact, err := GetPathToChapterText(courseId, chapterId)

	if err != nil {
		t.Fatalf(`Couldn't get path to chapter text: %v`, err)
	}

	if plan != fact {
		t.Fatalf(`Wrong path. Plan: %v Fact: %v`, plan, fact)
	}
}

func TestgenTaskTmpId(t *testing.T) {
	var opts Options
	opts.TaskId = "bash_chapter_0008_task_0011"
	opts.userId = "73563"

	err := FillOptionsByTaskId(&opts)
	if err != nil {
		t.Fatalf(`Couldn't fill options by task id: %v`, err)
	}

	planPrefix := "73563_bash_chapter_0008_task_0011_"
	fact := genTaskTmpId(opts)

	if !strings.HasPrefix(fact, planPrefix) {
		t.Fatalf(`Wrong task tmp id. Plan prefix: %v Fact string: %v`, planPrefix, fact)
	}
}
