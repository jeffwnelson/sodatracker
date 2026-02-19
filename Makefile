.PHONY: build build-server build-piagent build-lambda test lint run clean docker-up docker-down \
	web-install web-dev web-build deploy-frontend tf-plan tf-apply swagger

BINARY_DIR := bin
SERVER_BIN := $(BINARY_DIR)/server
PIAGENT_BIN := $(BINARY_DIR)/piagent
PIAGENT_ARM_BIN := $(BINARY_DIR)/piagent-arm
LAMBDA_BIN := $(BINARY_DIR)/bootstrap
LAMBDA_ZIP := $(BINARY_DIR)/lambda.zip

build: build-server build-piagent

build-server:
	go build -o $(SERVER_BIN) ./cmd/server

build-piagent:
	go build -o $(PIAGENT_BIN) ./cmd/piagent

build-piagent-arm:
	GOOS=linux GOARCH=arm GOARM=7 go build -tags rpi -o $(PIAGENT_ARM_BIN) ./cmd/piagent

build-lambda:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(LAMBDA_BIN) ./cmd/lambda
	cd $(BINARY_DIR) && zip lambda.zip bootstrap

test:
	go test ./...

lint:
	golangci-lint run ./...

run: build-server
	./$(SERVER_BIN)

clean:
	rm -rf $(BINARY_DIR)

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

deploy-frontend: web-build
	aws s3 sync web/dist/ s3://$$(cd infra && tofu output -raw frontend_bucket)/ --delete --profile personal
	aws cloudfront create-invalidation \
		--distribution-id $$(cd infra && tofu output -raw cloudfront_domain | xargs -I{} aws cloudfront list-distributions --query "DistributionList.Items[?DomainName=='{}'].Id" --output text --profile personal) \
		--paths "/*" --profile personal

swagger:
	swag init --generalInfo internal/api/router.go --dir ./ --output internal/api/docs --parseDependency --parseInternal

tf-plan:
	cd infra && tofu plan

tf-apply:
	cd infra && tofu apply
