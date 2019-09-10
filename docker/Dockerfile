# Build
FROM golang:1.13rc2 as builder
# protoc-gen-go version to clone and build
ENV GOLANG_PROTOBUF_VERSION v1.2.0
# protoc-gen-go version to clone and build
ENV GRPC_GATEWAY_VERSION v1.11.0
#
# Install unzip, required to unzip the protoc zip
RUN apt-get update && \
    apt-get upgrade -y -o Dpkg::Options::="--force-confold" && \
    apt-get install -y unzip && \
    apt-get clean && apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
#
# Protoc version to install
ENV PROTOC_VERSION 3.9.0
#
# Download and install protoc
RUN wget https://github.com/google/protobuf/releases/download/v$PROTOC_VERSION/protoc-$PROTOC_VERSION-linux-x86_64.zip && \
   unzip protoc-$PROTOC_VERSION-linux-x86_64.zip && \
   rm -f protoc-$PROTOC_VERSION-linux-x86_64.zip
# Import protobuf from source because go get doesn't support getting by version
# and build the protoc-gen-go binary for generating protobuf v2
RUN mkdir -p /go/src/github.com/golang && \
   cd /go/src/github.com/golang && \
   git clone https://github.com/golang/protobuf.git && \
   cd protobuf && \
   git checkout $GOLANG_PROTOBUF_VERSION && \
   cd protoc-gen-go && \
   go install
#
# Godna version to clone and build
ENV GODNA_VER v1.12.0
#
# godna; clone and build
RUN git clone https://github.com/wxio/godna /godna && \
    cd /godna && \
    git checkout $GODNA_VER && \
    go install -ldflags  "-X main.version=$GODNA_VER -X main.commit=$(git log --pretty=format:\"%H\" -1) -X main.date=$(git log --pretty=format:\"%ad\" -1 --date=format:'%F-%T%z')"
# grpc-gateway & swagger; clone and build
RUN git clone https://github.com/grpc-ecosystem/grpc-gateway.git && \
    cd grpc-gateway && \
    git checkout $GRPC_GATEWAY_VERSION && \
    cd protoc-gen-grpc-gateway && \
    go install && \
    cd ../protoc-gen-swagger && \
    go install
# micro
RUN go get github.com/micro/protoc-gen-micro

# Package
FROM golang:1.13rc2
#
COPY --from=builder /go/bin/* /go/bin/
COPY --from=builder /go/include /go/include
# GOPRIVATE="bitbucket.org"
ADD entrypoint.sh /entrypoint.sh
ENTRYPOINT [ "/entrypoint.sh" ]