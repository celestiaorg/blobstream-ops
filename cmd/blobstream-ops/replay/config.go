package replay

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"

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

	FlagVerify = "verify"

	FlagLogLevel  = "log.level"
	FlagLogFormat = "log.format"

	FlagCoreRPC = "core.rpc"
)

func addFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(FlagSourceEVMRPC, "http://localhost:8545", "Specify the Ethereum rpc address of the source EVM chain")
	cmd.Flags().String(FlagTargetEVMRPC, "http://localhost:8545", "Specify the Ethereum rpc address of the target EVM chain")
	cmd.Flags().String(FlagSourceEVMContractAddress, "", "Specify the source contract at which the source BlobstreamX contract is deployed")
	cmd.Flags().String(FlagTargetEVMContractAddress, "", "Specify the target contract at which the target BlobstreamX contract is deployed")
	cmd.Flags().String(FlagTargetChainGateway, "", "Specify the target chain succinct gateway contract address")
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
	cmd.Flags().Bool(FlagVerify, false, "Set to verify the commitments before replaying their proofs. Require the core rpc flag to be set")
	cmd.Flags().String(FlagEVMPrivateKey, "", "Specify the EVM private key, in hex format without the leading 0x, to use for replaying transaction in the target chain. Corresponding account should be funded")
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
}

func (cfg Config) ValidateBasics() error {
	if err := ValidateEVMAddress(cfg.SourceContractAddress); err != nil {
		return fmt.Errorf("%s: flag --%s", err.Error(), FlagSourceEVMContractAddress)
	}
	if err := ValidateEVMAddress(cfg.TargetContractAddress); err != nil {
		return fmt.Errorf("%s: flag --%s", err.Error(), FlagTargetEVMContractAddress)
	}
	if err := ValidateEVMAddress(cfg.TargetChainGateway); err != nil {
		return fmt.Errorf("%s: flag --%s", err.Error(), FlagTargetChainGateway)
	}
	if cfg.Verify && cfg.CoreRPC == "" {
		return fmt.Errorf("flag --%s is set but the core RPC flag --%s is not set", FlagVerify, FlagCoreRPC)
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

func parseFlags(cmd *cobra.Command) (Config, error) {
	// TODO add support for env variables
	sourceContractAddress, err := cmd.Flags().GetString(FlagSourceEVMContractAddress)
	if err != nil {
		return Config{}, err
	}

	targetContractAddress, err := cmd.Flags().GetString(FlagTargetEVMContractAddress)
	if err != nil {
		return Config{}, err
	}

	targetChainGateway, err := cmd.Flags().GetString(FlagTargetChainGateway)
	if err != nil {
		return Config{}, err
	}

	sourceEVMRPC, err := cmd.Flags().GetString(FlagSourceEVMRPC)
	if err != nil {
		return Config{}, err
	}

	targetEVMRPC, err := cmd.Flags().GetString(FlagTargetEVMRPC)
	if err != nil {
		return Config{}, err
	}

	coreRPC, err := cmd.Flags().GetString(FlagCoreRPC)
	if err != nil {
		return Config{}, err
	}

	logLevel, err := cmd.Flags().GetString(FlagLogLevel)
	if err != nil {
		return Config{}, err
	}

	logFormat, err := cmd.Flags().GetString(FlagLogFormat)
	if err != nil {
		return Config{}, err
	}

	rawPrivateKey, err := cmd.Flags().GetString(FlagEVMPrivateKey)
	if err != nil {
		return Config{}, err
	}
	if rawPrivateKey == "" {
		return Config{}, fmt.Errorf("please set the private key --%s", FlagEVMPrivateKey)
	}
	privateKey, err := crypto.HexToECDSA(rawPrivateKey)
	if err != nil {
		return Config{}, fmt.Errorf("failed to hex-decode Ethereum ECDSA Private Key: %w", err)
	}

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
	}, nil
}