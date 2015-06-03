package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"io"
	"os"
	"os/exec"
)

const (
	ExitCodeOK             int = 0
	ExitCodeError          int = 1
	ExitCodeFlagParseError     = 10 + iota
	ExitCodeAWSError
)

const HelpText string = `Usage: sqs-do [options] -- <command> [options]

Options:
  -h        Print this message and exit
  -q        The queue to listen to
  -v        Enable verbose output
  -version  Print version informtion
`

type CLI struct {
	Stdout, Stderr io.Writer
}

// invoke the cli with args
func (cli *CLI) Run(args []string) int {
	// parse args string
	flags := flag.NewFlagSet("cFlags", flag.ContinueOnError)
	flags.SetOutput(cli.Stdout)

	help := flags.Bool("h", false, "print help and quit")
	verbose := flags.Bool("v", false, "print verbose output")
	version := flags.Bool("version", false, "print version and exit")
	queue := flags.String("q", "", "the queue to listen on")

	// setup debugging
	debug := func(format string, a ...interface{}) (int, error) {
		if *verbose == false {
			return 0, nil
		}
		return fmt.Fprintf(os.Stderr, format, a...)
	}

	if err := flags.Parse(args[1:]); err != nil {
		fmt.Println(err.Error())
		return ExitCodeFlagParseError
	}

	if *version {
		fmt.Fprintf(cli.Stdout, "%s\n", Version)
		return ExitCodeOK
	}

	if *help {
		fmt.Fprintf(cli.Stderr, HelpText)
		return ExitCodeOK
	}

	opt := &sqs.ReceiveMessageInput{
		QueueURL:          queue,
		WaitTimeSeconds:   aws.Long(2),
		VisibilityTimeout: aws.Long(1),
	}
	svc := sqs.New(&aws.Config{Region: "us-east-1"})
	childArgs := flags.Args()

	// setup loop
	fmt.Fprintf(cli.Stdout, "Listening for messages on %s\n", *queue)
	for {
		resp, err := svc.ReceiveMessage(opt)
		count := len(resp.Messages)
		if err != nil {
			fmt.Println(err.Error())
			return ExitCodeAWSError
		}
		if count > 0 {
			debug("Received %d message(s)\n", len(resp.Messages))
		}
		for i := range resp.Messages {
			err := handleMessage(resp.Messages[i], childArgs)
			if err != nil {
				debug("Handler exited with non-zero exit code")
			} else {
				// remove the message
				delopt := &sqs.DeleteMessageInput{
					QueueURL:      queue,
					ReceiptHandle: resp.Messages[i].ReceiptHandle,
				}
				if _, err := svc.DeleteMessage(delopt); err != nil {
					fmt.Fprintln(cli.Stderr, "Failed to delete message")
				} else {
					debug("Delete message with ID: %s", *resp.Messages[i].MessageID)
				}
			}
		}
	}

	return ExitCodeOK
}

// handle incoming messages
func handleMessage(message *sqs.Message, args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// setup environment
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("SQS_BODY=\"%s\"", *message.Body),
		fmt.Sprintf("SQS_MESSAGE_ID=%s", *message.MessageID),
		fmt.Sprintf("SQS_RECEIPT_HANDLE=%s", *message.ReceiptHandle),
	)
	cmd.Env = env

	cmd.Start()
	return cmd.Wait()
}
