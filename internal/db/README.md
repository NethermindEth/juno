# Database Package

This package has all the managers, one for each type stored on the database. The managers are: `abi`, `state`
and `transaction`. Each manager has a protobuf file (`abi.proto`) to describe the model stored on its database.

After modifying the proto file of a certain manager, you need to generate the Go code for the models again. To do it
exists two Make commands:

- `make clean`: is used to clean all the files generated during the compilation, including the model files.
- `make generate`: generate the files for database models. This command overrides the previously generated files,
  so `make clean` is not required.
