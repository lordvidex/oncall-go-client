NAME:=./bin/oncall-go-client
EXPORTER-NAME:=./bin/oncall-roster-exporter
PROBER-NAME:=./bin/oncall-sla-prober
CHECKER-NAME:=./bin/oncall-sla-checker
CONFIG:=./configs/oncall.yaml
USER:=lordvidex

build:
	go build -o $(NAME) ./cmd/bootstrap/main.go

export-all:
	GOOS=linux GOARCH=amd64 go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go
	GOOS=linux GOARCH=amd64 go build -o $(PROBER-NAME) ./cmd/sla-prober/main.go
	GOOS=linux GOARCH=amd64 go build -o $(CHECKER-NAME) ./cmd/sla-checker/main.go
	docker build --no-cache -f ./deployments/roster-exporter/Dockerfile -t $(USER)/oncall-roster-exporter:latest .
	docker build --no-cache -f ./deployments/sla-prober/Dockerfile -t $(USER)/oncall-sla-prober:latest .
	docker build --no-cache -f ./deployments/sla-checker/Dockerfile -t $(USER)/oncall-sla-checker:latest .

deploy: export-all
	docker push $(USER)/oncall-roster-exporter:latest
	docker push $(USER)/oncall-sla-prober:latest
	docker push $(USER)/oncall-sla-checker:latest

build-exporter:
	go build -o $(EXPORTER-NAME) ./cmd/roster-exporter/main.go

build-sla-prober:
	go build -o $(PROBER-NAME) ./cmd/sla-prober/main.go

run: build
	$(NAME) -f $(CONFIG)

exporter: build-exporter
	$(EXPORTER-NAME)

prober: build-sla-prober
	$(PROBER-NAME)
