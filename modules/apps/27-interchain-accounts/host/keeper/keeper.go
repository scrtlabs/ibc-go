package keeper

import (
	"fmt"
	"strings"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/ibc-go/v2/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v2/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v2/modules/core/24-host"
)

// Keeper defines the IBC interchain accounts host keeper
type Keeper struct {
	storeKey   sdk.StoreKey
	cdc        codec.BinaryCodec
	paramSpace paramtypes.Subspace

	channelKeeper icatypes.ChannelKeeper
	portKeeper    icatypes.PortKeeper
	accountKeeper icatypes.AccountKeeper

	scopedKeeper capabilitykeeper.ScopedKeeper

	msgRouter *baseapp.MsgServiceRouter
}

// NewKeeper creates a new interchain accounts host Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec, key sdk.StoreKey, paramSpace paramtypes.Subspace,
	channelKeeper icatypes.ChannelKeeper, portKeeper icatypes.PortKeeper,
	accountKeeper icatypes.AccountKeeper, scopedKeeper capabilitykeeper.ScopedKeeper, msgRouter *baseapp.MsgServiceRouter,
) Keeper {

	// ensure ibc interchain accounts module account is set
	if addr := accountKeeper.GetModuleAddress(icatypes.ModuleName); addr == nil {
		panic("the Interchain Accounts module account has not been set")
	}

	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		storeKey:      key,
		cdc:           cdc,
		paramSpace:    paramSpace,
		channelKeeper: channelKeeper,
		portKeeper:    portKeeper,
		accountKeeper: accountKeeper,
		scopedKeeper:  scopedKeeper,
		msgRouter:     msgRouter,
	}
}

// Logger returns the application logger, scoped to the associated module
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s-%s", host.ModuleName, icatypes.ModuleName))
}

// BindPort stores the provided portID and binds to it, returning the associated capability
func (k Keeper) BindPort(ctx sdk.Context, portID string) *capabilitytypes.Capability {
	store := ctx.KVStore(k.storeKey)
	store.Set(icatypes.KeyPort(portID), []byte{0x01})

	return k.portKeeper.BindPort(ctx, portID)
}

// IsBound checks if the interchain account host module is already bound to the desired port
func (k Keeper) IsBound(ctx sdk.Context, portID string) bool {
	_, ok := k.scopedKeeper.GetCapability(ctx, host.PortPath(portID))
	return ok
}

// AuthenticateCapability wraps the scopedKeeper's AuthenticateCapability function
func (k Keeper) AuthenticateCapability(ctx sdk.Context, cap *capabilitytypes.Capability, name string) bool {
	return k.scopedKeeper.AuthenticateCapability(ctx, cap, name)
}

// ClaimCapability wraps the scopedKeeper's ClaimCapability function
func (k Keeper) ClaimCapability(ctx sdk.Context, cap *capabilitytypes.Capability, name string) error {
	return k.scopedKeeper.ClaimCapability(ctx, cap, name)
}

// GetActiveChannelID retrieves the active channelID from the store keyed by the provided portID
func (k Keeper) GetActiveChannelID(ctx sdk.Context, portID string) (string, bool) {
	store := ctx.KVStore(k.storeKey)
	key := icatypes.KeyActiveChannel(portID)

	if !store.Has(key) {
		return "", false
	}

	return string(store.Get(key)), true
}

// GetAllActiveChannels returns a list of all active interchain accounts host channels and their associated port identifiers
func (k Keeper) GetAllActiveChannels(ctx sdk.Context) []icatypes.ActiveChannel {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, []byte(icatypes.ActiveChannelKeyPrefix))
	defer iterator.Close()

	var activeChannels []icatypes.ActiveChannel
	for ; iterator.Valid(); iterator.Next() {
		keySplit := strings.Split(string(iterator.Key()), "/")

		ch := icatypes.ActiveChannel{
			PortId:    keySplit[1],
			ChannelId: string(iterator.Value()),
		}

		activeChannels = append(activeChannels, ch)
	}

	return activeChannels
}

// SetActiveChannelID stores the active channelID, keyed by the provided portID
func (k Keeper) SetActiveChannelID(ctx sdk.Context, portID, channelID string) {
	store := ctx.KVStore(k.storeKey)
	store.Set(icatypes.KeyActiveChannel(portID), []byte(channelID))
}

// DeleteActiveChannelID removes the active channel keyed by the provided portID stored in state
func (k Keeper) DeleteActiveChannelID(ctx sdk.Context, portID string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(icatypes.KeyActiveChannel(portID))
}

// IsActiveChannel returns true if there exists an active channel for the provided portID, otherwise false
func (k Keeper) IsActiveChannel(ctx sdk.Context, portID string) bool {
	_, ok := k.GetActiveChannelID(ctx, portID)
	return ok
}

// GetInterchainAccountAddress retrieves the InterchainAccount address from the store keyed by the provided portID
func (k Keeper) GetInterchainAccountAddress(ctx sdk.Context, portID string) (string, bool) {
	store := ctx.KVStore(k.storeKey)
	key := icatypes.KeyOwnerAccount(portID)

	if !store.Has(key) {
		return "", false
	}

	return string(store.Get(key)), true
}

// GetAllInterchainAccounts returns a list of all registered interchain account addresses and their associated controller port identifiers
func (k Keeper) GetAllInterchainAccounts(ctx sdk.Context) []icatypes.RegisteredInterchainAccount {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, []byte(icatypes.OwnerKeyPrefix))

	var interchainAccounts []icatypes.RegisteredInterchainAccount
	for ; iterator.Valid(); iterator.Next() {
		keySplit := strings.Split(string(iterator.Key()), "/")

		acc := icatypes.RegisteredInterchainAccount{
			PortId:         keySplit[1],
			AccountAddress: string(iterator.Value()),
		}

		interchainAccounts = append(interchainAccounts, acc)
	}

	return interchainAccounts
}

// SetInterchainAccountAddress stores the InterchainAccount address, keyed by the associated portID
func (k Keeper) SetInterchainAccountAddress(ctx sdk.Context, portID string, address string) {
	store := ctx.KVStore(k.storeKey)
	store.Set(icatypes.KeyOwnerAccount(portID), []byte(address))
}

// NegotiateAppVersion handles application version negotation for the IBC interchain accounts module
func (k Keeper) NegotiateAppVersion(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionID string,
	portID string,
	counterparty channeltypes.Counterparty,
	proposedVersion string,
) (string, error) {
	if proposedVersion != icatypes.VersionPrefix {
		return "", sdkerrors.Wrapf(icatypes.ErrInvalidVersion, "failed to negotiate app version: expected %s, got %s", icatypes.VersionPrefix, proposedVersion)
	}

	moduleAccAddr := k.accountKeeper.GetModuleAddress(icatypes.ModuleName)
	accAddr := icatypes.GenerateAddress(moduleAccAddr, counterparty.PortId)

	return icatypes.NewAppVersion(icatypes.VersionPrefix, accAddr.String()), nil
}
