# SQS issue system

A simple system for receiving issues over an HTTP API and forwarding them to a Slack channel, using an AWS SQS as an intermediary for the HTTP server and the Slack bot.

## Requirements

* Docker: https://docs.docker.com/engine/install/
* Docker-compose: https://docs.docker.com/compose/install/
* Go 1.17.6+: https://go.dev/dl/
* AWK SDK for Go: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html

## Quick start

Before launching everything, adjust the worker's configuration file (`worker/config.json`) to your needs!

```bash
docker-compose build
docker-compose up -d localstack
docker exec localstack_0_13_3 awslocal sqs create-queue --queue-name issues-queue
docker-compose up -d worker
docker-compose up -d server
curl -H 'Accept: application/json' --data '{"channel": "general", "message": ".done"}' http://localhost:8888/message
```

## Manual compilation

Start by building every container:

```bash
docker-compose build
```

### Localstack (SQS)

```bash
# Start the localstack
docker-compose up -d localstack
# Create a SQS named 'issues-queue'
docker exec localstack_0_13_3 awslocal sqs create-queue --queue-name issues-queue
```

### AWS configuration

To be able to use AWS stuff, configure the following environment variables:

```bash
# These keys may be anything for localstack
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
# The region MUST be "us-east-1"!
export AWS_DEFAULT_REGION="us-east-1"
```

These are used by both applications!

### Slack configuration

1. Create a workspace
1. Click on the workspace's name (top left) and select 'Settings & administration'
1. Click on Menu (top left) and select 'Configure apps'
1. Click on Build (on the top right corner)
1. Enable and add a Webhooks to the Workspace for each channel that may receive messages, in 'Incoming Webhooks'

### Python worker

Before launching the worker, be sure to adjust the configuration file (`worker/config.json`) to your needs!

```bash
docker-compose up -d worker
```

### Go server

The configuration file for the server is in `server-data`. It should work by default (as long as a queue named `issues-queue` was created).

```bash
docker-compose up -d server
```

### Compiling the Go server for testing

For testing purposes, it's easier to compile the server manually. In this case, use `server_builder` directly:

```bash
docker-compose build server_builder
docker-compose run server_builder go install .
```

The binary will be compiled to a `bin` directory, which will be created if it does not exist.
