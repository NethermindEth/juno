package starkware

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

/*

#cgo CFLAGS: -I. -Werror -fno-strict-aliasing -fPIC
#cgo CXXFLAGS: -I. -std=c++17 -fno-strict-aliasing -fPIC
#include <stdlib.h>
#include "ecdsa_wrappers.h"
#include "pedersen_hash_wrappers.h"

*/
import "C"
import "unsafe"

const (
	BufferSize    = 1024
	PubKeySize    = 32
	FeltSize      = 32
	SignatureSize = 64
)

var (
	curveOrder, _ = new(big.Int).SetString("800000000000010ffffffffffffffffb781126dcae7b2321e66a241adc64d2f", 16)
	cErr          = errors.New("encountered an error")
)

func invertOnCurveBinary(w []byte) []byte {
	wBigInt := new(big.Int).SetBytes(w)
	wBigInt.ModInverse(wBigInt, curveOrder)
	return wBigInt.Bytes()
}

// padHexString pads the given hex string with leading zeroes so that its length is 64 characters
// (32 bytes).
func padHexString(s string) string {
	padded := fmt.Sprintf("%064s", s[2:])
	return padded
}

func hash(x, y string) (string, error) {
	out := make([]byte, BufferSize)

	xDec, err := hex.DecodeString(padHexString(x))
	if err != nil {
		return "", err
	}

	yDec, err := hex.DecodeString(padHexString(y))
	if err != nil {
		return "", err
	}

	if err = Hash(xDec, yDec, out); err != nil {
		return "", err
	}

	return "0x" + hex.EncodeToString(out[:FeltSize]), nil
}

// Hash Computes the StarkWare version of the Pedersen hash of x and y.
// Full specification of the hash function can be found here:
// https://docs.starkware.co/starkex-docs/crypto/pedersen-hash-function
func Hash(x, y, out []byte) error {
	res := C.Hash(
		(*C.char)(unsafe.Pointer(&x[0])),
		(*C.char)(unsafe.Pointer(&y[0])),
		(*C.char)(unsafe.Pointer(&out[0])))

	if res != 0 {
		return cErr
	}

	return nil
}

func publicKey(privateKey string) (string, error) {
	privateKeyDec, err := hex.DecodeString(padHexString(privateKey))
	if err != nil {
		return "", err
	}

	out := make([]byte, BufferSize)

	if err = PublicKey(privateKeyDec, out); err != nil {
		return "", err
	}

	return "0x" + hex.EncodeToString(out[:PubKeySize]), nil
}

// PublicKey deduces the public key given a private key.
func PublicKey(privateKey, out []byte) error {
	res := C.GetPublicKey(
		(*C.char)(unsafe.Pointer(&privateKey[0])),
		(*C.char)(unsafe.Pointer(&out[0])))

	if res != 0 {
		return cErr
	}

	return nil
}

func verify(starkKey, msgHash, r, s string) (bool, error) {
	starkKeyDec, err := hex.DecodeString(padHexString(starkKey))
	if err != nil {
		return false, nil
	}
	messageDec, err := hex.DecodeString(padHexString(msgHash))
	if err != nil {
		return false, err
	}
	rDec, err := hex.DecodeString(padHexString(r))
	if err != nil {
		return false, err
	}
	sDec, err := hex.DecodeString(padHexString(s))
	if err != nil {
		return false, err
	}

	return Verify(starkKeyDec, messageDec, rDec, sDec), nil
}

// Verify verifies ECDSA signature of a given message hash z with a given public key.
// Returns true if public_key signs the message.
// NOTE: This function assumes that the public_key is on the curve.
func Verify(starkKey, msgHash, r, s []byte) bool {
	w := invertOnCurveBinary(s)

	res := C.Verify(
		(*C.char)(unsafe.Pointer(&starkKey[0])),
		(*C.char)(unsafe.Pointer(&msgHash[0])),
		(*C.char)(unsafe.Pointer(&r[0])),
		(*C.char)(unsafe.Pointer(&w[0])))
	return res != 0
}

func sign(privateKey, message, k string) (string, string, error) {
	privateKeyDec, err := hex.DecodeString(padHexString(privateKey))
	if err != nil {
		return "", "", err
	}
	messageDec, err := hex.DecodeString(padHexString(message))
	if err != nil {
		return "", "", err
	}
	kDec, err := hex.DecodeString(padHexString(k))
	if err != nil {
		return "", "", err
	}
	out := make([]byte, BufferSize)

	if err = Sign(privateKeyDec, messageDec, kDec, out); err != nil {
		return "", "", err
	}

	res := hex.EncodeToString(out[:SignatureSize])
	signatureR := "0x" + res[0:SignatureSize]
	signatureS := "0x" + res[SignatureSize:]
	return signatureR, signatureS, nil
}

// Sign signs the given message hash with the provided private_key, with randomness k.
//
// NOTE: k should be a strong cryptographical random, and not repeat.
// See: https://tools.ietf.org/html/rfc6979.
func Sign(privateKey, message, k, out []byte) error {
	ret := C.Sign(
		(*C.char)(unsafe.Pointer(&privateKey[0])),
		(*C.char)(unsafe.Pointer(&message[0])),
		(*C.char)(unsafe.Pointer(&k[0])),
		(*C.char)(unsafe.Pointer(&out[0])))

	copy(out[SignatureSize/2:SignatureSize], invertOnCurveBinary(out[SignatureSize/2:SignatureSize]))

	if ret != 0 {
		return cErr
	}

	return nil
}
