import ProofCheck.GeneratedProof.github_com.NethermindEth.juno.core

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def gasFieldNames : List GoString := [go!"PriceInWei", go!"PriceInFri"]

def gasFieldKeys : List cursor.StaticField := [
  ⟨array.mk 3 [W64 0x6e4965636972506a, W64 0x697246, W64 0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0],
   go!"jPriceInFri", W64 1, W64 2, true⟩,
  ⟨array.mk 3 [W64 0x6e4965636972506a, W64 0x696557, W64 0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0],
   go!"jPriceInWei", W64 0, W64 1, true⟩]

def headerFieldNames : List GoString := [go!"Hash", go!"ParentHash", go!"Number", go!"GlobalStateRoot", go!"SequencerAddress", go!"TransactionCount", go!"EventCount", go!"Timestamp", go!"ProtocolVersion", go!"gasprice", go!"Signatures", go!"gaspricestrk", go!"L1DAMode", go!"L1DataGasPrice", go!"L2GasPrice"]

def headerFieldKeys : List cursor.StaticField := [
  ⟨array.mk 3 [W64 0x6873614864, W64 0x0, W64 0x0],
   array.mk 3 [W64 0xffffffffff, W64 0x0, W64 0x0],
   go!"dHash", W64 0, W64 0x1, true⟩,
  ⟨array.mk 3 [W64 0x7265626d754e66, W64 0x0, W64 0x0],
   array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
   go!"fNumber", W64 2, W64 0x4, true⟩,
  ⟨array.mk 3 [W64 0x646f4d4144314c68, W64 0x65, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
   go!"hL1DAMode", W64 12, W64 0x1000, true⟩,
  ⟨array.mk 3 [W64 0x6369727073616768, W64 0x65, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
   go!"hgasprice", W64 9, W64 0x200, true⟩,
  ⟨array.mk 3 [W64 0x617473656d695469, W64 0x706d, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
   go!"iTimestamp", W64 7, W64 0x80, true⟩,
  ⟨array.mk 3 [W64 0x6f43746e6576456a, W64 0x746e75, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
   go!"jEventCount", W64 6, W64 0x40, true⟩,
  ⟨array.mk 3 [W64 0x7250736147324c6a, W64 0x656369, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
   go!"jL2GasPrice", W64 14, W64 0x4000, true⟩,
  ⟨array.mk 3 [W64 0x48746e657261506a, W64 0x687361, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
   go!"jParentHash", W64 1, W64 0x2, true⟩,
  ⟨array.mk 3 [W64 0x7574616e6769536a, W64 0x736572, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
   go!"jSignatures", W64 10, W64 0x400, true⟩,
  ⟨array.mk 3 [W64 0x636972707361676c, W64 0x6b72747365, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
   go!"lgaspricestrk", W64 11, W64 0x800, true⟩,
  ⟨array.mk 3 [W64 0x4761746144314c6e, W64 0x65636972507361, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffff, W64 0x0],
   go!"nL1DataGasPrice", W64 13, W64 0x2000, true⟩,
  ⟨array.mk 3 [W64 0x536c61626f6c476f, W64 0x746f6f5265746174, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
   go!"oGlobalStateRoot", W64 3, W64 0x8, true⟩,
  ⟨array.mk 3 [W64 0x6f636f746f72506f, W64 0x6e6f69737265566c, W64 0x0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
   go!"oProtocolVersion", W64 8, W64 0x100, true⟩,
  ⟨array.mk 3 [W64 0x636e657571655370, W64 0x7365726464417265, W64 0x73],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xff],
   go!"pSequencerAddress", W64 4, W64 0x10, true⟩,
  ⟨array.mk 3 [W64 0x6361736e61725470, W64 0x6e756f436e6f6974, W64 0x74],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xff],
   go!"pTransactionCount", W64 5, W64 0x20, true⟩]

/-- Immutable ownership of a complete field table, including both backing slices.
    This resource may be reused by independent decoder calls. -/
def StaticFieldsShared (fields : Loc) (keys : List cursor.StaticField)
    (names : List GoString) (mask : w64) : IProp GF :=
  iprop(∃ f : cursor.Fields, fields ↦□ f ∗ f.order' ↦*□ keys ∗
    f.names' ↦*□ names ∗ ⌜f.all' = mask⌝)

instance staticFieldsShared_persistent (fields : Loc) (keys : List cursor.StaticField)
    (names : List GoString) (mask : w64) :
    Persistent (StaticFieldsShared (GF := GF) fields keys names mask) := by
  unfold StaticFieldsShared
  infer_instance

/-- Published decoder metadata. Establishing this requires both actual global
    stores as well as the two factories; it does not assert that initialization
    has run merely because the factories have been proved. -/
def HeaderMetadata : IProp GF :=
  iprop(∃ (fields gasFields : Loc),
    globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗
    StaticFieldsShared fields headerFieldKeys headerFieldNames (W64 32767) ∗
    StaticFieldsShared gasFields gasFieldKeys gasFieldNames (W64 3))

instance headerMetadata_persistent : Persistent (HeaderMetadata (GF := GF)) := by
  unfold HeaderMetadata
  infer_instance

end github_com.NethermindEth.juno.core
end Perennial
