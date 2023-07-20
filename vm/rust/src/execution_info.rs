use blockifier;
use serde::{Serialize};
use starknet_api::core::{ContractAddress, EntryPointSelector, ClassHash};
use starknet_api::deprecated_contract_class::EntryPointType;
use starknet_api::hash::StarkFelt;
use starknet_api::transaction::{Calldata, EventContent, EthAddress, L2ToL1Payload};

#[derive(Debug,Serialize)]
pub struct TransactionExecutionInfo {
    pub validate_call_info: Option<CallInfo>,
    pub execute_call_info: Option<CallInfo>,
    pub fee_transfer_call_info: Option<CallInfo>,
}

type BlockifierTxInfo = blockifier::transaction::objects::TransactionExecutionInfo;
impl From<BlockifierTxInfo> for TransactionExecutionInfo {
    fn from(val: BlockifierTxInfo) -> Self {
        TransactionExecutionInfo {
            validate_call_info: match val.validate_call_info {
                Some(v) => Some(v.into()),
                None => None,
            },
            execute_call_info: match val.execute_call_info {
                Some(v) => Some(v.into()),
                None => None,
            },
            fee_transfer_call_info: match val.fee_transfer_call_info {
                Some(v) => Some(v.into()),
                None => None,
            },
        }
    }
}

#[derive(Debug,Serialize)]
pub struct CallInfo {
    pub call: CallEntryPoint,
    pub execution: CallExecution,
    pub inner_calls: Vec<CallInfo>,
}

type BlockifierCallInfo = blockifier::execution::entry_point::CallInfo;
impl From<BlockifierCallInfo> for CallInfo {
    fn from(val: BlockifierCallInfo) -> Self {
        CallInfo {
            call: val.call.into(),
            execution: val.execution.into(),
            inner_calls: val.inner_calls.into_iter().map(|v| v.into()).collect(),
        }
    }
}

#[derive(Debug,Serialize)]
pub struct CallEntryPoint {
    // todo add contract_address
    pub entry_point_selector: EntryPointSelector,
    pub calldata: Calldata,

    pub caller_address: ContractAddress,
    pub class_hash: Option<ClassHash>,
    pub entry_point_type: EntryPointType,
    pub call_type: CallType,
}

type BlockifierCallEntryPoint = blockifier::execution::entry_point::CallEntryPoint;
impl From<BlockifierCallEntryPoint> for CallEntryPoint {
    fn from(val: BlockifierCallEntryPoint) -> CallEntryPoint {
        CallEntryPoint {
            entry_point_selector: val.entry_point_selector,
            calldata: val.calldata,
            caller_address: val.caller_address,
            class_hash: val.class_hash,
            entry_point_type: val.entry_point_type,
            call_type: val.call_type.into(),
        }
    }
}

#[derive(Debug,Serialize)]
pub struct CallType(String);

type BlockifierCallType = blockifier::execution::entry_point::CallType;
impl From<BlockifierCallType> for CallType {
    fn from(val: BlockifierCallType) -> Self {
        match val {
            BlockifierCallType::Call => CallType("Call".to_string()),
            BlockifierCallType::Delegate => CallType("Delegate".to_string()),
        }
    }
}

#[derive(Debug,Serialize)]
pub struct CallExecution {
    pub retdata: Retdata,
    pub events: Vec<OrderedEvent>,
    pub l2_to_l1_messages: Vec<OrderedL2ToL1Message>,
}

type BlockifierCallExecution = blockifier::execution::entry_point::CallExecution;
impl From<BlockifierCallExecution> for CallExecution {
    fn from(val: BlockifierCallExecution) -> Self {
        CallExecution {
            retdata: Retdata(val.retdata.0),
            events: val.events.into_iter().map(|v| v.into()).collect(),
            l2_to_l1_messages: val.l2_to_l1_messages.into_iter().map(|v| v.into()).collect(),
        }
    }
}

#[derive(Debug,Serialize)]
pub struct OrderedEvent {
    pub order: usize,
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

#[derive(Debug,Serialize)]
pub struct OrderedL2ToL1Message {
    pub order: usize,
    pub message: MessageToL1,
}

type BlockifierOrderedL2ToL1Message = blockifier::execution::entry_point::OrderedL2ToL1Message;
impl From<BlockifierOrderedL2ToL1Message> for OrderedL2ToL1Message {
    fn from(val: BlockifierOrderedL2ToL1Message) -> Self {
        OrderedL2ToL1Message {
            order: val.order,
            message: val.message.into(),
        }
    }
}

#[derive(Debug,Serialize)]
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

#[derive(Debug,Serialize)]
pub struct Retdata(pub Vec<StarkFelt>);