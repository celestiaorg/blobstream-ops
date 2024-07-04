package replay

import (
	"context"

	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/common"
	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/version"
	"github.com/celestiaorg/blobstream-ops/replay"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	blobstreamxwrapper "github.com/succinctlabs/blobstreamx/bindings"
	"github.com/tendermint/tendermint/rpc/client/http"
)

// Command the replay command
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "replay",
		Short:        "BlobstreamX deployment verification",
		Long:         "verifies that a BlobstreamX contract is committing to valid data",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, err := parseFlags(cmd)
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
			logger.Info("initializing replay service", "version", buildInfo.SemanticVersion, "build_date", buildInfo.BuildTime)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Listen for and trap any OS signal to graceful shutdown and exit
			go common.TrapSignal(logger, cancel)

			// connecting to the source BlobstreamX contract
			sourceEVMClient, err := ethclient.Dial(config.SourceEVMRPC)
			if err != nil {
				return err
			}
			defer sourceEVMClient.Close()

			sourceBlobstreamReader, err := blobstreamxwrapper.NewBlobstreamXCaller(
				ethcmn.HexToAddress(config.SourceContractAddress),
				sourceEVMClient,
			)
			if err != nil {
				return err
			}

			// connecting to the target BlobstreamX contract
			targetEVMClient, err := ethclient.Dial(config.TargetEVMRPC)
			if err != nil {
				return err
			}
			defer targetEVMClient.Close()

			targetBlobstreamReader, err := blobstreamxwrapper.NewBlobstreamXCaller(
				ethcmn.HexToAddress(config.TargetContractAddress),
				targetEVMClient,
			)
			if err != nil {
				return err
			}

			logger.Info(
				"starting replay service",
				"evm.source.rpc",
				config.SourceEVMRPC,
				"evm.source.contract-address",
				config.SourceContractAddress,
				"evm.target.rpc",
				config.TargetEVMRPC,
				"evm.target.contract-address",
				config.TargetContractAddress,
				"core.rpc",
				config.CoreRPC,
			)

			latestSourceBlock, err := sourceBlobstreamReader.LatestBlock(&bind.CallOpts{})
			if err != nil {
				return err
			}
			logger.Info("found source blobstreamX contract", "latest_block", latestSourceBlock)

			latestTargetBlock, err := targetBlobstreamReader.LatestBlock(&bind.CallOpts{})
			if err != nil {
				return err
			}
			logger.Info("found target blobstreamX contract", "latest_block", latestTargetBlock)

			var trpc *http.HTTP
			if config.Verify {
				trpc, err = http.New(config.CoreRPC, "/websocket")
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
						logger.Error("error stopping tendermint RPC", "err", err.Error())
					}
				}(trpc)
			}

			if latestSourceBlock > latestTargetBlock {
				err = replay.Catchup(
					ctx,
					logger,
					config.Verify,
					trpc,
					sourceEVMClient,
					targetEVMClient,
					config.SourceContractAddress,
					config.TargetContractAddress,
					config.TargetChainGateway,
					config.PrivateKey,
					config.HeaderRangeFunctionID,
					config.NextHeaderFunctionID,
					config.FilterRange,
				)
				if err != nil {
					return err
				}
			} else {
				logger.Info("target contract is already up to date")
			}

			return replay.Follow(
				ctx,
				logger,
				config.Verify,
				trpc,
				sourceEVMClient,
				targetEVMClient,
				config.SourceContractAddress,
				config.TargetContractAddress,
				config.TargetChainGateway,
				config.PrivateKey,
				config.HeaderRangeFunctionID,
				config.NextHeaderFunctionID,
				config.FilterRange,
			)
		},
	}

	cmd.SetHelpCommand(&cobra.Command{})

	return addFlags(cmd)
}
