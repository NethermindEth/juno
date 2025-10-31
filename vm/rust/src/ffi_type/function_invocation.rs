use crate::ffi_type::execution_resources::ExecutionResources;
use blockifier::{
    blockifier_versioned_constants::VersionedConstants,
    execution::{
        call_info::{CallInfo, OrderedEvent as BlockifierOrderedEvent, OrderedL2ToL1Message},
        entry_point::CallType,
    },
};
use serde::Serialize;
use starknet_api::{
    contract_class::EntryPointType,
    core::{ClassHash, ContractAddress, EntryPointSelector, EthAddress},
    transaction::{
        fields::{Calldata, GasVectorComputationMode},
        EventContent, L2ToL1Payload,
    },
};
use starknet_types_core::felt::Felt;

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
            from_address: ContractAddress::default(),
            to_address: val.message.to_address,
            payload: val.message.payload,
        }
    }
}

#[derive(Serialize)]
pub struct OrderedEvent {
    pub order: usize,
    #[serde(flatten)]
    pub event: EventContent,
}

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
    pub result: Vec<Felt>,
    pub calls: Vec<FunctionInvocation>,
    pub events: Vec<OrderedEvent>,
    pub messages: Vec<OrderedMessage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub execution_resources: Option<ExecutionResources>,
    pub is_reverted: bool,
}

impl FunctionInvocation {
    pub fn from_call_info(
        val: CallInfo,
        versioned_constants: &VersionedConstants,
        gas_vector_computation_mode: &GasVectorComputationMode,
    ) -> Self {
        let gas_consumed = val
            .summarize(versioned_constants)
            .to_partial_gas_vector(versioned_constants, gas_vector_computation_mode);

        let execution_resources =
            ExecutionResources::from_resources_and_gas_vector(val.resources, gas_consumed);

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
            calls: val
                .inner_calls
                .into_iter()
                .map(|v| Self::from_call_info(v, versioned_constants, gas_vector_computation_mode))
                .collect(),
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
            execution_resources: Some(execution_resources),
            is_reverted: val.execution.failed,
        }
    }
}
