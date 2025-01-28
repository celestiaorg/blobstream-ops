# blobstream-ops

This repo contains a set of Blobstream related tooling.

Note: currently, this repo only supports BlobstreamX deployments. If there is a need to support SP1 Blobstream, please reach out to the team.

## Install

1. [Install Go](https://go.dev/doc/install) 1.22
2. Clone this repo
3. Install the Blobstream-ops CLI

 ```shell
make install
```

## Usage

```sh
# Print help
blobstream-ops --help
```

## BlobstreamX contract verification

One of the tools that the blobstream-ops CLI provides is the `verify` subcommand. It allows verifying that a BlobstreamX deployment is valid.
It works by taking a trusted Celestia RPC endpoints. Then, it goes over the data root tuple roots stored in the contract, and regenerates the
data commitment using the provided trusted RPC endpoint. Finally, it compares them and errors if any of the data root tuple roots is not valid.

Since Blobstream makes a 2/3rd honest validator set assumption, if more than 2/3rd of the validator set is malicious, they can commit to an invalid
data root tuple root. This tool allows verifying that that didn't happen using a trusted RPC endpoint.

### Verification command usage

After installing the blobstream-ops CLI, you can verify the contract using the following subcommand:

```shell
blobstream-ops verify contract <args>
```

The arguments can be provided either through the CLI, or more easily using environment variables.

The `.env.verify.example` contains an example set of how to provide the environment variables required for the verification.

First, populate the empty fields with the correct values. Then, run:

```shell
set -a
source .env
```

given that `.env` is the file created by populating the `.env.verify.example` file.

Then, run the CLI using:

```shell
blobstream-ops verify contract
```

And you should see the contract verification underway.

## Blobstream proofs replay

The replay command allows replaying proofs from an existing BlobstreamX deployment to a new one, in a different chain, without having to regenerate them.
This reduces the cost of maintaining the BlobstreamX deployment while keeping the same security properties.

### Pros

- Cheaper deployment
- Porting the BlobstreamX deployment to any new chain without having to run an operator

### Cons

- The ranges of blocks proven by proof will be the same between the existing and the new deployment
- Requires the contract to be initialized to a trusted header that is the `start_block` or any event of the existing deployment
- Adds dependency on the existing deployment. If it goes down, the new deployment also goes down.

### Requirements

To use the replay command, the whole BlobstreamX stack needs to be already deployed on the new chain.
Refer to [docs](https://docs.celestia.org/how-to-guides/blobstreamx#deploy-blobstream-x).

Also, make sure the trusted block used to initialise the BlobstreamX contract corresponds to a `start_block`
in the existing BlobstreamX deployment. Otherwise, the proofs will not be able to be relayed.

### Replay command usage

After installing the blobstream-ops CLI, you can use the replay tool using the following subcommand:

```shell
blobstream-ops replay --help
```

Similar to the above `verify` command, the arguments can be provided either using the CLI or through environment
variables.

To this matter, the `.env.replay.example` provides an example set of environment variables that need to be set in order
for the replay command to run.

First, populate the empty fields with the correct values. Then, run:

```shell
set -a
source .env
```

given that `.env` is the file created by populating the `.env.replay.example` file.

Then, run the CLI using:

```shell
blobstream-ops replay
```

And you should see the proofs being queried from the existing deployment and replayed in the new one.

## Contributing

### Tools

1. Install [golangci-lint](https://golangci-lint.run/welcome/install/)
2. Install [markdownlint](https://github.com/DavidAnson/markdownlint)

### Helpful Commands

```sh
# Build a new blobstream-ops binary and output to build/blobstream-ops
make build

# Run tests
make test

# Format code with linters (this assumes golangci-lint and markdownlint are installed)
make fmt
```

## Useful links

The Blobstream documentation is in [docs](https://docs.celestia.org/how-to-guides/blobstream).

The smart contract implementation is in [blobstream-contracts](https://github.com/celestiaorg/blobstream-contracts).

The BlobstreamX implementation is in [BlobstreamX repo](https://github.com/succinctlabs/blobstreamx).

Blobstream ADRs are in the [docs](https://github.com/celestiaorg/celestia-app/tree/main/docs/architecture).

Blobstream design explained in this [blog](https://blog.celestia.org/celestiums).
