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

	log "github.com/sirupsen/logrus"
)

const addrWatchman = "http://127.0.0.1:8000/check"
const addrWatchmanPlayground = "http://127.0.0.1:8000/playground"
const addrWatchmanPractice = "http://127.0.0.1:8000/practice"

type RunTaskResult struct {
	StatusCode     int    `json:"status_code"` // 0 = ok, 1 = err running code, 2 = didn't pass tests, other = unexpected
	UserCodeOutput string `json:"user_code_output"`
	TestsOutput    string `json:"tests_output,omitempty"`
}

type PracticeReq struct {
	ProjectContents string `json:"project_contents"`
	ProjectId       string `json:"project_id"`
	CourseId        string `json:"course_id"`
	CmdLineArgs     string `json:"user_cmd_line_args"`
	Action          string `json:"action"` // run, test, save
	userId          string
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

func extractOptionsRunTask(r *http.Request) (Options, error) {
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

	if opts.SourceCodeOriginal != " " {
		sourceCodeDecoded, err := base64.StdEncoding.DecodeString(opts.SourceCodeOriginal)
		if err != nil {
			return Options{}, errors.New("couldn't decode string from base64-encoded 'solution_text'")
		}

		opts.SourceCodeOriginal = string(sourceCodeDecoded)
	}

	opts.containerType = GetContainerType(opts.ChapterId)

	if len(opts.containerType) == 0 {
		return Options{}, errors.New("Couldn't specify container for chapter " + opts.ChapterId)
	}

	return opts, nil
}

func sendRequestToWatchman(api string, postBody *[]byte) ([]byte, error) {
	client := &http.Client{
		Timeout: 0,
	}
	req, err := http.NewRequest("POST", api, bytes.NewBuffer(*postBody))
	if err != nil {
		Logger.WithFields(log.Fields{
			"api":   api,
			"error": err,
		},
		).Error("Couldn't create request for watchman")

		return []byte{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	//req.Header.Set("Connection", "close")
	//req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		Logger.WithFields(log.Fields{
			"api":   api,
			"error": err,
		},
		).Error("Client.Do() error with watchman")

		return []byte{}, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Logger.WithFields(log.Fields{
			"api":   api,
			"error": err,
		},
		).Error("Couldnt' read watchman response body")

		return []byte{}, err
	}

	return body, nil
}

func getRequestBodyRunTask(opts *Options) ([]byte, error) {
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

	return json.Marshal(watchmanOpts)
}

func getRequestBodyPlayground(opts *OptionsPlayground) ([]byte, error) {
	var watchmanOpts WatchmanOptions
	watchmanOpts.ContainerType = opts.LangId
	watchmanOpts.SourceCodeRun = opts.UserCode
	watchmanOpts.Project = opts.Project

	return json.Marshal(watchmanOpts)
}

func HandleInjectPlaygroundCode(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractOptionsRunTask(r)
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

	opts, err := extractOptionsRunTask(r)
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

	bodyReq, err := getRequestBodyRunTask(&opts)
	if err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't communicate with tasks runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Error("/run_task: error communicating with watchman (getRequestBodyRunTask)")
		return
	}

	bodyResp, err := sendRequestToWatchman(addrWatchman, &bodyReq)

	if err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't communicate with tasks runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id":     opts.userId,
			"task_id":     opts.TaskId,
			"raw_request": string(bodyReq[:]),
			"error":       err.Error(),
		}).Error("/run_task: error communicating with watchman (sendRequestToWatchman)")
		return
	}

	res := new(RunTaskResult)
	err = json.Unmarshal(bodyResp, &res)
	if err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't communicate with tasks runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Error("/run_task: error extracting json from watchman resp")
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

func HandleSaveTask(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-type", "application/json")

	opts, err := extractOptionsRunTask(r)
	if err != nil {
		body, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Invalid request: %s", err),
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
			"error":   err.Error(),
		}).Warning("/save_task: couldn't parse request")

		return
	}

	if len(opts.TaskType) == 0 {
		opts.TaskType = "code"
	}

	Logger.WithFields(log.Fields{
		"user_id": opts.userId,
		"task_id": opts.TaskId,
	}).Info("/save_task: parsed options")

	if len(opts.userId) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Couldn't get user_id",
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
		}).Warning("/save_task: couldn't get user_id")
		return
	}

	// Replaces strange symbols (no-break space, ... for iOS users, etc)
	// https://github.com/senjun-team/senjun-courses/issues/31
	normalizeCode(&opts)

	if !SaveTask(opts.userId, opts.TaskId, opts.ChapterId, opts.CourseId, opts.SourceCodeOriginal) {
		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"task_id": opts.TaskId,
		}).Error("/save_task: couldn't save to DB")

		body, _ := json.Marshal(map[string]int{
			"status_code": 1,
		})

		w.Write(body)
	}

	Logger.WithFields(log.Fields{
		"user_id": opts.userId,
		"task_id": opts.TaskId,
	}).Info("/save_task: completed")

	body, _ := json.Marshal(map[string]int{
		"status_code": 0,
	})

	w.Write(body)
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

	if len(opts.Project) == 0 {
		countRunCodeErrClient.Inc()

		json.NewEncoder(w).Encode(map[string]int{
			"status_code": 1,
		})

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
		}).Warning("/run_code: required fields not set in request")
		return
	}

	// Replaces strange symbols (no-break space, ... for iOS users, etc)
	// https://github.com/senjun-team/senjun-courses/issues/31
	normalizeCodePlayground(&opts)

	bodyReq, err := getRequestBodyPlayground(&opts)
	if err != nil {
		countRunCodeErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't communicate with tasks runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
			"error":   err.Error(),
		}).Error("/run_code: error communicating watchman")
		return
	}

	bodyResp, err := sendRequestToWatchman(addrWatchmanPlayground, &bodyReq)

	if err != nil {
		countRunCodeErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't communicate with tasks runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
			"error":   err.Error(),
		}).Error("/run_code: error communicating watchman")
		return
	}

	res := new(RunTaskResult)
	err = json.Unmarshal(bodyResp, &res)
	if err != nil {
		countRunTaskErrServer.Inc()

		body, _ := json.Marshal(map[string]string{
			"error": "Couldn't communicate with tasks runner",
		})
		w.Write(body)

		Logger.WithFields(log.Fields{
			"user_id": opts.userId,
			"lang_id": opts.LangId,
			"error":   err.Error(),
		}).Error("/run_code: error extracting json from watchman resp")
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
