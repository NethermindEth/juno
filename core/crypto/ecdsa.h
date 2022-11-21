#ifndef STARKWARE_CRYPTO_ECDSA_H_
#define STARKWARE_CRYPTO_ECDSA_H_

#include <utility>

#include "elliptic_curve.h"
#include "prime_field_element.h"

namespace starkware {

/*
  Type representing a signature (r, w).
*/
using Signature = std::pair<PrimeFieldElement, PrimeFieldElement>;

/*
  Deduces the public key given a private key.
  The x coordinate of the public key is also known as the partial public key,
  and used in StarkEx to identify the user.
*/
EcPoint<PrimeFieldElement> GetPublicKey(const PrimeFieldElement::ValueType& private_key);

/*
  Signs message hash z with the provided private_key, with randomness k.

  NOTE: k should be a strong cryptographical random, and not repeat.
  See: https://tools.ietf.org/html/rfc6979.
*/
Signature SignEcdsa(
    const PrimeFieldElement::ValueType& private_key, const PrimeFieldElement& z,
    const PrimeFieldElement::ValueType& k);

/*
  Verifies ECDSA signature of a given hash message z with a given public key.
  Returns true if either public_key or -public_key signs the message.
  NOTE: This function assumes that the public_key is on the curve.
*/
bool VerifyEcdsa(
    const EcPoint<PrimeFieldElement>& public_key, const PrimeFieldElement& z, const Signature& sig);

/*
  Same as VerifyEcdsa() except that only the x coordinate of the public key is given. The y
  coordinate is computed, and both options are checked.
*/
bool VerifyEcdsaPartialKey(
    const PrimeFieldElement& public_key_x, const PrimeFieldElement& z, const Signature& sig);

}  // namespace starkware

#endif  // STARKWARE_CRYPTO_ECDSA_H_
