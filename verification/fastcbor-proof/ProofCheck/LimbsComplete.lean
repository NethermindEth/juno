import ProofCheck.LimbsFailure

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

/-- Limbs shares typed felt failures but always expects an array. The additional
    outer case includes null/undefined, which only the pointer wrapper accepts. -/
inductive LimbsFailure : Nat → List w8 → Nat → FeltFailureReason → Prop where
  | typed {d offset : Nat} {wire : List w8} {reason : FeltFailureReason}
      (failure : FeltFailure d wire offset reason) : LimbsFailure d wire offset reason
  | outer {d : Nat} {wire : List w8} {reason : ExpectedFailureReason}
      (failure : ExpectedFailure (W8 128) wire reason) :
      LimbsFailure d wire 0 (.expected (W8 128) reason)

def LimbsClassified (d : Nat) (wire : List w8) : Prop :=
  (∃ item rest words, wire = item ++ rest ∧ FeltValid d item (some words)) ∨
  (∃ offset reason, LimbsFailure d wire offset reason)

/-- Exhaustive in-place classification reuses the felt array specification and
    turns the pointer wrapper's nullish successes into array type failures. -/
theorem limbs_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) : LimbsClassified d wire := by
  rcases felt_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
  · cases value with
    | some words => exact Or.inl ⟨item, rest, words, split, valid⟩
    | none =>
      cases valid with
      | @nullish head hn =>
        rw [split]
        rcases hn with rfl | rfl
        all_goals exact Or.inr ⟨0, _, .outer (.typeMismatch rest (.inline _ (by decide)) (by decide) (by decide))⟩
  · exact Or.inr ⟨offset, reason, .typed failure⟩

theorem limbsFailure_offset (d offset : Nat) (wire : List w8) (reason : FeltFailureReason)
    (failure : LimbsFailure d wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | typed failure => exact feltFailure_offset d offset wire reason failure
  | outer failure => omega

/-- The actual public in-place reader realizes every nonnull felt value, storing
    its four words into the caller's owned array and preserving the exact suffix. -/
theorem wp_limbs_valid (c dst : Loc) (old words : GoArray w64 4) (v : cursor.Cursor)
    (xs wire rem : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltValid d wire (some words))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ words ∗ c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  cases valid with
  | @small head payload argument items enc hm count small bound =>
    have heq : argument = W64 items.length := by word
    rw [heq] at enc
    isimp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    wp_apply wp_limbs_small c dst old v xs payload rem head items dq enc hm small hp (by omega) hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hdst Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] using Hnext
  | @extended head payload argument items extras enc hm count four bound children =>
    have heq : argument = W64 (4 + extras.length) := by word
    rw [heq] at enc
    isimp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    wp_apply wp_limbs_extended c dst old v xs payload rem head items extras dq d hd bound enc hm four children hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hdst Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] using Hnext

/-- Every in-place rejection preserves the original four words, cursor and
    input while retaining the specified owned diagnostic. -/
theorem wp_limbs_failure (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : LimbsFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  cases failure with
  | typed failure => exact wp_limbs_felt_failure c dst old v xs wire dq d offset reason kind failure hd hdepth hp hbytes
  | outer failure =>
    have hz : v.pos' + W64 0 = v.pos' := by word
    simpa only [feltFailureAddr, FeltFailureError, Int.natCast_zero, hz] using
      wp_limbs_outer_failure (GF := GF) c dst old v xs wire dq _ kind failure hp hbytes

/-- The in-place outcome owns the caller's updated or unchanged four-word array;
    surrounding backing-array elements can stay framed by the caller. -/
def LimbsOutcome (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (words : GoArray w64 4),
    dst ↦ words ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ FeltValid d item (some words) ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : FeltFailureReason) (err : Loc), dst ↦ old ∗ c ↦ v ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      LimbsFailure d wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- Public Limbs has a specified outcome for arbitrary input, with no chosen
    outcome or helper-correctness precondition. It never accepts a null pointer
    encoding; nullable slice/field wrappers perform that check themselves. -/
theorem wp_limbs_complete (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ LimbsOutcome c dst old v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hdst, Hc, Hs⟩
  rcases limbs_classify d wire hdepth with ⟨item, rest, words, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_limbs_valid c dst old words v xs item rest dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold LimbsOutcome
    ileft
    iexists item, rest, words
    iframe Hdst Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hoff := limbsFailure_offset d offset wire reason failure
    have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_nil] using hbytes)
    have hpos : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by constructor <;> word
    wp_apply wp_limbs_failure c dst old v xs wire dq d offset reason (feltFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold LimbsOutcome
    iright
    iexists offset, reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, failure, hpos⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.limbs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.limbsFailure_offset
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_complete
