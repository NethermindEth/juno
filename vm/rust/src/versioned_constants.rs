use std::{collections::BTreeMap, path::Path};

use anyhow::{Context, Result};
use blockifier::blockifier_versioned_constants::VersionedConstants;
use starknet_api::block::StarknetVersion;

// this is wrong, it is more like a global variable
pub static mut CUSTOM_VERSIONED_CONSTANTS: Option<VersionedConstantsMap> = None;

#[derive(Debug)]
pub struct VersionedConstantsMap(pub BTreeMap<StarknetVersion, VersionedConstants>);

impl VersionedConstantsMap {
    pub fn from_file(version_with_path: BTreeMap<String, String>) -> Result<Self> {
        let mut result = BTreeMap::new();

        for (version, path) in version_with_path {
            let constants = VersionedConstants::from_path(Path::new(&path))
                .with_context(|| format!("Failed to parse JSON in file: {path}"))?;

            let parsed_version = StarknetVersion::try_from(version.as_str())
                .with_context(|| format!("Failed to parse version string: {version}"))?;

            result.insert(parsed_version, constants);
        }

        Ok(VersionedConstantsMap(result))
    }
}
