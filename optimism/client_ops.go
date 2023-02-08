package optimism

import (
	"math/big"

	RosettaTypes "github.com/coinbase/rosetta-sdk-go/types"
	OptimismTypes "github.com/ethereum-optimism/optimism/l2geth/core/types"
)

func feeOps(tx *legacyTransaction) []*RosettaTypes.Operation {
	return []*RosettaTypes.Operation{
		{
			OperationIdentifier: &RosettaTypes.OperationIdentifier{
				Index: 0,
			},
			Type:   FeeOpType,
			Status: RosettaTypes.String(SuccessStatus),
			Account: &RosettaTypes.AccountIdentifier{
				Address: MustChecksum(tx.From.String()),
			},
			Amount: &RosettaTypes.Amount{
				Value:    new(big.Int).Neg(tx.FeeAmount).String(),
				Currency: Currency,
			},
		},
		{
			OperationIdentifier: &RosettaTypes.OperationIdentifier{
				Index: 1,
			},
			RelatedOperations: []*RosettaTypes.OperationIdentifier{
				{
					Index: 0,
				},
			},
			Type:   FeeOpType,
			Status: RosettaTypes.String(SuccessStatus),
			Account: &RosettaTypes.AccountIdentifier{
				Address: MustChecksum(tx.Miner),
			},
			Amount: &RosettaTypes.Amount{
				Value:    tx.FeeAmount.String(),
				Currency: Currency,
			},
		},
	}
}

// Set the fees of applicable zero gas transactions to zero
func patchFeeOps(chainID *big.Int, block *OptimismTypes.Block, tx *OptimismTypes.Transaction, ops []*RosettaTypes.Operation) {
	if chainID.Cmp(goerliChainID) != 0 {
		return
	}
	if tx.GasPrice().Uint64() == 0 && block.NumberU64() < goerliRollupFeeEnforcementBlockHeight {
		for _, op := range ops {
			if op.Type == FeeOpType {
				op.Amount.Value = "0"
			}
		}
	}
}

// Amount returns a Rosetta Amount from a big.Int and currency.
func Amount(value *big.Int, currency *RosettaTypes.Currency) *RosettaTypes.Amount {
	if value == nil {
		return nil
	}
	return &RosettaTypes.Amount{
		Value:    value.String(),
		Currency: currency,
	}
}

// BedrockFeeOps returns the fee operations for a given bedrock transaction.
func BedrockFeeOps(tx *bedrockTransaction) ([]*RosettaTypes.Operation, error) {
	if tx.IsDepositTx() {
		return nil, nil
	}

	var receipt OptimismTypes.Receipt
	if err := receipt.UnmarshalJSON(tx.Receipt.RawMessage); err != nil {
		return nil, err
	}

	sequencerFeeAmount := new(big.Int).Set(tx.FeeAmount)
	if tx.FeeBurned != nil {
		sequencerFeeAmount.Sub(sequencerFeeAmount, tx.FeeBurned)
	}
	if receipt.L1Fee != nil {
		sequencerFeeAmount.Sub(sequencerFeeAmount, receipt.L1Fee)
	}

	if sequencerFeeAmount == nil {
		return nil, nil
	}

	feeRewarder := tx.Miner
	if len(tx.Author) > 0 {
		feeRewarder = tx.Author
	}

	ops := []*RosettaTypes.Operation{
		{
			OperationIdentifier: &RosettaTypes.OperationIdentifier{
				Index: 0,
			},
			Type:   FeeOpType,
			Status: RosettaTypes.String(SuccessStatus),
			Account: &RosettaTypes.AccountIdentifier{
				Address: MustChecksum(tx.From.String()),
			},
			Amount: Amount(new(big.Int).Neg(tx.Receipt.TransactionFee), Currency),
		},

		{
			OperationIdentifier: &RosettaTypes.OperationIdentifier{
				Index: 1,
			},
			RelatedOperations: []*RosettaTypes.OperationIdentifier{
				{
					Index: 0,
				},
			},
			Type:   FeeOpType,
			Status: RosettaTypes.String(SuccessStatus),
			Account: &RosettaTypes.AccountIdentifier{
				Address: MustChecksum(feeRewarder),
			},
			Amount: Amount(sequencerFeeAmount, Currency),
		},

		{
			OperationIdentifier: &RosettaTypes.OperationIdentifier{
				Index: 2,
			},
			RelatedOperations: []*RosettaTypes.OperationIdentifier{
				{
					Index: 0,
				},
			},
			Type:   FeeOpType,
			Status: RosettaTypes.String(SuccessStatus),
			Account: &RosettaTypes.AccountIdentifier{
				Address: BaseFeeVault.Hex(),
			},
			// Note: The basefee is not actually burned on L2
			Amount: Amount(tx.FeeBurned, Currency),
		},

		{
			OperationIdentifier: &RosettaTypes.OperationIdentifier{
				Index: 3,
			},
			RelatedOperations: []*RosettaTypes.OperationIdentifier{
				{
					Index: 0,
				},
			},
			Type:   FeeOpType,
			Status: RosettaTypes.String(SuccessStatus),
			Account: &RosettaTypes.AccountIdentifier{
				Address: L1FeeVault.Hex(),
			},
			Amount: Amount(receipt.L1Fee, Currency),
		},
	}

	return ops, nil
}
