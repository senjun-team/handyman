package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const timeoutReplyFromWatchman = 30 * time.Second
const addrWatchman = "http://127.0.0.1:8000/check"

type RunTaskResult struct {
	Status  int    `json:"error_code"`
	Message string `json:"output"`
	err     error
}

func extractRunTaskOptions(r *http.Request) (Options, error) {
	opts, err := ParseOptions(r)

	if err != nil {
		log.WithFields(log.Fields{
			"Error":     err,
			"http_body": r.Body,
		}).Error("Couldn't parse HTTP request for running task")
		return Options{}, err
	}

	if len(opts.ChapterId) == 0 {
		return Options{}, errors.New("empty chapter id")
	}

	if len(opts.TaskId) == 0 {
		return Options{}, errors.New("empty task id")
	}

	if len(opts.SourceCode) == 0 {
		return Options{}, errors.New("empty source code")
	}

	opts.containerType = GetContainerType(opts.ChapterId)
	if len(opts.containerType) == 0 {
		return Options{}, errors.New("Couldn't specify container for chapter " + opts.ChapterId)
	}

	return opts, nil
}

func genTaskTmpId(opts Options) string {
	return fmt.Sprintf("%s_%s_%s_%d", opts.ChapterId, opts.userId,
		opts.TaskId, time.Now().UnixNano())
}

func communicateWatchman(opts Options, c chan RunTaskResult) {
	defer close(c)
	res := new(RunTaskResult)

	taskTmpId := genTaskTmpId(opts)
	postBody, _ := json.Marshal(map[string]string{
		"container_type": opts.containerType,
		"source":         opts.SourceCode,
		"task_id":        taskTmpId,
	})
	reqBody := bytes.NewBuffer(postBody)

	client := http.Client{
		Timeout: timeoutReplyFromWatchman,
	}

	resp, err := client.Post(addrWatchman, "application/json", reqBody)

	if err != nil {
		log.WithFields(log.Fields{
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"Error": err,
		}).Error("Couldn't read body")
		res.err = err
		c <- *res
		return
	}

	err = json.Unmarshal(body, &res)

	if err != nil {
		log.WithFields(log.Fields{
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

	opts.SourceCode, err = InjectCodeToWrapper(opts.TaskId, opts.SourceCode)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't prepare tests for task runner",
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

	log.WithFields(log.Fields{
		"status": res.Status,
	}).Info("Successfully communicated watchman")

	json.NewEncoder(w).Encode(res)
}
