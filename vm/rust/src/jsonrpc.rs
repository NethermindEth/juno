use blockifier;
use blockifier::execution::entry_point::{CallType, OrderedL2ToL1Message};
use blockifier::state::cached_state::TransactionalState;
use blockifier::state::errors::StateError;
use blockifier::state::state_api::{State, StateReader};
use indexmap::IndexMap;
use serde::Serialize;
use starknet_api::core::{ClassHash, ContractAddress, EntryPointSelector, PatriciaKey};
use starknet_api::deprecated_contract_class::EntryPointType;
use starknet_api::hash::StarkFelt;
use starknet_api::state::ThinStateDiff;
use starknet_api::transaction::{Calldata, EthAddress, EventContent, L2ToL1Payload};
use starknet_api::transaction::{DeclareTransaction, Transaction as StarknetApiTransaction};

use crate::juno_state_reader::JunoStateReader;

#[derive(Serialize)]
#[serde(rename_all = "UPPERCASE")]
pub enum TransactionType {
    // dummy type for implementing Default trait
    Unknown,
    Invoke,
    Declare,
    #[serde(rename = "DEPLOY_ACCOUNT")]
    DeployAccount,
    #[serde(rename = "L1_HANDLER")]
    L1Handler,
}

#[derive(Serialize)]
pub struct TransactionTrace {
    #[serde(skip_serializing_if = "Option::is_none")]
    validate_invocation: Option<FunctionInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    execute_invocation: Option<ExecuteInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    fee_transfer_invocation: Option<FunctionInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    constructor_invocation: Option<FunctionInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    function_invocation: Option<FunctionInvocation>,
    r#type: TransactionType,
    #[serde(skip_serializing_if = "Option::is_none")]
    state_diff: Option<ThinStateDiff>,
}

impl Default for TransactionTrace {
    fn default() -> Self {
        Self {
            validate_invocation: None,
            execute_invocation: None,
            fee_transfer_invocation: None,
            constructor_invocation: None,
            function_invocation: None,
            r#type: TransactionType::Unknown,
            state_diff: None,
        }
    }
}

#[derive(Serialize)]
#[serde(untagged)]
pub enum ExecuteInvocation {
    Ok(FunctionInvocation),
    Revert { revert_reason: String },
}

type BlockifierTxInfo = blockifier::transaction::objects::TransactionExecutionInfo;
pub fn new_transaction_trace(
    tx: StarknetApiTransaction,
    info: BlockifierTxInfo,
    state: &mut TransactionalState<JunoStateReader>,
) -> Result<TransactionTrace, StateError> {
    let mut trace = TransactionTrace::default();
    let mut deprecated_declared_class: Option<ClassHash> = None;
    match tx {
        StarknetApiTransaction::L1Handler(_) => {
            trace.function_invocation = info.execute_call_info.map(|v| v.into());
            trace.r#type = TransactionType::L1Handler;
        }
        StarknetApiTransaction::DeployAccount(_) => {
            trace.validate_invocation = info.validate_call_info.map(|v| v.into());
            trace.constructor_invocation = info.execute_call_info.map(|v| v.into());
            trace.fee_transfer_invocation = info.fee_transfer_call_info.map(|v| v.into());
            trace.r#type = TransactionType::DeployAccount;
        }
        StarknetApiTransaction::Invoke(_) => {
            trace.validate_invocation = info.validate_call_info.map(|v| v.into());
            trace.execute_invocation = match info.revert_error {
                Some(str) => Some(ExecuteInvocation::Revert { revert_reason: str }),
                None => info
                    .execute_call_info
                    .map(|v| ExecuteInvocation::Ok(v.into())),
            };
            trace.fee_transfer_invocation = info.fee_transfer_call_info.map(|v| v.into());
            trace.r#type = TransactionType::Invoke;
        }
        StarknetApiTransaction::Declare(declare_txn) => {
            trace.validate_invocation = info.validate_call_info.map(|v| v.into());
            trace.fee_transfer_invocation = info.fee_transfer_call_info.map(|v| v.into());
            trace.r#type = TransactionType::Declare;
            deprecated_declared_class = if info.revert_error.is_none() {
                match declare_txn {
                    DeclareTransaction::V0(_) => Some(declare_txn.class_hash()),
                    DeclareTransaction::V1(_) => Some(declare_txn.class_hash()),
                    DeclareTransaction::V2(_) => None,
                }
            } else {
                None
            }
        }
        StarknetApiTransaction::Deploy(_) => {
            // shouldn't happen since we don't support deploy
            panic!("Can't create transaction trace for deploy transaction (unsupported)");
        }
    };

    trace.state_diff = Some(make_thin_state_diff(state, deprecated_declared_class)?);
    Ok(trace)
}

