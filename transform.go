package statediff

//go:generate go run ./types/gen ./types

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	hamt "github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/statediff/types"

	"github.com/filecoin-project/lotus/lib/blockstore"

	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// LotusType represents known types
type LotusType string

// LotusType enum
const (
	LotusTypeTipset                            LotusType = "tipset"
	LotusTypeStateroot                         LotusType = "stateRoot"
	AccountActorState                          LotusType = "accountActor"
	CronActorState                             LotusType = "cronActor"
	InitActorState                             LotusType = "initActor"
	InitActorAddresses                         LotusType = "initActorAddresses"
	MarketActorState                           LotusType = "storageMarketActor"
	MarketActorProposals                       LotusType = "storageMarketActor.Proposals"
	MarketActorStates                          LotusType = "storageMarketActor.States"
	MarketActorPendingProposals                LotusType = "storageMarketActor.PendingProposals"
	MarketActorEscrowTable                     LotusType = "storageMarketActor.EscrowTable"
	MarketActorLockedTable                     LotusType = "storageMarketActor.LockedTable"
	MarketActorDealOpsByEpoch                  LotusType = "storageMarketActor.DealOpsByEpoch"
	MultisigActorState                         LotusType = "multisigActor"
	MultisigActorPending                       LotusType = "multisigActor.PendingTxns"
	StorageMinerActorState                     LotusType = "storageMinerActor"
	StorageMinerActorInfo                      LotusType = "storageMinerActor.Info"
	StorageMinerActorVestingFunds              LotusType = "storageMinerActor.VestingFunds"
	StorageMinerActorPreCommittedSectors       LotusType = "storageMinerActor.PreCommittedSectors"
	StorageMinerActorPreCommittedSectorsExpiry LotusType = "storageMinerActor.PreCommittedSectorsExpiry"
	StorageMinerActorAllocatedSectors          LotusType = "storageMinerActor.AllocatedSectors"
	StorageMinerActorSectors                   LotusType = "storageMinerActor.Sectors"
	StorageMinerActorDeadlines                 LotusType = "storageMinerActor.Deadlines"
	StorageMinerActorDeadline                  LotusType = "storageMinerActor.Deadlines.Due"
	StorageMinerActorDeadlinePartitions        LotusType = "storageMinerActor.Deadlines.Due.Partitions"
	StorageMinerActorDeadlinePartitionExpiry   LotusType = "storageMinerActor.Deadlines.Due.Partitions.ExpirationsEpochs"
	StorageMinerActorDeadlinePartitionEarly    LotusType = "storageMinerActor.Deadlines.Due.Partitions.EarlyTerminated"
	StorageMinerActorDeadlineExpiry            LotusType = "storageMinerActor.Deadlines.Due.ExpirationsEpochs"
	StoragePowerActorState                     LotusType = "storagePowerActor"
	StoragePowerActorCronEventQueue            LotusType = "storagePowerCronEventQueue"
	StoragePowerActorClaims                    LotusType = "storagePowerClaims"
	RewardActorState                           LotusType = "rewardActor"
	VerifiedRegistryActorState                 LotusType = "verifiedRegistryActor"
	VerifiedRegistryActorVerifiers             LotusType = "verifiedRegistryActor.Verifiers"
	VerifiedRegistryActorVerifiedClients       LotusType = "verifiedRegistryActor.VerifiedClients"
	PaymentChannelActorState                   LotusType = "paymentChannelActor"
	PaymentChannelActorLaneStates              LotusType = "paymentChannelActor.LaneStates"
)

// LotusTypeAliases lists non-direct mapped aliases
var LotusTypeAliases = map[string]LotusType{
	"tipset.ParentStateRoot":                                      LotusTypeStateroot,
	"initActor.AddressMap":                                        InitActorAddresses,
	"storagePowerActor.CronEventQueue":                            StoragePowerActorCronEventQueue,
	"storagePowerActor.Claims":                                    StoragePowerActorClaims,
	"storageMinerActor.Deadlines.Due.ExpirationEpochs":            StorageMinerActorDeadlineExpiry,
	"storageMinerActor.Deadlines.Due.Partitions.ExpirationEpochs": StorageMinerActorDeadlinePartitionExpiry,
}

