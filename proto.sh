#!/bin/bash

protoc --go_out=paths=source_relative:. pb/config/config.v1.proto 
