NAME:=./bin/oncall-go-client
EXPORTER-NAME:=./bin/oncall-roster-exporter
CONFIG:=./configs/oncall.yaml

build:
	go build -o $(NAME) ./cmd/bootstrap/main.go

export:
	GOOS=linux GOARCH=amd64 go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go
	docker build --no-cache -t lordvidex/oncall-roster-exporter:latest .

deploy: export
	docker push lordvidex/oncall-roster-exporter:latest

build-exporter:
	go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go

run: build
	$(NAME) -f $(CONFIG)

exporter: build-exporter
	$(EXPORTER-NAME)
