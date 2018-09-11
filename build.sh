#!/bin/bash

go build -o bin/plasma main.go

#by default config file under $HOME directory will be used, if not set in command line
cp ./.plasma.yaml $HOME/