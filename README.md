go-sqs-do
=========

Pass SQS messages to a handler script.

[![GitHub Release](http://img.shields.io/github/release/fullscreen/go-sqs-do.svg)](https://github.com/fullscreen/go-sqs-do/releases)

usage
=====
```shell
sqs-do -q <QUEUE_URL> -- ./handler.sh
```

The handler script will be executed with the following environment variables:

```shell
SQS_BODY
SQS_MESSAGE_ID
SQS_RECEIPT_HANDLE
```

If the handler script exits with a zero status code, sqs-do will handle removing
the message from the queue. If the exit code is not zero, the message re-enters
the queue after the VisiblityTimeout.

install
=======
```shell
go get github.com/fullscreen/go-sqs-do
```

Make sure your `PATH` includes your `GOPATH` bin directory:

```shell
export PATH=$PATH:$GOPATH/bin
```
