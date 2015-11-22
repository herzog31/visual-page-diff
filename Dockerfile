FROM ubuntu:latest

MAINTAINER Mark J. Becker <mjb@marb.ec>

RUN apt-get update && apt-get install -y --no-install-recommends \
	docker.io \
	&& rm -rf /var/lib/apt/lists/*

RUN mkdir /go-root
RUN chmod -R 0777 /go-root
WORKDIR /go-root

COPY dist/linux_amd64_visual-page-diff ./visual-page-diff
RUN chmod +x ./visual-page-diff
CMD ["./visual-page-diff"]