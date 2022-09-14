clean:
	@rm -f ./abi/abi.pb.go
	@rm -f ./transaction/transaction.pb.go
	@rm -f ./state/state.pb.go

generate:
	@protoc --proto_path=./transaction --go_out=./transaction --go_opt=paths=source_relative transaction.proto
	@protoc --proto_path=./state --go_out=./state --go_opt=paths=source_relative state.proto
	@protoc --proto_path=./block --go_out=./block --go_opt=paths=source_relative block.proto
	@protoc --proto_path=./sync --go_out=./sync --go_opt=paths=source_relative sync.proto
	@protoc --proto_path=./class --go_out=./class --go_opt=paths=source_relative class.proto

add-notest:
	find . -name '*.pb.go' -exec sh -c "awk '/func/ { print \"// notest\"; }; 1;' {} > tmp_file && mv tmp_file {}" \;

rm-notest:
	find . -name '*.pb.go' -exec sh -c "awk '!/\/\/ notest/' {} > tmp_file && mv tmp_file {}" \;
