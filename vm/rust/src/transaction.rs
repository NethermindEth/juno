use crate::execution::TxnAndQueryBit;
use crate::juno_state_reader::{class_info_from_json_str, JunoStateReader};
use blockifier::transaction::account_transaction::ExecutionFlags;
use blockifier::transaction::transaction_execution::Transaction;
use blockifier::transaction::transactions::ExecutableTransaction;
use blockifier::{
    context::BlockContext,
    state::cached_state::{CachedState, TransactionalState},
    transaction::{errors::TransactionExecutionError, objects::TransactionExecutionInfo},
};
use starknet_api::contract_class::ClassInfo;
use starknet_api::executable_transaction::AccountTransaction;
use starknet_api::transaction::fields::ValidResourceBounds;
use starknet_api::transaction::{fields::Fee, Transaction as StarknetApiTransaction};
use starknet_api::transaction::{
    DeclareTransaction, DeployAccountTransaction, InvokeTransaction, TransactionHash,
};

pub fn execute_transaction(
    txn: &Transaction,
    txn_state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
) -> Result<TransactionExecutionInfo, TransactionExecutionError> {
    match estimate_fee_version(&txn) {
        FeeEstimationVersion::WithoutL2Gas => txn.execute(txn_state, &block_context),
        FeeEstimationVersion::WithL2Gas => txn.execute(txn_state, &block_context),
    }
}

pub fn preprocess_transaction(
    txn_and_query_bit: &TxnAndQueryBit,
    classes: &mut Vec<Box<serde_json::value::RawValue>>,
    paid_fees_on_l1: &mut Vec<Box<Fee>>,
    charge_fee: bool,
    validate: bool,
) -> Result<Transaction, String> {
    let class_info = match txn_and_query_bit.txn {
        StarknetApiTransaction::Declare(_) => {
            if classes.is_empty() {
                return Err("missing declared class".to_string());
            }
            let class_json_str = classes.remove(0);
            let maybe_cc = class_info_from_json_str(class_json_str.get());
            match maybe_cc {
                Ok(cc) => Some(cc),
                Err(e) => return Err(e),
            }
        }
        _ => None,
    };

    let paid_fee_on_l1 = match txn_and_query_bit.txn {
        StarknetApiTransaction::L1Handler(_) => {
            if paid_fees_on_l1.is_empty() {
                return Err("missing fee paid on l1b".to_string());
            }
            Some(*paid_fees_on_l1.remove(0))
        }
        _ => None,
    };

    let account_execution_flags = ExecutionFlags {
        only_query: txn_and_query_bit.query_bit,
        charge_fee,
        validate,
    };

    transaction_from_api(
        txn_and_query_bit.txn.clone(),
        txn_and_query_bit.txn_hash,
        class_info,
        paid_fee_on_l1,
        account_execution_flags,
    )
}

fn transaction_from_api(
    tx: StarknetApiTransaction,
    tx_hash: TransactionHash,
    class_info: Option<ClassInfo>,
    paid_fee_on_l1: Option<Fee>,
    execution_flags: ExecutionFlags,
) -> Result<Transaction, String> {
    match tx {
        StarknetApiTransaction::Deploy(_) => {
            return Err(format!(
                "Unsupported deploy transaction in the traced block (transaction_hash={})",
                tx_hash,
            ))
        }
        StarknetApiTransaction::Declare(_) if class_info.is_none() => {
            return Err(format!(
                "Declare transaction must be created with a ContractClass (transaction_hash={})",
                tx_hash,
            ))
        }
        _ => {} // all ok
    };

    Transaction::from_api(
        tx,
        tx_hash,
        class_info,
        paid_fee_on_l1,
        None,
        execution_flags,
    )
    .map_err(|err| format!("failed to create transaction from api: {:?}", err))
}

enum FeeEstimationVersion {
    WithoutL2Gas,
    WithL2Gas,
}

fn estimate_fee_version(transaction: &Transaction) -> FeeEstimationVersion {
    if let Transaction::Account(account_transaction) = transaction {
        match &account_transaction.tx {
            AccountTransaction::Declare(inner) => {
                if let DeclareTransaction::V3(tx) = &inner.tx {
                    return estimate_fee(&tx.resource_bounds);
                }
            }
            AccountTransaction::DeployAccount(inner) => {
                if let DeployAccountTransaction::V3(tx) = &inner.tx {
                    return estimate_fee(&tx.resource_bounds);
                }
            }
            AccountTransaction::Invoke(inner) => {
                if let InvokeTransaction::V3(tx) = &inner.tx {
                    return estimate_fee(&tx.resource_bounds);
                }
            }
        }
    }
    FeeEstimationVersion::WithoutL2Gas
}

fn estimate_fee(resource_bounds: &ValidResourceBounds) -> FeeEstimationVersion {
    match resource_bounds {
        ValidResourceBounds::AllResources(_) => FeeEstimationVersion::WithL2Gas,
        ValidResourceBounds::L1Gas(_) => FeeEstimationVersion::WithoutL2Gas,
    }
}
