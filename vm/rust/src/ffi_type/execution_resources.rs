use ::serde::Serialize;
use cairo_vm::{
    types::builtin_name::BuiltinName,
    vm::runners::cairo_runner::ExecutionResources as VmExecutionResources,
};
use starknet_api::execution_resources::GasVector;

#[derive(Serialize, Default)]
pub struct ExecutionResources {
    pub steps: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory_holes: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub range_check_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pedersen_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub poseidon_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ec_op_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ecdsa_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bitwise_builtin_applications: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub keccak_builtin_applications: Option<usize>,
    // https://github.com/starkware-libs/starknet-specs/pull/167
    #[serde(skip_serializing_if = "Option::is_none")]
    pub segment_arena_builtin: Option<usize>,
    pub l1_gas: u128,
    pub l2_gas: u128,
}

impl ExecutionResources {
    pub fn from_resources_and_gas_vector(
        vm_execution_resources: VmExecutionResources,
        gas_vector: GasVector,
    ) -> Self {
        ExecutionResources {
            steps: vm_execution_resources.n_steps,
            memory_holes: if vm_execution_resources.n_memory_holes > 0 {
                Some(vm_execution_resources.n_memory_holes)
            } else {
                None
            },
            range_check_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::range_check)
                .cloned(),
            pedersen_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::pedersen)
                .cloned(),
            poseidon_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::poseidon)
                .cloned(),
            ec_op_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::ec_op)
                .cloned(),
            ecdsa_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::ecdsa)
                .cloned(),
            bitwise_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::bitwise)
                .cloned(),
            keccak_builtin_applications: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::keccak)
                .cloned(),
            segment_arena_builtin: vm_execution_resources
                .builtin_instance_counter
                .get(&BuiltinName::segment_arena)
                .cloned(),
            l1_gas: gas_vector.l1_gas.0.into(),
            l2_gas: gas_vector.l2_gas.0.into(),
        }
    }
}
