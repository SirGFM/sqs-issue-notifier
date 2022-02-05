package local_storage

import (
	"bytes"
	"testing"
	"time"
	"os"
)

// TestLocalFS tests the basic behaviour for a local storage.
func TestLocalFS(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "local-fs*")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %+v", err)
	}
	defer os.RemoveAll(dir)

	store := NewFS(dir, time.Millisecond)

	// Check that the local storage properly times out when empty.
	err = store.Wait()
	if want, got := ErrTimedOut, err; want != got {
		t.Errorf("Wait: Expected error '%+v' but got '%+v'", want, got)
	}

	// Check that messages are properly sent, and that duplicated messages
	// are marked as such.
	msg := []byte("The quick brown fox jumps over the lazy old dog")
	err = store.Store(msg)
	if err != nil {
		t.Errorf("Store: Failed to store the message '%s': %+v", msg, err)
	}

	err = store.Store(msg)
	if want, got := ErrDuplicatedStore, err; want != got {
		t.Errorf("Store: Expected error '%+v' but got '%+v'", want, got)
	}

	num := store.Count()
	if want, got := 1, num; want != got {
		t.Errorf("Count: Expected '%+d' messages but got '%+d'", want, got)
	}

	// Check that the local storage is properly notified about events.
	err = store.Wait()
	if err != nil {
		t.Errorf("Wait: Failed to get notified about message: %+v", err)
	}

	// Check that retrieving a message does not remove it from the local
	// storage.
	data, err := store.Get()
	if err != nil {
		t.Errorf("Get: Failed to retrieve the message '%s': %+v", msg, err)
	} else if bytes.Compare(msg, data.Bytes()) != 0 {
		t.Errorf("Get: Message does not match! Want '%s' but got '%s'",
				string(msg), string(data.Bytes()))
	}

	num = store.Count()
	if want, got := 1, num; want != got {
		t.Errorf("Count: Expected '%+d' messages but got '%+d'", want, got)
	}

	// Also ensure that a message cannot be received until it was processed
	// and closed.
	_, err = store.Get()
	if want, got := ErrGetEmpty, err; want != got {
		t.Errorf("Get: Expected error '%+v' but got '%+v'", want, got)
	}
	data.Close()

	repData, err := store.Get()
	if err != nil {
		t.Errorf("Get: Failed to retrieve the message a second time '%s': %+v", msg, err)
	} else if bytes.Compare(msg, repData.Bytes()) != 0 {
		t.Errorf("Get: Repeated message does not match! Want '%s' but got '%s'",
				string(msg), string(repData.Bytes()))
	}

	// Remove the message, checking that both Get and Wait properly signals
	// that the local storage is now empty.
	err = repData.Remove()
	if err != nil {
		t.Errorf("Remove: Failed to remove the message '%s': %+v", msg, err)
	}

	num = store.Count()
	if want, got := 0, num; want != got {
		t.Errorf("Count: Expected '%+d' messages but got '%+d'", want, got)
	}

	_, err = store.Get()
	if want, got := ErrGetEmpty, err; want != got {
		t.Errorf("Get: Expected error '%+v' but got '%+v'", want, got)
	}

	err = store.Wait()
	if want, got := ErrTimedOut, err; want != got {
		t.Errorf("Wait: Expected error '%+v' but got '%+v'", want, got)
	}

	// Check that close properly signals Wait to stop.
	store.Close()
	err = store.Wait()
	if want, got := ErrStoreClosed, err; want != got {
		t.Errorf("Wait: Expected error '%+v' but got '%+v'", want, got)
	}
}

