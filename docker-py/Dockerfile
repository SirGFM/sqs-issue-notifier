FROM ubuntu:18.04

RUN apt update && \
	apt install -y python3 python3-pip && \
	pip3 install boto3 && \
	pip3 install requests && \
	useradd -r py-worker
