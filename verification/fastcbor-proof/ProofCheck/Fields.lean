import ProofCheck.Values

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Static metadata installation stores precisely the generated slices and mask. -/
theorem wp_staticFields (names order : GoSlice) (hlen : 0 ≤ sint.Z names.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (App (Val (@! cursor.NewStaticFields)) (Val #names)) (Val #order))
    {{ (fields : Loc), RET #fields;
       fields ↦ cursor.Fields.mk order names ((W64 1 <<< names.len) - W64 1) }} := by
  wp_start
  wp_auto
  wp_apply wp_shiftCount_int
  · ipureintro; exact hlen
  wp_end

def gasFieldNames : List GoString := [go!"PriceInWei", go!"PriceInFri"]

def gasFieldKeys : List cursor.StaticField := [
  ⟨array.mk 3 [W64 0x6e4965636972506a, W64 0x697246, W64 0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0],
   go!"jPriceInFri", W64 1, W64 2, true⟩,
  ⟨array.mk 3 [W64 0x6e4965636972506a, W64 0x696557, W64 0],
   array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0],
   go!"jPriceInWei", W64 0, W64 1, true⟩]

attribute [-instance] go.SliceSemantics.composite_literal_slice

/-- The generated factory allocates the actual complete GasPrice field table. -/
theorem wp_makeGasPriceFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core }}
      (App (Val (@! cborMakeGasPriceFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (W64 3) ∗
       names ↦* gasFieldNames ∗ order ↦* gasFieldKeys }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) gasFieldNames
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) gasFieldKeys
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (2 : Int) 0) 0 = 2 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 2) (W64 2)) (slice.mk order (W64 2) (W64 2)) (by change 0 ≤ sint.Z (W64 2); decide) as %fields Hfields
  isimp only [show (W64 1 <<< W64 2) - W64 1 = W64 3 by decide] at Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder Hfields

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

/-- The generated factory allocates the actual complete Header field table. -/
theorem wp_makeHeaderFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core }}
      (App (Val (@! cborMakeHeaderFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       names ↦* headerFieldNames ∗ order ↦* headerFieldKeys }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) headerFieldNames
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) headerFieldKeys
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (15 : Int) 0) 0 = 15 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 15) (W64 15)) (slice.mk order (W64 15) (W64 15)) (by change 0 ≤ sint.Z (W64 15); decide) as %fields Hfields
  isimp only [show (W64 1 <<< W64 15) - W64 1 = W64 32767 by decide] at Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder Hfields

/-- Independently compute the little-endian word represented by up to eight bytes. -/
def keyWord (bytes : List w8) : w64 :=
  bytes.zipIdx.foldl (fun out pair => out ||| (W64 (uint.Z pair.1) <<< W64 (8 * pair.2))) (W64 0)

/-- The encoding and cached comparisons agree with the field's actual wire name. -/
def fieldEntryValid (names : List GoString) (key : cursor.StaticField) : Bool :=
  match names[key.Index'.toNat]? with
  | none => false
  | some name =>
    let encoded := (if name.length < 24 then [W8 (96 + name.length)]
      else [W8 120, W8 name.length]) ++ name
    let chunks := [encoded.take 8, (encoded.drop 8).take 8, (encoded.drop 16).take 8]
    decide (name.length < 256) && name.all (fun b => decide (b.toNat < 128)) &&
    (key.Encoded' == encoded) && (key.Words'.arr == chunks.map keyWord) &&
    (key.Masks'.arr == chunks.map (fun chunk => keyWord (List.replicate chunk.length (W8 255)))) &&
    (key.Bit' == (W64 1 <<< key.Index')) && (key.Short' == decide (encoded.length ≤ 24))

theorem gasFieldKeys_valid : gasFieldKeys.all (fieldEntryValid gasFieldNames) = true := by
  rfl

theorem headerFieldKeys_valid : headerFieldKeys.all (fieldEntryValid headerFieldNames) = true := by
  rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_staticFields

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeGasPriceFields

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderFields
#print axioms Perennial.github_com.NethermindEth.juno.core.gasFieldKeys_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFieldKeys_valid