// TestConsumerRoutine starts a goroutine for consuming messages as they
// are stored in the local storage.
func TestConsumerRoutine(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "local-consumer-fs*")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %+v", err)
	}
	defer os.RemoveAll(dir)

	test_cases := []struct{ msg []byte; waitErr error } {
		{ msg: []byte("Twas brillig, and the slithy toves") },
		{ waitErr: ErrTimedOut },
		{ msg: []byte("Did gyre and gimble in the wabe;") },
		{ msg: []byte("All mimsy were the borogoves,") },
		{ msg: []byte("And the mome raths outgrabe.") },
		{ waitErr: ErrStoreClosed, },
	}

	done := make(chan struct{}, 1)

	timeout := time.Millisecond
	store := NewFS(dir, timeout)

	// Start a consumer goroutine.
	go func() {
		for i, tc := range test_cases {
			err := store.Wait()
			if want, got := tc.waitErr, err; want != got {
				t.Errorf("(bg-%d) Wait: Expected error '%+v' but got '%+v'", i, want, got)
			}

			if tc.msg != nil {
				data, err := store.Get()
				if err != nil {
					t.Errorf("(bg-%d) Get: Failed to retrieve the message '%s': %+v", i, tc.msg, err)
					continue
				} else if bytes.Compare(tc.msg, data.Bytes()) != 0 {
					t.Errorf("(bg-%d) Get: Message does not match! Want '%s' but got '%s'",
							i, string(tc.msg), string(data.Bytes()))
				}

				err = data.Remove()
				if err != nil {
					t.Errorf("(bg-%d) Remove: Failed to remove the message '%s': %+v", i, tc.msg, err)
				}
			}
		}

		done <- struct{}{}
	} ()

	// Send messages to the goroutine.
	for i, tc := range test_cases {
		if tc.waitErr == ErrTimedOut {
			time.Sleep(timeout + timeout / 2)
		} else if tc.msg != nil {
			err = store.Store(tc.msg)
			if err != nil {
				t.Errorf("(fg-%d) Store: Failed to store the message '%s': %+v",
						i, tc.msg, err)
			}
		}
	}

	// Wait for a while to ensure that everything was processed.
	time.Sleep(timeout / 8)
	store.Close()

	select {
	case <-time.After(timeout):
		t.Errorf("Main thread timed out waiting for BG thread to exit")
	case <-done:
		// Consumer finished successfully
	}
}

// TestPrepopulated populates a local storage then creates another one and
// check that every message is correctly received, simulating a failure to
// send messages and/or a service crash.
func TestPrepopulated(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "local-prepopulated-fs*")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %+v", err)
	}
	defer os.RemoveAll(dir)

	test_cases := [][]byte{
		[]byte("Beware the Jabberwock, my son!"),
		[]byte("The jaws that bite, the claws that catch!"),
		[]byte("Beware the Jubjub bird, and shun"),
		[]byte("The frumious Bandersnatch!"),
	}

	// Pre-populate a local storage
	store := NewFS(dir, time.Millisecond)
	for i, msg := range test_cases {
		err = store.Store(msg)
		if err != nil {
			t.Errorf("%d: Store: Failed to store the message '%s': %+v",
					i, msg, err)
		}
	}
	store.Close()

	// Create a new store in the same location and check that every message
	// is received.
	recv_msg := make([]bool, len(test_cases))
	recv := NewFS(dir, time.Millisecond)

	num := recv.Count()
	if want, got := len(test_cases), num; want != got {
		t.Errorf("Count: Expected '%+d' messages but got '%+d'", want, got)
	}

	for i := range test_cases {
		err := recv.Wait()
		if err != nil {
			t.Errorf("%d: Wait: Got error %+v", i, err)
		}

		data, err := recv.Get()
		if err != nil {
			t.Errorf("%d: Get: Failed to retrieve a message: %+v", i, err)
			continue
		}

		for j, msg := range test_cases {
			if bytes.Compare(msg, data.Bytes()) == 0 {
				recv_msg[j] = true
			}
		}

		err = data.Remove()
		if err != nil {
			t.Errorf("(bg-%d) Remove: Failed to remove the message '%s': %+v",
					i, string(data.Bytes()), err)
		}
	}

	// Ensure the local storage is empty.
	num = recv.Count()
	if want, got := 0, num; want != got {
		t.Errorf("Count: Expected '%+d' messages but got '%+d'", want, got)
	}

	err = recv.Wait()
	if want, got := ErrTimedOut, err; want != got {
		t.Errorf("Wait: Expected error '%+v' but got '%+v'", want, got)
	}
	recv.Close()

	// Check that every message was properly received.
	for i, msg := range test_cases {
		if !recv_msg[i] {
			t.Errorf("%d: Did not receive mesage '%s'", i, string(msg))
		}
	}
}
