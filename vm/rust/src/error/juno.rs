use serde_json::{json, Value};

pub struct JunoError {
    pub msg: String,
    pub txn_index: i64,
    pub execution_failed: bool,
}

impl JunoError {
    pub fn json_error(err: Value, txn_index: Option<usize>) -> Self {
        Self {
            msg: err.to_string(),
            txn_index: txn_index.map(|idx| idx as i64).unwrap_or(-1),
            execution_failed: false,
        }
    }

    pub fn block_error<E: ToString>(err: E) -> Self {
        Self {
            msg: json!(err.to_string()).to_string(),
            txn_index: -1,
            execution_failed: false,
        }
    }

    pub fn tx_non_execution_error<E: ToString>(err: E, txn_index: usize) -> Self {
        Self {
            msg: json!(err.to_string()).to_string(),
            txn_index: txn_index as i64,
            execution_failed: false,
        }
    }
}