// LotusActorCodes for v0 actor states
var LotusActorCodes = map[string]LotusType{
	"bafkqaddgnfwc6mjpon4xg5dfnu":                 LotusType("systemActor"),
	"bafkqactgnfwc6mjpnfxgs5a":                    InitActorState,
	"bafkqaddgnfwc6mjpojsxoylsmq":                 RewardActorState,
	"bafkqactgnfwc6mjpmnzg63q":                    CronActorState,
	"bafkqaetgnfwc6mjpon2g64tbm5sxa33xmvza":       StoragePowerActorState,
	"bafkqae3gnfwc6mjpon2g64tbm5sw2ylsnnsxi":      MarketActorState,
	"bafkqaftgnfwc6mjpozsxe2lgnfswi4tfm5uxg5dspe": VerifiedRegistryActorState,
	"bafkqadlgnfwc6mjpmfrwg33vnz2a":               AccountActorState,
	"bafkqadtgnfwc6mjpnv2wy5djonuwo":              MultisigActorState,
	"bafkqafdgnfwc6mjpobqxs3lfnz2gg2dbnzxgk3a":    PaymentChannelActorState,
	"bafkqaetgnfwc6mjpon2g64tbm5sw22lomvza":       StorageMinerActorState,
}

// LotusPrototypes provide expected node types for each state type.
var LotusPrototypes = map[LotusType]ipld.NodePrototype{
	LotusTypeTipset:                   types.Type.LotusBlockHeader__Repr,
	AccountActorState:                 types.Type.AccountV0State__Repr,
	CronActorState:                    types.Type.CronV0State__Repr,
	InitActorState:                    types.Type.InitV0State__Repr,
	MarketActorState:                  types.Type.MarketV0State__Repr,
	MultisigActorState:                types.Type.MultisigV0State__Repr,
	StorageMinerActorState:            types.Type.MinerV0State__Repr,
	StorageMinerActorInfo:             types.Type.MinerV0Info__Repr,
	StorageMinerActorVestingFunds:     types.Type.MinerV0VestingFunds__Repr,
	StorageMinerActorAllocatedSectors: types.Type.BitField__Repr,
	StorageMinerActorDeadlines:        types.Type.MinerV0Deadlines__Repr,
	StorageMinerActorDeadline:         types.Type.MinerV0Deadline__Repr,
	StoragePowerActorState:            types.Type.PowerV0State__Repr,
	RewardActorState:                  types.Type.RewardV0State__Repr,
	VerifiedRegistryActorState:        types.Type.VerifregV0State__Repr,
	PaymentChannelActorState:          types.Type.PaychV0State__Repr,
	// Complex types
	LotusTypeStateroot:                         types.Type.Map__LotusActors__Repr,
	InitActorAddresses:                         types.Type.Map__ActorID__Repr,
	StorageMinerActorPreCommittedSectors:       types.Type.Map__SectorPreCommitOnChainInfo__Repr,
	StorageMinerActorDeadlinePartitionEarly:    types.Type.Map__BitField__Repr,
	StorageMinerActorPreCommittedSectorsExpiry: types.Type.Map__BitField__Repr,
	StorageMinerActorSectors:                   types.Type.Map__SectorOnChainInfo__Repr,
	StorageMinerActorDeadlinePartitions:        types.Type.Map__MinerV0Partition__Repr,
	StorageMinerActorDeadlinePartitionExpiry:   types.Type.Map__MinerV0ExpirationSet__Repr,
	StorageMinerActorDeadlineExpiry:            types.Type.Map__BitField__Repr,
	StoragePowerActorCronEventQueue:            types.Type.Map__PowerV0CronEvent__Repr,
	StoragePowerActorClaims:                    types.Type.Map__PowerV0Claim__Repr,
	VerifiedRegistryActorVerifiers:             types.Type.Map__DataCap__Repr,
	VerifiedRegistryActorVerifiedClients:       types.Type.Map__DataCap__Repr,
	MarketActorPendingProposals:                types.Type.Map__MarketV0DealProposal__Repr,
	MarketActorProposals:                       types.Type.Map__MarketV0RawDealProposal__Repr,
	MarketActorStates:                          types.Type.Map__MarketV0DealState__Repr,
	MarketActorEscrowTable:                     types.Type.Map__BalanceTable__Repr,
	MarketActorLockedTable:                     types.Type.Map__BalanceTable__Repr,
	MarketActorDealOpsByEpoch:                  types.Type.Map__List__DealID__Repr,
	MultisigActorPending:                       types.Type.Map__MultisigV0Transaction__Repr,
	PaymentChannelActorLaneStates:              types.Type.Map__PaychV0LaneState__Repr,
}

