BINARY=engine
test: 
	go test -v -cover -covermode=atomic ./...

engine:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${BINARY} cmd/api/main.go

unittest:
	go test -short  ./...

test-coverage:
	go test -short -coverprofile cover.out -covermode=atomic ./...
	cat cover.out >> coverage.txt

clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi

docker:
	docker build -t shortener .

run:
	docker-compose up -d

stop:
	docker-compose down

lint-prepare:
	@echo "Installing golangci-lint" 
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s latest

lint:
	./bin/golangci-lint run ./...

generate-mocks:
	mockgen -source=./domain/url.go -destination=./url/mock/mock.go -package=mock
	mockgen -source=./domain/user.go -destination=./user/mock/mock.go -package=mock

authkey:
	go run ./cmd/admin/main.go keygen ./private.pem

migrate:
	go run ./cmd/admin/main.go migrate_mongo

seed: migrate
	go run ./cmd/admin/main.go seed

rebuild:
	docker compose stop backend
	docker-compose up --build --force-recreate --no-deps -d backend

.PHONY: test engine unittest test-coverage clean docker run stop lint-prepare lint generate-mocks authkey migrate seed rebuild