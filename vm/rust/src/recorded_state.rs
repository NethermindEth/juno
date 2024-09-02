use blockifier::context::BlockContext;
use blockifier::execution::contract_class::ContractClass;

use blockifier::state::cached_state::CachedState;
use blockifier::state::errors::StateError;
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
use std::fmt::Display;
use std::str::FromStr;

use crate::juno_state_reader::class_info_from_json_str;
use crate::{build_block_context, execute_transaction, VMArgs};

#[derive(Default, Serialize, Deserialize, Clone)]
pub struct RecordedState<Class> {
    pub storage: HashMap<StorageEntry, Felt>,
    pub nonce: HashMap<ContractAddress, Nonce>,
    pub class_hash: HashMap<ContractAddress, ClassHash>,
    pub contract_class: HashMap<ClassHash, Class>,
}

pub type NativeState = RecordedState<String>;
pub type CompiledNativeState = RecordedState<ContractClass>;

impl CompiledNativeState {
    pub fn new(state: NativeState) -> Self {
        let mut compiled_contract_class: HashMap<ClassHash, ContractClass> = Default::default();

        for class_hash in state.contract_class.keys() {
            let contract_class = state.get_compiled_contract_class(*class_hash).unwrap();
            compiled_contract_class.insert(*class_hash, contract_class);
        }

        RecordedState {
            storage: state.storage,
            nonce: state.nonce,
            class_hash: state.class_hash,
            contract_class: compiled_contract_class,
        }
    }
}

trait ContractClassTrait {
    fn compile(&self, class_hash: ClassHash) -> StateResult<ContractClass>;
}

impl ContractClassTrait for String {
    fn compile(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        class_info_from_json_str(self, class_hash)
            .map(|class_info| class_info.contract_class())
            .map_err(|err| {
                StateError::StateReadError(format!(
                    "parsing JSON string for class hash {}: {}",
                    class_hash.0, err
                ))
            })
    }
}

impl ContractClassTrait for ContractClass {
    fn compile(&self, _class_hash: ClassHash) -> StateResult<ContractClass> {
        Ok(self.clone())
    }
}

impl<C: ContractClassTrait> StateReader for RecordedState<C> {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
        Ok(*self
            .storage
            .get(&(contract_address, key))
            .expect("no storage"))
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

    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        let contract_class = self
            .contract_class
            .get(&class_hash)
            .expect("request non existed class");
        contract_class.compile(class_hash)
    }
}

pub fn run(vm_args: &mut VMArgs, state: &mut CachedState<impl StateReader>) {
    let block_context: BlockContext =
        build_block_context(state, &vm_args.block_info, &vm_args.chain_id, None);

    for txn_and_query_bit in vm_args.txns_and_query_bits.iter() {
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

pub enum Executor {
    Native,
    VM,
}

impl Display for Executor {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let msg = match self {
            Executor::Native => "Native",
            Executor::VM => "VM",
        };
        f.write_str(msg)
    }
}

impl FromStr for Executor {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "native" => Ok(Self::Native),
            "vm" => Ok(Self::VM),
            s => Err(format!("unsupported option: {s}")),
        }
    }
}