type Loader func(context.Context, cid.Cid, blockstore.Blockstore, ipld.NodeAssembler) error

var complexLoaders = map[ipld.NodePrototype]Loader{
	types.Type.Map__LotusActors__Repr:                transformStateRoot,
	types.Type.Map__ActorID__Repr:                    transformInitActor,
	types.Type.Map__SectorPreCommitOnChainInfo__Repr: transformMinerActorPreCommittedSectors,
	types.Type.Map__BitField__Repr:                   transformMinerActorBitfieldHamt,
	types.Type.Map__SectorOnChainInfo__Repr:          transformMinerActorSectors,
	types.Type.Map__MinerV0Partition__Repr:           transformMinerActorDeadlinePartitions,
	types.Type.Map__MinerV0ExpirationSet__Repr:       transformMinerActorDeadlinePartitionExpiry,
	types.Type.Map__PowerV0CronEvent__Repr:           transformPowerActorEventQueue,
	types.Type.Map__PowerV0Claim__Repr:               transformPowerActorClaims,
	types.Type.Map__DataCap__Repr:                    transformVerifiedRegistryDataCaps,
	types.Type.Map__MarketV0DealProposal__Repr:       transformMarketProposals,
	types.Type.Map__MarketV0RawDealProposal__Repr:    transformMarketPendingProposals,
	types.Type.Map__MarketV0DealState__Repr:          transformMarketStates,
	types.Type.Map__BalanceTable__Repr:               transformMarketBalanceTable,
	types.Type.Map__List__DealID__Repr:               transformMarketDealOpsByEpoch,
	types.Type.Map__MultisigV0Transaction__Repr:      transformMultisigPending,
	types.Type.Map__PaychV0LaneState__Repr:           transformPaymentChannelLaneStates,
}

var simplifyingRe = regexp.MustCompile(`\[\d+\]`)
var simplifyingRe2 = regexp.MustCompile(`\.\d+\.`)

// ResolveType maps incoming type strings to enum known types
func ResolveType(as string) LotusType {
	as = strings.ReplaceAll(as, string('/'), string('.'))
	as = string(simplifyingRe2.ReplaceAll(simplifyingRe.ReplaceAll([]byte(as), []byte("")), []byte(".")))
	if alias, ok := LotusTypeAliases[as]; ok {
		as = string(alias)
	}
	return LotusType(as)
}

func Load(ctx context.Context, c cid.Cid, store blockstore.Blockstore, into ipld.NodeAssembler) error {
	prototype := into.Prototype()
	if complexLoader, ok := complexLoaders[prototype]; ok {
		return complexLoader(ctx, c, store, into)
	}

	block, err := store.Get(c)
	if err != nil {
		return err
	}
	data := block.RawData()

	if err := dagcbor.Decoder(into, bytes.NewBuffer(data)); err != nil {
		return err
	}
	return nil
}

