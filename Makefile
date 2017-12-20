
all:

dockerfile:
	docker build -f Dockerfile -t digdeeply/gohs-service:0.0.1 .

dev:
	docker run --rm -p 19775:8080 -v $(PWD):/go/src/gohs-ladon -ti digdeeply/gohs-service:latest /bin/bash

build:
	docker run --rm -v $(PWD):/go/src/gohs-ladon -ti digdeeply/gohs-service:latest sh -c "cd /go/src/gohs-ladon && go build"

test:
	docker run --rm -v $(PWD):/go/src/gohs-ladon -ti digdeeply/gohs-service:latest sh -c "cd /go/src/gohs-ladon && go test"
