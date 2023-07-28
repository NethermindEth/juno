use serde::Serialize;
use crate::execution_info::{TransactionExecutionInfo, CallInfo, CallType, MessageToL1};
use starknet_api::core::{ContractAddress, EntryPointSelector, ClassHash};
use starknet_api::hash::StarkFelt;
use starknet_api::transaction::{Calldata, EventContent, EthAddress, L2ToL1Payload};
use starknet_api::deprecated_contract_class::EntryPointType;

#[derive(Serialize)]
pub struct TransactionTrace {
    pub validate_invocation: Option<FunctionInvocation>,
    pub execute_invocation: Option<FunctionInvocation>,
    pub fee_transfer_invocation: Option<FunctionInvocation>,
}

impl From<TransactionExecutionInfo> for TransactionTrace {
    fn from(info: TransactionExecutionInfo) -> Self {
        TransactionTrace {
            validate_invocation: match info.validate_call_info {
                Some(v) => Some(v.into()),
                None => None,
            },
            execute_invocation: match info.execute_call_info {
                Some(v) => Some(v.into()),
                None => None,
            },
            fee_transfer_invocation: match info.fee_transfer_call_info {
                Some(v) => Some(v.into()),
                None => None,
            },
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
    pub call_type: CallType,
    pub result: Option<Vec<StarkFelt>>,
    pub calls: Option<Vec<FunctionInvocation>>,
    pub events: Option<Vec<EventContent>>,
    pub messages: Option<Vec<MessageToL1>>,
}

impl From<CallInfo> for FunctionInvocation {
    fn from(val: CallInfo) -> Self {
        FunctionInvocation {
            entry_point_type: val.call.entry_point_type,
            call_type: val.call.call_type,
            caller_address: val.call.caller_address,
            class_hash: val.call.class_hash,
            result: Some(val.execution.retdata.0),
            function_call: FunctionCall {
                entry_point_selector: val.call.entry_point_selector,
                calldata: val.call.calldata,
            },
            calls: Some(val.inner_calls.into_iter().map(|v| v.into()).collect()),
            events: Some(val.execution.events.into_iter().map(|v| v.event).collect()),
            messages: Some(val.execution.l2_to_l1_messages.into_iter().map(|v| v.message).collect()),
        }
    }
}

#[derive(Serialize)]
pub struct FunctionCall {
    pub entry_point_selector: EntryPointSelector,
    pub calldata: Calldata,
}

/*
#[derive(Serialize, Deserialize)]
pub struct ExecuteInvocation {
    #[serde(flatten)]
    pub function_invocation: FunctionInvocation,
    pub revert_reason: Option<String>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "UPPERCASE")]
pub enum EntryPointType {
    External,
    L1Handler,
    Constructor,
}

*/