package replayer

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	blobstreamxwrapper "github.com/succinctlabs/blobstreamx/bindings"
	"github.com/succinctlabs/succinctx/bindings"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/rpc/client/http"
)

func Follow(
	ctx context.Context,
	logger tmlog.Logger,
	verify bool,
	trpc *http.HTTP,
	sourceEVMClient *ethclient.Client,
	targetEVMClient *ethclient.Client,
	sourceBlobstreamContractAddress string,
	targetBlobstreamContractAddress string,
	targetChainGatewayAddress string,
	privateKey *ecdsa.PrivateKey,
) error {
	logger.Info("listening for new proofs on the source chain")
	sourceBlobstreamX, err := blobstreamxwrapper.NewBlobstreamX(ethcmn.HexToAddress(sourceBlobstreamContractAddress), sourceEVMClient)
	if err != nil {
		return err
	}

	targetBlobstreamX, err := blobstreamxwrapper.NewBlobstreamX(ethcmn.HexToAddress(targetBlobstreamContractAddress), sourceEVMClient)
	if err != nil {
		return err
	}

	newEvents := make(chan *blobstreamxwrapper.BlobstreamXDataCommitmentStored)
	subscription, err := sourceBlobstreamX.WatchDataCommitmentStored(&bind.WatchOpts{Context: ctx}, newEvents, nil, nil, nil)
	if err != nil {
		return err
	}
	defer subscription.Unsubscribe()

	gateway, err := bindings.NewSuccinctGateway(ethcmn.HexToAddress(targetChainGatewayAddress), targetEVMClient)
	if err != nil {
		return err
	}
	abi, err := bindings.SuccinctGatewayMetaData.GetAbi()
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-newEvents:
			latestTargetContractNonce, err := targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
			if err != nil {
				return err
			}
			if event.ProofNonce.Int64() < int64(latestTargetContractNonce) {
				logger.Info("the target contract is at a higher nonce, waiting for new events", "event_nonce", event.ProofNonce, "target_contract_latest_nonce", latestTargetContractNonce)
				continue
			} else if event.ProofNonce.Int64() > int64(latestTargetContractNonce) {
				logger.Info("the target contract needs to catchup", "event_nonce", event.ProofNonce, "target_contract_latest_nonce", latestTargetContractNonce)
				err = Catchup(ctx, logger, verify, trpc, sourceEVMClient, targetEVMClient, sourceBlobstreamContractAddress, targetBlobstreamContractAddress, targetChainGatewayAddress, privateKey)
				if err != nil {
					return err
				}
			}
			logger.Debug("getting transaction containing the proof", "nonce", event.ProofNonce.Int64(), "hash", event.Raw.TxHash.Hex())
			tx, _, err := sourceEVMClient.TransactionByHash(ctx, event.Raw.TxHash)
			if err != nil {
				return err
			}

			logger.Debug("decoding the proof")
			rawMap := make(map[string]interface{})
			err = abi.UnpackIntoMap(rawMap, "fulfillCall", tx.Data())
			if err != nil {
				return err
			}

			decodedArgs, err := toFulfillCallArgs(rawMap)
			if err != nil {
				return err
			}

			// update the address to be the target blobstreamX contract for the callback
			decodedArgs.CallbackAddress = ethcmn.HexToAddress(targetBlobstreamContractAddress)

			logger.Info("replaying the proof", "nonce", event.ProofNonce.Int64())
			opts, err := newTransactOptsBuilder(privateKey)(ctx, targetEVMClient, 25000000)
			if err != nil {
				return err
			}
			err = submitProof(
				ctx,
				logger,
				targetEVMClient,
				opts,
				gateway,
				targetBlobstreamX,
				decodedArgs,
				event.ProofNonce.Int64(),
				3*time.Minute,
			)
			if err != nil {
				return err
			}
			logger.Info("successfully replayed proof", "nonce", event.ProofNonce.Int64())
		}
	}
}

