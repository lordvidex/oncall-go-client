# oncall-go-client
A simple oncall client in golang that bootstraps application with users, teams and their schedules

This app contains detailed logs of operations executed

## Requirements
- oncall-server: oncall server must be running before this client script can work. A clone can be gotten from https://github.com/linkedin/oncall.git

## sample yaml configuration
See [sample](./configs/oncall.yaml)

## How to Run?
`make build`: compiles the app and builds the binary file  `/bin/oncall-go-client`
`make run`: runs the binary file.
