package internal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const timeoutReplyFromWatchman = 30 * time.Second
const addrWatchman = "http://127.0.0.1:8000/check"

type RunTaskResult struct {
	StatusCode     int    `json:"status_code"`
	UserCodeOutput string `json:"user_code_output"`
	TestsOutput    string `json:"tests_output,omitempty"`
	err            error
}

func extractRunTaskOptions(r *http.Request) (Options, error) {
	opts, err := ParseOptions(r)

	if err != nil {
		Logger.WithFields(log.Fields{
			"Error":     err,
			"http_body": r.Body,
		}).Error("Couldn't parse HTTP request for running task")
		return Options{}, err
	}

	if len(opts.CourseId) == 0 {
		return Options{}, errors.New("empty course id")
	}

	if len(opts.SourceCodeOriginal) == 0 {
		return Options{}, errors.New("empty source code")
	}

	sourceCodeDecoded, err := base64.StdEncoding.DecodeString(opts.SourceCodeOriginal)
	if err != nil {
		return Options{}, errors.New("couldn't decode string from base64-encoded 'solution_text'")
	}

	opts.SourceCodeOriginal = string(sourceCodeDecoded)

	opts.containerType = GetContainerType(opts.ChapterId)

	if len(opts.containerType) == 0 {
		return Options{}, errors.New("Couldn't specify container for chapter " + opts.ChapterId)
	}

	return opts, nil
}

func communicateWatchman(opts Options, c chan RunTaskResult) {
	defer close(c)
	res := new(RunTaskResult)

	var watchmanOpts WatchmanOptions
	watchmanOpts.ContainerType = opts.containerType
	watchmanOpts.SourceCodeRun = opts.SourceCodeRun
	watchmanOpts.SourceCodeTest = opts.SourceCodeTest

	colorArg := "-c always"
	if !opts.ColorOutput {
		colorArg = "-c never"
	}

	watchmanOpts.CmdLineArgs = append(watchmanOpts.CmdLineArgs, colorArg)

	if opts.RunStaticTypeChecker {
		watchmanOpts.CmdLineArgs = append(watchmanOpts.CmdLineArgs, "-t strict")
	}

	watchmanOpts.CmdLineArgs = append(watchmanOpts.CmdLineArgs, "-v "+opts.TaskType)

	postBody, err := json.Marshal(watchmanOpts)
	if err != nil {
		Logger.WithFields(log.Fields{
			"Error": err},
		).Error("Couldn't json.marshal opts.RutTaskOptions for watchman")

		res.err = err
		c <- *res
		return
	}

	reqBody := bytes.NewBuffer(postBody)

	client := http.Client{
		Timeout: timeoutReplyFromWatchman,
	}

	resp, err := client.Post(addrWatchman, "application/json", reqBody)

	if err != nil {
		Logger.WithFields(log.Fields{
			"Error": err},
		).Error("Couldn't send request to watchman")

		res.err = err
		c <- *res
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		res.err = errors.New("HTTP error " + strconv.Itoa(resp.StatusCode))
		c <- *res
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Logger.WithFields(log.Fields{
			"Error": err,
		}).Error("Couldn't read body")
		res.err = err
		c <- *res
		return
	}

	err = json.Unmarshal(body, &res)

	if err != nil {
		Logger.WithFields(log.Fields{
			"Error": err,
			"Body":  body,
		}).Error("Couldn't parse json body")
		res.err = err
		c <- *res
		return
	}

	res.err = nil
	c <- *res
}

func HandleRunTask(w http.ResponseWriter, r *http.Request) {
	countRunTaskTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractRunTaskOptions(r)
	if err != nil {
		countRunTaskErrClient.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Warning("/run_task: couldn't parse request")

		return
	}

	if len(opts.TaskType) == 0 {
		opts.TaskType = "code"
	}

	Logger.WithFields(log.Fields{
		"user_id":      opts.userId,
		"task_id":      opts.TaskId,
		"color_output": opts.ColorOutput,
		"task_type":    opts.TaskType,
	}).Info("/run_task: parsed options")

	if len(opts.userId) == 0 {
		countRunTaskErrClient.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id",
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
		}).Warning("/run_task: couldn't get user_id")
		return
	}

	// Replaces symbols for iOS users
	// https://github.com/senjun-team/senjun-courses/issues/31
	normalizeCode(&opts)
	err = InjectCodeToTestWrapper(&opts)

	if err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't prepare tests for wrapper task runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"task_id":    opts.TaskId,
			"chapter_id": opts.ChapterId,
			"course_id":  opts.CourseId,
			"error":      err.Error(),
		}).Error("/run_task: couldn't inject code to test wrapper")
		return
	}

	err = InjectCodeToRunWrapper(&opts)

	if err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't prepare run wrapper for task runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":    opts.userId,
			"task_id":    opts.TaskId,
			"chapter_id": opts.ChapterId,
			"course_id":  opts.CourseId,
			"error":      err.Error(),
		}).Error("/run_task: couldn't inject code to run wrapper")
		return
	}

	c := make(chan RunTaskResult)
	go communicateWatchman(opts, c)
	res := <-c

	if res.err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't communicate with tasks runner: %s", res.err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   res.err.Error(),
		}).Error("/run_task: error communicating with watchman")
		return
	}

	if UpdateStatus(opts.userId, opts.TaskId, opts.ChapterId, opts.CourseId,
		res.StatusCode == 0, opts.SourceCodeOriginal) {
		countRunTaskOk.Inc()
	} else {
		countRunTaskErrServer.Inc()
	}

	Logger.WithFields(log.Fields{
		"user_id":     opts.userId,
		"task_id":     opts.TaskId,
		"status_code": res.StatusCode,
	}).Info("/run_task: completed")

	json.NewEncoder(w).Encode(res)
}
