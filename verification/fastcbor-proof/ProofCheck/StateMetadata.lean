import ProofCheck.InitializationState
import ProofCheck.Fields
import ProofCheck.FieldFold

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Exact implementation metadata for the 2 state-package tables.
    These tables do not by themselves specify or prove record payload decoding. -/
inductive StateSchema where
  | stateDiff
  | stateUpdate

def stateNames : StateSchema → List GoString
  | .stateDiff => [go!"StorageDiffs", go!"Nonces", go!"DeployedContracts", go!"DeclaredV0Classes", go!"DeclaredV1Classes", go!"ReplacedClasses", go!"MigratedClasses"]
  | .stateUpdate => [go!"BlockHash", go!"NewRoot", go!"OldRoot", go!"StateDiff"]

def stateKeys : StateSchema → List cursor.StaticField
  | .stateDiff => [
      ⟨array.mk 3 [W64 0x7365636e6f4e66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fNonces", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x656761726f74536c, W64 0x7366666944, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
       go!"lStorageDiffs", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6574617267694d6f, W64 0x73657373616c4364, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oMigratedClasses", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x6563616c7065526f, W64 0x73657373616c4364, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oReplacedClasses", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x6572616c63654471, W64 0x7373616c43305664, W64 0x7365],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffff],
       go!"qDeclaredV0Classes", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x6572616c63654471, W64 0x7373616c43315664, W64 0x7365],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffff],
       go!"qDeclaredV1Classes", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x65796f6c70654471, W64 0x636172746e6f4364, W64 0x7374],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffff],
       go!"qDeployedContracts", W64 2, W64 0x4, true⟩]
  | .stateUpdate => [
      ⟨array.mk 3 [W64 0x746f6f5277654e67, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gNewRoot", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x746f6f52646c4f67, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gOldRoot", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x61486b636f6c4269, W64 0x6873, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iBlockHash", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6944657461745369, W64 0x6666, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iStateDiff", W64 3, W64 0x8, true⟩]

def stateMask (schema : StateSchema) : w64 := (W64 1 <<< W64 (stateNames schema).length) - W64 1

def stateGlobal : StateSchema → GoString
  | .stateDiff => internal.cborstate.cborStateDiffFields
  | .stateUpdate => internal.cborstate.cborStateUpdateFields
/-- Published table ownership includes the actual global and both immutable
    backing slices; calls may borrow it without duplicating mutable storage. -/
def StateTable (schema : StateSchema) : IProp GF := iprop(∃ fields : Loc,
  globalAddr (stateGlobal schema) ↦□ fields ∗
  StaticFieldsShared fields (stateKeys schema) (stateNames schema) (stateMask schema))

instance stateTable_persistent (schema : StateSchema) : Persistent (StateTable (GF := GF) schema) := by
  unfold StateTable
  infer_instance

def StateMetadata : IProp GF := iprop(
  StateTable .stateDiff ∗
  StateTable .stateUpdate)

instance stateMetadata_persistent : Persistent (StateMetadata (GF := GF)) := by
  unfold StateMetadata
  infer_instance

instance stateIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate :=
  define_is_pkg_init (StateMetadata (GF := GF))
instance stateGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate :=
  build_get_is_pkg_init_wf

/-- Cached words/masks, encoded keys, indices and selection bits agree with
    explicitly listed metadata names for every state-package table. -/
theorem stateKeys_valid (schema : StateSchema) :
    (stateKeys schema).all (fieldEntryValid (stateNames schema)) = true := by
  cases schema <;> rfl

theorem stateNames_small (schema : StateSchema) : (stateNames schema).length ≤ 64 := by
  cases schema <;> decide

theorem stateNames_distinct (schema : StateSchema) :
    ((stateNames schema).map (List.map fieldFoldByte)).Nodup := by
  cases schema <;> decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateKeys_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.stateNames_small
#print axioms Perennial.github_com.NethermindEth.juno.core.stateNames_distinct
