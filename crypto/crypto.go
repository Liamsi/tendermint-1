package crypto

import (
	cmn "github.com/tendermint/tendermint/libs/common"
)

type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) (Signature, error)
	PubKey() PubKey
	Equals(PrivKey) bool
}

// An address is a []byte, but hex-encoded even in JSON.
// []byte leaves us the option to change the address length.
// Use an alias so Unmarshal methods (with ptr receivers) are available too.
type Address = cmn.HexBytes

type PubKey interface {
	Address() Address
	Bytes() []byte
	VerifyBytes(msg []byte, sig Signature) bool
	Equals(PubKey) bool
}

type Signature interface {
	Bytes() []byte
	IsZero() bool
	Equals(Signature) bool
}

type Symmetric interface {
	Keygen() []byte
	Encrypt(plaintext []byte, secret []byte) (ciphertext []byte)
	Decrypt(ciphertext []byte, secret []byte) (plaintext []byte, err error)
}
