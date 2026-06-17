use starknet_types_core::felt::Felt;
use starknet_types_core::hash::Poseidon;

/// juno_hades_permutation applies the Hades permutation in place over raw
/// Montgomery limbs. `state` points to 12 u64: three field elements, each four
/// little-endian limbs in gnark's Montgomery form (R = 2^256). lambdaworks
/// stores Montgomery limbs big-endian for the same R, so we only reverse the
/// limb order; there is no byte/Montgomery conversion and no heap allocation.
///
/// # Safety
/// `state` must be a valid, non-null pointer to at least 12 readable/writable u64.
#[no_mangle]
pub unsafe extern "C" fn juno_hades_permutation(state: *mut u64) {
    let limbs = std::slice::from_raw_parts_mut(state, 12);
    let mut s = [Felt::ZERO; 3];
    for (i, felt) in s.iter_mut().enumerate() {
        let mut m = [limbs[i * 4], limbs[i * 4 + 1], limbs[i * 4 + 2], limbs[i * 4 + 3]];
        m.reverse();
        *felt = Felt::from_raw(m);
    }
    Poseidon::hades_permutation(&mut s);
    for (i, felt) in s.iter().enumerate() {
        let mut m = felt.to_raw();
        m.reverse();
        limbs[i * 4..i * 4 + 4].copy_from_slice(&m);
    }
}
