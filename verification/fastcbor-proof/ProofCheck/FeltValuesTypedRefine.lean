import ProofCheck.FeltValuesTyped
import ProofCheck.FeltValuesUnique

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

/-- Select an independently specified successful outcome of the actual typed
    slice reader. This adapter adds no helper assumption. -/
theorem wp_readFeltValues_valid (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (List (GoArray w64 4))) (dq : DFrac) (d : Nat)
    (valid : FeltValuesSpec d (item ++ rest) (.success value rest))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ FeltValuesResult out value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  wp_apply +noauto wp_readFeltValues_complete c dst old v xs (item ++ rest) dq d hd depth hp hbytes
    $$ [Hdst Hc Hs] as %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hdst Hc Hs
  have eq := feltValuesSpec_unique d (item ++ rest) outcome (.success value rest) Hspec valid
  subst outcome
  isimp only [FeltValuesReadOwned, FeltValuesSuccess] at Houtcome
  icases Houtcome with ⟨%Hnil, %out, Hdst, Hvalue, %actual, Hc, %Hnext⟩
  have eq := List.append_cancel_right Hnext.1
  subst actual
  subst result
  iapply HΦ $$ %out
  iframe Hdst Hvalue Hc Hs
  ipureintro; exact Hnext.2.2

/-- Rejected slice reads preserve the entire cursor and old destination; the
    typed helper returns the owned error at the independently specified offset. -/
theorem wp_readFeltValues_rejected (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltValuesFailureReason)
    (failure : FeltValuesSpec d wire (.failure offset reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       (∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltValuesFailureError err (v.pos' + W64 offset) reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  wp_apply +noauto wp_readFeltValues_complete c dst old v xs wire dq d hd depth hp hbytes
    $$ [Hdst Hc Hs] as %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hdst Hc Hs
  have eq := feltValuesSpec_unique d wire outcome (.failure offset reason) Hspec failure
  subst outcome
  isimp only [FeltValuesReadOwned, FeltValuesDecoded] at Houtcome
  icases Houtcome with ⟨Hdst, Hc, %err, Herr, %Hnext⟩
  rcases Hnext with ⟨_, rfl, Hnext⟩
  iapply HΦ
  iframe Hdst Hc Hs
  isplitl [Herr Hglobals]
  · iexists kinds
    iframe Hglobals Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltValues_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltValues_rejected
