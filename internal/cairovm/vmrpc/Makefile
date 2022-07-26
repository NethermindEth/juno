generate:
	protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative vm.proto
	python3 -m grpc_tools.protoc --proto_path=. --python_out=. --grpc_python_out=. vm.proto
