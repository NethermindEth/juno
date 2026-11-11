import ProofCheck.TransactionHashOwnership

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

/-- The actual selected field updates only the stored hash and restores the
    precise struct ownership, including nullish preservation of a prior hash. -/
theorem wp_transactionHashField_valid (c dst : Loc) (old value : GoArray w64 4)
    (v : cursor.Cursor) (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : FeltValueValid d old item value) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       TransactionHashOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValue)) (Val #c))
        (Val #(structFieldRef transactionHashProjection go!"TransactionHash" dst)))
    {{ RET #interface.nil; TransactionHashOwned dst value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [TransactionHashOwned] at Howned
  icases transactionHashField_acc dst (transactionHashLayout old) $$ Howned with ⟨Hfield, Hrestore⟩
  isimp only [transactionHashLayout] at Hfield
  wp_apply wp_readFeltValue_valid c (structFieldRef transactionHashProjection go!"TransactionHash" dst) old
    v xs item rest value dq d hd depth valid hp hbytes $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hrestore $$ %value Hfield
  iapply HΦ
  isimp only [TransactionHashOwned, transactionHashLayout]
  iframe Howned Hc Hs
  ipureintro; exact Hnext

/-- Rejected selected hash values preserve the whole projection and cursor;
    the owned diagnostic may point within the rejected felt. -/
theorem wp_transactionHashField_rejected (c dst : Loc) (old : GoArray w64 4)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d offset : Nat)
    (reason : FeltFailureReason) (kind : GoInterface) (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ TransactionHashOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValue)) (Val #c))
        (Val #(structFieldRef transactionHashProjection go!"TransactionHash" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       TransactionHashOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Howned, Hc, Hs⟩
  isimp only [TransactionHashOwned] at Howned
  icases transactionHashField_acc dst (transactionHashLayout old) $$ Howned with ⟨Hfield, Hrestore⟩
  isimp only [transactionHashLayout] at Hfield
  wp_apply wp_readFeltValue_rejected c (structFieldRef transactionHashProjection go!"TransactionHash" dst) old
    v xs wire dq d offset reason kind failure hd depth hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hkind Hfield Hc Hs
  ihave Howned := Hrestore $$ %old Hfield
  iapply HΦ
  isimp only [TransactionHashOwned, transactionHashLayout]
  iframe Howned Hc Hs Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_transactionHashField_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_transactionHashField_rejected
