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
	Status      int    `json:"error_code"`
	Output      string `json:"output"`
	TestsStatus int    `json:"tests_error_code,omitempty"`
	TestsError  string `json:"tests_error,omitempty"`
	err         error
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

	postBody, _ := json.Marshal(map[string]string{
		"container_type": opts.containerType,
		"source_test":    opts.SourceCodeTest,
		"source_run":     opts.SourceCodeRun,
	})
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
		}).Error("Couldn't parse json body")
		res.err = err
		c <- *res
		return
	}

	res.err = nil
	c <- *res
}

func HandleRunTask(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractRunTaskOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)
		return
	}

	isAllowed, err := IsUserAllowedToRunTask(opts)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't check user permissions on this chapter: %s", err),
		})
		w.Write(body)
		return
	}

	if !isAllowed {
		body, _ := json.Marshal(map[string]string{
			"error": "User doesn't have permissions on this chapter",
		})
		w.Write(body)
		return
	}

	err = InjectCodeToTestWrapper(&opts)

	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't prepare tests for wrapper task runner",
		})
		w.Write(body)
		return
	}

	err = InjectCodeToRunWrapper(&opts)

	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't prepare run wrapper for task runner",
		})
		w.Write(body)
		return
	}

	c := make(chan RunTaskResult)
	go communicateWatchman(opts, c)
	res := <-c

	if res.err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't communicate with tasks runner: %s", res.err),
		})
		w.Write(body)
		return
	}

	WP.Submit(func() {
		UpdateTaskStatus(opts.userId, opts.TaskId, res.Status == 0 && res.TestsStatus == 0, opts.SourceCodeOriginal)
	})

	Logger.WithFields(log.Fields{
		"user_code_status": res.Status,
		"tests_code":       res.TestsStatus,
	}).Info("Successfully communicated watchman")

	json.NewEncoder(w).Encode(res)
}
