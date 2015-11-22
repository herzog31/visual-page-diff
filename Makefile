build:
	go get github.com/mitchellh/gox
	gox -osarch="linux/amd64" -output "dist/{{.OS}}_{{.Arch}}_{{.Dir}}"
	docker-compose build