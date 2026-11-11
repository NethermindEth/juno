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

/-- The eight actual receipt-package tables, including Event and its enclosing
    receipt dependencies. Wire-name order is stated independently below. -/
inductive ReceiptSchema where
  | builtinInstanceCounter
  | dataAvailability
  | gasConsumed
  | executionResources
  | event
  | l1ToL2Message
  | l2ToL1Message
  | transactionReceipt

def receiptNames : ReceiptSchema → List GoString
  | .builtinInstanceCounter => [go!"Pedersen", go!"RangeCheck", go!"Bitwise", go!"Output", go!"Ecsda", go!"EcOp", go!"Keccak", go!"Poseidon", go!"SegmentArena", go!"AddMod", go!"MulMod", go!"RangeCheck96"]
  | .dataAvailability => [go!"L1Gas", go!"L1DataGas"]
  | .gasConsumed => [go!"L1Gas", go!"L1DataGas", go!"L2Gas"]
  | .executionResources => [go!"BuiltinInstanceCounter", go!"MemoryHoles", go!"Steps", go!"DataAvailability", go!"TotalGasConsumed"]
  | .event => [go!"From", go!"Keys", go!"Data"]
  | .l1ToL2Message => [go!"From", go!"Nonce", go!"Payload", go!"Selector", go!"To"]
  | .l2ToL1Message => [go!"From", go!"Payload", go!"To"]
  | .transactionReceipt => [go!"Fee", go!"FeeUnit", go!"Events", go!"ExecutionResources", go!"L1ToL2Message", go!"L2ToL1Message", go!"TransactionHash", go!"Reverted", go!"RevertReason"]

