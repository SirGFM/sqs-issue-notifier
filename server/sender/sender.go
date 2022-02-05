/*
Package sender implements an interface for sending messages.

Currently, it only implements a sender to a AWS SQS, which requires the AWS
credentials to be defined as specified in AWS's SDK (so either in a local
file or through the environment variables AWS_ACCESS_KEY_ID,
AWS_SECRET_ACCESS_KEY and AWS_DEFAULT_REGION).

To send messages to a SQS, create a new sender by calling "NewSQSSender()",
then call "Send()" for each message. To send messages to a localstack
service, be sure to specify it's URL in the endpoint, as it will otherwise
fail!

Example (localstack):

	// Create a sender for "http://localhost:4566/000000000000/test-queue"
	s := sender.NewSQSSender("http://localhost:4566",
			"http://localhost:4566/000000000000/test-queue")

	// Send a simple message
	err := s.Send("hello")
	if err != nil {
		// handle err
	}
*/
package sender

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"log"
)

// Sender interface for sending messages to a receiver.
type Sender interface {
	// Send the given msg.
	Send(msg string) error
}

// sqsSender implements Sender for a AWS SQS.
type sqsSender struct {
	// The AWS session for sending requests.
	awsSession *session.Session

	// The queue's URL for sending messages (without the URL).
	queue string
}

func (s sqsSender) Send(msg string) error {
	svc := sqs.New(s.awsSession)

	input := &sqs.SendMessageInput{
		MessageBody: aws.String(msg),
		QueueUrl: aws.String(s.queue),
	}
	if err := input.Validate(); err != nil {
		log.Printf("sender/Send: Invalid input: %+v\n", err)
		return ErrInvalidInput
	}

	_, err := svc.SendMessage(input)
	if err != nil {
		log.Printf("sender/Send: Failed to send the message '%s': %+v\n", msg, err)
		return ErrSendFailed
	}

	return nil
}

// Create a new sender ready to send requests to a SQS service. To simplify
// simulating a AWS on localstack, endpoint may be supplied to define a
// custom SQS handler. Passing endpoint as the empty string will default to
// using the actual AWS. The queue URI must be specified as its full path,
// regardless of whether or not an endpoint was specified.
func NewSQSSender(endpoint, queue string) Sender {
	config := aws.Config{}
	if len(endpoint) > 0 {
		config.Endpoint = aws.String(endpoint)
	}

	awsSession := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: config,
	}))

	return sqsSender {
		awsSession: awsSession,
		queue: queue,
	}
}
