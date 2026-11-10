import ProofCheck.Utf8Reject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]

/-- The unresolved production-validator obligation, stated as a proposition.
    It is not an axiom or an instance: callers must explicitly supply a proof.
    The checked implementation loop reduces this contract to the explicit
    Utf8WordWindowContract. That actual helper remains unproved because of
    the documented generic-index semantics obstruction. -/
def Utf8ValidatorContract : Prop :=
  ∀ (data : GoSlice) (xs : List w8) (dq : DFrac),
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} xs }}
      (App (Val (@! unicode.utf8.Valid)) (Val #data))
    {{ (ok : Bool), RET #ok; data ↦*{dq} xs ∗ ⌜ok = true ↔ Utf8Valid xs⌝ }}

/-- Borrowing a payload range and reassembling its input requires only the
    explicit validator contract, without weakening ownership or UTF-8 validity. -/
theorem wp_utf8_contract_range (validator : Utf8ValidatorContract (GF := GF))
    (data : GoSlice) (xs body : List w8) (low high : w64) (dq : DFrac)
    (bounds : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z data.len)
    (bytes : subslice (sint.nat low) (sint.nat high) xs = body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} xs }}
      (App (Val (@! unicode.utf8.Valid)) (Val #(slice.slice data w8 low high)))
    {{ (ok : Bool), RET #ok; data ↦*{dq} xs ∗ ⌜ok = true ↔ Utf8Valid body⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice low high data dq xs bounds).1 $$ Hs with ⟨Hpre, Hbody, Hpost⟩
  isimp only [bytes] at Hbody
  wp_apply validator (slice.slice data w8 low high) body dq $$ [Hbody] as %ok ⟨Hbody, %Hvalid⟩
  · iframe Hbody
  iapply HΦ
  isplitr []
  · iapply (ownSlice_slice low high data dq xs bounds).2
    rw [bytes]
    iframe Hpre Hbody Hpost
  · ipureintro; exact Hvalid

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8_contract_range
