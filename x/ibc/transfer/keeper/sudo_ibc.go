package keeper

import (
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"

	"github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
)

func (k Keeper) forceIBCTransfer(ctx sdk.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error) {
	if !strings.HasPrefix(msg.Token.Denom, "ibc/") {
		return nil, sdkerrors.ErrInvalidCoins.Wrap(
			"only remote tokens (ibc/..) are allowed",
		)
	}

	fullDenomPath, err := k.DenomPathFromHash(ctx, msg.Token.Denom)
	if err != nil {
		return nil, errorsmod.Wrapf(
			err, "failed to found denom path: %s", msg.Token.Denom,
		)
	}

	channelCap, ok := k.scopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(msg.SourcePort, msg.SourceChannel))
	if !ok {
		return nil, errorsmod.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}

	packetData := types.NewFungibleTokenPacketData(
		fullDenomPath, msg.Token.Amount.String(), msg.Sender, msg.Receiver, msg.Memo,
	)

	sequence, err := k.ics4Wrapper.SendPacket(ctx, channelCap, msg.SourcePort, msg.SourceChannel, msg.TimeoutHeight, msg.TimeoutTimestamp, packetData.GetBytes())
	if err != nil {
		return nil, err
	}

	return &types.MsgTransferResponse{Sequence: sequence}, nil
}
