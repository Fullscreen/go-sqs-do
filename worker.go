package main

import (
	"fmt"
	"os"
	"os/exec"
)

type Worker struct {
	Command  []string
	JobQueue chan *Job
	Results  chan *Job
}

func (w Worker) Start() {
	for {
		job := <-w.JobQueue
		job.Error = w.Process(job)
		w.Results <- job
	}
}

func (w Worker) Process(job *Job) error {
	cmd := exec.Command(w.Command[0], w.Command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// setup environment
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("SQS_BODY=%s", *job.Message.Body),
		fmt.Sprintf("SQS_MESSAGE_ID=%s", *job.Message.MessageID),
		fmt.Sprintf("SQS_RECEIPT_HANDLE=%s", *job.Message.ReceiptHandle),
		fmt.Sprintf("SQS_REGION=%s", *job.Region),
	)
	cmd.Env = env

	cmd.Start()
	return cmd.Wait()
}
