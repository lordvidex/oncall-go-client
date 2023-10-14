NAME:=./bin/oncall-go-client
EXPORTER-NAME:=./bin/oncall-roster-exporter
CONFIG:=./configs/oncall.yaml

build:
	go build -o $(NAME) ./cmd/bootstrap/main.go

build-exporter:
	go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go

run: build
	$(NAME) -f $(CONFIG)

exporter: build-exporter
	$(EXPORTER-NAME)
