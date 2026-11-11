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

/-- Exact immutable implementation metadata for all seven transaction tables.
    Record-level application contracts and key matching are proved separately. -/
inductive TransactionSchema where
  | resourceBounds
  | declareTransaction
  | deployTransaction
  | invokeTransaction
  | l1HandlerTransaction
  | deployAccountTransaction
  | transactionHashProjection

def transactionNames : TransactionSchema → List GoString
  | .resourceBounds => [go!"MaxAmount", go!"MaxPricePerUnit"]
  | .declareTransaction => [go!"TransactionHash", go!"ClassHash", go!"SenderAddress", go!"MaxFee", go!"TransactionSignature", go!"Nonce", go!"Version", go!"CompiledClassHash", go!"ResourceBounds", go!"Tip", go!"PaymasterData", go!"AccountDeploymentData", go!"NonceDAMode", go!"FeeDAMode"]
  | .deployTransaction => [go!"TransactionHash", go!"ContractAddressSalt", go!"ContractAddress", go!"ClassHash", go!"ConstructorCallData", go!"Version"]
  | .invokeTransaction => [go!"TransactionHash", go!"CallData", go!"TransactionSignature", go!"MaxFee", go!"ContractAddress", go!"Version", go!"EntryPointSelector", go!"Nonce", go!"SenderAddress", go!"ResourceBounds", go!"Tip", go!"PaymasterData", go!"AccountDeploymentData", go!"NonceDAMode", go!"FeeDAMode", go!"ProofFacts"]
  | .l1HandlerTransaction => [go!"TransactionHash", go!"ContractAddress", go!"EntryPointSelector", go!"Nonce", go!"CallData", go!"Version"]
  | .deployAccountTransaction => [go!"TransactionHash", go!"ContractAddressSalt", go!"ContractAddress", go!"ClassHash", go!"ConstructorCallData", go!"Version", go!"MaxFee", go!"TransactionSignature", go!"Nonce", go!"ResourceBounds", go!"Tip", go!"PaymasterData", go!"NonceDAMode", go!"FeeDAMode"]
  | .transactionHashProjection => [go!"TransactionHash"]

