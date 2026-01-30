use blockifier::state::cached_state::StateMaps;
use starknet_api::{
    core::{ClassHash, ContractAddress},
};
use starknet_types_core::felt::Felt;

#[derive(serde::Serialize, Debug, Default, PartialEq)]
pub struct InitialReads {
    pub storage: Vec<StorageEntry>,
    pub nonces: Vec<NonceEntry>,
    pub class_hashes: Vec<ClassHashEntry>,
    pub declared_contracts: Vec<DeclaredContractEntry>,
}

#[derive(serde::Serialize, Debug, PartialEq)]
pub struct StorageEntry {
    pub contract_address: ContractAddress,
    pub key: Felt,
    pub value: Felt,
}

#[derive(serde::Serialize, Debug, PartialEq)]
pub struct NonceEntry {
    pub contract_address: ContractAddress,
    pub nonce: Felt,
}

#[derive(serde::Serialize, Debug, PartialEq)]
pub struct ClassHashEntry {
    pub contract_address: ContractAddress,
    pub class_hash: ClassHash,
}

#[derive(serde::Serialize, Debug, PartialEq)]
pub struct DeclaredContractEntry {
    pub class_hash: ClassHash,
    pub is_declared: bool,
}

impl From<StateMaps> for InitialReads {
    fn from(state_maps: StateMaps) -> Self {
        let storage = state_maps
            .storage
            .into_iter()
            .map(|((contract_address, storage_key), value)| StorageEntry {
                contract_address,
                key: storage_key.into(),
                value: value.into(),
            })
            .collect();

        let nonces = state_maps
            .nonces
            .into_iter()
            .map(|(contract_address, nonce)| NonceEntry {
                contract_address,
                nonce: (*nonce).into(),
            })
            .collect();

        let class_hashes = state_maps
            .class_hashes
            .into_iter()
            .map(|(contract_address, class_hash)| ClassHashEntry {
                contract_address,
                class_hash,
            })
            .collect();

        let declared_contracts = state_maps
            .declared_contracts
            .into_iter()
            .map(|(class_hash, is_declared)| DeclaredContractEntry {
                class_hash,
                is_declared,
            })
            .collect();

        Self {
            storage,
            nonces,
            class_hashes,
            declared_contracts,
        }
    }
}
