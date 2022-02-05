FROM ubuntu:18.04

ENV PATH=${PATH}:/opt/go/ws/bin:/opt/go/go1.17.6/bin
ENV GOROOT=/opt/go/go1.17.6
ENV GOPATH=/opt/go/ws
ENV GOBIN=/opt/go/ws/bin

RUN apt update && \
	apt install -y curl && \
	mkdir -p /opt/go && \
	curl -L https://go.dev/dl/go1.17.6.linux-amd64.tar.gz -o - | tar -zx -C /opt/go && \
	mv /opt/go/go /opt/go/go1.17.6 && \
	go get -u github.com/aws/aws-sdk-go/... && \
	go get github.com/theckman/go-flock
