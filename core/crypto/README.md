This directory contains files obtained from [starkware-libs/crypto-cpp](https://github.com/starkware-libs/crypto-cpp) 
with the following changes:
 - For `go` support files contain under `crypto-cpp/src/starkware` have been flattened on a single 
   level with some changes to the files names where the file names conflicted, such as, files 
   contained under `crypto-cpp/src/starkware/crypto/ffi` have the suffix `wrappers` and 
   `crypto-cpp/src/starkware/utils/math.h` have been renamed to `starkware_math.h`.
 - Add support for big endian arrays in `utils.cc` and `utils.h`.
 - Rename from `crypto_lib.go` and `crypto_lib_test.go` to `crypto.go` and `crypto_test.go`, 
   respectively. The following changes were made compared to the files:
   - Use go convention on comments and variable names.
   - Delete use of unnecessary variables which were created for C since C can access go memory.
   - Refactor different array sizes into consts.
   - Create functions which act on byte slices instead of strings.
