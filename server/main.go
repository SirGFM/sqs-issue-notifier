package main

import (
	"github.com/SirGFM/sqs-issue-notifier/server/local_storage"
	"github.com/SirGFM/sqs-issue-notifier/server/sender"
	"log"
	"os"
	"os/signal"
	"time"
)

// startStorage and launch a goroutine to forward requests to a SQS.
func startStorage(args Args) local_storage.Store {
	timeout := time.Duration(args.TimeoutMS) * time.Millisecond

	store := local_storage.NewFS(args.LocalStore, timeout)
	sqs := sender.NewSQSSender(args.Endpoint, args.Queue)

	go func() {
		for {
			err := store.Wait()
			if err == local_storage.ErrStoreClosed {
				return
			} else if err != nil && err != local_storage.ErrTimedOut {
				log.Printf("local_store.Wait failed with: %+v\n", err)
				continue
			}

			data, err := store.Get()
			if err == local_storage.ErrGetEmpty {
				continue
			} else if err != nil {
				log.Printf("local_store.Get failed with: %+v\n", err)
				continue
			}

			err = sqs.Send(string(data.Bytes()))
			if err != nil {
				log.Printf("sender.Send failed with: %+v\n", err)
				// Release this data so it may be retrieved again at a
				// later time.
				data.Close()
				continue
			}

			err = data.Remove()
			if err != nil {
				log.Printf("local_store.Remove failed with: %+v\n", err)
				// Release the data, although it's already been sent.
				data.Close()
			}
		}
	} ()

	return store
}

// startServer and configure its signal handler.
func startServer() {
	args := parseArgs()

	store := startStorage(args)

	intHndlr := make(chan os.Signal, 1)
	signal.Notify(intHndlr, os.Interrupt)

	closer := RunWeb(args, store)

	<-intHndlr
	log.Printf("Exiting...")
	closer.Close()
	store.Close()
}

func main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)

	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("Application panicked! %+v", r)
		}
	} ()

	startServer()
}
