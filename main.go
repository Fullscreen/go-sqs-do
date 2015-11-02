package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"io"
	"os"
	"sync"
)

const (
	ExitCodeOK             int = 0
	ExitCodeError          int = 1
	ExitCodeFlagParseError     = 10 + iota
	ExitCodeAWSError
)

const HelpText string = `Usage: sqs-do -queue <url> [options] -- <command> [options]

Options:
  -h           Print this message and exit
  -count       The number of messages to receive (default 1)
  -concurrency The number of messages to process in parallel
  -hide        Time to keep messages hidden from other subscribers (seconds)
  -queue       The queue to listen to
  -region      The region the queue is in
  -verbose     Enable verbose output
  -version     Print version informtion
  -wait        Time to wait for new messages (seconds)
`

type CLI struct {
	Stdout, Stderr io.Writer
}

type Job struct {
	Command []string
	Error   error
	Message *sqs.Message
	Queue   *string
	Region  *string
}

type Jobs struct {
	running int
	cond    *sync.Cond
}

// invoke the cli with args
func (cli *CLI) Run(args []string) int {
	// parse args string
	flags := flag.NewFlagSet("cFlags", flag.ContinueOnError)
	flags.SetOutput(cli.Stdout)

	count := flags.Int64("count", 1, "The number of message to receive")
	concurrency := flags.Int("concurrency", 1, "The number of messages to process in parallel")
	help := flags.Bool("h", false, "print help and quit")
	hide := flags.Int64("hide", 0, "Time to keep messages hidden")
	queue := flags.String("queue", "", "The queue to listen to")
	region := flags.String("region", "us-east-1", "The region the queue is in")
	verbose := flags.Bool("verbose", false, "Enable verbose output")
	version := flags.Bool("version", false, "Print version information")
	wait := flags.Int64("wait", 10, "Time to wait for new messages")

	// check flag values
	if err := flags.Parse(args[1:]); err != nil {
		fmt.Println(err.Error())
		return ExitCodeFlagParseError
	}
	handlerArgs := flags.Args()

	if *version {
		fmt.Fprintf(cli.Stdout, "%s\n", Version)
		return ExitCodeOK
	}

	if *help {
		fmt.Fprintf(cli.Stderr, HelpText)
		return ExitCodeOK
	}

	if *queue == "" || len(handlerArgs) == 0 {
		fmt.Fprintf(cli.Stderr, HelpText)
		return ExitCodeFlagParseError
	}

	// setup debugging
	debug := func(format string, a ...interface{}) (int, error) {
		if *verbose == false {
			return 0, nil
		}
		return fmt.Fprintf(os.Stderr, format, a...)
	}

	opt := &sqs.ReceiveMessageInput{
		QueueURL:            queue,
		WaitTimeSeconds:     wait,
		MaxNumberOfMessages: count,
	}
	// override queue default
	if *hide > 0 {
		opt.VisibilityTimeout = hide
	}
	svc := sqs.New(&aws.Config{Region: *region})

	// track number of running jobs
	jobs := &Jobs{
		running: 0,
		cond:    &sync.Cond{L: &sync.Mutex{}}}

	// create queue and result channels
	results := make(chan *Job)
	jobQueue := make(chan *Job, *concurrency-1)

	// setup loop
	debug("Listening for messages on %s\n", *queue)

	// setup workers
	for i := 0; i < *concurrency; i++ {
		worker := &Worker{
			Command:  handlerArgs,
			JobQueue: jobQueue,
			Results:  results}
		go worker.Start()
	}

	// watch responses
	go func() {
		for {
			result := <-results
			if result.Error != nil {
				debug("Handler exited with non-zero exit code for ID: %s", *result.Message.MessageID)
			} else {
				// remove the message
				delopt := &sqs.DeleteMessageInput{
					QueueURL:      result.Queue,
					ReceiptHandle: result.Message.ReceiptHandle,
				}
				if _, err := svc.DeleteMessage(delopt); err != nil {
					fmt.Fprintln(cli.Stderr, "Failed to delete message")
				} else {
					debug("Deleted message with ID: %s\n", *result.Message.MessageID)
				}
			}
			jobs.cond.L.Lock()
			jobs.running--
			jobs.cond.L.Unlock()
			jobs.cond.Signal()
		}
	}()

	// start fetching messages
	for {
		// stop fetching messages to avoid refetching if the concurrency
		// limit has been reached
		jobs.cond.L.Lock()
		for jobs.running >= *concurrency {
			jobs.cond.Wait()
		}
		jobs.cond.L.Unlock()

		resp, err := svc.ReceiveMessage(opt)
		if err != nil {
			fmt.Println(err.Error())
			return ExitCodeAWSError
		}

		// update the running job count
		jobs.cond.L.Lock()
		jobs.running += len(resp.Messages)
		jobs.cond.L.Unlock()

		// push each message onto the job queue
		for i := range resp.Messages {
			debug("Receieved message with ID: %s\n", *resp.Messages[i].MessageID)

			jobQueue <- &Job{
				Command: handlerArgs,
				Message: resp.Messages[i],
				Queue:   queue,
				Region:  region}
		}
	}
}
