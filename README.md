# SQS issue system

A simple system for receiving issues over an HTTP API and forwarding them to a Slack channel, using an AWS SQS as an intermediary for the HTTP server and the Slack bot.

## Requirements

* Docker: https://docs.docker.com/engine/install/
* Docker-compose: https://docs.docker.com/compose/install/

## Quick start

```bash
# Start the localstack
docker-compose up -d localstack
# Create a SQS named 'issues-queue'
docker exec localstack_0_13_3 awslocal sqs create-queue --queue-name issues-queue
```
