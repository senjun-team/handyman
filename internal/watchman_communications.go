package internal

import (
	"bytes"
	"database/sql"
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
const addrWatchmanPlayground = "http://127.0.0.1:8000/playground"

type RunTaskResult struct {
	StatusCode     int    `json:"status_code"`
	UserCodeOutput string `json:"user_code_output"`
	TestsOutput    string `json:"tests_output,omitempty"`
	err            error
}

func extractOptionsPlayground(r *http.Request) (OptionsPlayground, error) {
	var opts OptionsPlayground
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return OptionsPlayground{}, err
	}

	if len(opts.UserCode) > 0 {
		sourceCodeDecoded, err := base64.StdEncoding.DecodeString(opts.UserCode)
		if err != nil {
			return OptionsPlayground{}, errors.New("couldn't decode string from base64-encoded 'solution_text'")
		}

		opts.UserCode = string(sourceCodeDecoded)
	}

	opts.LangId = GetContainerType(opts.LangId)

	opts.userId = GetUserId(r)
	return opts, nil
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

func communicateWatchmanPlayround(opts OptionsPlayground, c chan RunTaskResult) {
	defer close(c)
	res := new(RunTaskResult)

	var watchmanOpts WatchmanOptions
	watchmanOpts.ContainerType = opts.LangId
	watchmanOpts.SourceCodeRun = opts.UserCode

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

	resp, err := client.Post(addrWatchmanPlayground, "application/json", reqBody)

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

func HandleInjectPlaygroundCode(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractRunTaskOptions(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Warning("/inject_playground_code: couldn't parse request")

		return
	}

	Logger.WithFields(log.Fields{
		"user_id":         opts.userId,
		"task_id":         opts.TaskId,
		"source_code_len": len(opts.SourceCodeOriginal),
	}).Info("/inject_playground_code: parsed options")

	opts.TaskType = "code"

	if len(opts.userId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id",
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
		}).Warning("/inject_playground_code: couldn't get user_id")
		return
	}
	err = InjectCodeToWrapper(&opts, "wrapper_playground")

	// There is no wrapper file
	if err != nil {
		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
		}).Info("/inject_playground_code: completed (no inject)")

		json.NewEncoder(w).Encode(map[string]string{
			"user_code": opts.SourceCodeOriginal,
		})
		return
	}

	Logger.WithFields(log.Fields{
		"user_id": opts.userId,
		"task_id": opts.TaskId,
	}).Info("/inject_playground_code: completed")

	json.NewEncoder(w).Encode(map[string]string{
		"user_code": opts.SourceCodeRun,
	})
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

	// Replaces strange symbols (no-break space, ... for iOS users, etc)
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

	err = InjectCodeToWrapper(&opts, "wrapper_run")

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

func HandleRunCode(w http.ResponseWriter, r *http.Request) {
	countRunCodeTotal.Inc()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractOptionsPlayground(r)
	if err != nil {
		countRunCodeErrClient.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":       opts.userId,
			"lang_id":       opts.LangId,
			"playground_id": opts.PlaygroundId,
			"error":         err.Error(),
		}).Warning("/run_code: couldn't parse request")

		return
	}

	Logger.WithFields(log.Fields{
		"user_id":       opts.userId,
		"lang_id":       opts.LangId,
		"playground_id": opts.PlaygroundId,
	}).Info("/run_code: parsed options")

	// Replaces strange symbols (no-break space, ... for iOS users, etc)
	// https://github.com/senjun-team/senjun-courses/issues/31
	normalizeCodePlayground(&opts)

	c := make(chan RunTaskResult)
	go communicateWatchmanPlayround(opts, c)
	res := <-c

	if res.err != nil {
		countRunCodeErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Couldn't communicate with tasks runner: %s", res.err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":       opts.userId,
			"lang_id":       opts.LangId,
			"playground_id": opts.PlaygroundId,
			"error":         res.err.Error(),
		}).Error("/run_code: error communicating with watchman")
		return
	}

	// TODO: update record in 'playgrounds' table

	Logger.WithFields(log.Fields{
		"user_id":       opts.userId,
		"lang_id":       opts.LangId,
		"playground_id": opts.PlaygroundId,
		"status_code":   res.StatusCode,
	}).Info("/run_code: completed")

	json.NewEncoder(w).Encode(res)

	countRunCodeOk.Inc()
}

func HandleGetPlaygroundCode(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractOptionsPlayground(r)

	if err != nil {
		//countUpdateCourseProgressClientError.Inc()

		body, _ := json.Marshal(map[string]int{
			"status_code": 1,
		})

		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
			"error":   err.Error(),
		}).Warning("/get_playground_code: couldn't parse request")
		return
	}

	if len(opts.PlaygroundId) == 0 {
		//countUpdateCourseProgressClientError.Inc()

		json.NewEncoder(w).Encode(map[string]int{
			"status_code": 1,
		})

		Logger.WithFields(log.Fields{
			"user_id":       opts.userId,
			"playground_id": opts.PlaygroundId,
		}).Warning("/get_playground_code: required fields not set in request")
		return
	}

	userCode, err := GetPlaygroundCode(opts.PlaygroundId)
	if err != nil {
		if err == sql.ErrNoRows {
			json.NewEncoder(w).Encode(map[string]int{
				"status_code": 2,
			})

			Logger.WithFields(log.Fields{
				"playground_id": opts.PlaygroundId,
			}).Info("/get_playground_code: no playground found")
			return

		}

		json.NewEncoder(w).Encode(map[string]int{
			"status_code": 3,
		})

		Logger.WithFields(log.Fields{
			"playground_id": opts.PlaygroundId,
			"error":         err.Error(),
		}).Warning("/get_playground_code: couldn't select playground")
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"user_code": userCode,
	})

	Logger.WithFields(log.Fields{
		"playground_id": opts.PlaygroundId,
	}).Info("/get_playground_code: completed")
}

func HandleCreatePlayground(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractOptionsPlayground(r)

	if err != nil {
		//countUpdateCourseProgressClientError.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
			"error":   err.Error(),
		}).Warning("/create_playground: couldn't parse request")
		return
	}

	if len(opts.LangId) == 0 {
		//countUpdateCourseProgressClientError.Inc()

		json.NewEncoder(w).Encode(map[string]string{
			"error": "Required fields are not set in request",
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
		}).Warning("/create_playground: required fields not set in request")
		return
	}

	Logger.WithFields(log.Fields{
		"playround_id": opts.PlaygroundId,
		"lang_id":      opts.LangId,
		"user_id":      opts.userId,
	}).Info("/create_playground: parsed options")

	err = createPlayground(opts.PlaygroundId, opts.LangId, opts.userId, opts.UserCode)
	if err != nil {
		body, _ := json.Marshal(map[string]int{
			"status_code": 3,
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"playround_id": opts.PlaygroundId,
			"lang_id":      opts.LangId,
			"user_id":      opts.userId,
			"error":        err.Error(),
		}).Error("/create_playground: couldn't insert row into table")
		return
	}

	body, _ := json.Marshal(map[string]int{
		"status_code": 0,
	})
	w.Write(body)

	Logger.WithFields(log.Fields{
		"playround_id": opts.PlaygroundId,
		"lang_id":      opts.LangId,
		"user_id":      opts.userId,
	}).Info("/create_playground: completed")

}