#[derive(Serialize)]
pub struct OrderedEvent {
    pub order: usize,
    #[serde(flatten)]
    pub event: EventContent,
}

type BlockifierOrderedEvent = blockifier::execution::entry_point::OrderedEvent;
impl From<BlockifierOrderedEvent> for OrderedEvent {
    fn from(val: BlockifierOrderedEvent) -> Self {
        OrderedEvent {
            order: val.order,
            event: val.event,
        }
    }
}

#[derive(Serialize)]
pub struct FunctionInvocation {
    #[serde(flatten)]
    pub function_call: FunctionCall,
    pub caller_address: ContractAddress,
    pub class_hash: Option<ClassHash>,
    pub entry_point_type: EntryPointType,
    pub call_type: String,
    pub result: Option<Vec<StarkFelt>>,
    pub calls: Option<Vec<FunctionInvocation>>,
    pub events: Option<Vec<OrderedEvent>>,
    pub messages: Option<Vec<OrderedMessage>>,
}

type BlockifierCallInfo = blockifier::execution::entry_point::CallInfo;
impl From<BlockifierCallInfo> for FunctionInvocation {
    fn from(val: BlockifierCallInfo) -> Self {
        FunctionInvocation {
            entry_point_type: val.call.entry_point_type,
            call_type: match val.call.call_type {
                CallType::Call => "CALL",
                CallType::Delegate => "LIBRARY_CALL",
            }
            .to_string(),
            caller_address: val.call.caller_address,
            class_hash: val.call.class_hash,
            result: Some(val.execution.retdata.0),
            function_call: FunctionCall {
                contract_address: val.call.storage_address,
                entry_point_selector: val.call.entry_point_selector,
                calldata: val.call.calldata,
            },
            calls: Some(val.inner_calls.into_iter().map(|v| v.into()).collect()),
            events: Some(val.execution.events.into_iter().map(|v| v.into()).collect()),
            messages: Some(
                val.execution
                    .l2_to_l1_messages
                    .into_iter()
                    .map(|v| {
                        let mut ordered_message: OrderedMessage = v.into();
                        ordered_message.from_address = val.call.storage_address;
                        ordered_message
                    })
                    .collect(),
            ),
        }
    }
}

#[derive(Serialize)]
pub struct FunctionCall {
    pub contract_address: ContractAddress,
    pub entry_point_selector: EntryPointSelector,
    pub calldata: Calldata,
}

#[derive(Serialize)]
pub struct OrderedMessage {
    pub order: usize,
    pub from_address: ContractAddress,
    pub to_address: EthAddress,
    pub payload: L2ToL1Payload,
}

impl From<OrderedL2ToL1Message> for OrderedMessage {
    fn from(val: OrderedL2ToL1Message) -> Self {
        OrderedMessage {
            order: val.order,
            from_address: ContractAddress(PatriciaKey::default()),
            to_address: val.message.to_address,
            payload: val.message.payload,
        }
    }
}

#[derive(Debug, Serialize)]
pub struct Retdata(pub Vec<StarkFelt>);

fn make_thin_state_diff(
    state: &mut TransactionalState<JunoStateReader>,
    deprecated_declared_class: Option<ClassHash>,
) -> Result<ThinStateDiff, StateError> {
    let diff = state.to_state_diff();
    let mut deployed_contracts = IndexMap::new();
    let mut replaced_classes = IndexMap::new();

    for pair in diff.address_to_class_hash {
        let existing_class_hash = state.state.get_class_hash_at(pair.0)?;
        if existing_class_hash == ClassHash::default() {
            deployed_contracts.insert(pair.0, pair.1);
        } else {
            replaced_classes.insert(pair.0, pair.1);
        }
    }

    let mut deprecated_declared_classes = Vec::default();
    if deprecated_declared_class.is_some() {
        deprecated_declared_classes.push(deprecated_declared_class.unwrap())
    }
    Ok(ThinStateDiff {
        deployed_contracts: deployed_contracts,
        storage_diffs: diff.storage_updates,
        declared_classes: diff.class_hash_to_compiled_class_hash,
        deprecated_declared_classes: deprecated_declared_classes,
        nonces: diff.address_to_nonce,
        replaced_classes: replaced_classes,
    })
}
