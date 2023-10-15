NAME:=./bin/oncall-go-client
EXPORTER-NAME:=./bin/oncall-roster-exporter
CONFIG:=./configs/oncall.yaml
USER:=lordvidex

build:
	go build -o $(NAME) ./cmd/bootstrap/main.go

export:
	GOOS=linux GOARCH=amd64 go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go
	docker build --no-cache -t $(USER)/oncall-roster-exporter:latest .

deploy: export
	docker push $(USER)/oncall-roster-exporter:latest

build-exporter:
	go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go

run: build
	$(NAME) -f $(CONFIG)

exporter: build-exporter
	$(EXPORTER-NAME)
