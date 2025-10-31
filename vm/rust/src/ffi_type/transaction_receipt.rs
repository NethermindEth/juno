use starknet_api::{execution_resources::GasVector, transaction::fields::Fee};

#[derive(serde::Serialize, Default, Debug, PartialEq)]
pub struct TransactionReceipt {
    pub fee: Fee,
    pub gas: GasVector,
    pub da_gas: GasVector,
}
