package replay

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
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
	headerRangeFunctionID [32]byte,
	nextHeaderFunctionID [32]byte,
	filterRange int64,
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
			latestTargetContractBlock, err := targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
			if err != nil {
				return err
			}
			if event.StartBlock < latestTargetContractBlock {
				logger.Info("the target contract is at a higher block, waiting for new events", "event_start_block", event.StartBlock, "target_contract_latest_block", latestTargetContractBlock)
				continue
			} else if event.StartBlock > latestTargetContractBlock {
				logger.Info("the target contract needs to catchup", "event_start_block", event.StartBlock, "target_contract_latest_block", latestTargetContractBlock)
				err = Catchup(
					ctx,
					logger,
					verify,
					trpc,
					sourceEVMClient,
					targetEVMClient,
					sourceBlobstreamContractAddress,
					targetBlobstreamContractAddress,
					targetChainGatewayAddress,
					privateKey,
					headerRangeFunctionID,
					nextHeaderFunctionID,
					filterRange,
				)
				if err != nil {
					return err
				}
				latestTargetContractBlock, err = targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
				if err != nil {
					return err
				}
				if event.EndBlock == latestTargetContractBlock {
					// the contract is already up to date
					logger.Info("contract up to date", "target_contract_latest_block", event.EndBlock)
					continue
				}
			}
			logger.Debug("getting transaction containing the proof", "nonce", event.ProofNonce.Int64(), "hash", event.Raw.TxHash.Hex(), "start_block", event.StartBlock)
			tx, _, err := sourceEVMClient.TransactionByHash(ctx, event.Raw.TxHash)
			if err != nil {
				return err
			}

			logger.Debug("decoding the proof")
			rawMap := make(map[string]interface{})
			inputArgs := abi.Methods["fulfillCall"].Inputs
			err = inputArgs.UnpackIntoMap(rawMap, tx.Data()[4:])
			if err != nil {
				return err
			}

			decodedArgs, err := toFulfillCallArgs(rawMap)
			if err != nil {
				return err
			}

			// update the address to be the target blobstreamX contract for the callback
			decodedArgs.CallbackAddress = ethcmn.HexToAddress(targetBlobstreamContractAddress)
			if event.EndBlock-event.StartBlock > 1 {
				// this is a header range proof
				decodedArgs.FunctionID = headerRangeFunctionID
			} else {
				// this is a next header proof
				decodedArgs.FunctionID = nextHeaderFunctionID
			}

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
	headerRangeFunctionID [32]byte,
	nextHeaderFunctionID [32]byte,
	filterRange int64,
) error {
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

	latestSourceContractBlock, err := sourceBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	latestTargetContractBlock, err := targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	logger.Info("catching up", "latest_source_contract_block", latestSourceContractBlock, "latest_target_contract_block", latestTargetContractBlock)

	latestSourceContractNonce, err := sourceBlobstreamX.StateProofNonce(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	dataCommitmentEvents, err := getAllDataCommitmentStoredEvents(
		ctx,
		logger,
		&sourceBlobstreamX.BlobstreamXFilterer,
		int64(lookupStartHeight),
		filterRange,
		latestSourceContractNonce.Int64(),
		int64(latestTargetContractBlock),
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

	for startHeight := latestTargetContractBlock; startHeight < latestSourceContractBlock; {
		event, exists := dataCommitmentEvents[int64(startHeight)]
		if !exists {
			return fmt.Errorf("couldn't find a proof that starts at height %d in events", startHeight)
		}

		if verify {
			logger.Info("verifying data root tuple root", "proof_nonce_in_source_contract", event.ProofNonce, "start_block", event.StartBlock, "end_block", event.EndBlock)
			coreDataCommitment, err := trpc.DataCommitment(ctx, event.StartBlock, event.EndBlock)
			if err != nil {
				return err
			}
			if bytes.Equal(coreDataCommitment.DataCommitment.Bytes(), event.DataCommitment[:]) {
				logger.Info("data commitment verified")
			} else {
				logger.Error(
					"data commitment mismatch!! quitting",
					"proof_nonce_in_source_contract",
					event.ProofNonce,
					"start_block",
					event.StartBlock,
					"end_block",
					event.EndBlock,
					"expected_data_commitment",
					hex.EncodeToString(coreDataCommitment.DataCommitment.Bytes()),
					"actual_data_commitment",
					hex.EncodeToString(event.DataCommitment[:]),
				)
				return fmt.Errorf("data commitment mistmatch. start height %d end height %d", event.StartBlock, event.EndBlock)
			}
		}

		latestSourceBlock, err := sourceBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
		if err != nil {
			return err
		}

		latestTargetContractBlock, err = targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
		if err != nil {
			return err
		}
		if latestTargetContractBlock >= latestSourceBlock {
			// contract already up to date
			return nil
		}

		logger.Debug("getting transaction containing the proof", "startHeight", startHeight, "hash", event.Raw.TxHash.Hex())
		tx, _, err := sourceEVMClient.TransactionByHash(ctx, event.Raw.TxHash)
		if err != nil {
			return err
		}

		logger.Debug("decoding the proof")
		rawMap := make(map[string]interface{})
		inputArgs := abi.Methods["fulfillCall"].Inputs
		err = inputArgs.UnpackIntoMap(rawMap, tx.Data()[4:])
		if err != nil {
			return err
		}

		decodedArgs, err := toFulfillCallArgs(rawMap)
		if err != nil {
			return err
		}

		// update the address to be the target blobstreamX contract for the callback
		decodedArgs.CallbackAddress = ethcmn.HexToAddress(targetBlobstreamContractAddress)

		if event.EndBlock-event.StartBlock > 1 {
			// this is a header range proof
			decodedArgs.FunctionID = headerRangeFunctionID
		} else {
			// this is a next header proof
			decodedArgs.FunctionID = nextHeaderFunctionID
		}

		logger.Info("replaying the proof", "startHeight", startHeight)
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
			int64(startHeight),
			3*time.Minute,
		)
		if err != nil {
			return err
		}
		// make sure the contract was updated
		latestTargetContractBlock, err = targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
		if err != nil {
			return err
		}
		if latestTargetContractBlock == event.EndBlock {
			// contract updated successfully, we can advance
			startHeight = event.EndBlock
		} else {
			logger.Error("contract did not update successfully, retrying the same proof", "expected_target_height", event.EndBlock, "actual_target_height", latestTargetContractBlock)
		}
	}

	latestTargetContractBlock, err = targetBlobstreamX.LatestBlock(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	logger.Info("contract up to date", "latest_target_contract_block", latestTargetContractBlock)
	return nil
}

func getAllDataCommitmentStoredEvents(
	ctx context.Context,
	logger tmlog.Logger,
	blobstreamLogFilterer *blobstreamxwrapper.BlobstreamXFilterer,
	lookupStartHeight int64,
	filterRange int64,
	latestSourceContractNonce int64,
	latestTargetContractBlock int64,
) (map[int64]blobstreamxwrapper.BlobstreamXDataCommitmentStored, error) {
	logger.Info("querying all the data commitment stored events in the source contract...")
	dataCommitmentEvents := make(map[int64]blobstreamxwrapper.BlobstreamXDataCommitmentStored)
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

		gatheredTheNecessaryEvents := false
		for {
			if events.Event != nil {
				_, exists := dataCommitmentEvents[int64(events.Event.StartBlock)]
				if exists {
					continue
				} else {
					dataCommitmentEvents[int64(events.Event.StartBlock)] = *events.Event
					if int64(events.Event.StartBlock) < latestTargetContractBlock {
						gatheredTheNecessaryEvents = true
					}
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
		if gatheredTheNecessaryEvents {
			logger.Info("found enough events to cover the needed range", "count", len(dataCommitmentEvents))
			break
		}
		logger.Info("found events", "count", len(dataCommitmentEvents))
	}
	return dataCommitmentEvents, nil
}
