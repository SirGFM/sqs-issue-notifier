package sender

import (
	"encoding/json"
	"testing"
	"os"
)

// TestSQSSend tests sending a simple message to a queue using the
// configuration specified in local variables.
func TestSQSSend(t *testing.T) {
	endpoint := os.Getenv("SQS_ENDPOINT")
	queue := os.Getenv("SQS_QUEUE")

	if len(queue) == 0 {
		t.Fatal("No queue was specified! Set the queue's address in the environment variable SQS_QUEUE. Optionally, set the endpoint in SQS_ENDPOINT.")
	}

	s := NewSQSSender(endpoint, queue)
	err := s.Send("this is a test")
	if err != nil {
		t.Errorf("Send: Failed to send a test message: %+v", err)
	}

	dummy_struct := struct {
		Str string
		Int int
	} {
		Str: "Test string",
		Int: 1,
	}
	data, err := json.Marshal(&dummy_struct)
	if err != nil {
		t.Fatalf("Failed to encode the struct as a JSON: %+v", err)
	}

	err = s.Send(string(data))
	if err != nil {
		t.Errorf("Send: Failed to send a test struct: %+v", err)
	}
}
