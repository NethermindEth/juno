use blockifier;
use blockifier::execution::entry_point::CallType;
use serde::Serialize;
use starknet_api::core::{ClassHash, ContractAddress, EntryPointSelector};
use starknet_api::deprecated_contract_class::EntryPointType;
use starknet_api::hash::StarkFelt;
use starknet_api::transaction::{Calldata, EthAddress, EventContent, L2ToL1Payload};
use starknet_api::transaction::{Transaction as StarknetApiTransaction};

#[derive(Serialize)]
#[serde(untagged)]
pub enum TransactionTrace {
    // used for INVOKE_TXN_TRACE and DECLARE_TXN_TRACE
    Common {
        #[serde(skip_serializing_if = "Option::is_none")]
        validate_invocation: Option<FunctionInvocation>,
        #[serde(skip_serializing_if = "Option::is_none")]
        execute_invocation: Option<ExecuteInvocation>,
        #[serde(skip_serializing_if = "Option::is_none")]
        fee_transfer_invocation: Option<FunctionInvocation>,
    },
    DeployAccount {
        #[serde(skip_serializing_if = "Option::is_none")]
        validate_invocation: Option<FunctionInvocation>,
        #[serde(skip_serializing_if = "Option::is_none")]
        constructor_invocation: Option<FunctionInvocation>,
        #[serde(skip_serializing_if = "Option::is_none")]
        fee_transfer_invocation: Option<FunctionInvocation>,
    },
    L1Handler {
        #[serde(skip_serializing_if = "Option::is_none")]
        function_invocation: Option<FunctionInvocation>
    }
}

#[derive(Serialize)]
#[serde(untagged)]
pub enum ExecuteInvocation {
    Ok(FunctionInvocation),
    Revert {
        revert_reason: String
    },
}

type BlockifierTxInfo = blockifier::transaction::objects::TransactionExecutionInfo;
pub fn new_transaction_trace(tx: StarknetApiTransaction, info: BlockifierTxInfo) -> TransactionTrace {
    match tx {
        StarknetApiTransaction::L1Handler(_) => {
            TransactionTrace::L1Handler {
                function_invocation: info.execute_call_info.map(|v| v.into()),
            }
        },
        StarknetApiTransaction::DeployAccount(_) => {
            TransactionTrace::DeployAccount {
                validate_invocation: info.validate_call_info.map(|v| v.into()),
                constructor_invocation: info.execute_call_info.map(|v| v.into()),
                fee_transfer_invocation: info.fee_transfer_call_info.map(|v| v.into()),
            }
        },
        StarknetApiTransaction::Declare(_) | StarknetApiTransaction::Invoke(_) => {
            TransactionTrace::Common {
                validate_invocation: info.validate_call_info.map(|v| v.into()),
                execute_invocation: match info.revert_error {
                    Some(str) => Some(ExecuteInvocation::Revert{revert_reason: str}),
                    None => info.execute_call_info.map(|v| ExecuteInvocation::Ok(v.into())),
                },
                fee_transfer_invocation: info.fee_transfer_call_info.map(|v| v.into()),
            }
        },
        StarknetApiTransaction::Deploy(_) => {
            // shouldn't happen since we don't support deploy
            panic!("Can't create transaction trace for deploy transaction (unsupported)");
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
    pub events: Option<Vec<EventContent>>,
    pub messages: Option<Vec<MessageToL1>>,
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
            events: Some(val.execution.events.into_iter().map(|v| v.event).collect()),
            messages: Some(
                val.execution
                    .l2_to_l1_messages
                    .into_iter()
                    .map(|v| v.message.into())
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

#[derive(Debug, Serialize)]
pub struct MessageToL1 {
    pub to_address: EthAddress,
    pub payload: L2ToL1Payload,
}

type BlockifierMessageToL1 = blockifier::execution::entry_point::MessageToL1;
impl From<BlockifierMessageToL1> for MessageToL1 {
    fn from(val: BlockifierMessageToL1) -> Self {
        MessageToL1 {
            to_address: val.to_address,
            payload: val.payload,
        }
    }
}

#[derive(Debug, Serialize)]
pub struct Retdata(pub Vec<StarkFelt>);
