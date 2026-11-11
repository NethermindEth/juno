import ProofCheck.ReadLimbsRejected
import ProofCheck.LimbItemsFailureSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Every independent sequence failure is realized by the production loop.
    Earlier decoded values are retained; all other destination slots are framed. -/
theorem wp_readLimbs_rejected (c : Loc) (out : GoSlice) (old done : List (GoArray w64 4))
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (depth count consumed offset : Nat)
    (reason : FeltFailureReason) (kind : GoInterface) (size : old.length = count)
    (failure : LimbItemsRejected depth count wire done consumed offset reason)
    (hd : sint.Z v.depth' = depth) (bound : depth < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ out ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       out ↦* (done ++ old.drop done.length) ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  iintro %Φ ⟨#Hpkg, #Hkind, Hout, Hc, Hs⟩ HΦ
  obtain ⟨items, rest, inner, more, split, values, used, error, rejected⟩ :=
    limbItems_rejected_witness depth count consumed offset wire done reason failure
  subst done
  subst consumed
  subst offset
  rw [split] at hbytes
  wp_apply wp_readLimbs_prefix_failure c out old v xs rest dq depth inner reason kind rejected
    items (by omega) hd bound hp hbytes $$ [Hout Hc Hs] as %err ⟨Hout, Hc, Hs, Herr⟩
  · iframe Hkind Hout Hc Hs
  have hsum : v.pos' + W64 ((items.flatMap LimbValueWire.bytes).length + inner : Nat) =
      v.pos' + W64 (items.flatMap LimbValueWire.bytes).length + W64 inner := by word
  isimp only [List.length_map, hsum] at HΦ
  wp_end

def ReadLimbsOutcome (c : Loc) (out : GoSlice) (old : List (GoArray w64 4))
    (v : cursor.Cursor) (xs wire : List w8) (depth : Nat) (kinds : FeltErrorKinds)
    (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (values : List (GoArray w64 4)),
    out ↦* values ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ LimbItemsValid depth old.length item values ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (done : List (GoArray w64 4)) (consumed offset : Nat) (reason : FeltFailureReason) (err : Loc),
    out ↦* (done ++ old.drop done.length) ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      LimbItemsRejected depth old.length wire done consumed offset reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- All inputs have a checked successful or rejected execution contract for the
    actual internal loop, with output ownership, exact cursor and diagnostics.
    The enclosing slice reader ensures the required available felt depth. -/
theorem wp_readLimbs_complete (c : Loc) (out : GoSlice) (old : List (GoArray w64 4))
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (depth : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = depth) (bound : depth < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ out ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ReadLimbsOutcome c out old v xs wire depth kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hout, Hc, Hs⟩ HΦ
  rcases limbItems_classify depth old.length wire (by omega) with
    ⟨item, rest, values, split, valid⟩ | ⟨done, consumed, offset, reason, rejected⟩
  · wp_apply wp_readLimbs_valid c out old values v xs item rest dq depth old.length rfl valid
      hd bound hp (hbytes.trans split) $$ [Hout Hc Hs] with ⟨Hout, Hc, Hs, %Hnext⟩
    · iframe Hout Hc Hs
    iapply HΦ
    iframe Hs
    unfold ReadLimbsOutcome
    ileft
    iexists item, rest, values
    iframe Hout Hc
    ipureintro; exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have bounds := limbItems_rejected_bounds depth old.length consumed offset wire done reason rejected
    have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_nil] using hbytes)
    have hpositions : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by repeat' constructor <;> word
    wp_apply wp_readLimbs_rejected c out old done v xs wire dq depth old.length consumed offset reason
      (feltFailureKind reason kinds) rfl rejected hd bound hp hbytes
      $$ [Hout Hc Hs] as %err ⟨Hout, Hc, Hs, Herr⟩
    · iframe Hkind Hout Hc Hs
    iapply HΦ
    iframe Hs
    unfold ReadLimbsOutcome
    iright
    iexists done, consumed, offset, reason, err
    iframe Hout Hc Herr
    ipureintro; exact ⟨rfl, rejected, hpositions⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_complete
