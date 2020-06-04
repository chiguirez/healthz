ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
protoc = @docker run -u1000 --rm -it -v $(ROOT_DIR):/go/src/healthz --workdir /go/src/healthz protoc

init:
	@docker build . -t protoc

proto:
	$(protoc) \
		--proto_path=/go/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--proto_path=/usr/local/include \
		--proto_path=proto/health/v1/ \
		--go_out=plugins=grpc:. \
		--grpc-gateway_out=logtostderr=true,paths=source_relative:proto/health/v1/ \
		$$(find -name "*.proto")
.PHONY: proto