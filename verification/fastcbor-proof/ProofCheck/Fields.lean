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

attribute [-instance] go.SliceSemantics.composite_literal_slice

/-- The generated factory allocates the actual complete GasPrice field table. -/
theorem wp_makeGasPriceFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborschema.cborMakeGasPriceFields)) (Val #()))
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

/-- The generated factory allocates the actual complete Header field table. -/
theorem wp_makeHeaderFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborschema.cborMakeHeaderFields)) (Val #()))
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
