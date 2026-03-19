package verify

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/cmdutil"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagEVMRPC             = "evm.rpc"
	FlagEVMContractAddress = "evm.contract-address"

	FlagLogLevel  = "log.level"
	FlagLogFormat = "log.format"

	FlagCoreRPC = "core.rpc"
)

func addStartFlags(cmd *cobra.Command) *cobra.Command {
	viper.AutomaticEnv()

	cmd.Flags().String(
		FlagEVMRPC,
		"http://localhost:8545",
		fmt.Sprintf("Specify the ethereum rpc address. Corresponding environment variable %s", cmdutil.ToEnvVariableFormat(FlagEVMRPC)),
	)
	cmdutil.BindFlagAndEnvVar(cmd, FlagEVMRPC)

	cmd.Flags().String(
		FlagEVMContractAddress,
		"",
		fmt.Sprintf("Specify the contract at which the BlobstreamX contract is deployed. Corresponding environment variable %s", cmdutil.ToEnvVariableFormat(FlagEVMContractAddress)),
	)
	cmdutil.BindFlagAndEnvVar(cmd, FlagEVMContractAddress)

	cmd.Flags().String(
		FlagLogLevel,
		"info",
		fmt.Sprintf("The logging level (trace|debug|info|warn|error|fatal|panic). Corresponding environment variable %s", cmdutil.ToEnvVariableFormat(FlagLogLevel)),
	)
	cmdutil.BindFlagAndEnvVar(cmd, FlagLogLevel)

	cmd.Flags().String(
		FlagLogFormat,
		"plain",
		fmt.Sprintf("The logging format (json|plain). Corresponding environment variable %s", cmdutil.ToEnvVariableFormat(FlagLogFormat)),
	)
	cmdutil.BindFlagAndEnvVar(cmd, FlagLogFormat)

	cmd.Flags().String(
		FlagCoreRPC,
		"tcp://localhost:26657",
		fmt.Sprintf("The celestia app rpc address. Corresponding environment variable %s", cmdutil.ToEnvVariableFormat(FlagCoreRPC)),
	)
	cmdutil.BindFlagAndEnvVar(cmd, FlagCoreRPC)

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

func parseStartFlags() (StartConfig, error) {
	contractAddress := viper.GetString(FlagEVMContractAddress)
	evmRPC := viper.GetString(FlagEVMRPC)
	coreRPC := viper.GetString(FlagCoreRPC)
	logLevel := viper.GetString(FlagLogLevel)
	logFormat := viper.GetString(FlagLogFormat)

	return StartConfig{
		EVMRPC:          evmRPC,
		ContractAddress: contractAddress,
		CoreRPC:         coreRPC,
		LogLevel:        logLevel,
		LogFormat:       logFormat,
	}, nil
}