def transactionKeys : TransactionSchema → List cursor.StaticField
  | .resourceBounds => [
      ⟨array.mk 3 [W64 0x756f6d4178614d69, W64 0x746e, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iMaxAmount", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6369725078614d6f, W64 0x74696e5572655065, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oMaxPricePerUnit", W64 1, W64 0x2, true⟩]
  | .declareTransaction => [
      ⟨array.mk 3 [W64 0x70695463, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffff, W64 0x0, W64 0x0],
       go!"cTip", W64 9, W64 0x200, true⟩,
      ⟨array.mk 3 [W64 0x65636e6f4e65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eNonce", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x65654678614d66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fMaxFee", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x6e6f697372655667, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gVersion", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x61487373616c4369, W64 0x6873, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iClassHash", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6f4d414465654669, W64 0x6564, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iFeeDAMode", W64 13, W64 0x2000, true⟩,
      ⟨array.mk 3 [W64 0x414465636e6f4e6b, W64 0x65646f4d, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kNonceDAMode", W64 12, W64 0x1000, true⟩,
      ⟨array.mk 3 [W64 0x7473616d7961506d, W64 0x617461447265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mPaymasterData", W64 10, W64 0x400, true⟩,
      ⟨array.mk 3 [W64 0x417265646e65536d, W64 0x737365726464, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mSenderAddress", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6372756f7365526e, W64 0x73646e756f4265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffff, W64 0x0],
       go!"nResourceBounds", W64 8, W64 0x100, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x656c69706d6f4371, W64 0x61487373616c4364, W64 0x6873],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffff],
       go!"qCompiledClassHash", W64 7, W64 0x80, true⟩,
      ⟨array.mk 3 [W64 0x6361736e61725474, W64 0x6e6769536e6f6974, W64 0x6572757461],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffff],
       go!"tTransactionSignature", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x746e756f63634175, W64 0x656d796f6c706544, W64 0x61746144746e],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffffff],
       go!"uAccountDeploymentData", W64 11, W64 0x800, true⟩]
  | .deployTransaction => [
      ⟨array.mk 3 [W64 0x6e6f697372655667, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gVersion", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x61487373616c4369, W64 0x6873, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iClassHash", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x636172746e6f436f, W64 0x7373657264644174, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oContractAddress", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x757274736e6f4373, W64 0x6c6c6143726f7463, W64 0x61746144],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffff],
       go!"sConstructorCallData", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x636172746e6f4373, W64 0x7373657264644174, W64 0x746c6153],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffff],
       go!"sContractAddressSalt", W64 1, W64 0x2, true⟩]
  | .invokeTransaction => [
      ⟨array.mk 3 [W64 0x70695463, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffff, W64 0x0, W64 0x0],
       go!"cTip", W64 10, W64 0x400, true⟩,
      ⟨array.mk 3 [W64 0x65636e6f4e65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eNonce", W64 7, W64 0x80, true⟩,
      ⟨array.mk 3 [W64 0x65654678614d66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fMaxFee", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x6e6f697372655667, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gVersion", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x7461446c6c614368, W64 0x61, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hCallData", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6f4d414465654669, W64 0x6564, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iFeeDAMode", W64 14, W64 0x4000, true⟩,
      ⟨array.mk 3 [W64 0x6146666f6f72506a, W64 0x737463, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
       go!"jProofFacts", W64 15, W64 0x8000, true⟩,
      ⟨array.mk 3 [W64 0x414465636e6f4e6b, W64 0x65646f4d, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kNonceDAMode", W64 13, W64 0x2000, true⟩,
      ⟨array.mk 3 [W64 0x7473616d7961506d, W64 0x617461447265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mPaymasterData", W64 11, W64 0x800, true⟩,
      ⟨array.mk 3 [W64 0x417265646e65536d, W64 0x737365726464, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mSenderAddress", W64 8, W64 0x100, true⟩,
      ⟨array.mk 3 [W64 0x6372756f7365526e, W64 0x73646e756f4265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffff, W64 0x0],
       go!"nResourceBounds", W64 9, W64 0x200, true⟩,
      ⟨array.mk 3 [W64 0x636172746e6f436f, W64 0x7373657264644174, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oContractAddress", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6f507972746e4572, W64 0x63656c6553746e69, W64 0x726f74],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffff],
       go!"rEntryPointSelector", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x6361736e61725474, W64 0x6e6769536e6f6974, W64 0x6572757461],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffff],
       go!"tTransactionSignature", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x746e756f63634175, W64 0x656d796f6c706544, W64 0x61746144746e],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffffff],
       go!"uAccountDeploymentData", W64 12, W64 0x1000, true⟩]
  | .l1HandlerTransaction => [
      ⟨array.mk 3 [W64 0x65636e6f4e65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eNonce", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x6e6f697372655667, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gVersion", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x7461446c6c614368, W64 0x61, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hCallData", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x636172746e6f436f, W64 0x7373657264644174, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oContractAddress", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6f507972746e4572, W64 0x63656c6553746e69, W64 0x726f74],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffff],
       go!"rEntryPointSelector", W64 2, W64 0x4, true⟩]
  | .deployAccountTransaction => [
      ⟨array.mk 3 [W64 0x70695463, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffff, W64 0x0, W64 0x0],
       go!"cTip", W64 10, W64 0x400, true⟩,
      ⟨array.mk 3 [W64 0x65636e6f4e65, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eNonce", W64 8, W64 0x100, true⟩,
      ⟨array.mk 3 [W64 0x65654678614d66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fMaxFee", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x6e6f697372655667, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gVersion", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x61487373616c4369, W64 0x6873, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iClassHash", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x6f4d414465654669, W64 0x6564, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iFeeDAMode", W64 13, W64 0x2000, true⟩,
      ⟨array.mk 3 [W64 0x414465636e6f4e6b, W64 0x65646f4d, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kNonceDAMode", W64 12, W64 0x1000, true⟩,
      ⟨array.mk 3 [W64 0x7473616d7961506d, W64 0x617461447265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mPaymasterData", W64 11, W64 0x800, true⟩,
      ⟨array.mk 3 [W64 0x6372756f7365526e, W64 0x73646e756f4265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffff, W64 0x0],
       go!"nResourceBounds", W64 9, W64 0x200, true⟩,
      ⟨array.mk 3 [W64 0x636172746e6f436f, W64 0x7373657264644174, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oContractAddress", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x757274736e6f4373, W64 0x6c6c6143726f7463, W64 0x61746144],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffff],
       go!"sConstructorCallData", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x636172746e6f4373, W64 0x7373657264644174, W64 0x746c6153],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffff],
       go!"sContractAddressSalt", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6361736e61725474, W64 0x6e6769536e6f6974, W64 0x6572757461],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffff],
       go!"tTransactionSignature", W64 7, W64 0x80, true⟩]
  | .transactionHashProjection => [
      ⟨array.mk 3 [W64 0x6361736e6172546f, W64 0x687361486e6f6974, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oTransactionHash", W64 0, W64 0x1, true⟩]

def transactionMask (schema : TransactionSchema) : w64 := (W64 1 <<< W64 (transactionNames schema).length) - W64 1

def transactionGlobal : TransactionSchema → GoString
  | .resourceBounds => internal.cbortransactions.cborResourceBoundsFields
  | .declareTransaction => internal.cbortransactions.cborDeclareTransactionFields
  | .deployTransaction => internal.cbortransactions.cborDeployTransactionFields
  | .invokeTransaction => internal.cbortransactions.cborInvokeTransactionFields
  | .l1HandlerTransaction => internal.cbortransactions.cborL1HandlerTransactionFields
  | .deployAccountTransaction => internal.cbortransactions.cborDeployAccountTransactionFields
  | .transactionHashProjection => internal.cbortransactions.cborTransactionHashProjectionFields
/-- Published table ownership includes the actual global and both immutable
    backing slices; calls may borrow it without duplicating mutable storage. -/
def TransactionTable (schema : TransactionSchema) : IProp GF := iprop(∃ fields : Loc,
  globalAddr (transactionGlobal schema) ↦□ fields ∗
  StaticFieldsShared fields (transactionKeys schema) (transactionNames schema) (transactionMask schema))

instance transactionTable_persistent (schema : TransactionSchema) : Persistent (TransactionTable (GF := GF) schema) := by
  unfold TransactionTable
  infer_instance

def TransactionMetadata : IProp GF := iprop(
  TransactionTable .resourceBounds ∗
  TransactionTable .declareTransaction ∗
  TransactionTable .deployTransaction ∗
  TransactionTable .invokeTransaction ∗
  TransactionTable .l1HandlerTransaction ∗
  TransactionTable .deployAccountTransaction ∗
  TransactionTable .transactionHashProjection)

instance transactionMetadata_persistent : Persistent (TransactionMetadata (GF := GF)) := by
  unfold TransactionMetadata
  infer_instance

instance transactionIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions :=
  define_is_pkg_init (TransactionMetadata (GF := GF))
instance transactionGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions :=
  build_get_is_pkg_init_wf

/-- Cached words/masks, encoded keys, indices and selection bits agree with
    explicitly listed metadata names for every transaction-package table. -/
theorem transactionKeys_valid (schema : TransactionSchema) :
    (transactionKeys schema).all (fieldEntryValid (transactionNames schema)) = true := by
  cases schema <;> rfl

theorem transactionNames_small (schema : TransactionSchema) : (transactionNames schema).length ≤ 64 := by
  cases schema <;> decide

theorem transactionNames_distinct (schema : TransactionSchema) :
    ((transactionNames schema).map (List.map fieldFoldByte)).Nodup := by
  cases schema <;> decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionKeys_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionNames_small
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionNames_distinct
