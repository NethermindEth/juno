use anyhow::Context;
use blockifier::versioned_constants::VersionedConstants;

// https://github.com/eqlabs/pathfinder/blob/main/crates/executor/src/execution_state.rs#L24
pub mod versioned_constants {

    use super::VersionedConstants;

    const BLOCKIFIER_VERSIONED_CONSTANTS_JSON_0_13_0: &[u8] =
        include_bytes!("../versioned_constants_13_0.json");

    const STARKNET_VERSION_MATCHING_LATEST_BLOCKIFIER: semver::Version =
        semver::Version::new(0, 13, 1);

    lazy_static::lazy_static! {
        pub static ref BLOCKIFIER_VERSIONED_CONSTANTS_0_13_0: VersionedConstants =
            serde_json::from_slice(BLOCKIFIER_VERSIONED_CONSTANTS_JSON_0_13_0).unwrap();
    }

    pub fn for_version(
        version: &super::StarknetVersion,
    ) -> anyhow::Result<&'static VersionedConstants> {
        let versioned_constants = match version.parse_as_semver()? {
            Some(version) => {
                // Right now we only properly support two versions: 0.13.0 and 0.13.1.
                // We use 0.13.0 for all blocks _before_ 0.13.1.
                if version < STARKNET_VERSION_MATCHING_LATEST_BLOCKIFIER {
                    &BLOCKIFIER_VERSIONED_CONSTANTS_0_13_0
                } else {
                    VersionedConstants::latest_constants()
                }
            }
            // The default for blocks that don't have a version number.
            // This is supposed to be the _oldest_ version we support.
            None => &BLOCKIFIER_VERSIONED_CONSTANTS_0_13_0,
        };

        Ok(versioned_constants)
    }
}


// https://github.com/eqlabs/pathfinder/blob/main/crates/common/src/lib.rs#L422
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct StarknetVersion(String);

impl StarknetVersion {
   

    /// Parses the version string.
    ///
    /// Note: there are known deviations from semver such as version 0.11.0.2, which
    /// will be truncated to 0.11.0 to still allow for parsing.
    pub fn parse_as_semver(&self) -> anyhow::Result<Option<semver::Version>> {
        // Truncate the 4th segment if present. This is a work-around for semver violating
        // version strings like `0.11.0.2`.
        let str = if self.0.is_empty() {
            return Ok(None);
        } else {
            &self.0
        };
        let truncated = str
            .match_indices('.')
            .nth(2)
            .map(|(index, _)| str.split_at(index).0)
            .unwrap_or(str);

        Some(semver::Version::parse(truncated).context("Parsing semver string")).transpose()
    }

}

impl From<String> for StarknetVersion {
    fn from(value: String) -> Self {
        Self(value)
    }
}
impl StarknetVersion{
    pub fn from_str(value: &str) -> Self {
        Self(value.to_string())
    }
}