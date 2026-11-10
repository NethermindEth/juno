import ProofCheck.FeltPointersReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Pointer-slice failure records both the retained cursor position and the
    diagnostic offset. Failed outer heads/counts/depth do not enter the array;
    a failed child retains outer entry and the valid prefix but publishes nothing. -/
inductive FeltPointersFailure : Nat → List w8 → Nat → Nat → Bool → FeltFailureReason → Prop where
  | outer {d : Nat} {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      FeltPointersFailure d wire 0 0 false (.expected (W8 128) reason)
  | count {d : Nat} {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (short : tail.length < uint.Z argument) :
      FeltPointersFailure d ((head :: payload) ++ tail) 0 0 false (.skipped .count)
  | depth {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (fit : uint.Z argument ≤ tail.length) :
      FeltPointersFailure 32 ((head :: payload) ++ tail) 0 0 false .depth
  | child {d count offset : Nat} {head : w8} {payload wire : List w8}
      (items : List (FeltWire (d + 1))) {reason : FeltFailureReason}
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 128)
      (depth : d < 32) (more : items.length < count)
      (fit : count ≤ (items.flatMap FeltWire.bytes ++ wire).length)
      (failure : FeltFailure (d + 1) wire offset reason) :
      FeltPointersFailure d ((head :: payload) ++ items.flatMap FeltWire.bytes ++ wire)
        (1 + payload.length + (items.flatMap FeltWire.bytes).length)
        (1 + payload.length + (items.flatMap FeltWire.bytes).length + offset) true reason

/-- The retained cursor and diagnostic both lie within the original unread
    sequence, with the diagnostic at or after the failed child's start. -/
theorem feltPointersFailure_bounds (d consumed offset : Nat) (wire : List w8)
    (entered : Bool) (reason : FeltFailureReason)
    (failure : FeltPointersFailure d wire consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | outer _ _ _ => simp
  | count _ _ _ => simp
  | depth _ _ _ => simp
  | child items enc major depth more fit failure =>
    have := feltFailure_offset _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

/-- Every specified failure of the actual pointer-slice reader preserves the
    old destination and input, and owns the exact diagnostic. All child and
    outer-reader contracts are proved; there is no helper-correctness premise. -/
theorem wp_readFeltPointers_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltPointersFailure d wire consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  have hz : v.pos' + W64 0 = v.pos' := by word
  cases failure with
  | outer hn hu failure =>
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_zero, hz, Bool.false_eq_true, ite_false] using
      wp_readFeltPointers_outer_failure (GF := GF) c dst old v xs _ dq _ kind hn hu failure hp hbytes
  | count enc major short =>
    simpa only [feltFailureAddr, FeltFailureError, skipFailureAddr, SkipFailureError,
      Int.natCast_zero, hz, Bool.false_eq_true, ite_false] using
      wp_readFeltPointers_count_failure (GF := GF) c dst old v xs _ _ _ _ dq kind enc major short hp hbytes
  | depth enc major fit =>
    simpa only [feltFailureAddr, FeltFailureError, SkipFailureError,
      Int.natCast_zero, hz, Bool.false_eq_true, ite_false] using
      wp_readFeltPointers_depth_failure (GF := GF) c dst old v xs _ _ _ _ dq kind enc major fit (by omega) hp hbytes
  | @child d count offset head payload wire items reason enc major depth more fit failure =>
    have hpos : v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length + offset : Nat) =
        v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length) + W64 offset := by word
    simp only [Int.natCast_add, Int.natCast_one] at hpos
    simpa only [ite_true, Int.natCast_add, Int.natCast_one, hpos] using
      wp_readFeltPointers_prefix_failure (GF := GF) c dst old v xs payload wire head dq d count offset items reason kind
        enc major hd depth more fit failure hp hbytes

/-- Both retained cursor and diagnostic offsets are exact nonwrapping signed
    positions; an inner EOF is allowed at the end of the original buffer. -/
theorem wp_readFeltPointers_failure_bounded (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (entered : Bool)
    (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltPointersFailure d wire consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hoff := feltPointersFailure_bounds d consumed offset wire entered reason failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hpos : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by repeat' constructor <;> word
  wp_apply wp_readFeltPointers_failure c dst old v xs wire dq d consumed offset entered reason kind
    failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointersFailure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_failure_bounded
