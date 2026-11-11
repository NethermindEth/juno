import ProofCheck.GasUsedBoundary

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Every independently accepted direct GasConsumed record is realized by the actual
    generated reader, with exact owned contents, suffix and restored depth. -/
theorem wp_readGasUsed_record_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : GasUsedValue) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac) (d : Nat)
    (valid : GasUsedRecordValid d old item rem final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       GasUsedOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasConsumed)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; GasUsedOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @nullish head suffix hn =>
    wp_apply wp_readGasUsed_nullish c dst old v xs rem head dq hn hp hbytes
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; simpa only [Int.add_assoc] using Hnext
  | @map head payload body suffix argument final count enc major size depth pairs =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readGasUsed_map_valid validator c dst old final v xs payload body rem head dq d count
      enc major hd depth pairs hp hbytes $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    have length : ((head :: payload) ++ body).length = 1 + payload.length + body.length := by simp; omega
    isimp only [length, Int.natCast_add, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; simpa only [Int.add_assoc] using Hnext

/-- Every independent GasConsumed rejection gives the specified partial record,
    retained cursor/depth and separately bounded owned diagnostic. -/
theorem wp_readGasUsed_record_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : GasUsedValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (entered : Bool) (reason : GasUsedFailureReason)
    (failure : GasUsedRecordRejected d old wire final consumed offset entered reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       GasUsedOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasConsumed)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       GasUsedOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ GasUsedFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases failure with
  | opening failure =>
    wp_apply wp_readGasUsed_open_rejected c dst old v xs wire dq d _ failure hd hp hbytes
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
    · iframe Howned Hc Hs
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [Int.natCast_zero, Int.add_zero, Bool.false_eq_true, ite_false, hz] at HΦ
    iapply HΦ
    iframe Howned Hc Hs Herr
    ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @map head payload body argument final consumed offset reason count enc major size depth fit pairs =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readGasUsed_map_rejected validator c dst old final v xs payload body head dq d count consumed offset reason
      enc major hd depth fit pairs hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    isimp only [Int.natCast_add, Int.natCast_one, ite_true] at HΦ
    iapply HΦ
    iframe Howned Hc Hs Herr
    ipureintro; simpa only [Int.add_assoc] using Hnext

/-- Concrete outcomes retain the independent relation on every byte input.
    This is the direct record reader, before enclosing pointer-slice allocation
    or public receipt/projection guarding and cleanup. -/
def GasUsedRecordOutcome (c dst : Loc) (old : GasUsedValue) (v : cursor.Cursor)
    (xs wire : List w8) (d : Nat) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rem : List w8) (final : GasUsedValue),
    GasUsedOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rem ∧ GasUsedRecordValid d old item rem final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝) ∨
  (∃ (final : GasUsedValue) (consumed offset : Nat) (entered : Bool) (reason : GasUsedFailureReason) (err : Loc),
    GasUsedOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
    GasUsedFailureOwned err (v.pos' + W64 offset) reason ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      GasUsedRecordRejected d old wire final consumed offset entered reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- Every actual direct GasConsumed call at supported depth has a specified outcome,
    for arbitrary owned initial contents and all bytes. Initialization is proved;
    the shared UTF-8 validator remains the sole explicit helper premise. -/
theorem wp_readGasUsed_record_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GasUsedValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       GasUsedOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasConsumed)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ GasUsedRecordOutcome c dst old v xs wire d result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  rcases gasUsedRecord_classify d old wire depth with
    ⟨item, rem, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · wp_apply wp_readGasUsed_record_valid validator c dst old final v xs item rem dq d valid hd hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    unfold GasUsedRecordOutcome
    ileft
    iexists item, rem, final
    iframe Howned Hc
    ipureintro; exact ⟨rfl, split, valid, Hnext⟩
  · wp_apply wp_readGasUsed_record_rejected validator c dst old final v xs wire dq d consumed offset entered reason failure hd hp hbytes
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    unfold GasUsedRecordOutcome
    iright
    iexists final, consumed, offset, entered, reason, err
    iframe Howned Hc Herr
    ipureintro; exact ⟨rfl, failure, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasUsed_record_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasUsed_record_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasUsed_record_complete
