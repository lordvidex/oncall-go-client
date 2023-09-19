NAME:=./bin/oncall-go-client
CONFIG:=./configs/oncall.yaml

build:
	go build -o $(NAME) ./cmd/bootstrap.go

run: build
	$(NAME) -f $(CONFIG)
