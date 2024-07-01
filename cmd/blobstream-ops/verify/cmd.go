package verify

import (
	"bytes"
	"context"
	"fmt"

	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/common"
	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/version"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	blobstreamxwrapper "github.com/succinctlabs/blobstreamx/bindings"
	"github.com/tendermint/tendermint/rpc/client/http"
)

// Command the verify command
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "verify",
		Short:        "BlobstreamX deployment verification",
		Long:         "verifies a BlobstreamX deployment",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		VerifyContractCommand(),
	)

	cmd.SetHelpCommand(&cobra.Command{})

	return cmd
}

// VerifyContractCommand the contract verifier start command.
//
//nolint:all
func VerifyContractCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "contract <flags>",
		Short: "Starts the BlobstreamX contract verifier",
		Long:  "verifies that a BlobstreamX contract is committing to valid data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, err := parseStartFlags(cmd)
			if err != nil {
				return err
			}
			if err := config.ValidateBasics(); err != nil {
				return err
			}

			logger, err := common.GetLogger(config.LogLevel, config.LogFormat)
			if err != nil {
				return err
			}

			buildInfo := version.GetBuildInfo()
			logger.Info("initializing verifier", "version", buildInfo.SemanticVersion, "build_date", buildInfo.BuildTime)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// connecting to a BlobstreamX contract
			evmClient, err := ethclient.Dial(config.EVMRPC)
			if err != nil {
				return err
			}
			defer evmClient.Close()
			blobstreamReader, err := blobstreamxwrapper.NewBlobstreamXCaller(ethcmn.HexToAddress(config.ContractAddress), evmClient)
			if err != nil {
				return err
			}
			blobstreamLogFilterer, err := blobstreamxwrapper.NewBlobstreamXFilterer(ethcmn.HexToAddress(config.ContractAddress), evmClient)
			if err != nil {
				return err
			}

			// Listen for and trap any OS signal to graceful shutdown and exit
			go common.TrapSignal(logger, cancel)

			logger.Info(
				"starting verifier",
				"evm.rpc",
				config.EVMRPC,
				"evm.contract-address",
				config.ContractAddress,
				"core.rpc",
				config.CoreRPC,
			)

			latestNonce, err := blobstreamReader.StateProofNonce(&bind.CallOpts{})
			if err != nil {
				return err
			}

			logger.Info("found latest blobstreamX contract nonce", "nonce", latestNonce.Int64())

			evmChainTip, err := evmClient.BlockNumber(ctx)
			if err != nil {
				return err
			}

			logger.Debug("evm chain latest block number", "number", evmChainTip)

			maxFilterRange := int64(5000)
			dataCommitmentEvents := make(map[int]blobstreamxwrapper.BlobstreamXDataCommitmentStored)
			for eventLookupEnd := int64(evmChainTip); eventLookupEnd > 0; eventLookupEnd -= maxFilterRange {
				logger.Debug("querying all the data commitment stored events", "evm_block_start", eventLookupEnd, "evm_block_end", eventLookupEnd-maxFilterRange)
				rangeStart := eventLookupEnd - maxFilterRange
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
					return err
				}

				for {
					if events.Event != nil {
						_, exists := dataCommitmentEvents[int(events.Event.ProofNonce.Int64())]
						if exists {
							continue
						}
						dataCommitmentEvents[int(events.Event.ProofNonce.Int64())] = *events.Event
					}
					if !events.Next() {
						break
					}
				}
				if int64(len(dataCommitmentEvents)) >= latestNonce.Int64()-1 {
					// found all the events
					logger.Info("found all events", "count", len(dataCommitmentEvents))
					break
				}
				logger.Info("found events", "count", len(dataCommitmentEvents))
			}

			trpc, err := http.New(config.CoreRPC, "/websocket")
			if err != nil {
				return err
			}
			err = trpc.Start()
			if err != nil {
				return err
			}
			defer func(trpc *http.HTTP) {
				err := trpc.Stop()
				if err != nil {
					fmt.Println(err.Error())
				}
			}(trpc)

			for nonce := 1; nonce < int(latestNonce.Int64()); nonce++ {
				event, exists := dataCommitmentEvents[nonce]
				if !exists {
					return fmt.Errorf("couldn't find nonce %d in events", nonce)
				}
				logger.Info("verifying data root tuple root", "nonce", event.ProofNonce, "start_block", event.StartBlock, "end_block", event.EndBlock)
				coreDataCommitment, err := trpc.DataCommitment(ctx, event.StartBlock, event.EndBlock)
				if err != nil {
					return err
				}
				if bytes.Equal(coreDataCommitment.DataCommitment.Bytes(), event.DataCommitment[:]) {
					logger.Info("data commitment matches")
				} else {
					logger.Error("data commitment mismatch!! quitting", "nonce", event.ProofNonce)
					return fmt.Errorf("data commitment mistmatch. nonce %d", event.ProofNonce)
				}
			}
			logger.Info("blobstreamX contract verified")
			return nil
		},
	}
	return addStartFlags(command)
}
