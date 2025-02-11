use starknet_types_core::felt::Felt;
use std::{
    ffi::{c_char, c_uchar, c_ulonglong},
    str::FromStr,
};

#[repr(C)]
#[derive(Clone, Copy)]
pub struct BlockInfo {
    pub block_number: c_ulonglong,
    pub block_timestamp: c_ulonglong,
    pub sequencer_address: [c_uchar; 32],
    pub l1_gas_price_wei: [c_uchar; 32],
    pub l1_gas_price_fri: [c_uchar; 32],
    pub version: *const c_char,
    pub block_hash_to_be_revealed: [c_uchar; 32],
    pub l1_data_gas_price_wei: [c_uchar; 32],
    pub l1_data_gas_price_fri: [c_uchar; 32],
    pub use_blob_data: c_uchar,
    pub l2_gas_price_wei: [c_uchar; 32],
    pub l2_gas_price_fri: [c_uchar; 32],
}

#[repr(C)]
#[derive(Clone, Copy)]
pub struct CallInfo {
    pub contract_address: [c_uchar; 32],
    pub class_hash: [c_uchar; 32],
    pub entry_point_selector: [c_uchar; 32],
    pub calldata: *const *const c_uchar,
    pub len_calldata: usize,
}

pub type StarkFelt = Felt;

#[derive(Default, PartialEq, Eq, PartialOrd, Ord)]
pub struct StarknetVersion(u8, u8, u8, u8);

impl StarknetVersion {
    pub const fn new(a: u8, b: u8, c: u8, d: u8) -> Self {
        StarknetVersion(a, b, c, d)
    }
}

impl FromStr for StarknetVersion {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        if s.is_empty() {
            return Ok(StarknetVersion::new(0, 0, 0, 0));
        }

        let parts: Vec<_> = s.split('.').collect();
        anyhow::ensure!(
            parts.len() == 3 || parts.len() == 4,
            "Invalid version string, expected 3 or 4 parts but got {}",
            parts.len()
        );

        let a = parts[0].parse()?;
        let b = parts[1].parse()?;
        let c = parts[2].parse()?;
        let d = parts.get(3).map(|x| x.parse()).transpose()?.unwrap_or(0);

        Ok(StarknetVersion(a, b, c, d))
    }
}
