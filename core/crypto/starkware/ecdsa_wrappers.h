#ifndef STARKWARE_CRYPTO_FFI_ECDSA_H_
#define STARKWARE_CRYPTO_FFI_ECDSA_H_

int GetPublicKey(const char* private_key, char* out);

int Verify(const char* stark_key, const char* msg_hash, const char* r_bytes, const char* w_bytes);

int Sign(const char* private_key, const char* message, const char* k, char* out);

#endif  // STARKWARE_CRYPTO_FFI_ECDSA_H_
