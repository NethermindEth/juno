This directory contains files obtained from [starkware-libs/crypto-cpp](https://github.com/starkware-libs/crypto-cpp) 
with the following changes:
 - For `go` support files contain under `crypto-cpp/src/starkware` have been flattened on a single 
   level with some changes to the files names where the file names conflicted, such as, files 
   contained under `crypto-cpp/src/starkware/crypto/ffi` have the suffix `wrappers` and 
   `crypto-cpp/src/starkware/utils/math.h` have been renamed to `starkware_math.h`.
 - Add support for big endian arrays in `utils.cc` and `utils.h`.
