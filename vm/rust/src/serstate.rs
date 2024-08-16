use blockifier::execution::contract_class::ContractClass;

use blockifier::state::{
    cached_state::StorageEntry,
    state_api::{StateReader, StateResult},
};
use serde::{Deserialize, Serialize};
use starknet_api::{
    core::{ClassHash, CompiledClassHash, ContractAddress, Nonce},
    state::StorageKey,
};
use starknet_types_core::felt::Felt;
use std::collections::HashMap;

use crate::juno_state_reader::class_info_from_json_str;

#[derive(Default, Serialize, Deserialize)]
pub struct SerState {
    // Could have been wrapped around with felt
    pub storage: HashMap<StorageEntry, [u8; 32]>,
    pub nonce: HashMap<ContractAddress, Nonce>,
    pub class_hash: HashMap<ContractAddress, ClassHash>,
    pub contract_class: HashMap<ClassHash, String>,
}

impl StateReader for SerState {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
        // might have to deal with
        let bytes = self
            .storage
            .get(&(contract_address, key))
            .expect("no storage");
        Ok(Felt::from_bytes_be(bytes))
    }

    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
        Ok(*self
            .nonce
            .get(&contract_address)
            .expect("no nonce for this"))
    }

    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        Ok(*self
            .class_hash
            .get(&contract_address)
            .expect("no class_hash for this"))
    }

    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        // Passing along version information could make this a V1 and V1Native test
        let json_str = self
            .contract_class
            .get(&class_hash)
            .expect("request non existed class");
        Ok(class_info_from_json_str(json_str, class_hash)
            .expect("decoding class went wrong")
            .contract_class())
    }

    fn get_compiled_class_hash(&self, _class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }
}
