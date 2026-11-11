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

/-- The seven actual tables owned by the projection package. Names are stated
    independently of the generated factory expressions. -/
inductive ProjectionSchema where
  | headerHash
  | headerGlobalStateRoot
  | headerTransactionCount
  | headerTimestamp
  | headerHashAndStateRoot
  | receiptExecutionStatus
  | receiptEvents

def projectionNames : ProjectionSchema → List GoString
  | .headerHash => [go!"Hash"]
  | .headerGlobalStateRoot => [go!"GlobalStateRoot"]
  | .headerTransactionCount => [go!"TransactionCount"]
  | .headerTimestamp => [go!"Timestamp"]
  | .headerHashAndStateRoot => [go!"Hash", go!"GlobalStateRoot"]
  | .receiptExecutionStatus => [go!"Reverted", go!"RevertReason"]
  | .receiptEvents => [go!"Events", go!"TransactionHash"]

def projectionKeys : ProjectionSchema → List cursor.StaticField
  | .headerHash => [
      ⟨array.mk 3 [W64 0x6873614864, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dHash", W64 0, W64 1, true⟩]
  | .headerGlobalStateRoot => [
      ⟨array.mk 3 [W64 0x536c61626f6c476f, W64 0x746f6f5265746174, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oGlobalStateRoot", W64 0, W64 1, true⟩]
  | .headerTransactionCount => [
      ⟨array.mk 3 [W64 0x6361736e61725470, W64 0x6e756f436e6f6974, W64 0x74],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xff],
       go!"pTransactionCount", W64 0, W64 1, true⟩]
  | .headerTimestamp => [
      ⟨array.mk 3 [W64 0x617473656d695469, W64 0x706d, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iTimestamp", W64 0, W64 1, true⟩]
  | .headerHashAndStateRoot => [
      ⟨array.mk 3 [W64 0x6873614864, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dHash", W64 0, W64 1, true⟩,
      ⟨array.mk 3 [W64 0x536c61626f6c476f, W64 0x746f6f5265746174, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oGlobalStateRoot", W64 1, W64 2, true⟩]
  | .receiptExecutionStatus => [
      ⟨array.mk 3 [W64 0x6574726576655268, W64 0x64, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hReverted", W64 0, W64 1, true⟩,
      ⟨array.mk 3 [W64 0x527472657665526c, W64 0x6e6f736165, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
       go!"lRevertReason", W64 1, W64 2, true⟩]
  | .receiptEvents => [
      ⟨array.mk 3 [W64 0x73746e65764566, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fEvents", W64 0, W64 1, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 1, W64 2, true⟩]

def projectionMask (schema : ProjectionSchema) : w64 :=
  (W64 1 <<< W64 (projectionNames schema).length) - W64 1

def projectionGlobal : ProjectionSchema → GoString
  | .headerHash => internal.cborprojections.cborHeaderHashProjectionFields
  | .headerGlobalStateRoot => internal.cborprojections.cborHeaderGlobalStateRootProjectionFields
  | .headerTransactionCount => internal.cborprojections.cborHeaderTransactionCountProjectionFields
  | .headerTimestamp => internal.cborprojections.cborHeaderTimestampProjectionFields
  | .headerHashAndStateRoot => internal.cborprojections.cborHeaderHashAndStateRootProjectionFields
  | .receiptExecutionStatus => internal.cborprojections.cborReceiptExecutionStatusProjectionFields
  | .receiptEvents => internal.cborprojections.cborReceiptEventsProjectionFields

/-- An initialized table owns both its published pointer and exact immutable
    backing storage. The record reader may borrow it repeatedly. -/
def ProjectionTable (schema : ProjectionSchema) : IProp GF := iprop(∃ fields : Loc,
  globalAddr (projectionGlobal schema) ↦□ fields ∗
  StaticFieldsShared fields (projectionKeys schema) (projectionNames schema) (projectionMask schema))

instance projectionTable_persistent (schema : ProjectionSchema) :
    Persistent (ProjectionTable (GF := GF) schema) := by
  unfold ProjectionTable
  infer_instance

def ProjectionMetadata : IProp GF := iprop(
  ProjectionTable .headerHash ∗
  ProjectionTable .headerGlobalStateRoot ∗
  ProjectionTable .headerTransactionCount ∗
  ProjectionTable .headerTimestamp ∗
  ProjectionTable .headerHashAndStateRoot ∗
  ProjectionTable .receiptExecutionStatus ∗
  ProjectionTable .receiptEvents)

instance projectionMetadata_persistent : Persistent (ProjectionMetadata (GF := GF)) := by
  unfold ProjectionMetadata
  infer_instance

instance projectionIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections :=
  define_is_pkg_init (ProjectionMetadata (GF := GF))
instance projectionGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections :=
  build_get_is_pkg_init_wf

/-- Every cached word, mask, encoded key, field index and selection bit agrees
    with the independently listed wire names. -/
theorem projectionKeys_valid (schema : ProjectionSchema) :
    (projectionKeys schema).all (fieldEntryValid (projectionNames schema)) = true := by
  cases schema <;> rfl

theorem projectionNames_small (schema : ProjectionSchema) :
    (projectionNames schema).length ≤ 64 := by cases schema <;> decide

theorem projectionNames_distinct (schema : ProjectionSchema) :
    ((projectionNames schema).map (List.map fieldFoldByte)).Nodup := by
  cases schema <;> decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionKeys_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionNames_small
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionNames_distinct
