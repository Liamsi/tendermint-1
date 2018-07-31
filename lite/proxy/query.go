package proxy

import (
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/lite"
	lerr "github.com/tendermint/tendermint/lite/errors"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// GetWithProof will query the key on the given node, and verify it has
// a valid proof, as defined by the certifier.
//
// If there is any error in checking, returns an error.
func GetWithProof(prt *merkle.ProofRuntime, key []byte, reqHeight int64, node rpcclient.Client,
	cert lite.Certifier) (
	val cmn.HexBytes, height int64, proof *merkle.Proof, err error) {

	if reqHeight < 0 {
		err = cmn.NewError("Height cannot be negative")
		return
	}

	res, err := GetWithProofOptions(prt, "/key", key,
		rpcclient.ABCIQueryOptions{Height: int64(reqHeight), Prove: true},
		node, cert)
	if err != nil {
		return
	}

	resp := res.Response
	val, height, proof = resp.Value, resp.Height, resp.Proof
	return val, height, proof, err
}

// GetWithProofOptions is useful if you want full access to the ABCIQueryOptions.
// XXX Usage of path?  It's not used, and sometimes it's /, sometimes /key, sometimes /store.
func GetWithProofOptions(prt *merkle.ProofRuntime, path string, key []byte, opts rpcclient.ABCIQueryOptions,
	node rpcclient.Client, cert lite.Certifier) (
	*ctypes.ResultABCIQuery, error) {

	if !opts.Prove {
		return nil, cmn.NewError("require ABCIQueryOptions.Prove to be true")
	}

	res, err := node.ABCIQueryWithOptions(path, key, opts)
	if err != nil {
		return nil, err
	}
	resp := res.Response

	// Validate the response, e.g. height.
	if resp.IsErr() {
		err = cmn.NewError("Query error for key %d: %d", key, resp.Code)
		return nil, err
	}

	if len(resp.Key) == 0 || resp.Proof == nil {
		return nil, lerr.ErrEmptyTree()
	}
	if resp.Height == 0 {
		return nil, cmn.NewError("Height returned is zero")
	}

	// AppHash for height H is in header H+1
	signedHeader, err := GetCertifiedCommit(resp.Height+1, node, cert)
	if err != nil {
		return nil, err
	}

	// Validate the proof against the certified header to ensure data integrity.
	if resp.Value != nil {
		// Value exists
		// XXX How do we encode the key into a string...
		err = prt.VerifyValue(resp.Proof, signedHeader.AppHash, resp.Value, string(resp.Key))
		if err != nil {
			return nil, cmn.ErrorWrap(err, "Couldn't verify value proof")
		}
		return &ctypes.ResultABCIQuery{Response: resp}, nil
	} else {
		// Value absent
		// Validate the proof against the certified header to ensure data integrity.
		// XXX How do we encode the key into a string...
		err = prt.VerifyAbsence(resp.Proof, signedHeader.AppHash, string(resp.Key))
		if err != nil {
			return nil, cmn.ErrorWrap(err, "Couldn't verify absence proof")
		}
		return &ctypes.ResultABCIQuery{Response: resp}, nil
	}
}

// GetCertifiedCommit gets the signed header for a given height and certifies
// it. Returns error if unable to get a proven header.
func GetCertifiedCommit(h int64, client rpcclient.Client, cert lite.Certifier) (types.SignedHeader, error) {

	// FIXME: cannot use cert.GetByHeight for now, as it also requires
	// Validators and will fail on querying tendermint for non-current height.
	// When this is supported, we should use it instead...
	rpcclient.WaitForHeight(client, h, nil)
	cresp, err := client.Commit(&h)
	if err != nil {
		return types.SignedHeader{}, err
	}

	// Validate downloaded checkpoint with our request and trust store.
	sh := cresp.SignedHeader
	if sh.Height != h {
		return types.SignedHeader{}, fmt.Errorf("height mismatch: want %v got %v",
			h, sh.Height)
	}

	if err = cert.Certify(sh); err != nil {
		return types.SignedHeader{}, err
	}

	return sh, nil
}
