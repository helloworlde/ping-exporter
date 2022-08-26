image:
	docker build -t hellowoodes/ping-exporter .

test:
	docker rm -f ping-exporter
	docker run --name ping-exporter -it -p 9001:9001 hellowoodes/ping-exporter

image-test:
	make image
	make test