import ProofCheck.LimbsComplete
import ProofCheck.FeltDeterminism
import ProofCheck.FeltDisjoint
import ProofCheck.InitializationErrors

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

/-- TransactionVersion is stored as the same four Montgomery limbs as a felt.
    This proves the actual typed allocation/cast and Limbs call, including nil
    versus allocated zero, without imposing protocol-version validity. -/
theorem wp_readTransactionVersion_valid (c dst old : Loc) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32) (valid : FeltValid d item value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadTransactionVersion)) (Val #c)) (Val #dst))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ FeltResult out value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  cases value with
  | none =>
    obtain ⟨head, rfl, hn⟩ := feltValid_none d item valid
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ $$ %null
    isimp only [FeltResult, List.length_singleton, Int.natCast_one]
    iframe Hdst Hc Hs
    ipureintro; exact Hnext
  | some words =>
    obtain ⟨head, tail, split, major⟩ := feltValid_some_head d item words valid
    have hn : (item ++ rest).head? ≠ some (W8 246) ∧ (item ++ rest).head? ≠ some (W8 247) := by
      rw [split]
      simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
      constructor <;> intro eq <;> subst head <;> contradiction
    wp_apply wp_null_not_nullish c v xs (item ++ rest) dq hn.1 hn.2 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    ihave %Hnn := typedPointsto_not_null «$r0_ptr» (zero_val TransactionVersion) (DFrac.own 1) $$ «$r0»
    wp_apply wp_limbs_valid c «$r0_ptr» (zero_val TransactionVersion) words v xs item rest dq d hd depth valid hp hbytes
      $$ [«$r0» Hc Hs] with ⟨Hvalue, Hc, Hs, %Hstep⟩
    · iframe «$r0» Hc Hs
    iapply HΦ $$ %«$r0_ptr»
    isimp only [FeltResult]
    iframe Hdst Hvalue Hc Hs
    ipureintro; exact ⟨Hnn, Hstep⟩

/-- A failed private version allocation is never published. The old pointer,
    input and full cursor are preserved and the exact Limbs error is returned. -/
theorem wp_readTransactionVersion_rejected (c dst old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadTransactionVersion)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound := feltFailure_offset d offset wire reason failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hpos : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by constructor <;> word
  wp_auto
  have hn := feltFailure_not_nullish d offset wire reason failure
  wp_apply wp_null_not_nullish c v xs wire dq hn.1 hn.2 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_felt_failure c «$r0_ptr» (zero_val TransactionVersion) v xs wire dq d offset reason kind failure hd depth hp hbytes
    $$ [«$r0» Hc Hs] as %err ⟨_, Hc, Hs, Herr⟩
  · iframe Hkind «$r0» Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs Herr
  ipureintro; exact hpos

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readTransactionVersion_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readTransactionVersion_rejected
