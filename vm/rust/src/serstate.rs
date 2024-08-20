use blockifier::context::BlockContext;
use blockifier::execution::contract_class::{ContractClass, ContractClassV1};

use blockifier::state::cached_state::CachedState;
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
use crate::{build_block_context, execute_transaction, VMArgs};

#[derive(Default, Serialize, Deserialize, Clone)]
pub struct SerState<Class> {
    // Todo(xrvdg) make this felt
    pub storage: HashMap<StorageEntry, [u8; 32]>,
    pub nonce: HashMap<ContractAddress, Nonce>,
    pub class_hash: HashMap<ContractAddress, ClassHash>,
    pub contract_class: HashMap<ClassHash, Class>,
}

#[derive(Clone)]
pub struct NativeState(pub SerState<String>);

#[derive(Clone)]
pub struct CompiledNativeState(SerState<ContractClass>);

impl CompiledNativeState {
    pub fn new(state: NativeState) -> Self {
        let mut compiled_contract_class: HashMap<ClassHash, ContractClass> = Default::default();

        for class_hash in state.0.contract_class.keys() {
            let contract_class = state.get_compiled_contract_class(*class_hash).unwrap();
            compiled_contract_class.insert(*class_hash, contract_class);
        }

        Self(SerState {
            storage: state.0.storage,
            nonce: state.0.nonce,
            class_hash: state.0.class_hash,
            contract_class: compiled_contract_class,
        })
    }
}

impl StateReader for CompiledNativeState {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
        self.state.get_storage_at(contract_address, key)
    }

    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
        self.state.get_nonce_at(contract_address)
    }

    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        self.state.get_class_hash_at(contract_address)
    }

    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        Ok(self
            .compiled_contract_class
            .get(&class_hash)
            .expect("there should be a compiled contract")
            .clone())
    }

    fn get_compiled_class_hash(&self, _class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }
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

    fn get_compiled_class_hash(&self, _class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }

    fn get_compiled_contract_class(&self, _class_hash: ClassHash) -> StateResult<ContractClass> {
        unimplemented!()
    }
}

impl StateReader for NativeState {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
        self.0.get_storage_at(contract_address, key)
    }

    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
        self.0.get_nonce_at(contract_address)
    }

    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        self.0.get_class_hash_at(contract_address)
    }

    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        // Passing along version information could make this a V1 and V1Native test
        let json_str = self
            .0
            .contract_class
            .get(&class_hash)
            .expect("request non existed class");
        Ok(class_info_from_json_str(json_str, class_hash)
            .expect("decoding class went wrong")
            .contract_class())
    }

    fn get_compiled_class_hash(&self, class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        self.0.get_compiled_class_hash(class_hash)
    }
}

pub fn run(vm_args: &mut VMArgs, state: &mut CachedState<impl StateReader>) {
    // Replace by git root path

    let block_context: BlockContext =
        build_block_context(state, &vm_args.block_info, &vm_args.chain_id, None);

    for (_txn_index, txn_and_query_bit) in vm_args.txns_and_query_bits.iter().enumerate() {
        execute_transaction(
            txn_and_query_bit,
            state,
            &mut vm_args.classes,
            &mut vm_args.paid_fees_on_l1,
            &block_context,
            vm_args.charge_fee,
            vm_args.validate,
            vm_args.err_on_revert,
        )
        .unwrap();
    }
}
