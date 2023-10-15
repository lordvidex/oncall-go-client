## Content

<!-- vim-markdown-toc GFM -->

* [oncall-go-client](#oncall-go-client)
    * [Requirements](#requirements)
    * [sample yaml configuration](#sample-yaml-configuration)
    * [How to Run?](#how-to-run)
* [oncall-roster-exporter](#oncall-roster-exporter)
    * [How to Run?](#how-to-run-1)
    * [Usage](#usage)

<!-- vim-markdown-toc -->

## oncall-go-client

A simple oncall client in golang that bootstraps application with users, teams and their schedules

This app contains detailed logs of operations executed

### Requirements

- oncall-server: oncall server must be running before this client script can work. A clone can be gotten from https://github.com/linkedin/oncall.git

### sample yaml configuration

See [sample](./configs/oncall.yaml)

### How to Run?

`make build`: compiles the app and builds the binary file `/bin/oncall-go-client` \
`make run`: runs the binary file.

## oncall-roster-exporter

This is a custom exporter that exposes metrics related to teams and their current members on-duty

### How to Run?

`make exporter`: compiles and build the binary file `./bin/oncall-roster-exporter`  
`make deploy`: builds exporter docker image to docker hub. If user is not specified, it will try to push to my dockerhub @lordvidex 👀

### Usage

Run `oncall-roster-exporter -h` anytime to view usage

> NOTE: if you don't want logs, add the -silent flag
