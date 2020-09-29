package statediff

//go:generate go run ./types/gen ./types

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"regexp"

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
	"tipset.ParentStateRoot":           LotusTypeStateroot,
	"initActor.AddressMap":             InitActorAddresses,
	"storagePowerActor.CronEventQueue": StoragePowerActorCronEventQueue,
	"storagePowerActor.Claims":         StoragePowerActorClaims,
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

var simplifyingRe = regexp.MustCompile(`\[\d+\]`)
var simplifyingRe2 = regexp.MustCompile(`\.\d+\.`)

// ResolveType maps incoming type strings to enum known types
func ResolveType(as string) LotusType {
	as = string(simplifyingRe2.ReplaceAll(simplifyingRe.ReplaceAll([]byte(as), []byte("")), []byte(".")))
	if alias, ok := LotusTypeAliases[as]; ok {
		as = string(alias)
	}
	return LotusType(as)
}

// Transform will unmarshal cbor data based on a provided type hint.
func Transform(ctx context.Context, c cid.Cid, store blockstore.Blockstore, as string) (ipld.Node, error) {
	// First select types which do their own store loading.
	switch ResolveType(as) {
	case LotusTypeStateroot:
		return transformStateRoot(ctx, c, store)
	case InitActorAddresses:
		return transformInitActor(ctx, c, store)
	case StorageMinerActorPreCommittedSectors:
		return transformMinerActorPreCommittedSectors(ctx, c, store)
	case StorageMinerActorDeadlinePartitionEarly:
		fallthrough
	case StorageMinerActorPreCommittedSectorsExpiry:
		return transformMinerActorPreCommittedSectorsExpiry(ctx, c, store)
	case StorageMinerActorSectors:
		return transformMinerActorSectors(ctx, c, store)
	case StorageMinerActorDeadlinePartitions:
		return transformMinerActorDeadlinePartitions(ctx, c, store)
	case StorageMinerActorDeadlinePartitionExpiry:
		return transformMinerActorDeadlinePartitionExpiry(ctx, c, store)
	case StorageMinerActorDeadlineExpiry:
		return transformMinerActorDeadlineExpiry(ctx, c, store)
	case StoragePowerActorCronEventQueue:
		return transformPowerActorEventQueue(ctx, c, store)
	case StoragePowerActorClaims:
		return transformPowerActorClaims(ctx, c, store)
	case MarketActorProposals:
		return transformMarketProposals(ctx, c, store)
	case MarketActorStates:
		return transformMarketStates(ctx, c, store)
	case MarketActorPendingProposals:
		return transformMarketPendingProposals(ctx, c, store)
	case MarketActorEscrowTable:
		fallthrough
	case MarketActorLockedTable:
		return transformMarketBalanceTable(ctx, c, store)
	case MarketActorDealOpsByEpoch:
		return transformMarketDealOpsByEpoch(ctx, c, store)
	case MultisigActorPending:
		return transformMultisigPending(ctx, c, store)
	case VerifiedRegistryActorVerifiers:
		fallthrough
	case VerifiedRegistryActorVerifiedClients:
		return transformVerifiedRegistryDataCaps(ctx, c, store)
	case PaymentChannelActorLaneStates:
		return transformPaymentChannelLaneStates(ctx, c, store)
	default:
	}

	block, err := store.Get(c)
	if err != nil {
		return nil, err
	}
	data := block.RawData()

	// Then select types which use block data.
	var assembler ipld.NodeBuilder
	switch ResolveType(as) {
	case LotusTypeTipset:
		assembler = types.Type.LotusBlockHeader__Repr.NewBuilder()
	case AccountActorState:
		assembler = types.Type.AccountV0State__Repr.NewBuilder()
	case CronActorState:
		assembler = types.Type.CronV0State__Repr.NewBuilder()
	case InitActorState:
		assembler = types.Type.InitV0State__Repr.NewBuilder()
	case MarketActorState:
		assembler = types.Type.MarketV0State__Repr.NewBuilder()
	case MultisigActorState:
		assembler = types.Type.MultisigV0State__Repr.NewBuilder()
	case StorageMinerActorState:
		assembler = types.Type.MinerV0State__Repr.NewBuilder()
	case StorageMinerActorInfo:
		assembler = types.Type.MinerV0Info__Repr.NewBuilder()
	case StorageMinerActorVestingFunds:
		assembler = types.Type.MinerV0VestingFunds__Repr.NewBuilder()
	case StorageMinerActorAllocatedSectors:
		assembler = types.Type.BitField__Repr.NewBuilder()
	case StorageMinerActorDeadlines:
		assembler = types.Type.MinerV0Deadlines__Repr.NewBuilder()
	case StorageMinerActorDeadline:
		assembler = types.Type.MinerV0Deadline__Repr.NewBuilder()
	case StoragePowerActorState:
		assembler = types.Type.PowerV0State__Repr.NewBuilder()
	case RewardActorState:
		assembler = types.Type.RewardV0State__Repr.NewBuilder()
	case VerifiedRegistryActorState:
		assembler = types.Type.VerifregV0State__Repr.NewBuilder()
	case PaymentChannelActorState:
		assembler = types.Type.PaychV0State__Repr.NewBuilder()
	default:
		return nil, fmt.Errorf("unknown type: %s", as)
	}

	if err := dagcbor.Decoder(assembler, bytes.NewBuffer(data)); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformStateRoot(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}
	assembler := types.Type.Map__LotusActors__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
	}

	node.ForEach(ctx, func(k string, val interface{}) error {
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
	})
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformInitActor(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}
	assembler := types.Type.Map__ActorID__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
	}

	var actorID cbg.CborInt
	node.ForEach(ctx, func(k string, val interface{}) error {
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
	})
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMinerActorPreCommittedSectors(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__SectorPreCommitOnChainInfo__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMinerActorPreCommittedSectorsExpiry(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__BitField__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
	}

	value := CBORBytes{}
	if err := list.ForEach(&value, func(k int64) error {
		v, err := mapper.AssembleEntry(fmt.Sprintf("%d", k))
		if err != nil {
			return err
		}

		return v.AssignBytes(value)
	}); err != nil {
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMinerActorSectors(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__SectorOnChainInfo__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMinerActorDeadlinePartitions(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__MinerV0Partition__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMinerActorDeadlinePartitionExpiry(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__MinerV0ExpirationSet__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMinerActorDeadlineExpiry(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	return transformMinerActorPreCommittedSectorsExpiry(ctx, c, store)
}

func transformPowerActorEventQueue(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := adt.AsMultimap(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}
	assembler := types.Type.Multimap__PowerV0CronEvent__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformPowerActorClaims(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__PowerV0Claim__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformVerifiedRegistryDataCaps(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__DataCap__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMarketPendingProposals(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__MarketV0DealProposal__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMarketProposals(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__MarketV0RawDealProposal__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMarketStates(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__MarketV0DealState__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMarketBalanceTable(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__BalanceTable__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMarketDealOpsByEpoch(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	adtStore := adt.WrapStore(ctx, cbor.NewCborStore(store))
	table, err := adt.AsMap(adtStore, c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__List__DealID__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformMultisigPending(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	node, err := hamt.LoadNode(ctx, cborStore, c, hamt.UseTreeBitWidth(5))
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__MultisigV0Transaction__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}

func transformPaymentChannelLaneStates(ctx context.Context, c cid.Cid, store blockstore.Blockstore) (ipld.Node, error) {
	cborStore := cbor.NewCborStore(store)
	list, err := adt.AsArray(adt.WrapStore(ctx, cborStore), c)
	if err != nil {
		return nil, err
	}

	assembler := types.Type.Map__PaychV0LaneState__Repr.NewBuilder()
	mapper, err := assembler.BeginMap(0)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := mapper.Finish(); err != nil {
		return nil, err
	}
	return assembler.Build(), nil
}
