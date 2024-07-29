use blockifier;
use blockifier::execution::call_info::OrderedL2ToL1Message;
use blockifier::execution::entry_point::CallType;
use blockifier::state::cached_state::CachedState;
use blockifier::state::cached_state::{CommitmentStateDiff, TransactionalState};
use blockifier::state::errors::StateError;
use blockifier::state::state_api::StateReader;
use cairo_vm::types::builtin_name::BuiltinName;
use serde::Serialize;
use starknet_api::core::{ClassHash, ContractAddress, EntryPointSelector, EthAddress, PatriciaKey};
use starknet_api::deprecated_contract_class::EntryPointType;
use starknet_api::transaction::{Calldata, EventContent, L2ToL1Payload};
use starknet_api::transaction::{DeclareTransaction, Transaction as StarknetApiTransaction};
use starknet_types_core::felt::Felt;

type StarkFelt = Felt;

use crate::juno_state_reader::JunoStateReader;

#[derive(Serialize, Default)]
#[serde(rename_all = "UPPERCASE")]
pub enum TransactionType {
    // dummy type for implementing Default trait
    #[default]
    Unknown,
    Invoke,
    Declare,
    #[serde(rename = "DEPLOY_ACCOUNT")]
    DeployAccount,
    #[serde(rename = "L1_HANDLER")]
    L1Handler,
}

#[derive(Serialize, Default)]
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
    state_diff: StateDiff,
}

#[derive(Serialize, Default)]
struct StateDiff {
    storage_diffs: Vec<StorageDiff>,
    nonces: Vec<Nonce>,
    deployed_contracts: Vec<DeployedContract>,
    deprecated_declared_classes: Vec<StarkFelt>,
    declared_classes: Vec<DeclaredClass>,
    replaced_classes: Vec<ReplacedClass>,
}

#[derive(Serialize)]
struct Nonce {
    contract_address: StarkFelt,
    nonce: StarkFelt,
}

#[derive(Serialize)]
struct StorageDiff {
    address: StarkFelt,
    storage_entries: Vec<Entry>,
}

#[derive(Serialize)]
struct Entry {
    key: StarkFelt,
    value: StarkFelt,
}

#[derive(Serialize)]
struct DeployedContract {
    address: StarkFelt,
    class_hash: StarkFelt,
}

#[derive(Serialize)]
struct ReplacedClass {
    contract_address: StarkFelt,
    class_hash: StarkFelt,
}

#[derive(Serialize)]
struct DeclaredClass {
    class_hash: StarkFelt,
    compiled_class_hash: StarkFelt,
}

#[derive(Serialize)]
#[serde(untagged)]
pub enum ExecuteInvocation {
    Ok(FunctionInvocation),
    Revert { revert_reason: String },
}

type BlockifierTxInfo = blockifier::transaction::objects::TransactionExecutionInfo;
pub fn new_transaction_trace(
    tx: &StarknetApiTransaction,
    info: BlockifierTxInfo,
    state: &mut TransactionalState<CachedState<JunoStateReader>>,
) -> Result<TransactionTrace, StateError> {
    let mut trace = TransactionTrace::default();
    let mut deprecated_declared_class_hash: Option<ClassHash> = None;
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
            deprecated_declared_class_hash = if info.revert_error.is_none() {
                match declare_txn {
                    DeclareTransaction::V0(_) => Some(declare_txn.class_hash()),
                    DeclareTransaction::V1(_) => Some(declare_txn.class_hash()),
                    _ => None,
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

    trace.state_diff = make_state_diff(state, deprecated_declared_class_hash)?;
    Ok(trace)
}

#[derive(Serialize)]
pub struct OrderedEvent {
    pub order: usize,
    #[serde(flatten)]
    pub event: EventContent,
}

use blockifier::execution::call_info::OrderedEvent as BlockifierOrderedEvent;
impl From<BlockifierOrderedEvent> for OrderedEvent {
    fn from(val: BlockifierOrderedEvent) -> Self {
        OrderedEvent {
            order: val.order,
            event: val.event,
        }
    }
}

#[derive(Serialize, Default)]
pub struct ExecutionResources {
    pub steps: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory_holes: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub range_check_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pedersen_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub poseidon_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ec_op_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ecdsa_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bitwise_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub keccak_builtin_applications: Option<usize>,
    // https://github.com/starkware-libs/starknet-specs/pull/167
    #[serde(skip_serializing_if = "Option::is_none")]
    pub segment_arena_builtin: Option<usize>,
}

use cairo_vm::vm::runners::cairo_runner::ExecutionResources as VmExecutionResources;
impl From<VmExecutionResources> for ExecutionResources {
    fn from(val: VmExecutionResources) -> Self {
        ExecutionResources {
            steps: val.n_steps,
            memory_holes: if val.n_memory_holes > 0 {
                Some(val.n_memory_holes)
            } else {
                None
            },
            range_check_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::range_check)
                .cloned(),
            pedersen_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::pedersen)
                .cloned(),
            poseidon_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::poseidon)
                .cloned(),
            ec_op_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::ec_op)
                .cloned(),
            ecdsa_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::ecdsa)
                .cloned(),
            bitwise_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::bitwise)
                .cloned(),
            keccak_builtin_applications: val
                .builtin_instance_counter
                .get(&BuiltinName::keccak)
                .cloned(),
            segment_arena_builtin: val
                .builtin_instance_counter
                .get(&BuiltinName::segment_arena)
                .cloned(),
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
    pub result: Vec<StarkFelt>,
    pub calls: Vec<FunctionInvocation>,
    pub events: Vec<OrderedEvent>,
    pub messages: Vec<OrderedMessage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub execution_resources: Option<ExecutionResources>,
}

