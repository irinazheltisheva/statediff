package v0

import (
	"github.com/ipld/go-ipld-prime/schema"
)

func accumulateMultisig(ts schema.TypeSystem) {
	ts.Accumulate(schema.SpawnStruct("MultisigV0State",
		[]schema.StructField{
			schema.SpawnStructField("Signers", "List__Address", false, false),
			schema.SpawnStructField("NumApprovalsThreshold", "Int", false, false),
			schema.SpawnStructField("NextTxnID", "MultisigV0TxnID", false, false),
			schema.SpawnStructField("InitialBalance", "TokenAmount", false, false),
			schema.SpawnStructField("StartEpoch", "ChainEpoch", false, false),
			schema.SpawnStructField("UnlockDuration", "ChainEpoch", false, false),
			schema.SpawnStructField("PendingTxns", "Link", false, false), //hamt[TxnID]Multisigv0Transaction
		},
		schema.StructRepresentation_Tuple{},
	))
	ts.Accumulate(schema.SpawnInt("MultisigV0TxnID"))
	ts.Accumulate(schema.SpawnStruct("MultisigV0Transaction",
		[]schema.StructField{
			schema.SpawnStructField("To", "Address", false, false),
			schema.SpawnStructField("Value", "TokenAmount", false, false),
			schema.SpawnStructField("Method", "MethodNum", false, false),
			schema.SpawnStructField("Params", "Bytes", false, false),
			schema.SpawnStructField("Approved", "List__Address", false, false),
		},
		schema.StructRepresentation_Tuple{},
	))
}
