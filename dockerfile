FROM golang:alpine

Env PROTOC_VERSION=3.12.3
env PROTOC_ZIP=protoc-$PROTOC_VERSION-linux-x86_64.zip

RUN apk add --no-cache git curl unzip protoc

RUN go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
    && go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger \
    && go get github.com/golang/protobuf/protoc-gen-go \
    && go install \
    github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
    github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger \
    github.com/golang/protobuf/protoc-gen-go

RUN curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v$PROTOC_VERSION/$PROTOC_ZIP \
    && unzip -o $PROTOC_ZIP -d /usr/local "include/*" \
    && rm -f $PROTOC_ZIP

ENTRYPOINT ["protoc"]
