use blockifier;
use blockifier::execution::entry_point::CallType;
use serde::Serialize;
use starknet_api::core::{ClassHash, ContractAddress, EntryPointSelector};
use starknet_api::deprecated_contract_class::EntryPointType;
use starknet_api::hash::StarkFelt;
use starknet_api::transaction::{Calldata, EthAddress, EventContent, L2ToL1Payload};

#[derive(Serialize)]
pub struct TransactionTrace {
    pub validate_invocation: Option<FunctionInvocation>,
    pub execute_invocation: Option<FunctionInvocation>,
    pub fee_transfer_invocation: Option<FunctionInvocation>,
}

type BlockifierTxInfo = blockifier::transaction::objects::TransactionExecutionInfo;
impl From<BlockifierTxInfo> for TransactionTrace {
    fn from(info: BlockifierTxInfo) -> Self {
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
