# Build everything inside the first container
FROM sqs_issue_notifier_server_builder:latest AS builder
ADD . /opt/go/ws/src/github.com/SirGFM/sqs-issue-notifier/server/
WORKDIR /opt/go/ws/src/github.com/SirGFM/sqs-issue-notifier/server
RUN go install .

# Create a separated container
FROM ubuntu:18.04 AS prod
RUN groupadd -g 999 host-group && \
	useradd -r -m -g 999 go-server && \
	mkdir /opt/server
COPY --from=builder /opt/go/ws/bin/server /opt/server/
