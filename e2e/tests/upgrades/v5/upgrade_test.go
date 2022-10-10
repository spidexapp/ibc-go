package v5

import (
	"context"
	"os"
	"testing"

	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/v6/test"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/ibc-go/e2e/testconfig"
	"github.com/cosmos/ibc-go/e2e/tests/upgrades"
	"github.com/cosmos/ibc-go/e2e/testsuite"
	"github.com/cosmos/ibc-go/e2e/testvalues"
)

const (
	// CurrentVersion defines the pre-upgrade version tag
	CurrentVersion = "v4.0.0"
	// UpgradeVersion defines the post-upgrade version tag
	UpgradeVersion = "v5.0.0"
	// DefaultUpgradeName runs a no-op upgrade handler
	DefaultUpgradeName = "normal upgrade"
)

func TestUpgradeV5TestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeV5TestSuite))
}

type UpgradeV5TestSuite struct {
	testsuite.E2ETestSuite
}

// Name returns the chain upgrade name.
func (s *UpgradeV5TestSuite) Name() string {
	return DefaultUpgradeName
}

// HaltHeight returns the chain upgrade halt height.
func (s *UpgradeV5TestSuite) HaltHeight() uint64 {
	return upgrades.DefaultHaltHeight
}

// CurrentVersion returns the chain upgrade current version.
func (s *UpgradeV5TestSuite) CurrentVersion() string {
	return CurrentVersion
}

// UpgradeVersion returns the chain upgrade target version.
func (s *UpgradeV5TestSuite) UpgradeVersion() string {
	return UpgradeVersion
}

// GetDockerClient returns the upgrade suite docker clent.
func (s *UpgradeV5TestSuite) GetDockerClient() *dockerclient.Client {
	return s.DockerClient
}

func (s *UpgradeV5TestSuite) TestV4ToV5ChainUpgrade() {
	t := s.T()

	s.Require().NoError(os.Setenv(testconfig.ChainATagEnv, CurrentVersion))
	s.Require().NoError(os.Setenv(testconfig.ChainBTagEnv, CurrentVersion))

	ctx := context.Background()
	relayer, channelA := s.SetupChainsRelayerAndChannel(ctx)
	chainA, chainB := s.GetChains()

	var (
		chainADenom    = chainA.Config().Denom
		chainBIBCToken = testsuite.GetIBCToken(chainADenom, channelA.Counterparty.PortID, channelA.Counterparty.ChannelID) // IBC token sent to chainB

		chainBDenom    = chainB.Config().Denom
		chainAIBCToken = testsuite.GetIBCToken(chainBDenom, channelA.PortID, channelA.ChannelID) // IBC token sent to chainA
	)

	// create separate user specifically for the upgrade proposal to more easily verify starting
	// and end balances of the chainA users.
	chainAUpgradeProposalWallet := s.CreateUserOnChainA(ctx, testvalues.StartingTokenAmount)

	chainAWallet := s.CreateUserOnChainA(ctx, testvalues.StartingTokenAmount)
	chainAAddress := chainAWallet.Bech32Address(chainA.Config().Bech32Prefix)

	chainBWallet := s.CreateUserOnChainB(ctx, testvalues.StartingTokenAmount)
	chainBAddress := chainBWallet.Bech32Address(chainB.Config().Bech32Prefix)

	s.Require().NoError(test.WaitForBlocks(ctx, 1, chainA, chainB), "failed to wait for blocks")

	t.Run("native IBC token transfer from chainA to chainB, sender is source of tokens", func(t *testing.T) {
		transferTxResp, err := s.Transfer(ctx, chainA, chainAWallet, channelA.PortID, channelA.ChannelID, testvalues.DefaultTransferAmount(chainADenom), chainAAddress, chainBAddress, s.GetTimeoutHeight(ctx, chainB), 0)
		s.Require().NoError(err)
		s.AssertValidTxResponse(transferTxResp)
	})

	t.Run("tokens are escrowed", func(t *testing.T) {
		actualBalance, err := s.GetChainANativeBalance(ctx, chainAWallet)
		s.Require().NoError(err)

		expected := testvalues.StartingTokenAmount - testvalues.IBCTransferAmount
		s.Require().Equal(expected, actualBalance)
	})

	t.Run("start relayer", func(t *testing.T) {
		s.StartRelayer(relayer)
	})

	t.Run("packets are relayed", func(t *testing.T) {
		s.AssertPacketRelayed(ctx, chainA, channelA.PortID, channelA.ChannelID, 1)

		actualBalance, err := chainB.GetBalance(ctx, chainBAddress, chainBIBCToken.IBCDenom())
		s.Require().NoError(err)

		expected := testvalues.IBCTransferAmount
		s.Require().Equal(expected, actualBalance)
	})

	s.Require().NoError(test.WaitForBlocks(ctx, 5, chainA, chainB), "failed to wait for blocks")

	t.Run("upgrade chainA", func(t *testing.T) {
		upgrades.UpgradeChain(ctx, s, chainA, chainAUpgradeProposalWallet)
	})

	t.Run("restart relayer", func(t *testing.T) {
		s.StopRelayer(ctx, relayer)
		s.StartRelayer(relayer)
	})

	t.Run("native IBC token transfer from chainA to chainB, sender is source of tokens", func(t *testing.T) {
		transferTxResp, err := s.Transfer(ctx, chainA, chainAWallet, channelA.PortID, channelA.ChannelID, testvalues.DefaultTransferAmount(chainADenom), chainAAddress, chainBAddress, s.GetTimeoutHeight(ctx, chainB), 0)
		s.Require().NoError(err)
		s.AssertValidTxResponse(transferTxResp)
	})

	s.Require().NoError(test.WaitForBlocks(ctx, 5, chainA, chainB), "failed to wait for blocks")

	t.Run("packets are relayed", func(t *testing.T) {
		s.AssertPacketRelayed(ctx, chainA, channelA.PortID, channelA.ChannelID, 2)

		actualBalance, err := chainB.GetBalance(ctx, chainBAddress, chainBIBCToken.IBCDenom())
		s.Require().NoError(err)

		expected := testvalues.IBCTransferAmount * 2
		s.Require().Equal(expected, actualBalance)
	})

	t.Run("ensure packets can be received, send from chainB to chainA", func(t *testing.T) {
		t.Run("send from chainB to chainA", func(t *testing.T) {
			transferTxResp, err := s.Transfer(ctx, chainB, chainBWallet, channelA.Counterparty.PortID, channelA.Counterparty.ChannelID, testvalues.DefaultTransferAmount(chainBDenom), chainBAddress, chainAAddress, s.GetTimeoutHeight(ctx, chainA), 0)
			s.Require().NoError(err)
			s.AssertValidTxResponse(transferTxResp)
		})

		s.Require().NoError(test.WaitForBlocks(ctx, 5, chainA, chainB), "failed to wait for blocks")

		t.Run("packets are relayed", func(t *testing.T) {
			s.AssertPacketRelayed(ctx, chainA, channelA.Counterparty.PortID, channelA.Counterparty.ChannelID, 1)

			actualBalance, err := chainA.GetBalance(ctx, chainAAddress, chainAIBCToken.IBCDenom())
			s.Require().NoError(err)

			expected := testvalues.IBCTransferAmount
			s.Require().Equal(expected, actualBalance)
		})
	})
}
