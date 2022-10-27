package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
)

var _ types.IbcTransferHooks = MultiIbcTransferHooks{}

// MultiIbcTransferHooks combine multiple evm hooks, all hook functions are run in array sequence
type MultiIbcTransferHooks []types.IbcTransferHooks

// NewMultiIbcTransferHooks combine multiple evm hooks
func NewMultiIbcTransferHooks(hooks ...types.IbcTransferHooks) MultiIbcTransferHooks {
	return hooks
}

// AfterRecvPacket delegate the call to underlying hooks
func (mh MultiIbcTransferHooks) AfterRecvPacket(ctx sdk.Context, reciver sdk.AccAddress, voucher sdk.Coin) error {
	for i := range mh {
		if err := mh[i].AfterRecvPacket(ctx, reciver, voucher); err != nil {
			return sdkerrors.Wrapf(err, "EVM hook %T failed", mh[i])
		}
	}
	return nil
}