func Catchup(
	ctx context.Context,
	logger tmlog.Logger,
	verify bool,
	trpc *http.HTTP,
	sourceEVMClient *ethclient.Client,
	targetEVMClient *ethclient.Client,
	sourceBlobstreamContractAddress string,
	targetBlobstreamContractAddress string,
	targetChainGatewayAddress string,
	privateKey *ecdsa.PrivateKey,
) error {
	filterRange := int64(5000)

	lookupStartHeight, err := sourceEVMClient.BlockNumber(ctx)
	if err != nil {
		return err
	}

	sourceBlobstreamX, err := blobstreamxwrapper.NewBlobstreamX(ethcmn.HexToAddress(sourceBlobstreamContractAddress), sourceEVMClient)
	if err != nil {
		return err
	}

	targetBlobstreamX, err := blobstreamxwrapper.NewBlobstreamX(ethcmn.HexToAddress(targetBlobstreamContractAddress), sourceEVMClient)
	if err != nil {
		return err
	}

	latestSourceContractNonce, err := sourceBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	latestTargetContractNonce, err := targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	logger.Info("catching up", "latest_source_contract_nonce", latestSourceContractNonce, "latest_target_contract_nonce", latestTargetContractNonce)

	// TODO: this could be improved in the future to only get the events needed
	dataCommitmentEvents, err := getAllDataCommitmentStoredEvents(
		ctx,
		logger,
		&sourceBlobstreamX.BlobstreamXFilterer,
		int64(lookupStartHeight),
		filterRange,
		int64(latestSourceContractNonce),
	)
	if err != nil {
		return err
	}

	gateway, err := bindings.NewSuccinctGateway(ethcmn.HexToAddress(targetChainGatewayAddress), targetEVMClient)
	if err != nil {
		return err
	}
	abi, err := bindings.SuccinctGatewayMetaData.GetAbi()
	if err != nil {
		return err
	}

	for nonce := latestTargetContractNonce; nonce < latestSourceContractNonce; nonce++ {
		event, exists := dataCommitmentEvents[int(nonce)]
		if !exists {
			return fmt.Errorf("couldn't find nonce %d in events", nonce)
		}

		if verify {
			logger.Info("verifying data root tuple root", "nonce", event.ProofNonce, "start_block", event.StartBlock, "end_block", event.EndBlock)
			coreDataCommitment, err := trpc.DataCommitment(ctx, event.StartBlock, event.EndBlock)
			if err != nil {
				return err
			}
			if bytes.Equal(coreDataCommitment.DataCommitment.Bytes(), event.DataCommitment[:]) {
				logger.Info("data commitment verified")
			} else {
				logger.Error("data commitment mismatch!! quitting", "nonce", event.ProofNonce)
				return fmt.Errorf("data commitment mistmatch. nonce %d", event.ProofNonce)
			}
		}

		latestSourceBlock, err := sourceBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
		if err != nil {
			return err
		}

		latestTargetBlock, err := targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
		if err != nil {
			return err
		}
		if latestTargetBlock > latestSourceBlock {
			// contract already up to date
			return nil
		}
		if latestTargetBlock != event.StartBlock {
			return fmt.Errorf("can't replay event to contract. mismatch latest target block %d and start block %d", latestTargetBlock, event.StartBlock)
		}

		logger.Debug("getting transaction containing the proof", "nonce", nonce, "hash", event.Raw.TxHash.Hex())
		tx, _, err := sourceEVMClient.TransactionByHash(ctx, event.Raw.TxHash)
		if err != nil {
			return err
		}

		logger.Debug("decoding the proof")
		rawMap := make(map[string]interface{})
		err = abi.UnpackIntoMap(rawMap, "fulfillCall", tx.Data())
		if err != nil {
			return err
		}

		decodedArgs, err := toFulfillCallArgs(rawMap)
		if err != nil {
			return err
		}

		// update the address to be the target blobstreamX contract for the callback
		decodedArgs.CallbackAddress = ethcmn.HexToAddress(targetBlobstreamContractAddress)

		logger.Info("replaying the proof", "nonce", nonce)
		opts, err := newTransactOptsBuilder(privateKey)(ctx, targetEVMClient, 25000000)
		if err != nil {
			return err
		}
		err = submitProof(
			ctx,
			logger,
			targetEVMClient,
			opts,
			gateway,
			targetBlobstreamX,
			decodedArgs,
			int64(nonce),
			3*time.Minute,
		)
		if err != nil {
			return err
		}
	}

	latestTargetContractNonce, err = targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	logger.Info("contract up to date", "latest_nonce", latestTargetContractNonce)
	return nil
}

func getAllDataCommitmentStoredEvents(
	ctx context.Context,
	logger tmlog.Logger,
	blobstreamLogFilterer *blobstreamxwrapper.BlobstreamXFilterer,
	lookupStartHeight int64,
	filterRange int64,
	latestSourceContractNonce int64,
) (map[int]blobstreamxwrapper.BlobstreamXDataCommitmentStored, error) {
	logger.Info("querying all the data commitment stored events in the source contract...")
	dataCommitmentEvents := make(map[int]blobstreamxwrapper.BlobstreamXDataCommitmentStored)
	for eventLookupEnd := lookupStartHeight; eventLookupEnd > 0; eventLookupEnd -= filterRange {
		logger.Debug("querying all the data commitment stored events", "evm_block_start", eventLookupEnd, "evm_block_end", eventLookupEnd-filterRange)
		rangeStart := eventLookupEnd - filterRange
		rangeEnd := uint64(eventLookupEnd)
		events, err := blobstreamLogFilterer.FilterDataCommitmentStored(
			&bind.FilterOpts{
				Context: ctx,
				Start:   uint64(rangeStart),
				End:     &rangeEnd,
			},
			nil,
			nil,
			nil,
		)
		if err != nil {
			return nil, err
		}

		for {
			if events.Event != nil {
				_, exists := dataCommitmentEvents[int(events.Event.ProofNonce.Int64())]
				if exists {
					continue
				} else {
					dataCommitmentEvents[int(events.Event.ProofNonce.Int64())] = *events.Event
				}
			}
			if !events.Next() {
				break
			}
		}
		if int64(len(dataCommitmentEvents)) >= latestSourceContractNonce-1 {
			// found all the events
			logger.Info("found all events", "count", len(dataCommitmentEvents))
			break
		}
		logger.Info("found events", "count", len(dataCommitmentEvents))
	}
	return dataCommitmentEvents, nil
}
