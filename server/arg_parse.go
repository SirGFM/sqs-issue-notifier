package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

type Args struct {
	// IP on which the server will accept connections. Defaults to 0.0.0.0
	IP string
	// Port on which the server will accept connections. Defaults to 8888
	Port int
	// Timeout for the server to check if there are any messages, in milliseconds. Defaults to 1 min (60000 ms)
	TimeoutMS int
	// Directory where the local storage saves messages temporarily. Will
	// be created if it does not exist. Defaults to "/tmp/local-store"!
	LocalStore string
	// URI where a custom AWS simulator (e.g., localstack) may be accessed.
	// Should be left empty to use the AWS.
	Endpoint string
	// URI where the SQS may be accessed.
	Queue string
}

// parseArgs either from the command line or from the supplied JSON file.
//
// If a JSON file is supplied, it's used as the default parameters, which may be overriden by CLI-supplied arguments.
func parseArgs() Args {
	var args Args
	var confFile string
	const defaultIP = "0.0.0.0"
	const defaultPort = 8888
	const defaultTimeoutMS = 60000
	const defaultLocalStore = "/tmp/local-store"
	const defaultWriteSize = 1024
	const defaultIgnoreOrigin = true
	const defaultDebug = true

	flag.StringVar(&args.IP, "IP", defaultIP, "IP on which the server will accept connections")
	flag.IntVar(&args.Port, "Port", defaultPort, "Port on which the server will accept connections")
	flag.IntVar(&args.TimeoutMS, "TimeoutMS", defaultTimeoutMS, "Timeout for the server to check if there are any messages, in milliseconds")
	flag.StringVar(&args.LocalStore, "LocalStore", defaultLocalStore, "Directory where the local storage saves messages temporarily")
	flag.StringVar(&args.Endpoint, "Endpoint", "", "URI where a custom AWS simulator (e.g., localstack) may be accessed.")
	flag.StringVar(&args.Queue, "Queue", "", "URI where the SQS may be accessed")
	flag.StringVar(&confFile, "confFile", "", "JSON file with the configuration options. May be overriden by other CLI arguments")
	flag.Parse()

	if len(confFile) != 0 {
		var jsonArgs Args

		f, err := os.Open(confFile)
		if err != nil {
			log.Fatalf("Couldn't open the configuration file '%+v': %+v", confFile, err)
		}
		defer f.Close()

		dec := json.NewDecoder(f)
		err = dec.Decode(&jsonArgs)
		if err != nil {
			log.Fatalf("Couldn't decode the configuration file '%+v': %+v", confFile, err)
		}

		// Walk over every set argument to override the JSON file
		flag.Visit(func (f *flag.Flag) {
			if f.Name == "confFile" {
				// Skip the file itself
				return
			}

			var tmp interface{}
			tmp = f.Value
			get, ok := tmp.(flag.Getter)
			if !ok {
				log.Fatalf("'%s' doesn't have an associated flag.Getter", f.Name)
			}

			switch f.Name {
			case "IP":
				val, _ := get.Get().(string)
				log.Printf("Overriding JSON's IP (%+v) with CLI's value (%+v)", jsonArgs.IP, val)
				jsonArgs.IP = val
			case "Port":
				val, _ := get.Get().(int)
				log.Printf("Overriding JSON's Port (%+v) with CLI's value (%+v)", jsonArgs.Port, val)
			case "TimeoutMS":
				val, _ := get.Get().(int)
				log.Printf("Overriding JSON's TimeoutMS (%+v) with CLI's value (%+v)", jsonArgs.TimeoutMS, val)
				jsonArgs.TimeoutMS = val
			case "LocalStore":
				val, _ := get.Get().(string)
				log.Printf("Overriding JSON's LocalStore (%+v) with CLI's value (%+v)", jsonArgs.LocalStore, val)
				jsonArgs.LocalStore = val
			case "Endpoint":
				val, _ := get.Get().(string)
				log.Printf("Overriding JSON's Endpoint (%+v) with CLI's value (%+v)", jsonArgs.Endpoint, val)
				jsonArgs.Endpoint = val
			case "Queue":
				val, _ := get.Get().(string)
				log.Printf("Overriding JSON's Queue (%+v) with CLI's value (%+v)", jsonArgs.Queue, val)
				jsonArgs.Queue = val
			}
		})

		args = jsonArgs
	}

	log.Printf("Starting server with options:")
	log.Printf("  - IP: %+v", args.IP)
	log.Printf("  - Port: %+v", args.Port)
	log.Printf("  - TimeoutMS: %+v", args.TimeoutMS)
	log.Printf("  - LocalStore: %+v", args.LocalStore)
	log.Printf("  - Endpoint: %+v", args.Endpoint)
	log.Printf("  - Queue: %+v", args.Queue)

	return args
}