use blockifier::execution::call_info::CallInfo as BlockifierCallInfo;
impl From<BlockifierCallInfo> for FunctionInvocation {
    fn from(val: BlockifierCallInfo) -> Self {
        FunctionInvocation {
            entry_point_type: val.call.entry_point_type,
            call_type: match val.call.call_type {
                CallType::Call => "CALL",
                CallType::Delegate => "DELEGATE",
            }
            .to_string(),
            caller_address: val.call.caller_address,
            class_hash: val.call.class_hash,
            result: val.execution.retdata.0,
            function_call: FunctionCall {
                contract_address: val.call.storage_address,
                entry_point_selector: val.call.entry_point_selector,
                calldata: val.call.calldata,
            },
            calls: val.inner_calls.into_iter().map(|v| v.into()).collect(),
            events: val.execution.events.into_iter().map(|v| v.into()).collect(),
            messages: val
                .execution
                .l2_to_l1_messages
                .into_iter()
                .map(|v| {
                    let mut ordered_message: OrderedMessage = v.into();
                    ordered_message.from_address = val.call.storage_address;
                    ordered_message
                })
                .collect(),
            execution_resources: Some(val.resources.into()),
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

fn make_state_diff(
    state: &mut TransactionalState<CachedState<JunoStateReader>>,
    deprecated_declared_class_hash: Option<ClassHash>,
) -> Result<StateDiff, StateError> {
    let diff: CommitmentStateDiff = state.to_state_diff()?.into();
    let mut deployed_contracts = Vec::new();
    let mut replaced_classes = Vec::new();

    for (addr, class_hash) in diff.address_to_class_hash {
        let existing_class_hash = state.state.get_class_hash_at(addr)?;
        let addr: StarkFelt = addr.into();

        if existing_class_hash == ClassHash::default() {
            #[rustfmt::skip]
            deployed_contracts.push(DeployedContract {
                address: addr,
                class_hash: class_hash.0,
            });
        } else {
            #[rustfmt::skip]
            replaced_classes.push(ReplacedClass {
                contract_address: addr,
                class_hash: class_hash.0,
            });
        }
    }

    let mut deprecated_declared_class_hashes = Vec::default();
    if let Some(v) = deprecated_declared_class_hash {
        deprecated_declared_class_hashes.push(v.0)
    }
    Ok(StateDiff {
        deployed_contracts,
        #[rustfmt::skip]
        storage_diffs: diff.storage_updates.into_iter().map(| v | StorageDiff {
            address: *v.0.0.key(),
            storage_entries: v.1.into_iter().map(| e | Entry {
                key: *e.0.0.key(),
                value: e.1
            }).collect()
        }).collect(),
        #[rustfmt::skip]
        declared_classes: diff.class_hash_to_compiled_class_hash.into_iter().map(| v | DeclaredClass {
            class_hash: v.0.0,
            compiled_class_hash: v.1.0,
        }).collect(),
        deprecated_declared_classes: deprecated_declared_class_hashes,
        #[rustfmt::skip]
        nonces: diff.address_to_nonce.into_iter().map(| v | Nonce {
          contract_address: *v.0.0.key(),
          nonce: v.1.0,
        }).collect(),
        replaced_classes,
    })
}
