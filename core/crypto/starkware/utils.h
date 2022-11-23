#ifndef STARKWARE_CRYPTO_FFI_UTILS_H_
#define STARKWARE_CRYPTO_FFI_UTILS_H_

#include <cstddef>

#include "pedersen_hash.h"

#include "gsl/gsl-lite.hpp"

namespace starkware {

using ValueType = PrimeFieldElement::ValueType;

/*
  Handles an error, and outputs a relevant error message as a C string to out.
*/
int HandleError(const char* msg, gsl::span<gsl::byte> out);

/*
  Deserializes a BigInt (PrimeFieldElement::ValueType) from a byte span.
*/
ValueType Deserialize(const gsl::span<const gsl::byte> span, bool le = true);

/*
  Serializes a BigInt (PrimeFieldElement::ValueType) to a byte span.
*/
void Serialize(const ValueType& val, const gsl::span<gsl::byte> span_out, bool le = true);

}  // namespace starkware

#endif  // STARKWARE_CRYPTO_FFI_UTILS_H_
