use blockifier::execution::stack_trace::{
    gen_tx_execution_error_trace, Cairo1RevertFrame, Cairo1RevertSummary,
    ErrorStack as BlockifierErrorStack, ErrorStackSegment,
};
use blockifier::transaction::errors::TransactionExecutionError;
use blockifier::transaction::objects::RevertError;
use serde_json::json;
use starknet_types_core::felt::Felt;

#[derive(Clone, Debug, PartialEq)]
pub struct CallFrame {
    pub storage_address: Felt,
    pub class_hash: Felt,
    pub selector: Option<Felt>,
}

#[derive(Clone, Debug, PartialEq)]
pub enum Frame {
    CallFrame(CallFrame),
    StringFrame(String),
}

impl From<Cairo1RevertFrame> for Frame {
    fn from(value: Cairo1RevertFrame) -> Self {
        Self::CallFrame(CallFrame {
            storage_address: *value.contract_address.0,
            class_hash: value.class_hash.unwrap_or_default().0,
            selector: Some(value.selector.0),
        })
    }
}

struct Frames(pub Vec<Frame>);

impl From<ErrorStackSegment> for Frames {
    fn from(value: ErrorStackSegment) -> Self {
        match value {
            ErrorStackSegment::EntryPoint(entry_point) => Self(vec![Frame::CallFrame(CallFrame {
                storage_address: *entry_point.storage_address.0,
                class_hash: entry_point.class_hash.0,
                selector: entry_point.selector.map(|s| s.0),
            })]),
            ErrorStackSegment::Cairo1RevertSummary(revert_summary) => revert_summary.into(),
            ErrorStackSegment::Vm(vm_exception) => {
                Self(vec![Frame::StringFrame(String::from(&vm_exception))])
            }
            ErrorStackSegment::StringFrame(string_frame) => {
                Self(vec![Frame::StringFrame(string_frame)])
            }
        }
    }
}

impl From<Cairo1RevertSummary> for Frames {
    fn from(value: Cairo1RevertSummary) -> Self {
        let failure_reason =
            starknet_api::execution_utils::format_panic_data(&value.last_retdata.0);
        Self(
            value
                .stack
                .into_iter()
                .map(Into::into)
                .chain(std::iter::once(Frame::StringFrame(failure_reason)))
                .collect(),
        )
    }
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct ErrorStack(pub Vec<Frame>);

impl From<BlockifierErrorStack> for ErrorStack {
    fn from(value: BlockifierErrorStack) -> Self {
        Self(
            value
                .stack
                .into_iter()
                .flat_map(|v| Frames::from(v).0)
                .collect(),
        )
    }
}

impl From<TransactionExecutionError> for ErrorStack {
    fn from(value: TransactionExecutionError) -> Self {
        gen_tx_execution_error_trace(&value).into()
    }
}

impl From<RevertError> for ErrorStack {
    fn from(value: RevertError) -> Self {
        match value {
            RevertError::Execution(error_stack) => error_stack.into(),
            RevertError::PostExecution(fee_check_error) => {
                Self(vec![Frame::StringFrame(fee_check_error.to_string())])
            }
        }
    }
}

impl From<Cairo1RevertSummary> for ErrorStack {
    fn from(value: Cairo1RevertSummary) -> Self {
        Self(Frames::from(value).0)
    }
}

// todo(rdr): Probably is better to implement the From trait
pub fn error_stack_frames_to_json(err_stack: ErrorStack) -> serde_json::Value {
    let frames = err_stack.0;

    // We are assuming they will be only one of these
    let string_frame = frames
        .iter()
        .rev()
        .find_map(|frame| match frame {
            Frame::StringFrame(string) => Some(string.clone()),
            _ => None,
        })
        .unwrap_or_else(|| "Unknown error, no string frame available.".to_string());

    // Start building the error from the ground up, starting from the string frame
    // into the parent call frame and so on ...
    frames
        .into_iter()
        .filter_map(|frame| match frame {
            Frame::CallFrame(call_frame) => Some(call_frame),
            _ => None,
        })
        .rev()
        .fold(json!(string_frame), |prev_err, frame| {
            json!({
                "contract_address": frame.storage_address,
                "class_hash": frame.class_hash,
                "selector": frame.selector,
                "error": prev_err,
            })
        })
}
