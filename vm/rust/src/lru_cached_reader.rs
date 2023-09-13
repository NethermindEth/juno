use starknet_api::{
    core::{ClassHash as StarknetClassHash, CompiledClassHash, ContractAddress, Nonce},
    hash::StarkFelt,
    state::StorageKey,
    StarknetApiError,
};

use cached::{Cached, SizedCache};
use std::sync::{MutexGuard, Mutex, Arc};

use blockifier::{
    execution::contract_class::ContractClass,
    state::{
        cached_state::GlobalContractCache, errors::StateError, state_api::StateReader,
        state_api::StateResult,
    },
};

// lazy_static::lazy_static!(
//
// );
static CONTRACT_CACHE: GlobalContractCache = {
    let inner = SizedCache::with_size(16 * 1024 * 1024);
    GlobalContractCache(Arc::new(Mutex::new(inner)))
};

pub(super) struct LruCachedReader<R>
    where
        R: StateReader,
{
    compiled_class_cache: GlobalContractCache,
    inner_reader: R,
}

impl<R> LruCachedReader<R>
    where
        R: StateReader,
{
    pub fn new(inner_reader: R) -> Self {
        Self {
            compiled_class_cache: CONTRACT_CACHE.clone(),
            inner_reader,
        }
    }

    fn locked_cache(
        &mut self,
    ) -> StateResult<MutexGuard<'_, SizedCache<StarknetClassHash, ContractClass>>> {
        self.compiled_class_cache.0.lock().map_err(|err| {
            warn!("Contract class cache lock is poisoned. Cause: {}.", err);
            StateError::StateReadError("Poisoned lock".to_string())
        })
    }
}

impl<R> StateReader for LruCachedReader<R>
    where
        R: StateReader,
{
    fn get_storage_at(
        &mut self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<StarkFelt> {
        self.inner_reader.get_storage_at(contract_address, key)
    }

    fn get_nonce_at(&mut self, contract_address: ContractAddress) -> StateResult<Nonce> {
        self.inner_reader.get_nonce_at(contract_address)
    }

    fn get_class_hash_at(
        &mut self,
        contract_address: ContractAddress,
    ) -> StateResult<StarknetClassHash> {
        self.inner_reader.get_class_hash_at(contract_address)
    }

    fn get_compiled_contract_class(
        &mut self,
        class_hash: &StarknetClassHash,
    ) -> StateResult<ContractClass> {
        // Check the cache, if not found then lookup & insert.
        // Because the lookup can take quite a lot of time and classes are insert-only, it's better
        // to separate the get & set operations and release the lock in the meantime.
        if let Some(contract_class) = self.locked_cache()?.cache_get(class_hash) {
            return Ok(contract_class.clone());
        }

        let contract_class = self.inner_reader.get_compiled_contract_class(class_hash)?;

        self.locked_cache()?
            .cache_set(*class_hash, contract_class.clone());

        Ok(contract_class)
    }

    fn get_compiled_class_hash(
        &mut self,
        class_hash: StarknetClassHash,
    ) -> StateResult<CompiledClassHash> {
        self.inner_reader.get_compiled_class_hash(class_hash)
    }
}