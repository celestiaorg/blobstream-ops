package replayer

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	coregethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	bindings2 "github.com/succinctlabs/blobstreamx/bindings"
	"github.com/succinctlabs/succinctx/bindings"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type fulfillCallArgs struct {
	FunctionID      [32]byte       `json:"_functionId"`
	Input           []byte         `json:"_input"`
	Output          []byte         `json:"_output"`
	Proof           []byte         `json:"_proof"`
	CallbackAddress ethcmn.Address `json:"_callbackAddress"`
	CallbackData    []byte         `json:"_callbackData"`
}

func toFulfillCallArgs(args map[string]interface{}) (fulfillCallArgs, error) {
	fID, ok := args["_functionId"]
	if !ok {
		return fulfillCallArgs{}, fmt.Errorf("couldn't find the _functionId in map")
	}
	input, ok := args["_input"]
	if !ok {
		return fulfillCallArgs{}, fmt.Errorf("couldn't find the _input in map")
	}
	output, ok := args["_output"]
	if !ok {
		return fulfillCallArgs{}, fmt.Errorf("couldn't find the _output in map")
	}
	proof, ok := args["_proof"]
	if !ok {
		return fulfillCallArgs{}, fmt.Errorf("couldn't find the _proof in map")
	}
	callbackAddress, ok := args["_callbackAddress"]
	if !ok {
		return fulfillCallArgs{}, fmt.Errorf("couldn't find the _callbackAddress in map")
	}
	callbackData, ok := args["_callbackData"]
	if !ok {
		return fulfillCallArgs{}, fmt.Errorf("couldn't find the _callbackData in map")
	}

	return fulfillCallArgs{
		FunctionID:      fID.([32]byte),
		Input:           input.([]byte),
		Output:          output.([]byte),
		Proof:           proof.([]byte),
		CallbackAddress: callbackAddress.(ethcmn.Address),
		CallbackData:    callbackData.([]byte),
	}, nil
}

type transactOpsBuilder func(ctx context.Context, client *ethclient.Client, gasLim uint64) (*bind.TransactOpts, error)

func newTransactOptsBuilder(privKey *ecdsa.PrivateKey) transactOpsBuilder {
	publicKey := privKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		panic(fmt.Errorf("invalid public key; expected: %T, got: %T", &ecdsa.PublicKey{}, publicKey))
	}

	evmAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	return func(ctx context.Context, client *ethclient.Client, gasLim uint64) (*bind.TransactOpts, error) {
		nonce, err := client.PendingNonceAt(ctx, evmAddress)
		if err != nil {
			return nil, err
		}

		ethChainID, err := client.ChainID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get Ethereum chain ID: %w", err)
		}

		auth, err := bind.NewKeyedTransactorWithChainID(privKey, ethChainID)
		if err != nil {
			return nil, fmt.Errorf("failed to create Ethereum transactor: %w", err)
		}

		bigGasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get Ethereum gas estimate: %w", err)
		}

		auth.Nonce = new(big.Int).SetUint64(nonce)
		auth.Value = big.NewInt(0) // in wei
		auth.GasLimit = gasLim     // in units
		auth.GasPrice = bigGasPrice

		return auth, nil
	}
}

func submitProof(
	ctx context.Context,
	logger tmlog.Logger,
	client *ethclient.Client,
	opts *bind.TransactOpts,
	succinctGateway *bindings.SuccinctGateway,
	targetBlobstreamXContract *bindings2.BlobstreamX,
	args fulfillCallArgs,
	proofNonce int64,
	waitTimeout time.Duration,
) error {
	for i := 0; i < 10; i++ {
		logger.Info("submitting proof", "nonce", proofNonce, "gas_price", opts.GasPrice.Int64())
		tx, err := succinctGateway.FulfillCall(
			opts,
			args.FunctionID,
			args.Input,
			args.Output,
			args.Proof,
			args.CallbackAddress,
			args.CallbackData,
		)
		if err != nil {
			return err
		}
		_, err = waitForTransaction(ctx, logger, client, tx, waitTimeout)
		if err != nil {
			actualNonce, err := targetBlobstreamXContract.StateProofNonce(&bind.CallOpts{})
			if err != nil {
				return err
			}
			if actualNonce.Int64() > proofNonce {
				logger.Info("no need to replay this nonce, the contract has already committed to it", "nonce", actualNonce)
				return nil
			}

			if errors.Is(err, context.DeadlineExceeded) {
				// we need to speed up the transaction by increasing the gas price
				bigGasPrice, err := client.SuggestGasPrice(ctx)
				if err != nil {
					return fmt.Errorf("failed to get Ethereum gas estimate: %w", err)
				}

				// 20% increase of the suggested gas price
				opts.GasPrice = big.NewInt(bigGasPrice.Int64() + bigGasPrice.Int64()/5)
				continue
			} else {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("failed to submit proof nonce %d", proofNonce)
}

func waitForTransaction(
	ctx context.Context,
	logger tmlog.Logger,
	backend bind.DeployBackend,
	tx *coregethtypes.Transaction,
	timeout time.Duration,
) (*coregethtypes.Receipt, error) {
	logger.Debug("waiting for transaction to be confirmed", "hash", tx.Hash().String())

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, backend, tx)
	if err == nil && receipt != nil && receipt.Status == 1 {
		logger.Info("transaction confirmed", "hash", tx.Hash().String(), "block", receipt.BlockNumber.Uint64())
		return receipt, nil
	}

	return receipt, err
}
