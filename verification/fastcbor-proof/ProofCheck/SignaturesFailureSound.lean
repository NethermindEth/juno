import ProofCheck.SignaturesReject

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

/-- Signature failure records both the retained cursor position and the
    diagnostic offset. Failed outer heads/counts/depth do not enter the array;
    a failed child retains the valid outer prefix and its own specified progress,
    including one or two entered arrays, while publishing no outer destination. -/
inductive SignaturesFailure : Nat → List w8 → Nat → Nat → Nat → FeltFailureReason → Prop where
  | outer {d : Nat} {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      SignaturesFailure d wire 0 0 0 (.expected (W8 128) reason)
  | count {d : Nat} {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (short : tail.length < uint.Z argument) :
      SignaturesFailure d ((head :: payload) ++ tail) 0 0 0 (.skipped .count)
  | depth {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (fit : uint.Z argument ≤ tail.length) :
      SignaturesFailure 32 ((head :: payload) ++ tail) 0 0 0 .depth
  | child {d count consumed offset : Nat} {entered : Bool} {head : w8} {payload wire : List w8}
      (items : List (SignatureWire (d + 1))) {reason : FeltFailureReason}
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 128)
      (depth : d < 32) (more : items.length < count)
      (fit : count ≤ (items.flatMap SignatureWire.bytes ++ wire).length)
      (failure : FeltPointersFailure (d + 1) wire consumed offset entered reason) :
      SignaturesFailure d ((head :: payload) ++ items.flatMap SignatureWire.bytes ++ wire)
        (1 + payload.length + (items.flatMap SignatureWire.bytes).length + consumed)
        (1 + payload.length + (items.flatMap SignatureWire.bytes).length + offset) (if entered then 2 else 1) reason

/-- The retained cursor and diagnostic both lie within the original unread
    sequence, with the diagnostic at or after the failed child's start. -/
theorem signaturesFailure_bounds (d consumed offset : Nat) (wire : List w8)
    (extraDepth : Nat) (reason : FeltFailureReason)
    (failure : SignaturesFailure d wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | outer _ _ _ => simp
  | count _ _ _ => simp
  | depth _ _ _ => simp
  | child items enc major depth more fit failure =>
    have := feltPointersFailure_bounds _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

/-- The inner reader's retained outer entry never exceeds the configured
    nesting bound, including failures reached inside a felt's ignored extras. -/
theorem feltPointersFailure_depth (d consumed offset : Nat) (wire : List w8)
    (entered : Bool) (reason : FeltFailureReason) (hdepth : d ≤ 32)
    (failure : FeltPointersFailure d wire consumed offset entered reason) :
    d + (if entered then 1 else 0 : Nat) ≤ 32 := by
  cases failure with
  | outer _ _ _ => simpa using hdepth
  | count _ _ _ => simpa using hdepth
  | depth _ _ _ => simp
  | child items enc major depth more fit failure => simp; omega

/-- Retaining both enclosing signature arrays still respects the nesting bound.
    The independent failure relation therefore determines an in-range depth. -/
theorem signaturesFailure_depth (d consumed offset : Nat) (wire : List w8)
    (extraDepth : Nat) (reason : FeltFailureReason) (hdepth : d ≤ 32)
    (failure : SignaturesFailure d wire consumed offset extraDepth reason) :
    d + extraDepth ≤ 32 := by
  cases failure with
  | outer _ _ _ => simpa using hdepth
  | count _ _ _ => simpa using hdepth
  | depth _ _ _ => simp
  | @child d count consumed offset entered head payload wire items reason enc major depth more fit failure =>
    have hi := feltPointersFailure_depth (d + 1) consumed offset wire entered reason (by omega) failure
    cases entered <;> simp only [Bool.false_eq_true, ite_true, ite_false] at * <;> omega

/-- Every specified failure of the actual signature reader preserves the
    old destination and input, and owns the exact diagnostic. All child and
    outer-reader contracts are proved; there is no helper-correctness premise. -/
theorem wp_readSignatures_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (extraDepth : Nat)
    (reason : FeltFailureReason) (kind : GoInterface)
    (failure : SignaturesFailure d wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  have hz : v.pos' + W64 0 = v.pos' := by word
  have hdz : v.depth' + W64 0 = v.depth' := by word
  cases failure with
  | outer hn hu failure =>
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_zero, hz, hdz] using
      wp_readSignatures_outer_failure (GF := GF) c dst old v xs _ dq _ kind hn hu failure hp hbytes
  | count enc major short =>
    simpa only [feltFailureAddr, FeltFailureError, skipFailureAddr, SkipFailureError,
      Int.natCast_zero, hz, hdz] using
      wp_readSignatures_count_failure (GF := GF) c dst old v xs _ _ _ _ dq kind enc major short hp hbytes
  | depth enc major fit =>
    simpa only [feltFailureAddr, FeltFailureError, SkipFailureError,
      Int.natCast_zero, hz, hdz] using
      wp_readSignatures_depth_failure (GF := GF) c dst old v xs _ _ _ _ dq kind enc major fit (by omega) hp hbytes
  | @child d count consumed offset entered head payload wire items reason enc major depth more fit failure =>
    have hcpos : v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length + consumed : Nat) =
        v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length) + W64 consumed := by word
    have hepos : v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length + offset : Nat) =
        v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length) + W64 offset := by word
    have hdstep : v.depth' + W64 (if entered then 2 else 1 : Nat) =
        (if entered then v.depth' + W64 1 + W64 1 else v.depth' + W64 1) := by
      cases entered <;> simp
      word
    simp only [Int.natCast_add, Int.natCast_one] at hcpos hepos
    simpa only [Int.natCast_add, Int.natCast_one, hcpos, hepos, hdstep] using
      wp_readSignatures_prefix_failure (GF := GF) c dst old v xs payload wire head dq d count consumed offset entered items reason kind
        enc major hd depth more fit failure hp hbytes

/-- Both retained cursor and diagnostic offsets are exact nonwrapping signed
    positions; an inner EOF is allowed at the end of the original buffer. -/
theorem wp_readSignatures_failure_bounded (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (extraDepth : Nat)
    (reason : FeltFailureReason) (kind : GoInterface)
    (failure : SignaturesFailure d wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hoff := signaturesFailure_bounds d consumed offset wire extraDepth reason failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hpos : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by repeat' constructor <;> word
  wp_apply wp_readSignatures_failure c dst old v xs wire dq d consumed offset extraDepth reason kind
    failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesFailure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_failure_bounded
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointersFailure_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesFailure_depth
