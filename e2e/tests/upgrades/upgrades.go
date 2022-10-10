package upgrades

import (
	"context"
	"fmt"
	"time"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/test"
)

const (
	// DefaultHaltHeight is the height at which the upgrade takes place.
	DefaultHaltHeight = uint64(100)
)

// ChainUpgradeSuite defines flexible interface interface necessary to perform a successful chain upgrade procedure.
type ChainUpgradeSuite interface {
	// Name returns the chain upgrade name.
	Name() string

	// HaltHeight returns the chain upgrade halt height.
	HaltHeight() uint64

	// CurrentVersion returns the chain upgrade current version.
	CurrentVersion() string

	// UpgradeVersion returns the chain upgrade target version.
	UpgradeVersion() string

	// ExecuteGovProposal submits the given governance proposal using the provided user and uses all validators to vote yes on the proposal.
	// It ensures the proposal successfully passes.
	ExecuteGovProposal(ctx context.Context, chain *cosmos.CosmosChain, user *ibc.Wallet, content govtypes.Content)

	// GetDockerClient returns the upgrade suite docker clent.
	GetDockerClient() *dockerclient.Client
}

// UpgradeChain upgrades a chain to a specific version using the planName provided.
// The software upgrade proposal is broadcast by the provided wallet.
func UpgradeChain(ctx context.Context, upgradeSuite ChainUpgradeSuite, chain *cosmos.CosmosChain, wallet *ibc.Wallet) error {
	plan := upgradetypes.Plan{
		Name:   upgradeSuite.Name(),
		Height: int64(upgradeSuite.HaltHeight()),
		Info:   fmt.Sprintf("upgrade version test from %s to %s", upgradeSuite.CurrentVersion(), upgradeSuite.UpgradeVersion()),
	}

	upgradeProposal := upgradetypes.NewSoftwareUpgradeProposal(fmt.Sprintf("upgrade from %s to %s", upgradeSuite.CurrentVersion(), upgradeSuite.UpgradeVersion()), "upgrade chain E2E test", plan)
	upgradeSuite.ExecuteGovProposal(ctx, chain, wallet, upgradeProposal)

	height, err := chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("error fetching height: %w", err)
	}

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, time.Minute*2)
	defer timeoutCtxCancel()

	err = test.WaitForBlocks(timeoutCtx, int(upgradeSuite.HaltHeight()-height)+1, chain)
	if err != nil {
		return fmt.Errorf("chain did not halt at halt height: %w", err)
	}

	err = chain.StopAllNodes(ctx)
	if err != nil {
		return fmt.Errorf("error stopping node(s): %w", err)
	}

	chain.UpgradeVersion(ctx, upgradeSuite.GetDockerClient(), upgradeSuite.UpgradeVersion())

	err = chain.StartAllNodes(ctx)
	if err != nil {
		return fmt.Errorf("error starting upgraded node(s): %w", err)
	}

	timeoutCtx, timeoutCtxCancel = context.WithTimeout(ctx, time.Minute*2)
	defer timeoutCtxCancel()

	err = test.WaitForBlocks(timeoutCtx, 10, chain)
	if err != nil {
		return fmt.Errorf("chain did not produce blocks after upgrade: %w", err)
	}

	height, err = chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("error fetching height after upgrade: %w", err)
	}

	if height <= upgradeSuite.HaltHeight() {
		// suite.Require().GreaterOrEqual(height, haltHeight+blocksAfterUpgrade, "height did not increment enough after upgrade")
		return fmt.Errorf("height did not increment enough after upgrade")
	}

	return nil
}
