use std::{collections::BTreeMap, path::Path};

use anyhow::Result;
use blockifier::blockifier_versioned_constants::VersionedConstants;
use once_cell::sync::Lazy;
use starknet_api::block::StarknetVersion;
use starknet_types_core::felt::Felt;

// Allow users to call CONSTRUCTOR entry point type which has fixed entry_point_felt
// "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"
pub static CONSTRUCTOR_ENTRY_POINT_FELT: Lazy<Felt> = Lazy::new(|| {
    Felt::from_hex("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194")
        .expect("Invalid hex string")
});

// this is wrong, it is more like a global variable
pub static mut CUSTOM_VERSIONED_CONSTANTS: Option<VersionedConstantsMap> = None;

#[derive(Debug)]
pub struct VersionedConstantsMap(pub BTreeMap<StarknetVersion, VersionedConstants>);

impl VersionedConstantsMap {
    pub fn from_file(version_with_path: BTreeMap<String, String>) -> Result<Self> {
        let mut result = BTreeMap::new();

        for (version, path) in version_with_path {
            let constants = VersionedConstants::from_path(Path::new(&path))
                .with_context(|| format!("Failed to parse JSON in file: {}", path))?;

            let parsed_version = StarknetVersion::try_from(version.as_str())
                .with_context(|| format!("Failed to parse version string: {}", version))?;

            result.insert(parsed_version, constants);
        }

        Ok(VersionedConstantsMap(result))
    }
}
