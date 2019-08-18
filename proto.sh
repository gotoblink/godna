#!/bin/bash

protoc --go_out=paths=source_relative:pb -I ../dna/store/projects/dna config/config.v1.proto 