// Transform will unmarshal cbor data based on a provided type hint.
func Transform(ctx context.Context, c cid.Cid, store blockstore.Blockstore, as string) (ipld.Node, error) {
	proto, ok := LotusPrototypes[ResolveType(as)]
	if !ok {
		return nil, fmt.Errorf("unknown type: %s", as)
	}
	assembler := proto.NewBuilder()
	if err := Load(ctx, c, store, assembler); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformStateRoot(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		v, err := mapper.AssembleEntry(k)
		if err != nil {
			return err
		}

		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		actor := types.Type.LotusActors__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(asDef.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	if err := mapper.Finish(); err != nil {
		return err
	}
	return nil
}

func transformInitActor(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	var actorID cbg.CborInt
	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		v, err := mapper.AssembleEntry(k)
		if err != nil {
			return err
		}

		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}
		if err := cbor.DecodeInto(asDef.Raw, &actorID); err != nil {
			return err
		}
		return v.AssignInt(int(actorID))
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMinerActorPreCommittedSectors(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		i := big.NewInt(0)
		i.SetBytes([]byte(k))
		v, err := mapper.AssembleEntry(i.String())
		if err != nil {
			return err
		}

		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		actor := types.Type.MinerV0SectorPreCommitOnChainInfo__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(asDef.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMinerActorBitfieldHamt(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := CBORBytes{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		return v.AssignBytes(value)
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMinerActorSectors(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := cbg.Deferred{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		actor := types.Type.MinerV0SectorOnChainInfo__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(value.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMinerActorDeadlinePartitions(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := cbg.Deferred{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		actor := types.Type.MinerV0Partition__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(value.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMinerActorDeadlinePartitionExpiry(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := cbg.Deferred{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		actor := types.Type.MinerV0ExpirationSet__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(value.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformPowerActorEventQueue(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := adt.AsMultimap(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForAll(func(k string, val *adt.Array) error {
		bi := big.NewInt(0)
		bi.SetBytes([]byte(k))
		v, err := mapper.AssembleEntry(bi.String())
		if err != nil {
			return err
		}

		amt := types.Type.Map__PowerV0CronEvent__Repr.NewBuilder()
		amtM, err := amt.BeginMap(0)
		if err != nil {
			return err
		}

		var eval cbg.Deferred
		if err := val.ForEach(&eval, func(i int64) error {
			subv, err := amtM.AssembleEntry(fmt.Sprintf("%d", i))
			if err != nil {
				return err
			}

			actor := types.Type.PowerV0CronEvent__Repr.NewBuilder()
			if err := dagcbor.Decoder(actor, bytes.NewBuffer(eval.Raw)); err != nil {
				return err
			}
			return subv.AssignNode(actor.Build())
		}); err != nil {
			return err
		}
		if err := amtM.Finish(); err != nil {
			return err
		}
		return v.AssignNode(amt.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformPowerActorClaims(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		v, err := mapper.AssembleEntry(k)
		if err != nil {
			return err
		}

		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		actor := types.Type.PowerV0Claim__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(asDef.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformVerifiedRegistryDataCaps(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		v, err := mapper.AssembleEntry(k)
		if err != nil {
			return err
		}

		// Deferred parsing of big.Int
		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		return v.AssignBytes(asDef.Raw)
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMarketPendingProposals(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		v, err := mapper.AssembleEntry(k)
		if err != nil {
			return err
		}

		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		actor := types.Type.MarketV0DealProposal__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(asDef.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMarketProposals(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := cbg.Deferred{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		actor := types.Type.MarketV0DealProposal__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(value.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMarketStates(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := cbg.Deferred{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		actor := types.Type.MarketV0DealState__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(value.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMarketBalanceTable(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		v, err := mapper.AssembleEntry(k)
		if err != nil {
			return err
		}

		// Deferred parsing of big.Int
		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		return v.AssignBytes(asDef.Raw)
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformMarketDealOpsByEpoch(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	adtStore := adt.WrapStore(ctx, cbor.NewCborStore(store))
	table, err := adt.AsMap(adtStore, c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	var value cbg.CborCid
	if err := table.ForEach(&value, func(k string) error {
		set, err := adt.AsSet(adtStore, cid.Cid(value))
		if err != nil {
			return err
		}

		b := big.NewInt(0)
		b.SetBytes([]byte(k))
		v, err := mapper.AssembleEntry(b.String())
		if err != nil {
			return err
		}

		amt := types.Type.List__DealID__Repr.NewBuilder()
		amtL, err := amt.BeginList(0)
		if err != nil {
			return err
		}

		set.ForEach(func(d string) error {
			key, err := abi.ParseUIntKey(d)
			if err != nil {
				return err
			}
			return amtL.AssembleValue().AssignInt(int(key))
		})

		if err := amtL.Finish(); err != nil {
			return err
		}

		return v.AssignNode(amt.Build())
	}); err != nil {
		return err
	}

	return mapper.Finish()
}

func transformMultisigPending(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	if err := node.ForEach(ctx, func(k string, val interface{}) error {
		i := big.NewInt(0)
		i.SetBytes([]byte(k))
		v, err := mapper.AssembleEntry(i.String())
		if err != nil {
			return err
		}

		asDef, ok := val.(*cbg.Deferred)
		if !ok {
			return fmt.Errorf("unexpected non-cbg.Deferred")
		}

		actor := types.Type.MultisigV0Transaction__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(asDef.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}

func transformPaymentChannelLaneStates(ctx context.Context, c cid.Cid, store blockstore.Blockstore, assembler ipld.NodeAssembler) error {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return err
	}

	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return err
	}

	value := cbg.Deferred{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		actor := types.Type.PaychV0LaneState__Repr.NewBuilder()
		if err := dagcbor.Decoder(actor, bytes.NewBuffer(value.Raw)); err != nil {
			return err
		}
		return v.AssignNode(actor.Build())
	}); err != nil {
		return err
	}
	return mapper.Finish()
}
