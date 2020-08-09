#!/usr/bin/env bash

protoc --experimental_allow_proto3_optional --proto_path=./pb --go_out=plugins=grpc:./ddt ./pb/*.proto