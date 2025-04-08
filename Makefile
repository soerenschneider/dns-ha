BUILD_DIR = builds
MODULE = github.com/soerenschneider/dns-ha
BINARY_NAME = dns-ha
CHECKSUM_FILE = $(BUILD_DIR)/checksum.sha256
SIGNATURE_KEYFILE = ~/.signify/github.sec
DOCKER_PREFIX = ghcr.io/soerenschneider

generate:
	go generate  ./...

tests:
	go test ./... -race -covermode=atomic -coverprofile=coverage.out
	go tool cover -html=coverage.out -o=coverage.html
	go tool cover -func=coverage.out -o=coverage.out

clean:
	git diff --quiet || { echo 'Dirty work tree' ; false; }
	rm -rf ./$(BUILD_DIR)

build: version-info generate
	CGO_ENABLED=0 go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BINARY_NAME) ./cmd

release: clean version-info cross-build
	sha256sum $(BUILD_DIR)/dns-ha-* > $(CHECKSUM_FILE)

signed-release: release
	pass keys/signify/github | signify -S -s $(SIGNATURE_KEYFILE) -m $(CHECKSUM_FILE)
	gh-upload-assets -o soerenschneider -r dns-ha -f ~/.gh-token builds

cross-build: build
	GOOS=linux GOARCH=amd64       CGO_ENABLED=0 go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64    ./cmd
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv6    ./cmd
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7    ./cmd
	GOOS=linux GOARCH=arm64       CGO_ENABLED=0 go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-aarch64  ./cmd

docker-build:
	docker build -t "$(DOCKER_PREFIX)/dns-ha-server" .

version-info:
	$(eval VERSION := $(shell git describe --tags --abbrev=0 || echo "dev"))
	$(eval COMMIT_HASH := $(shell git rev-parse HEAD))

fmt:
	find . -iname "*.go" -exec go fmt {} \; 

pre-commit-init:
	pre-commit install
	pre-commit install --hook-type commit-msg

pre-commit-update:
	pre-commit autoupdate

docs:
	rm -rf go-diagrams
	go run doc/main.go
	cd go-diagrams && dot -Tpng diagram.dot > ../overview.png

tofu:
	tofu -chdir=contrib/terraform destroy -auto-approve; tofu -chdir=contrib/terraform apply -auto-approve
