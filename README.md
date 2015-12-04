[![Build Status](https://travis-ci.org/herzog31/visual-page-diff.svg?branch=master)](https://travis-ci.org/herzog31/visual-page-diff)
[![Docker Hub](https://img.shields.io/docker/pulls/herzog31/visual-page-diff.svg)](https://hub.docker.com/r/herzog31/visual-page-diff)

# visual-page-diff
Visual Page Diff tool, written in Go

# Usage
Create a `docker-compose.yml` file like the following and start the tool as a container using the `docker-compose up` command:

```YAML
visual-page-diff:
  image: herzog31/visual-page-diff
  environment:
    - PAGES=http://marb.ec,http://github.com
    - INTERVAL=30
    - THRESHOLD=0.01
    - SCALE=1
    - FUZZ=5
    - WIDTH=1024
    - HEIGHT=768
    - SMTP_USER=username
    - SMTP_PASSWORD=password
    - SMTP_HOST=host
    - SMTP_FROM=from@example.org
    - SMTP_TO=to@example.org
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - /output:/output
  restart: always
```

