use serde::{Serialize};
use blockifier;
use starknet_api::core::{ContractAddress};

#[derive(Debug,Serialize)]
pub struct TransactionExecutionInfo {
    pub validate_call_info: Option<CallInfo>,
    pub execute_call_info: Option<CallInfo>,
    pub fee_transfer_call_info: Option<CallInfo>,
}

impl From<blockifier::transaction::objects::TransactionExecutionInfo> for TransactionExecutionInfo {
    fn from(val: blockifier::transaction::objects::TransactionExecutionInfo) -> Self {
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
            }
        }
    }
}

#[derive(Debug,Serialize)]
pub struct CallInfo {
    pub call: CallEntryPoint,
}

impl From<blockifier::execution::entry_point::CallInfo> for CallInfo {
    fn from(val: blockifier::execution::entry_point::CallInfo) -> Self {
        CallInfo {
            call: val.call.into(),
        }
    }
}

#[derive(Debug,Serialize)]
pub struct CallEntryPoint {
    pub storage_address: ContractAddress,
}

impl From<blockifier::execution::entry_point::CallEntryPoint> for CallEntryPoint {
    fn from(val: blockifier::execution::entry_point::CallEntryPoint) -> Self {
        CallEntryPoint {
            storage_address: val.storage_address,
        }
    }
}

