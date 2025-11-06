use blockifier;
use blockifier::state::cached_state::{CachedState, StateMaps};
use blockifier::state::cached_state::{CommitmentStateDiff, TransactionalState};
use blockifier::state::errors::StateError;
use blockifier::state::state_api::StateReader;
use indexmap::IndexMap;
use serde::Serialize;
use starknet_api::core::ClassHash;
use starknet_types_core::felt::Felt;

use crate::state_reader::JunoStateReader;

#[derive(Serialize)]
struct Entry {
    key: Felt,
    value: Felt,
}
#[derive(Serialize)]
struct StorageDiff {
    address: Felt,
    storage_entries: Vec<Entry>,
}

#[derive(Serialize)]
struct Nonce {
    contract_address: Felt,
    nonce: Felt,
}

#[derive(Serialize)]
struct DeployedContract {
    address: Felt,
    class_hash: Felt,
}

#[derive(Serialize)]
struct ReplacedClass {
    contract_address: Felt,
    class_hash: Felt,
}

#[derive(Serialize)]
struct DeclaredClass {
    class_hash: Felt,
    compiled_class_hash: Felt,
}

#[derive(Serialize, Default)]
pub struct StateDiff {
    storage_diffs: Vec<StorageDiff>,
    nonces: Vec<Nonce>,
    deployed_contracts: Vec<DeployedContract>,
    deprecated_declared_classes: Vec<Felt>,
    declared_classes: Vec<DeclaredClass>,
    replaced_classes: Vec<ReplacedClass>,
}

impl From<StateMaps> for StateDiff {
    fn from(state_maps: StateMaps) -> Self {
        let storage_diffs = state_maps
            .storage
            .into_iter()
            .fold(
                IndexMap::<Felt, Vec<Entry>>::new(),
                |mut acc, ((address, key), value)| {
                    let starkfelt_address = address.into();
                    let entry = Entry {
                        key: key.into(),
                        value,
                    };

                    acc.entry(starkfelt_address)
                        .or_insert_with(Vec::new)
                        .push(entry);

                    acc
                },
            )
            .into_iter()
            .map(|(address, storage_entries)| StorageDiff {
                address,
                storage_entries,
            })
            .collect();

        let nonces = state_maps
            .nonces
            .into_iter()
            .map(|(address, nonce)| Nonce {
                contract_address: address.into(),
                nonce: *nonce,
            })
            .collect();

        let deployed_contracts = state_maps
            .class_hashes
            .into_iter()
            .map(|(address, class_hash)| DeployedContract {
                address: address.into(),
                class_hash: *class_hash,
            })
            .collect();

        let deprecated_declared_classes = state_maps
            .declared_contracts
            .into_iter()
            .filter(|(_, is_deprecated)| *is_deprecated)
            .map(|(class_hash, _)| *class_hash)
            .collect();

        let declared_classes = state_maps
            .compiled_class_hashes
            .into_iter()
            .map(|(class_hash, compiled_class_hash)| DeclaredClass {
                class_hash: *class_hash,
                compiled_class_hash: compiled_class_hash.0,
            })
            .collect();

        // Currently this field is unneeded, since we don't declare and
        // immediately replace a contracts class hash in a single block.
        // If we decide to support this field, then we could handle it
        // in the genesis pkg, since there is no corresponding field in StateMaps.
        // Only the genesis pkg ever uses this logic.
        let replaced_classes = Default::default();

        Self {
            storage_diffs,
            nonces,
            deployed_contracts,
            deprecated_declared_classes,
            declared_classes,
            replaced_classes,
        }
    }
}

pub fn make_state_diff(
    state: &mut TransactionalState<CachedState<JunoStateReader>>,
    deprecated_declared_class_hash: Option<ClassHash>,
) -> Result<StateDiff, StateError> {
    let diff: CommitmentStateDiff = state.to_state_diff()?.state_maps.into();
    let mut deployed_contracts = Vec::new();
    let mut replaced_classes = Vec::new();

    for (addr, class_hash) in diff.address_to_class_hash {
        let existing_class_hash = state.state.get_class_hash_at(addr)?;
        let addr: Felt = addr.into();

        if existing_class_hash == ClassHash::default() {
            deployed_contracts.push(DeployedContract {
                address: addr,
                class_hash: class_hash.0,
            });
        } else {
            replaced_classes.push(ReplacedClass {
                contract_address: addr,
                class_hash: class_hash.0,
            });
        }
    }

    let mut deprecated_declared_class_hashes = Vec::default();
    if let Some(v) = deprecated_declared_class_hash {
        deprecated_declared_class_hashes.push(v.0)
    }
    Ok(StateDiff {
        deployed_contracts,
        storage_diffs: diff
            .storage_updates
            .into_iter()
            .map(|v| StorageDiff {
                address: *v.0 .0.key(),
                storage_entries: v
                    .1
                    .into_iter()
                    .map(|e| Entry {
                        key: *e.0 .0.key(),
                        value: e.1,
                    })
                    .collect(),
            })
            .collect(),
        declared_classes: diff
            .class_hash_to_compiled_class_hash
            .into_iter()
            .map(|v| DeclaredClass {
                class_hash: v.0 .0,
                compiled_class_hash: v.1 .0,
            })
            .collect(),
        deprecated_declared_classes: deprecated_declared_class_hashes,
        #[rustfmt::skip]
        nonces: diff.address_to_nonce.into_iter().map(| v | Nonce {
          contract_address: *v.0.0.key(),
          nonce: v.1.0,
        }).collect(),
        replaced_classes,
    })
}
