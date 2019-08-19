#!/bin/bash

mkdir -p pb/extensions/store
WD=`pwd`
(
    cd ../dna/store/extensions/0000STORE
    protoc --go_out=paths=source_relative:$WD/pb/extensions/store validation.proto 
)
(
    cd ../dna/store/projects
    protoc --go_out=paths=source_relative:$WD/pb -I $WD/.. -I. dna/config/config.v1.proto 
    protoc --go_out=paths=source_relative:$WD/pb -I $WD/.. -I. dna/source/source.v1.proto 
)
