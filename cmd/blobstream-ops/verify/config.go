package verify

import (
	"errors"
	"fmt"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

const (
	FlagEVMRPC             = "evm.rpc"
	FlagEVMContractAddress = "evm.contract-address"

	FlagLogLevel  = "log.level"
	FlagLogFormat = "log.format"

	FlagCoreRPC = "core.rpc"
)

func addStartFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(FlagEVMRPC, "http://localhost:8545", "Specify the ethereum rpc address")
	cmd.Flags().String(FlagEVMContractAddress, "", "Specify the contract at which the BlobstreamX contract is deployed")
	cmd.Flags().String(
		FlagLogLevel,
		"info",
		"The logging level (trace|debug|info|warn|error|fatal|panic)",
	)
	cmd.Flags().String(
		FlagLogFormat,
		"plain",
		"The logging format (json|plain)",
	)
	cmd.Flags().String(
		FlagCoreRPC,
		"tcp://localhost:26657",
		"The celestia app rpc address",
	)
	return cmd
}

type StartConfig struct {
	EVMRPC          string
	ContractAddress string
	LogLevel        string
	LogFormat       string
	CoreRPC         string
}

func (cfg StartConfig) ValidateBasics() error {
	if err := ValidateEVMAddress(cfg.ContractAddress); err != nil {
		return fmt.Errorf("%s: flag --%s", err.Error(), FlagEVMContractAddress)
	}
	return nil
}

func ValidateEVMAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("the EVM address cannot be empty")
	}
	if !ethcmn.IsHexAddress(addr) {
		return errors.New("valid EVM address is required")
	}
	return nil
}

func parseStartFlags(cmd *cobra.Command) (StartConfig, error) {
	contractAddress, err := cmd.Flags().GetString(FlagEVMContractAddress)
	if err != nil {
		return StartConfig{}, err
	}

	evmRPC, err := cmd.Flags().GetString(FlagEVMRPC)
	if err != nil {
		return StartConfig{}, err
	}

	coreRPC, err := cmd.Flags().GetString(FlagCoreRPC)
	if err != nil {
		return StartConfig{}, err
	}

	logLevel, err := cmd.Flags().GetString(FlagLogLevel)
	if err != nil {
		return StartConfig{}, err
	}

	logFormat, err := cmd.Flags().GetString(FlagLogFormat)
	if err != nil {
		return StartConfig{}, err
	}

	return StartConfig{
		EVMRPC:          evmRPC,
		ContractAddress: contractAddress,
		CoreRPC:         coreRPC,
		LogLevel:        logLevel,
		LogFormat:       logFormat,
	}, nil
}
