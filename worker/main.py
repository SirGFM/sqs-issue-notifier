import argparse
import boto3
import logging
import json
import requests

# For how long, in seconds, messages received should be hidden from other receives.
visibility_timeout = 10

# The default logger
logger = logging.getLogger('worker')

def main():
	parser = argparse.ArgumentParser(description='Worker for dequeuing messages from an AWS SQS and sending them to Slack')
	parser.add_argument('--channels', action='store', help='A JSON object associating each Slack channel to its hook URL')
	parser.add_argument('--aws_endpoint', action='store', help='A custom endpoint where the AWS SQS is accessible (for using localstack).')
	parser.add_argument('--aws_queue', action='store', help='The URL of the AWS SQS to be accessed by this service.')
	parser.add_argument('--timeout', action='store', default=20, help='For how long the service should wait for new messages, in seconds. Must be between 0 and 20.', type=int)
	parser.add_argument('--config', action='store', help='A JSON file with every option. This JSON is overriden by command line arguments.')

	args = parser.parse_args()

	# Load the configuration file and override it with CLI options
	config = {}
	log_args = False
	if 'config' in args and args.config is not None:
		with open(args.config, 'rb') as f:
			config = json.load(f)
		log_args = True
	if 'channels' in args and args.channels is not None:
		if log_args:
			logger.info('Overriding JSON\'s channels with CLI\'s value')
		channels = json.loads(args.channels)
		config['channels'] = channels
	if 'aws_endpoint' in args and args.aws_endpoint is not None:
		if log_args:
			logger.info('Overriding JSON\'s aws_endpoint with CLI\'s value')
		config['aws_endpoint'] = args.aws_endpoint
	if 'aws_queue' in args and args.aws_queue is not None:
		if log_args:
			logger.info('Overriding JSON\'s aws_queue with CLI\'s value')
		config['aws_queue'] = args.aws_queue
	if 'timeout' in args and args.timeout is not None:
		if log_args:
			logger.info('Overriding JSON\'s timeout with CLI\'s value')
		config['timeout'] = args.timeout

	# Check that every required parameter was somehow supplied.
	if not 'channels' in config:
		raise Exception('"channels" wasn\'t specified neither on the config file nor on the CLI!')
	if not 'aws_queue' in config:
		raise Exception('"aws_queue" wasn\'t specified neither on the config file nor on the CLI!')

	logger.info('Starting worker with options:')
	logger.info('  - Channels:')
	for k in config['channels']:
		logger.info('    - {}'.format(k))
	if 'aws_endpoint' in config:
		logger.info('  - AWS Endpoint: "{}"'.format(config['aws_endpoint']))
	else:
		logger.info('  - AWS Endpoint: default')
	logger.info('  - AWS Queue: "{}"'.format(config['aws_queue']))

	# Initialize the SQS client using either the default or the specified endpoint.
	endpoint = None
	if 'aws_endpoint' in config:
		endpoint = config['aws_endpoint']
	sqs = boto3.client('sqs', endpoint_url=endpoint)

	run(sqs, config['timeout'], config['aws_queue'], config['channels'])

def run(sqs, timeout, queue_url, channels_dict):
	"""
	run try to read messages from the SQS and forward them to the Slack
	channel specified in the message.
	"""
	if timeout < 0:
		logger.warning('Timeout ({}) is too small! Defaulting to 0...'.format(timeout))
		timeout = 0
	if timeout > 20:
		logger.warning('Timeout ({}) is too big! Defaulting to 20...'.format(timeout))
		timeout = 20

	while True:
		try:
			response = sqs.receive_message(
				QueueUrl=queue_url,
				MaxNumberOfMessages=10,
				MessageAttributeNames=['All'],
				VisibilityTimeout=visibility_timeout,
				WaitTimeSeconds=timeout
			)
		except Exception as e:
			logger.error('Couldn\'t receive any message: {}'.format(e))
			continue

		if not 'Messages' in response or response['Messages'] is None:
			# No messages queued
			continue

		for message in response['Messages']:
			channel = ''
			msg = ''

			try:
				data = json.loads(message['Body'])
				channel = data['Channel']
				msg = data['Message']
			except Exception as e:
				logger.error('Couldn\'t decode the received message: {} (contents: {})'.format(e, message['Body']))
				continue

			if send_slack(channel, msg, channels_dict):
				receipt_handle = message['ReceiptHandle']
				sqs.delete_message(
					QueueUrl=queue_url,
					ReceiptHandle=receipt_handle
				)

def send_slack(channel, msg, channel_dict):
	"""
	send the given message to the specified channel. On error (e.g., if the
	channel isn't on the supplied list), the issue is logged but no extra
	action is taken!

	Return True if the message was sent successfully, False otherwise.
	"""

	if not channel in channel_dict:
		logger.error('Channel "{}" isn\'t on the list of channels!'.format(channel))
		logger.warning('Dropping message "{}" for channel "{}"...'.format(msg, channel))
		return False
	url = channel_dict[channel]

	try:
		r = requests.post(url, json={'text': msg})
	except Exception as e:
		logger.error('Failed to send message "{}" to channel "{}": {}'.format(msg, channel, e))
		return False

	if r.status_code != requests.codes.ok:
		logger.error('Failed to send message "{}" to channel "{}": {}'.format(msg, channel, r.text))
		return False
	return True

if __name__ == '__main__':
	main()
