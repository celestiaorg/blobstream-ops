package replay

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

const (
	FlagSourceEVMRPC             = "evm.source.rpc"
	FlagSourceEVMContractAddress = "evm.source.contract-address"
	FlagTargetEVMRPC             = "evm.target.rpc"
	FlagTargetEVMContractAddress = "evm.target.contract-address"
	FlagTargetChainGateway       = "evm.target.gateway"
	FlagEVMPrivateKey            = "evm.private-key"
	FlagEVMFilterRange           = "evm.filter-range"

	FlagHeaderRangeFunctionID = "circuits.header-range.functionID"
	FlagNextHeaderFunctionID  = "circuits.next-header.functionID"

	FlagVerify = "verify"

	FlagLogLevel  = "log.level"
	FlagLogFormat = "log.format"

	FlagCoreRPC = "core.rpc"
)

func addFlags(cmd *cobra.Command) *cobra.Command {
	viper.AutomaticEnv()

	cmd.Flags().String(
		FlagSourceEVMRPC,
		"http://localhost:8545",
		fmt.Sprintf("Specify the Ethereum rpc address of the source EVM chain. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagSourceEVMRPC)),
	)
	common.BindFlagAndEnvVar(cmd, FlagSourceEVMRPC)

	cmd.Flags().String(
		FlagTargetEVMRPC,
		"http://localhost:8545",
		fmt.Sprintf("Specify the Ethereum rpc address of the target EVM chain. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagTargetEVMRPC)),
	)
	common.BindFlagAndEnvVar(cmd, FlagTargetEVMRPC)

	cmd.Flags().String(
		FlagSourceEVMContractAddress,
		"",
		fmt.Sprintf("Specify the source contract at which the source BlobstreamX contract is deployed. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagSourceEVMContractAddress)),
	)
	common.BindFlagAndEnvVar(cmd, FlagSourceEVMContractAddress)

	cmd.Flags().String(
		FlagTargetEVMContractAddress,
		"",
		fmt.Sprintf("Specify the target contract at which the target BlobstreamX contract is deployed. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagTargetEVMContractAddress)),
	)
	common.BindFlagAndEnvVar(cmd, FlagTargetEVMContractAddress)

	cmd.Flags().String(
		FlagTargetChainGateway,
		"",
		fmt.Sprintf("Specify the target chain succinct gateway contract address. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagTargetChainGateway)),
	)
	common.BindFlagAndEnvVar(cmd, FlagTargetChainGateway)

	cmd.Flags().String(
		FlagLogLevel,
		"info",
		fmt.Sprintf("The logging level (trace|debug|info|warn|error|fatal|panic). Corresponding environment variable %s", common.ToEnvVariableFormat(FlagLogLevel)),
	)
	common.BindFlagAndEnvVar(cmd, FlagLogLevel)

	cmd.Flags().String(
		FlagLogFormat,
		"plain",
		fmt.Sprintf("The logging format (json|plain). Corresponding environment variable %s", common.ToEnvVariableFormat(FlagLogFormat)),
	)
	common.BindFlagAndEnvVar(cmd, FlagLogFormat)

	cmd.Flags().String(
		FlagCoreRPC,
		"tcp://localhost:26657",
		fmt.Sprintf("The celestia app rpc address. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagCoreRPC)),
	)
	common.BindFlagAndEnvVar(cmd, FlagCoreRPC)

	cmd.Flags().Bool(
		FlagVerify,
		false,
		fmt.Sprintf("Set to verify the commitments before replaying their proofs. Require the core rpc flag to be set. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagVerify)),
	)
	common.BindFlagAndEnvVar(cmd, FlagVerify)

	cmd.Flags().String(
		FlagEVMPrivateKey,
		"",
		fmt.Sprintf("Specify the EVM private key, in hex format, to use for replaying transaction in the target chain. Corresponding account should be funded. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagEVMPrivateKey)),
	)
	common.BindFlagAndEnvVar(cmd, FlagEVMPrivateKey)

	cmd.Flags().String(
		FlagHeaderRangeFunctionID,
		"",
		fmt.Sprintf("Specify the function ID of the header range circuit in the target BlobstreamX contract, in hex format. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagHeaderRangeFunctionID)),
	)
	common.BindFlagAndEnvVar(cmd, FlagHeaderRangeFunctionID)

	cmd.Flags().String(
		FlagNextHeaderFunctionID,
		"",
		fmt.Sprintf("Specify the function ID of the next header circuit in the target BlobstreamX contract, in hex format. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagNextHeaderFunctionID)),
	)
	common.BindFlagAndEnvVar(cmd, FlagNextHeaderFunctionID)

	cmd.Flags().Int64(
		FlagEVMFilterRange,
		5000,
		fmt.Sprintf("Specify the eth_getLogs filter range. Corresponding environment variable %s", common.ToEnvVariableFormat(FlagEVMFilterRange)),
	)
	common.BindFlagAndEnvVar(cmd, FlagEVMFilterRange)

	return cmd
}

type Config struct {
	SourceEVMRPC          string
	TargetEVMRPC          string
	SourceContractAddress string
	TargetContractAddress string
	TargetChainGateway    string
	LogLevel              string
	LogFormat             string
	CoreRPC               string
	Verify                bool
	PrivateKey            *ecdsa.PrivateKey
	HeaderRangeFunctionID [32]byte
	NextHeaderFunctionID  [32]byte
	FilterRange           int64
}

func (cfg Config) ValidateBasics() error {
	if err := ValidateEVMAddress(cfg.SourceContractAddress); err != nil {
		return fmt.Errorf(
			"%s: flag --%s or environment variable %s",
			err.Error(),
			FlagSourceEVMContractAddress,
			common.ToEnvVariableFormat(FlagSourceEVMContractAddress),
		)
	}
	if err := ValidateEVMAddress(cfg.TargetContractAddress); err != nil {
		return fmt.Errorf("%s: flag --%s or environment variable %s", err.Error(), FlagTargetEVMContractAddress, common.ToEnvVariableFormat(FlagTargetEVMContractAddress))
	}
	if err := ValidateEVMAddress(cfg.TargetChainGateway); err != nil {
		return fmt.Errorf("%s: flag --%s or environment variable %s", err.Error(), FlagTargetChainGateway, common.ToEnvVariableFormat(FlagTargetChainGateway))
	}
	if cfg.Verify && cfg.CoreRPC == "" {
		return fmt.Errorf("flag --%s is set but the core RPC flag --%s is not set. Please set --%s or environment variable %s", FlagVerify, FlagCoreRPC, FlagCoreRPC, common.ToEnvVariableFormat(FlagCoreRPC))
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

func parseFlags() (Config, error) {
	sourceContractAddress := viper.GetString(FlagSourceEVMContractAddress)

	targetContractAddress := viper.GetString(FlagTargetEVMContractAddress)

	targetChainGateway := viper.GetString(FlagTargetChainGateway)

	sourceEVMRPC := viper.GetString(FlagSourceEVMRPC)

	targetEVMRPC := viper.GetString(FlagTargetEVMRPC)

	coreRPC := viper.GetString(FlagCoreRPC)

	logLevel := viper.GetString(FlagLogLevel)

	logFormat := viper.GetString(FlagLogFormat)

	rawPrivateKey := viper.GetString(FlagEVMPrivateKey)
	if rawPrivateKey == "" {
		return Config{}, fmt.Errorf("please set the private key --%s or %s", FlagEVMPrivateKey, common.ToEnvVariableFormat(FlagEVMPrivateKey))
	}
	rawPrivateKey = strings.TrimPrefix(rawPrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(rawPrivateKey)
	if err != nil {
		return Config{}, fmt.Errorf("failed to hex-decode Ethereum ECDSA Private Key: %w", err)
	}

	strHeaderRange := viper.GetString(FlagHeaderRangeFunctionID)
	if strHeaderRange == "" {
		return Config{}, fmt.Errorf("please set the header range function ID --%s or %s", FlagHeaderRangeFunctionID, common.ToEnvVariableFormat(FlagHeaderRangeFunctionID))
	}
	strHeaderRange = strings.TrimPrefix(strHeaderRange, "0x")
	decodedHeaderRange, err := hex.DecodeString(strHeaderRange)
	if err != nil {
		return Config{}, err
	}
	var bzHeaderRange [32]byte
	copy(bzHeaderRange[:], decodedHeaderRange)

	strNextHeader := viper.GetString(FlagNextHeaderFunctionID)
	if strNextHeader == "" {
		return Config{}, fmt.Errorf("please set the next header function ID --%s or %s", FlagNextHeaderFunctionID, common.ToEnvVariableFormat(FlagNextHeaderFunctionID))
	}
	strNextHeader = strings.TrimPrefix(strNextHeader, "0x")
	decodedNextHeader, err := hex.DecodeString(strNextHeader)
	if err != nil {
		return Config{}, err
	}
	var bzNextHeader [32]byte
	copy(bzNextHeader[:], decodedNextHeader)

	filterRange := viper.GetInt64(FlagEVMFilterRange)

	verify := viper.GetBool(FlagVerify)

	// TODO add rate limiting flag
	// TODO add gas price multiplier flag
	return Config{
		SourceEVMRPC:          sourceEVMRPC,
		TargetEVMRPC:          targetEVMRPC,
		SourceContractAddress: sourceContractAddress,
		TargetContractAddress: targetContractAddress,
		TargetChainGateway:    targetChainGateway,
		CoreRPC:               coreRPC,
		LogLevel:              logLevel,
		LogFormat:             logFormat,
		PrivateKey:            privateKey,
		NextHeaderFunctionID:  bzNextHeader,
		HeaderRangeFunctionID: bzHeaderRange,
		FilterRange:           filterRange,
		Verify:                verify,
	}, nil
}