def receiptKeys : ReceiptSchema → List cursor.StaticField
  | .builtinInstanceCounter => [
      ⟨array.mk 3 [W64 0x704f634564, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dEcOp", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x616473634565, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eEcsda", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x646f4d64644166, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fAddMod", W64 9, W64 0x200, true⟩,
      ⟨array.mk 3 [W64 0x6b616363654b66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fKeccak", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x646f4d6c754d66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fMulMod", W64 10, W64 0x400, true⟩,
      ⟨array.mk 3 [W64 0x74757074754f66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fOutput", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x6573697774694267, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gBitwise", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6573726564655068, W64 0x6e, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hPedersen", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6f646965736f5068, W64 0x6e, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hPoseidon", W64 7, W64 0x80, true⟩,
      ⟨array.mk 3 [W64 0x684365676e61526a, W64 0x6b6365, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
       go!"jRangeCheck", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x684365676e61526c, W64 0x36396b6365, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
       go!"lRangeCheck96", W64 11, W64 0x800, true⟩,
      ⟨array.mk 3 [W64 0x746e656d6765536c, W64 0x616e657241, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
       go!"lSegmentArena", W64 8, W64 0x100, true⟩]
  | .dataAvailability => [
      ⟨array.mk 3 [W64 0x736147314c65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eL1Gas", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x4761746144314c69, W64 0x7361, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iL1DataGas", W64 1, W64 0x2, true⟩]
  | .gasConsumed => [
      ⟨array.mk 3 [W64 0x736147314c65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eL1Gas", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x736147324c65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eL2Gas", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x4761746144314c69, W64 0x7361, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iL1DataGas", W64 1, W64 0x2, true⟩]
  | .executionResources => [
      ⟨array.mk 3 [W64 0x737065745365, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eSteps", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x4879726f6d654d6b, W64 0x73656c6f, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kMemoryHoles", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6176416174614470, W64 0x74696c6962616c69, W64 0x79],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xff],
       go!"pDataAvailability", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x61476c61746f5470, W64 0x656d75736e6f4373, W64 0x64],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xff],
       go!"pTotalGasConsumed", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x6e69746c69754276, W64 0x65636e6174736e49, W64 0x7265746e756f43],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffffffff],
       go!"vBuiltinInstanceCounter", W64 0, W64 0x1, true⟩]
  | .event => [
      ⟨array.mk 3 [W64 0x6174614464, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dData", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6d6f724664, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dFrom", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x7379654b64, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dKeys", W64 1, W64 0x2, true⟩]
  | .l1ToL2Message => [
      ⟨array.mk 3 [W64 0x6f5462, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffff, W64 0x0, W64 0x0],
       go!"bTo", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x6d6f724664, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dFrom", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x65636e6f4e65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eNonce", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x64616f6c79615067, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gPayload", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6f7463656c655368, W64 0x72, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hSelector", W64 3, W64 0x8, true⟩]
  | .l2ToL1Message => [
      ⟨array.mk 3 [W64 0x6f5462, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffff, W64 0x0, W64 0x0],
       go!"bTo", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6d6f724664, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
       go!"dFrom", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x64616f6c79615067, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gPayload", W64 1, W64 0x2, true⟩]
  | .transactionReceipt => [
      ⟨array.mk 3 [W64 0x65654663, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffff, W64 0x0, W64 0x0],
       go!"cFee", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x73746e65764566, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fEvents", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x74696e5565654667, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gFeeUnit", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6574726576655268, W64 0x64, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hReverted", W64 7, W64 0x80, true⟩,
      ⟨array.mk 3 [W64 0x527472657665526c, W64 0x6e6f736165, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
       go!"lRevertReason", W64 8, W64 0x100, true⟩,
      ⟨array.mk 3 [W64 0x4d324c6f54314c6d, W64 0x656761737365, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mL1ToL2Message", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x4d314c6f54324c6d, W64 0x656761737365, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mL2ToL1Message", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x6974756365784572, W64 0x72756f7365526e6f, W64 0x736563],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffff],
       go!"rExecutionResources", W64 3, W64 0x8, true⟩]

def receiptMask (schema : ReceiptSchema) : w64 := (W64 1 <<< W64 (receiptNames schema).length) - W64 1

def receiptGlobal : ReceiptSchema → GoString
  | .builtinInstanceCounter => internal.cborreceipts.cborBuiltinInstanceCounterFields
  | .dataAvailability => internal.cborreceipts.cborDataAvailabilityFields
  | .gasConsumed => internal.cborreceipts.cborGasConsumedFields
  | .executionResources => internal.cborreceipts.cborExecutionResourcesFields
  | .event => internal.cborreceipts.cborEventFields
  | .l1ToL2Message => internal.cborreceipts.cborL1ToL2MessageFields
  | .l2ToL1Message => internal.cborreceipts.cborL2ToL1MessageFields
  | .transactionReceipt => internal.cborreceipts.cborTransactionReceiptFields

/-- Published table ownership includes the actual global and both immutable
    backing slices; calls may borrow it without duplicating mutable storage. -/
def ReceiptTable (schema : ReceiptSchema) : IProp GF := iprop(∃ fields : Loc,
  globalAddr (receiptGlobal schema) ↦□ fields ∗
  StaticFieldsShared fields (receiptKeys schema) (receiptNames schema) (receiptMask schema))

instance receiptTable_persistent (schema : ReceiptSchema) : Persistent (ReceiptTable (GF := GF) schema) := by
  unfold ReceiptTable
  infer_instance

def ReceiptMetadata : IProp GF := iprop(
  ReceiptTable .builtinInstanceCounter ∗
  ReceiptTable .dataAvailability ∗
  ReceiptTable .gasConsumed ∗
  ReceiptTable .executionResources ∗
  ReceiptTable .event ∗
  ReceiptTable .l1ToL2Message ∗
  ReceiptTable .l2ToL1Message ∗
  ReceiptTable .transactionReceipt)

instance receiptMetadata_persistent : Persistent (ReceiptMetadata (GF := GF)) := by
  unfold ReceiptMetadata
  infer_instance

instance receiptIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts :=
  define_is_pkg_init (ReceiptMetadata (GF := GF))
instance receiptGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts :=
  build_get_is_pkg_init_wf

/-- Cached words/masks, encoded keys, indices and selection bits agree with
    independently listed schema names for every receipt-package table. -/
theorem receiptKeys_valid (schema : ReceiptSchema) :
    (receiptKeys schema).all (fieldEntryValid (receiptNames schema)) = true := by
  cases schema <;> rfl

theorem receiptNames_small (schema : ReceiptSchema) : (receiptNames schema).length ≤ 64 := by
  cases schema <;> decide

theorem receiptNames_distinct (schema : ReceiptSchema) :
    ((receiptNames schema).map (List.map fieldFoldByte)).Nodup := by
  cases schema <;> decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptKeys_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptNames_small
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptNames_distinct
